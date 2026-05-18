package awsstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
	"github.com/robertDouglass/claude-log-analyzer/internal/analyzer"
	"github.com/robertDouglass/claude-log-analyzer/internal/app"
)

type Store struct {
	s3           *s3.Client
	presign      *s3.PresignClient
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
	store.presign = s3.NewPresignClient(store.s3)
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

func (s *Store) CreateDirectUpload(jobID string, expiresIn time.Duration, maxBytes int64) (app.DirectUpload, error) {
	now := time.Now().UTC()
	key := "uploads/" + jobID + ".log"
	job := app.Job{
		ID:             jobID,
		Status:         app.StatusUploading,
		UploadPath:     "s3://" + s.uploadBucket + "/" + key,
		MaxUploadBytes: maxBytes,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.putJob(job); err != nil {
		return app.DirectUpload{}, err
	}
	request, err := s.presign.PresignPutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(s.uploadBucket),
		Key:         aws.String(key),
		ContentType: aws.String("application/octet-stream"),
	}, func(options *s3.PresignOptions) {
		options.Expires = expiresIn
	})
	if err != nil {
		return app.DirectUpload{}, err
	}
	return app.DirectUpload{
		JobID:        jobID,
		Method:       request.Method,
		URL:          request.URL,
		Headers:      signedHeaders(request.SignedHeader),
		ExpiresAt:    now.Add(expiresIn),
		MaxBytes:     maxBytes,
		FinalizePath: "/api/jobs/" + jobID + "/finalize",
	}, nil
}

func (s *Store) FinalizeDirectUpload(jobID string) error {
	job, err := s.GetJob(jobID)
	if err != nil {
		return err
	}
	switch job.Status {
	case app.StatusUploading:
	case app.StatusPending, app.StatusProcessing, app.StatusCompleted:
		return nil
	default:
		return errors.New("job is not waiting for upload")
	}
	bucket, key, err := parseS3Path(job.UploadPath)
	if err != nil {
		return err
	}
	if bucket != s.uploadBucket {
		return errors.New("upload bucket mismatch")
	}
	head, err := s.s3.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}
	if job.MaxUploadBytes > 0 && head.ContentLength != nil && *head.ContentLength > job.MaxUploadBytes {
		_, _ = s.s3.DeleteObject(context.Background(), &s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		return errors.New("upload exceeds maximum size")
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

func signedHeaders(headers http.Header) map[string]string {
	out := map[string]string{}
	for key, values := range headers {
		if len(values) > 0 {
			out[key] = values[0]
		}
	}
	return out
}

func (s *Store) putJob(job app.Job) error {
	item := map[string]dynamodbtypes.AttributeValue{
		"id":          &dynamodbtypes.AttributeValueMemberS{Value: job.ID},
		"status":      &dynamodbtypes.AttributeValueMemberS{Value: string(job.Status)},
		"upload_path": &dynamodbtypes.AttributeValueMemberS{Value: job.UploadPath},
		"updated_at":  &dynamodbtypes.AttributeValueMemberS{Value: job.UpdatedAt.Format(time.RFC3339Nano)},
	}
	if !job.CreatedAt.IsZero() {
		item["created_at"] = &dynamodbtypes.AttributeValueMemberS{Value: job.CreatedAt.Format(time.RFC3339Nano)}
	}
	if job.MaxUploadBytes > 0 {
		item["max_upload_bytes"] = &dynamodbtypes.AttributeValueMemberN{Value: strconv.FormatInt(job.MaxUploadBytes, 10)}
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
		ID:             stringAttr(item, "id"),
		Status:         app.JobStatus(stringAttr(item, "status")),
		UploadPath:     stringAttr(item, "upload_path"),
		MaxUploadBytes: int64Attr(item, "max_upload_bytes"),
		ReportPath:     stringAttr(item, "report_path"),
		Error:          stringAttr(item, "error"),
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
	job.CompletedAt, err = parseTimeAttr(item, "completed_at")
	return job, err
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
