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

LocalStack smoke:

```bash
./scripts/smoke-aws-local.sh
```

The sweeper also supports AWS mode, so production retention can be enforced by a scheduled `claude-analyzer-sweeper` task instead of relying only on S3 lifecycle rules.

## Required AWS Resources

- S3 quarantine upload bucket with 15 minute lifecycle deletion.
- S3 sanitized report bucket for durable private-link reports and token-scoped plugin artifacts.
- SQS standard queue for analysis jobs.
- DynamoDB jobs table keyed by `id`; email unlock records are stored in the same table under `record_type=email_unlock` until a dedicated subscriber table is needed.
- SES/Postmark transactional email support for future receipts and support workflows; email is no longer required for the public scan path.
- ECS/Fargate worker task role with read access to upload bucket, write access to report bucket, SQS consume/delete, and DynamoDB job update.
- API role with write access to upload bucket, SQS send, DynamoDB job write/read, and report read.

## Current Status

The AWS adapter compiles, is config-gated, and has been exercised against LocalStack. It is not deployed to real AWS yet.
