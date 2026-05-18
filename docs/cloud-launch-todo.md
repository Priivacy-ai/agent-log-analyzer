# Cloud Launch TODO

This is the remaining work to move from green Docker/LocalStack gates to real cloud services.

## 0. Credential And Account Prep

- [x] Install AWS CLI v2 locally.
- [x] Configure access with AWS SSO or a named AWS profile. Do not paste long-lived AWS access keys into chat.
- [x] Confirm the target account:

  ```sh
  aws sts get-caller-identity --profile <profile>
  ```

- [x] Decide production region, default `us-east-1`.
- [ ] Confirm Route 53 or external DNS provider access.
- [ ] Confirm ACM certificate path:
  - [ ] Use existing cert ARN, or
  - [ ] Request a new public ACM cert in the production region.
- [x] Confirm container image naming and whether ECR should be created by Terraform or pre-existing.

## 1. Terraform State Bootstrap

The current `infra/aws` scaffold uses local Terraform state. Before production apply:

- [x] Create a remote Terraform state bucket.
- [x] Create a DynamoDB lock table.
- [x] Add `backend "s3"` config to `infra/aws/versions.tf`.
- [x] Run:

  ```sh
  terraform -chdir=infra/aws init -migrate-state
  terraform -chdir=infra/aws validate
  terraform -chdir=infra/aws plan
  ```

Acceptance:

- [x] Terraform state is remote, encrypted, and locked.
- [x] No local `.tfstate` is required for production operations.

## 2. Infrastructure Review Before Apply

- [ ] Review `infra/aws/main.tf` for:
  - [ ] VPC CIDR conflicts.
  - [ ] ALB naming length.
  - [ ] IAM least privilege.
  - [ ] S3 public access blocks.
  - [ ] S3 encryption.
  - [ ] SQS visibility timeout.
  - [ ] ECS CPU/memory sizing.
  - [ ] Scheduled sweeper every 5 minutes.
  - [ ] No NAT gateway and no broad outbound internet from workers.
- [ ] Run:

  ```sh
  terraform -chdir=infra/aws fmt -check -recursive
  terraform -chdir=infra/aws validate
  terraform -chdir=infra/aws plan
  ```

Acceptance:

- [x] Plan contains only expected resources.
- [x] No public S3 bucket policy.
- [x] ECS tasks are in private subnets.

## 3. First AWS Apply

- [x] Apply base infrastructure:

  ```sh
  terraform -chdir=infra/aws apply
  ```

- [x] Capture outputs:

  ```sh
  terraform -chdir=infra/aws output
  ```

- [ ] Confirm created resources:
  - [x] ECR repository.
  - [x] Upload bucket.
  - [x] Report bucket.
  - [x] SQS queue.
  - [x] DynamoDB job table.
  - [x] ECS cluster/services.
  - [x] ALB target group.
  - [x] CloudWatch log group.

Acceptance:

- [x] Terraform apply exits cleanly.
- [x] ECS services may initially fail until the image is pushed; that is acceptable for first apply.

## 4. Build And Push Container Image

- [x] Authenticate Docker to ECR:

  ```sh
  aws ecr get-login-password --region <region> --profile <profile> \
    | docker login --username AWS --password-stdin <account>.dkr.ecr.<region>.amazonaws.com
  ```

- [x] Build and push:

  ```sh
  ECR_REPO="$(terraform -chdir=infra/aws output -raw ecr_repository_url)"
  docker build -t "$ECR_REPO:latest" .
  docker push "$ECR_REPO:latest"
  ```

- [x] Force ECS services to pull the image:

  ```sh
  aws ecs update-service --cluster <cluster> --service <api-service> --force-new-deployment --profile <profile>
  aws ecs update-service --cluster <cluster> --service <worker-service> --force-new-deployment --profile <profile>
  ```

Acceptance:

- [x] API service reaches steady state.
- [x] Worker service reaches steady state.
- [x] No container crash loops.

## 5. Cloud Smoke Test

- [x] Get ALB DNS:

  ```sh
  terraform -chdir=infra/aws output -raw alb_dns_name
  ```

- [x] Verify health:

  ```sh
  curl -fsS "http://<alb-dns>/healthz"
  ```

- [x] Upload `testdata/fixtures/sample-claude.jsonl` to the cloud API.
- [x] Poll job status until completed.
- [x] Fetch report.
- [ ] Verify:
  - [ ] Report contains no raw secret.
  - [ ] Report contains `raw_transcript_sent_to_llm=false`.
  - [ ] Raw upload object is deleted by the sweeper after TTL.
  - [ ] Report object is deleted by the sweeper after TTL.
  - [ ] Logs contain request metadata only, not raw upload/report contents.

Acceptance:

- [x] One full real-AWS job completes.
- [ ] Retention works in real AWS, not only LocalStack.

## 6. TLS, DNS, CDN, And WAF

- [ ] Add or pass `certificate_arn` for HTTPS listener.
- [ ] Configure DNS record for the launch domain.
- [ ] Put CloudFront in front of the ALB.
- [ ] Add WAF protections:
  - [x] Managed common rule set.
  - [x] Rate-based rule for upload/job endpoints.
  - [x] Body size limits aligned with app max upload size.
  - [ ] Bot-control only if cost is acceptable.
- [ ] Cache static assets aggressively.
- [ ] Do not cache job or report JSON unless report URLs are made unguessable and TTL-safe.

Acceptance:

- [ ] Public domain serves HTTPS.
- [ ] Static UI is CDN-backed.
- [ ] API endpoints are reachable and not cached incorrectly.

## 7. Claude/Curl Upload Path

The public upload UX is Claude/prompt/curl only. There is no browser file upload form, no public multipart upload endpoint, and no direct browser-to-S3 upload surface.

- [x] Add API endpoint to create a one-time analysis session token.
- [x] Set upload token expiry to 15 minutes or less.
- [x] Upload one free-scan JSONL log with `PUT /api/uploads/{job_id}` and `Authorization: Bearer <token>`.
- [x] Enqueue analysis only after `POST /api/uploads/{job_id}/finalize`.
- [x] Serve reports only through tokenized `/r/{job_id}/{report_token}` URLs.
- [x] Update LocalStack smoke to cover the token/curl flow.
- [x] Add paid bundle upload endpoint for paid-token jobs.
- [x] Enforce paid scan upload contract: `limit=100` and `X-Scan-Limit: 100`.
- [x] Validate paid tar/gzip bundles for max 100 JSONL files and hostile archive entries.
- [x] Add worker aggregate analysis path for paid bundles.
- [ ] Add the Stripe-gated paid-session endpoint that creates paid-token jobs.
- [ ] Connect paid command generation to the Stripe-created session.

Acceptance:

- [x] Browser upload and direct-upload routes are not mounted.
- [ ] API upload tasks autoscale separately enough to survive Product Hunt/HN upload spikes.

## 8. Observability Without Privacy Leakage

- [x] Add CloudWatch dashboards:
  - [x] ALB 5xx/4xx.
  - [x] API target response time.
  - [x] ECS CPU/memory.
  - [x] SQS visible and not-visible depth.
  - [x] SQS oldest message age.
  - [x] Worker completed/failed counts.
  - [x] Sweeper deleted object counts.
- [x] Add alarms:
  - [x] API 5xx > 0.1%.
  - [x] Worker failures > 1%.
  - [x] Queue age > target threshold.
  - [x] ECS tasks unhealthy.
  - [x] Sweeper not running.
- [ ] Add structured aggregate metrics only:
  - [x] Score bucket.
  - [x] Waste bucket.
  - [x] Finding IDs/severities.
  - [x] Redaction family counts.
  - [x] Public ecosystem IDs.
  - [x] Unknown private-name counts only.
- [ ] Confirm logs do not include:
  - [ ] Raw uploads.
  - [ ] Raw report JSON.
  - [ ] Raw secrets.
  - [ ] Unknown private tool names.
  - [ ] Full job/report URLs with identifiers.

Acceptance:

- [ ] We can diagnose load and failures without seeing user logs.

## 9. Load Testing Against Cloud

- [ ] Create cloud-safe load fixtures with fake secrets only.
- [ ] Run staged load:
  - [ ] 10 uploads.
  - [ ] 100 uploads.
  - [ ] 1,000 uploads.
  - [ ] Burst upload-init traffic without uploads.
  - [ ] Worker backlog test.
- [ ] Measure:
  - [ ] Upload-init p95.
  - [ ] Job creation p95.
  - [ ] Report fetch p95.
  - [ ] Analysis p95.
  - [ ] Queue wait p95.
  - [ ] API 5xx rate.
  - [ ] Worker failure rate.
  - [ ] Sweeper lag.
- [ ] Tune:
  - [ ] API desired count.
  - [ ] Worker desired count.
  - [ ] SQS visibility timeout.
  - [ ] `CLAUDE_ANALYZER_MAX_QUEUE_DEPTH`.
  - [ ] ALB idle timeout if needed.

Acceptance:

- [ ] Static landing p95 < 300ms from CDN.
- [ ] Upload-init p95 < 250ms.
- [ ] Job creation p95 < 300ms.
- [ ] Report shell p95 < 500ms from CDN.
- [ ] Normal analysis p95 < 3 minutes.
- [ ] Burst queue wait p95 < 20 minutes.
- [ ] API 5xx rate < 0.1%.
- [ ] Worker failure rate < 1%.

## 10. Security Review

- [ ] Run dependency scan.
- [ ] Run container scan.
- [ ] Review IAM policies.
- [ ] Review WAF logs.
- [ ] Confirm no public bucket access.
- [ ] Confirm ECS workers have no NAT route.
- [ ] Confirm scrubber coverage on hosted environment.
- [ ] Confirm malformed uploads fail safely.
- [ ] Confirm no prompt-injection text reaches any LLM layer.
- [ ] Confirm optional LLM interpretation layer remains disabled until separately reviewed.

Acceptance:

- [ ] A hostile upload cannot exfiltrate data.
- [ ] A leaked report URL expires.
- [ ] A parser failure does not leak raw logs to logs, metrics, or reports.

## 11. Payment And Paid Pack Delivery

- [ ] Create Stripe account/products.
- [ ] Define paid artifact TTL separately from free report TTL.
- [ ] Implement Checkout flow.
- [ ] Generate optimization pack artifact after successful payment.
- [x] Define deterministic Claude plugin artifact contract.
- [x] Add initial plugin artifact generator and archive safety tests.
- [ ] Wire paid scan aggregate metrics into plugin generation.
- [ ] Render short-lived install page with plugin commands.
- [ ] Store paid artifact in separate prefix/bucket with short TTL.
- [ ] Add receipt/support email.
- [ ] Add refund/support process.

Acceptance:

- [ ] Free analysis works without account.
- [ ] Paid pack delivery does not require persistent user accounts.
- [ ] Paid artifact storage is still TTL-bound.

## 12. Launch Readiness Drill

- [ ] Run full local Docker smoke.
- [ ] Run LocalStack AWS smoke.
- [x] Run Terraform validate.
- [x] Run cloud smoke.
- [ ] Run cloud load test.
- [ ] Verify dashboards and alarms.
- [ ] Verify support email.
- [ ] Verify privacy policy and retention copy match implementation.
- [ ] Freeze production config.
- [ ] Tag release.
- [ ] Prepare rollback:
  - [ ] Scale API down/up.
  - [ ] Scale workers down/up.
  - [ ] Disable upload endpoint by queue-depth setting or WAF rule.
  - [ ] Revert ECS service to previous task definition.

Acceptance:

- [ ] We can launch without creating infrastructure manually in the AWS console.
- [ ] We can stop intake without losing queued work.
- [ ] We can explain exactly what is stored, for how long, and why.
