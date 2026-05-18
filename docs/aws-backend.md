# AWS Backend Preparation

The default backend is local file storage:

```bash
CLAUDE_ANALYZER_BACKEND=local
```

The production backend is selected with:

```bash
CLAUDE_ANALYZER_BACKEND=aws
AWS_REGION=us-east-1
CLAUDE_ANALYZER_UPLOAD_BUCKET=claude-analyzer-uploads
CLAUDE_ANALYZER_REPORT_BUCKET=claude-analyzer-reports
CLAUDE_ANALYZER_JOB_TABLE=claude-analyzer-jobs
CLAUDE_ANALYZER_JOB_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/123456789012/claude-analyzer-jobs
```

For LocalStack later:

```bash
AWS_ENDPOINT_URL=http://localstack:4566
AWS_ACCESS_KEY_ID=test
AWS_SECRET_ACCESS_KEY=test
```

## Required AWS Resources

- S3 quarantine upload bucket with 15 minute lifecycle deletion.
- S3 sanitized report bucket with 15 minute free-report deletion and 24 hour paid-artifact deletion.
- SQS standard queue for analysis jobs.
- DynamoDB jobs table keyed by `id`.
- ECS/Fargate worker task role with read access to upload bucket, write access to report bucket, SQS consume/delete, and DynamoDB job update.
- API role with write access to upload bucket, SQS send, DynamoDB job write/read, and report read.

## Current Status

The AWS adapter compiles and is config-gated. It is not deployed and should next be exercised against LocalStack before touching real AWS.

