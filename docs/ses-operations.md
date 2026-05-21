# Transactional Email Operations

Agent Analyzer no longer requires email for the public scan path. SES remains supported for legacy testing and future transactional receipts/support workflows. Postmark is also supported as an optional transactional provider once the Postmark account review is complete.

## What We Send

- Future receipts, support messages, and paid plugin delivery notifications.
- Legacy test-only confirmation/full-scan messages while the old endpoint remains available for compatibility coverage.

We do not send raw logs, raw report JSON, or transcript excerpts by email.

## Recipient List Rules

- Recipients enter their own email address in a future checkout/support flow.
- Any marketing or product-update email must use explicit consent.
- Marketing consent is stored separately from transactional eligibility.
- Bounces, complaints, and rejects suppress future transactional sends to that recipient hash.

## Monitoring Controls

SES production uses an SES configuration set for transactional sends. SES publishes send, delivery, bounce, complaint, reject, rendering-failure, and delivery-delay events to SNS, then SQS. The `claude-analyzer-email-events` worker consumes those messages and stores bounded delivery telemetry.

Postmark production sends through the configured transactional message stream, normally `outbound`. The app sends plain-text transactional messages with open tracking disabled and link tracking set to `None`.

Stored event data is limited to:

- hashed recipient identity
- event type
- SES message id
- bounded detail such as `permanent` bounce type or complaint feedback category
- timestamp

Message bodies and raw email addresses are not stored in delivery-event records. Raw email addresses are already present only in the user-requested unlock record needed to send the transactional messages.

## Suppression

SES has two suppression layers enabled:

- SES account-level suppression for `BOUNCE` and `COMPLAINT`.
- App-level suppression before send for `bounce`, `complaint`, and `reject` records.

If the app-level guard finds a suppressed recipient hash, it blocks the send before calling SES and returns a conflict to the unlock flow.

Postmark also suppresses hard bounces and spam complaints at the provider level. App-level suppression remains active for any provider-backed send once events are recorded.

## Provider Selection

Runtime provider selection is controlled by environment variables:

```sh
CLAUDE_ANALYZER_EMAIL_PROVIDER=ses
CLAUDE_ANALYZER_EMAIL_FROM=robert@spec-kitty.ai
```

or:

```sh
CLAUDE_ANALYZER_EMAIL_PROVIDER=postmark
CLAUDE_ANALYZER_EMAIL_FROM=robert@spec-kitty.ai
CLAUDE_ANALYZER_POSTMARK_MESSAGE_STREAM=outbound
POSTMARK_SERVER_TOKEN=<from AWS Secrets Manager in production>
```

Do not store `POSTMARK_SERVER_TOKEN` in Terraform files, `.env`, documentation, or logs. In AWS, inject it into the API ECS task from Secrets Manager.

## Alarms

CloudWatch alarms cover:

- SES bounces
- SES complaints
- SES rejects
- SES event queue age
- SES event worker failures
- SES event worker CPU

The launch dashboard includes SES transactional outcomes and event-worker throughput.

## Operations Response

If SES bounces or complaints alarm:

1. Confirm the source is transactional unlock email, not marketing.
2. Check the SES event queue age and event-worker failure alarms.
3. Inspect CloudWatch logs by bounded event type and hashed recipient only.
4. Do not paste raw email addresses, message bodies, AWS keys, report JSON, or logs into tickets or chat.
5. If complaint rate is non-zero, pause any new non-essential email flow until the cause is understood.
