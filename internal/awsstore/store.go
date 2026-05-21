package awsstore

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/priivacy-ai/agent-log-analyzer/internal/analytics"
	"github.com/priivacy-ai/agent-log-analyzer/internal/analyzer"
	"github.com/priivacy-ai/agent-log-analyzer/internal/app"
)

type Store struct {
	s3           *s3.Client
	sqs          *sqs.Client
	dynamodb     *dynamodb.Client
	uploadBucket string
	reportBucket string
	jobTable     string
	queueURL     string
}

func NewFromEnv() (*Store, error) {
	region := getenv("AWS_REGION", "us-east-1")
	endpoint := os.Getenv("AWS_ENDPOINT_URL")
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return nil, err
	}
	store := &Store{
		uploadBucket: requiredEnv("CLAUDE_ANALYZER_UPLOAD_BUCKET"),
		reportBucket: requiredEnv("CLAUDE_ANALYZER_REPORT_BUCKET"),
		jobTable:     requiredEnv("CLAUDE_ANALYZER_JOB_TABLE"),
		queueURL:     requiredEnv("CLAUDE_ANALYZER_JOB_QUEUE_URL"),
	}
	if store.uploadBucket == "" || store.reportBucket == "" || store.jobTable == "" || store.queueURL == "" {
		return nil, errors.New("missing required AWS backend configuration")
	}
	store.s3 = s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		}
	})
	store.sqs = sqs.NewFromConfig(cfg, func(o *sqs.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})
	store.dynamodb = dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})
	return store, nil
}

func (s *Store) CreateUploadSession(job app.Job) error {
	now := time.Now().UTC()
	job.Status = app.StatusUploading
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now
	return s.putJob(job)
}

func (s *Store) AppendAnalyticsEvent(event analytics.Event) error {
	data, err := analytics.MarshalJSONLine(event)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	key, err := analyticsObjectKey(now)
	if err != nil {
		return err
	}
	_, err = s.s3.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:               aws.String(s.reportBucket),
		Key:                  aws.String(key),
		Body:                 bytes.NewReader(data),
		ServerSideEncryption: "AES256",
		ContentType:          aws.String("application/x-ndjson"),
	})
	return err
}

func analyticsObjectKey(now time.Time) (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"analytics/events/date=%s/hour=%02d/%x.jsonl",
		now.Format("2006-01-02"),
		now.Hour(),
		random[:],
	), nil
}

func (s *Store) StoreUploadSession(job app.Job, data []byte) (app.Job, error) {
	uploadPath, err := s.SaveUpload(job.ID, data)
	if err != nil {
		return job, err
	}
	job.UploadPath = uploadPath
	job.UpdatedAt = time.Now().UTC()
	return job, s.putJob(job)
}

func (s *Store) FinalizeUploadSession(job app.Job) error {
	switch job.Status {
	case app.StatusUploading:
	case app.StatusPending, app.StatusProcessing, app.StatusCompleted:
		return nil
	default:
		return errors.New("job is not waiting for upload")
	}
	if job.UploadPath == "" {
		return errors.New("upload missing")
	}
	return s.enqueueJob(job)
}

func (s *Store) SaveUpload(jobID string, data []byte) (string, error) {
	key := "uploads/" + jobID + ".log"
	_, err := s.s3.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:               aws.String(s.uploadBucket),
		Key:                  aws.String(key),
		Body:                 bytes.NewReader(data),
		ServerSideEncryption: "AES256",
		ContentType:          aws.String("application/octet-stream"),
	})
	if err != nil {
		return "", err
	}
	return "s3://" + s.uploadBucket + "/" + key, nil
}

func (s *Store) ReadUpload(path string) ([]byte, error) {
	bucket, key, err := parseS3Path(path)
	if err != nil {
		return nil, err
	}
	if bucket != s.uploadBucket {
		return nil, errors.New("upload bucket mismatch")
	}
	output, err := s.s3.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer output.Body.Close()
	return analyzer.ReadAllLimited(output.Body, 100*1024*1024)
}

func (s *Store) CreateJob(job app.Job) error {
	now := time.Now().UTC()
	job.Status = app.StatusPending
	job.CreatedAt = now
	job.UpdatedAt = now
	if err := s.putJob(job); err != nil {
		return err
	}
	return s.sendJobMessage(job.ID)
}

func (s *Store) enqueueJob(job app.Job) error {
	job.Status = app.StatusPending
	job.UpdatedAt = time.Now().UTC()
	if err := s.putJob(job); err != nil {
		return err
	}
	return s.sendJobMessage(job.ID)
}

func (s *Store) sendJobMessage(jobID string) error {
	body, err := json.Marshal(map[string]string{"job_id": jobID})
	if err != nil {
		return err
	}
	_, err = s.sqs.SendMessage(context.Background(), &sqs.SendMessageInput{
		QueueUrl:    aws.String(s.queueURL),
		MessageBody: aws.String(string(body)),
	})
	return err
}

func (s *Store) ClaimNextJob() (app.Job, bool, error) {
	output, err := s.sqs.ReceiveMessage(context.Background(), &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(s.queueURL),
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     5,
		VisibilityTimeout:   120,
	})
	if err != nil {
		return app.Job{}, false, err
	}
	if len(output.Messages) == 0 {
		return app.Job{}, false, nil
	}
	message := output.Messages[0]
	jobID, err := jobIDFromMessage(message)
	if err != nil {
		return app.Job{}, false, err
	}
	job, err := s.GetJob(jobID)
	if err != nil {
		return app.Job{}, false, err
	}
	job.Status = app.StatusProcessing
	job.UpdatedAt = time.Now().UTC()
	job.QueueReceipt = aws.ToString(message.ReceiptHandle)
	if err := s.putJob(job); err != nil {
		return app.Job{}, false, err
	}
	return job, true, nil
}

func (s *Store) CompleteJob(job app.Job, report analyzer.Report) error {
	reportKey := "reports/" + job.ID + ".json"
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	_, err = s.s3.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:               aws.String(s.reportBucket),
		Key:                  aws.String(reportKey),
		Body:                 bytes.NewReader(data),
		ServerSideEncryption: "AES256",
		ContentType:          aws.String("application/json"),
	})
	if err != nil {
		return err
	}
	job.Status = app.StatusCompleted
	job.ReportPath = "s3://" + s.reportBucket + "/" + reportKey
	job.UpdatedAt = time.Now().UTC()
	job.CompletedAt = job.UpdatedAt
	if err := s.putJob(job); err != nil {
		return err
	}
	return s.deleteQueueMessage(job)
}

func (s *Store) CreateCompletedReport(job app.Job, report analyzer.Report) error {
	reportKey := "reports/" + job.ID + ".json"
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	_, err = s.s3.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:               aws.String(s.reportBucket),
		Key:                  aws.String(reportKey),
		Body:                 bytes.NewReader(data),
		ServerSideEncryption: "AES256",
		ContentType:          aws.String("application/json"),
	})
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.Status = app.StatusCompleted
	job.ReportPath = "s3://" + s.reportBucket + "/" + reportKey
	job.UpdatedAt = now
	job.CompletedAt = now
	return s.putJob(job)
}

func (s *Store) CreateEmailUnlock(unlock app.EmailUnlock) error {
	now := time.Now().UTC()
	if unlock.CreatedAt.IsZero() {
		unlock.CreatedAt = now
	}
	if unlock.UpdatedAt.IsZero() {
		unlock.UpdatedAt = now
	}
	if unlock.Status == "" {
		unlock.Status = app.EmailUnlockPending
	}
	return s.putEmailUnlock(unlock)
}

func (s *Store) GetEmailUnlock(id string) (app.EmailUnlock, error) {
	output, err := s.dynamodb.GetItem(context.Background(), &dynamodb.GetItemInput{
		TableName: aws.String(s.jobTable),
		Key: map[string]dynamodbtypes.AttributeValue{
			"id": &dynamodbtypes.AttributeValueMemberS{Value: id},
		},
	})
	if err != nil {
		return app.EmailUnlock{}, err
	}
	if len(output.Item) == 0 {
		return app.EmailUnlock{}, os.ErrNotExist
	}
	return emailUnlockFromItem(output.Item)
}

func (s *Store) GetEmailUnlockByFullScanTokenHash(tokenHash string) (app.EmailUnlock, error) {
	if tokenHash == "" {
		return app.EmailUnlock{}, os.ErrNotExist
	}
	output, err := s.dynamodb.Scan(context.Background(), &dynamodb.ScanInput{
		TableName:        aws.String(s.jobTable),
		FilterExpression: aws.String("full_scan_token_hash = :token"),
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":token": &dynamodbtypes.AttributeValueMemberS{Value: tokenHash},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return app.EmailUnlock{}, err
	}
	if len(output.Items) == 0 {
		return app.EmailUnlock{}, os.ErrNotExist
	}
	return emailUnlockFromItem(output.Items[0])
}

func (s *Store) UpdateEmailUnlock(unlock app.EmailUnlock) error {
	unlock.UpdatedAt = time.Now().UTC()
	return s.putEmailUnlock(unlock)
}

func (s *Store) FailJob(job app.Job, jobErr error) error {
	job.Status = app.StatusFailed
	job.Error = jobErr.Error()
	job.UpdatedAt = time.Now().UTC()
	if err := s.putJob(job); err != nil {
		return err
	}
	return s.deleteQueueMessage(job)
}

func (s *Store) GetJob(id string) (app.Job, error) {
	output, err := s.dynamodb.GetItem(context.Background(), &dynamodb.GetItemInput{
		TableName: aws.String(s.jobTable),
		Key: map[string]dynamodbtypes.AttributeValue{
			"id": &dynamodbtypes.AttributeValueMemberS{Value: id},
		},
	})
	if err != nil {
		return app.Job{}, err
	}
	if len(output.Item) == 0 {
		return app.Job{}, os.ErrNotExist
	}
	return jobFromItem(output.Item)
}

func (s *Store) QueueDepth() (int, error) {
	output, err := s.sqs.GetQueueAttributes(context.Background(), &sqs.GetQueueAttributesInput{
		QueueUrl: aws.String(s.queueURL),
		AttributeNames: []sqstypes.QueueAttributeName{
			sqstypes.QueueAttributeNameApproximateNumberOfMessages,
			sqstypes.QueueAttributeNameApproximateNumberOfMessagesNotVisible,
		},
	})
	if err != nil {
		return 0, err
	}
	total := 0
	for _, key := range []sqstypes.QueueAttributeName{
		sqstypes.QueueAttributeNameApproximateNumberOfMessages,
		sqstypes.QueueAttributeNameApproximateNumberOfMessagesNotVisible,
	} {
		raw := output.Attributes[string(key)]
		var value int
		if _, err := fmt.Sscanf(raw, "%d", &value); err != nil {
			return 0, err
		}
		total += value
	}
	return total, nil
}

func (s *Store) GetReport(id string) (analyzer.Report, error) {
	output, err := s.s3.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(s.reportBucket),
		Key:    aws.String("reports/" + id + ".json"),
	})
	if err != nil {
		return analyzer.Report{}, err
	}
	defer output.Body.Close()
	data, err := analyzer.ReadAllLimited(output.Body, 10*1024*1024)
	if err != nil {
		return analyzer.Report{}, err
	}
	var report analyzer.Report
	return report, json.Unmarshal(data, &report)
}

func (s *Store) SweepExpired(now time.Time, rawUploadTTL, reportTTL time.Duration) (app.SweepResult, error) {
	result := app.SweepResult{}
	uploadsDeleted, err := s.sweepS3Prefix(context.Background(), s.uploadBucket, "uploads/", now, rawUploadTTL)
	if err != nil {
		return result, err
	}
	reportsDeleted, err := s.sweepS3Prefix(context.Background(), s.reportBucket, "reports/", now, reportTTL)
	if err != nil {
		return result, err
	}
	result.UploadsDeleted = uploadsDeleted
	result.ReportsDeleted = reportsDeleted
	return result, nil
}

func (s *Store) sweepS3Prefix(ctx context.Context, bucket, prefix string, now time.Time, ttl time.Duration) (int, error) {
	if ttl <= 0 {
		return 0, nil
	}
	deleted := 0
	paginator := s3.NewListObjectsV2Paginator(s.s3, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return deleted, err
		}
		for _, object := range page.Contents {
			if object.LastModified == nil || now.Sub(*object.LastModified) <= ttl {
				continue
			}
			if _, err := s.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(bucket),
				Key:    object.Key,
			}); err != nil {
				return deleted, err
			}
			deleted++
		}
	}
	return deleted, nil
}

func (s *Store) putJob(job app.Job) error {
	item := map[string]dynamodbtypes.AttributeValue{
		"id":          &dynamodbtypes.AttributeValueMemberS{Value: job.ID},
		"status":      &dynamodbtypes.AttributeValueMemberS{Value: string(job.Status)},
		"upload_path": &dynamodbtypes.AttributeValueMemberS{Value: job.UploadPath},
		"updated_at":  &dynamodbtypes.AttributeValueMemberS{Value: job.UpdatedAt.Format(time.RFC3339Nano)},
	}
	if job.ScanType != "" {
		item["scan_type"] = &dynamodbtypes.AttributeValueMemberS{Value: string(job.ScanType)}
	}
	if !job.CreatedAt.IsZero() {
		item["created_at"] = &dynamodbtypes.AttributeValueMemberS{Value: job.CreatedAt.Format(time.RFC3339Nano)}
	}
	if job.MaxUploadBytes > 0 {
		item["max_upload_bytes"] = &dynamodbtypes.AttributeValueMemberN{Value: strconv.FormatInt(job.MaxUploadBytes, 10)}
	}
	if job.UploadTokenHash != "" {
		item["upload_token_hash"] = &dynamodbtypes.AttributeValueMemberS{Value: job.UploadTokenHash}
	}
	if job.ReportTokenHash != "" {
		item["report_token_hash"] = &dynamodbtypes.AttributeValueMemberS{Value: job.ReportTokenHash}
	}
	if !job.UploadTokenExpiresAt.IsZero() {
		item["upload_token_expires_at"] = &dynamodbtypes.AttributeValueMemberS{Value: job.UploadTokenExpiresAt.Format(time.RFC3339Nano)}
	}
	if !job.WaiverAcceptedAt.IsZero() {
		item["waiver_accepted_at"] = &dynamodbtypes.AttributeValueMemberS{Value: job.WaiverAcceptedAt.Format(time.RFC3339Nano)}
	}
	if !job.CompletedAt.IsZero() {
		item["completed_at"] = &dynamodbtypes.AttributeValueMemberS{Value: job.CompletedAt.Format(time.RFC3339Nano)}
	}
	if job.ReportPath != "" {
		item["report_path"] = &dynamodbtypes.AttributeValueMemberS{Value: job.ReportPath}
	}
	if job.Error != "" {
		item["error"] = &dynamodbtypes.AttributeValueMemberS{Value: job.Error}
	}
	_, err := s.dynamodb.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: aws.String(s.jobTable),
		Item:      item,
	})
	return err
}

func (s *Store) putEmailUnlock(unlock app.EmailUnlock) error {
	item := map[string]dynamodbtypes.AttributeValue{
		"id":               &dynamodbtypes.AttributeValueMemberS{Value: unlock.ID},
		"record_type":      &dynamodbtypes.AttributeValueMemberS{Value: "email_unlock"},
		"email":            &dynamodbtypes.AttributeValueMemberS{Value: unlock.Email},
		"email_hash":       &dynamodbtypes.AttributeValueMemberS{Value: unlock.EmailHash},
		"marketing_opt_in": &dynamodbtypes.AttributeValueMemberBOOL{Value: unlock.MarketingOptIn},
		"status":           &dynamodbtypes.AttributeValueMemberS{Value: string(unlock.Status)},
		"created_at":       &dynamodbtypes.AttributeValueMemberS{Value: unlock.CreatedAt.Format(time.RFC3339Nano)},
		"updated_at":       &dynamodbtypes.AttributeValueMemberS{Value: unlock.UpdatedAt.Format(time.RFC3339Nano)},
	}
	if unlock.SourceReportJobID != "" {
		item["source_report_job_id"] = &dynamodbtypes.AttributeValueMemberS{Value: unlock.SourceReportJobID}
	}
	if unlock.ConfirmationTokenHash != "" {
		item["confirmation_token_hash"] = &dynamodbtypes.AttributeValueMemberS{Value: unlock.ConfirmationTokenHash}
	}
	if unlock.FullScanTokenHash != "" {
		item["full_scan_token_hash"] = &dynamodbtypes.AttributeValueMemberS{Value: unlock.FullScanTokenHash}
	}
	if !unlock.FullScanTokenExpiresAt.IsZero() {
		item["full_scan_token_expires_at"] = &dynamodbtypes.AttributeValueMemberS{Value: unlock.FullScanTokenExpiresAt.Format(time.RFC3339Nano)}
	}
	if unlock.FullScanJobID != "" {
		item["full_scan_job_id"] = &dynamodbtypes.AttributeValueMemberS{Value: unlock.FullScanJobID}
	}
	if !unlock.ConfirmedAt.IsZero() {
		item["confirmed_at"] = &dynamodbtypes.AttributeValueMemberS{Value: unlock.ConfirmedAt.Format(time.RFC3339Nano)}
	}
	if !unlock.LastTransactionalEmailSentAt.IsZero() {
		item["last_transactional_email_sent_at"] = &dynamodbtypes.AttributeValueMemberS{Value: unlock.LastTransactionalEmailSentAt.Format(time.RFC3339Nano)}
	}
	_, err := s.dynamodb.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: aws.String(s.jobTable),
		Item:      item,
	})
	return err
}

func (s *Store) deleteQueueMessage(job app.Job) error {
	if job.QueueReceipt == "" {
		return nil
	}
	_, err := s.sqs.DeleteMessage(context.Background(), &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(s.queueURL),
		ReceiptHandle: aws.String(job.QueueReceipt),
	})
	return err
}

func jobIDFromMessage(message sqstypes.Message) (string, error) {
	var body map[string]string
	if err := json.Unmarshal([]byte(aws.ToString(message.Body)), &body); err != nil {
		return "", err
	}
	if body["job_id"] == "" {
		return "", errors.New("SQS message missing job_id")
	}
	return body["job_id"], nil
}

func jobFromItem(item map[string]dynamodbtypes.AttributeValue) (app.Job, error) {
	job := app.Job{
		ID:              stringAttr(item, "id"),
		Status:          app.JobStatus(stringAttr(item, "status")),
		ScanType:        app.ScanType(stringAttr(item, "scan_type")),
		UploadPath:      stringAttr(item, "upload_path"),
		MaxUploadBytes:  int64Attr(item, "max_upload_bytes"),
		UploadTokenHash: stringAttr(item, "upload_token_hash"),
		ReportTokenHash: stringAttr(item, "report_token_hash"),
		ReportPath:      stringAttr(item, "report_path"),
		Error:           stringAttr(item, "error"),
	}
	var err error
	job.CreatedAt, err = parseTimeAttr(item, "created_at")
	if err != nil {
		return job, err
	}
	job.UpdatedAt, err = parseTimeAttr(item, "updated_at")
	if err != nil {
		return job, err
	}
	job.UploadTokenExpiresAt, err = parseTimeAttr(item, "upload_token_expires_at")
	if err != nil {
		return job, err
	}
	job.WaiverAcceptedAt, err = parseTimeAttr(item, "waiver_accepted_at")
	if err != nil {
		return job, err
	}
	job.CompletedAt, err = parseTimeAttr(item, "completed_at")
	return job, err
}

func emailUnlockFromItem(item map[string]dynamodbtypes.AttributeValue) (app.EmailUnlock, error) {
	unlock := app.EmailUnlock{
		ID:                    stringAttr(item, "id"),
		Email:                 stringAttr(item, "email"),
		EmailHash:             stringAttr(item, "email_hash"),
		MarketingOptIn:        boolAttr(item, "marketing_opt_in"),
		SourceReportJobID:     stringAttr(item, "source_report_job_id"),
		ConfirmationTokenHash: stringAttr(item, "confirmation_token_hash"),
		FullScanTokenHash:     stringAttr(item, "full_scan_token_hash"),
		FullScanJobID:         stringAttr(item, "full_scan_job_id"),
		Status:                app.EmailUnlockStatus(stringAttr(item, "status")),
	}
	var err error
	unlock.CreatedAt, err = parseTimeAttr(item, "created_at")
	if err != nil {
		return unlock, err
	}
	unlock.UpdatedAt, err = parseTimeAttr(item, "updated_at")
	if err != nil {
		return unlock, err
	}
	unlock.ConfirmedAt, err = parseTimeAttr(item, "confirmed_at")
	if err != nil {
		return unlock, err
	}
	unlock.FullScanTokenExpiresAt, err = parseTimeAttr(item, "full_scan_token_expires_at")
	if err != nil {
		return unlock, err
	}
	unlock.LastTransactionalEmailSentAt, err = parseTimeAttr(item, "last_transactional_email_sent_at")
	return unlock, err
}

func stringAttr(item map[string]dynamodbtypes.AttributeValue, key string) string {
	value, ok := item[key].(*dynamodbtypes.AttributeValueMemberS)
	if !ok {
		return ""
	}
	return value.Value
}

func int64Attr(item map[string]dynamodbtypes.AttributeValue, key string) int64 {
	value, ok := item[key].(*dynamodbtypes.AttributeValueMemberN)
	if !ok {
		return 0
	}
	parsed, _ := strconv.ParseInt(value.Value, 10, 64)
	return parsed
}

func boolAttr(item map[string]dynamodbtypes.AttributeValue, key string) bool {
	value, ok := item[key].(*dynamodbtypes.AttributeValueMemberBOOL)
	if !ok {
		return false
	}
	return value.Value
}

func parseTimeAttr(item map[string]dynamodbtypes.AttributeValue, key string) (time.Time, error) {
	raw := stringAttr(item, key)
	if raw == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, raw)
}

func parseS3Path(path string) (string, string, error) {
	if !strings.HasPrefix(path, "s3://") {
		return "", "", fmt.Errorf("invalid s3 path %q", path)
	}
	rest := strings.TrimPrefix(path, "s3://")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid s3 path %q", path)
	}
	return parts[0], parts[1], nil
}

func requiredEnv(key string) string {
	return os.Getenv(key)
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
