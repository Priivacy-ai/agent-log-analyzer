# Architecture Plan

## Production Target

```text
CloudFront/CDN
  |
  +--> static landing page, sample reports, report shell
  |
API Gateway / tiny Go control plane
  |
  +--> signed upload URL
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

The launch architecture must keep static traffic, upload traffic, and analysis work on separate failure domains.

## Local Target

The local implementation uses Docker Compose with one API container, one worker container, and one shared data volume.

```text
browser
  |
  v
api container
  |
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

## Scale Gates

- Static pages must be CDN cacheable.
- Uploads must not proxy through the API service in production.
- Analysis must never be synchronous.
- Worker backlog must degrade into wait time, not API failure.
- Optional LLM interpretation must be load-sheddable.

