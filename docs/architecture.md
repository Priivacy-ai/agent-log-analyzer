# Architecture Plan

## Production Target

```text
CloudFront/CDN
  |
  +--> static landing page, sample reports, report shell
  |
local CLI
  |
  +--> parse/scrub/analyze on the user's machine
  +--> write reviewable sanitized report JSON
  +--> upload sanitized report only
  |
API Gateway / tiny Go control plane
  |
  +--> sanitized report intake
  +--> short-lived job/report metadata
  +--> short-lived report storage
```

The launch architecture keeps raw agent logs on the user's machine. The public upload UX is local CLI analysis plus sanitized-report upload; there is no browser file upload form, no public multipart upload endpoint, and no public raw-log upload prompt.

## Local Target

The local implementation uses Docker Compose with one API container, one worker container, and one shared data volume.

```text
browser
  |
  v
api container
  |
  +--> sanitized report intake
  +--> /data/jobs/completed
  +--> /data/reports
```

This is deliberately simpler than production but preserves the important product boundary: the raw log is analyzed locally, the server receives only a sanitized report artifact, and reports are short-lived.

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

- local CLI analyzes one newest log per supported source, currently Claude Code, Codex, and OpenCode
- user reviews `agent-analyzer-report.json`
- server receives sanitized report JSON only
- tokenized report URL

Email-confirmed full scan:

- user confirms email and receives a one-time full-scan token
- local CLI analyzes up to 100 recent logs per supported source after email unlock
- user reviews a sanitized aggregate report
- server receives sanitized aggregate report JSON only
- plugin artifact retention is separate from the free report TTL

## Scale Gates

- Static pages must be CDN cacheable.
- API report intake must be horizontally scalable and isolated from report/static traffic.
- Raw-log analysis must not be required for the public cloud path.
- Optional LLM interpretation must be load-sheddable.
