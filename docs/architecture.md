# Architecture Plan

## Production Target

```text
CloudFront/CDN
  |
  +--> static landing page, sample reports, report shell
  |
API Gateway / tiny Go control plane
  |
  +--> one-time upload token
  +--> tokenized curl upload endpoint
  +--> short-lived job/report metadata
  +--> analysis queue
  |
isolated workers, no outbound internet
  |
  +--> read upload
  +--> parse/scrub/analyze
  +--> write sanitized report JSON
  +--> delete raw/intermediate data
```

The launch architecture must keep static traffic, upload traffic, and analysis work on separate failure domains. The only public upload UX is the Claude/prompt/curl flow; there is no browser file upload form and no public multipart upload endpoint.

## Local Target

The local implementation uses Docker Compose with one API container, one worker container, and one shared data volume.

```text
browser
  |
  v
api container
  |
  +--> one-time token/session creation
  +--> curl PUT upload
  +--> /data/uploads
  +--> /data/jobs/pending
  |
  v
worker container
  |
  +--> /data/jobs/processing
  +--> /data/reports
```

This is deliberately simpler than production but preserves the important product boundary: upload is asynchronous, analysis is done by a separate worker, and reports are sanitized artifacts.

## Production Mapping

| Local | Production |
| --- | --- |
| `/data/uploads` | S3 quarantine bucket with 15 minute lifecycle |
| `/data/jobs/pending` | SQS |
| `/data/reports` | S3 report bucket with TTL |
| API container | CDN + API Gateway + Go/Lambda control plane |
| Worker container | ECS Fargate worker in private subnet |

The code now has a backend selector:

```text
CLAUDE_ANALYZER_BACKEND=local -> local file store
CLAUDE_ANALYZER_BACKEND=aws   -> S3 + SQS + DynamoDB
```

AWS mode is intended to be tested against LocalStack before real cloud resources.

The first AWS deployment scaffold lives in `infra/aws`. It provisions the S3/SQS/DynamoDB backend, private ECS API/worker/sweeper tasks, ALB ingress, and VPC endpoints so the workers do not need general outbound internet.

## Load Shedding

`CLAUDE_ANALYZER_MAX_QUEUE_DEPTH` lets the API reject new analysis-session creation before issuing an upload token when the queue is saturated. This keeps launch spikes from turning into unbounded upload pressure.

## Upload Modes

Free scan:

- one-time token
- 15 minute token TTL
- one latest Claude Code JSONL log
- tokenized report URL

Paid scan:

- separate paid upload token after Stripe unlock
- command includes `CLAUDE_ANALYZER_SCAN_LIMIT=100`
- upload request includes `limit=100` and `X-Scan-Limit: 100`
- command bundles the 100 most recent `~/.claude/projects/**/*.jsonl` files
- paid artifact retention is separate from the free report TTL

## Scale Gates

- Static pages must be CDN cacheable.
- API upload endpoints must be horizontally scalable and isolated from report/static traffic.
- Analysis must never be synchronous.
- Worker backlog must degrade into wait time, not API failure.
- Optional LLM interpretation must be load-sheddable.
