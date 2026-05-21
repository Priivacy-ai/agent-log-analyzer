# SES Transactional Email Operations

Agent Analyzer uses SES only for transactional email in the email-confirmed full-scan flow.

## What We Send

- Confirmation link after a user submits their email address.
- One-time full-scan NPX command after the user confirms ownership of that address.
- Plugin retrieval instructions after the local full scan uploads sanitized report JSON.

We do not send raw logs, raw report JSON, or transcript excerpts by email.

## Recipient List Rules

- Recipients enter their own email address in the unlock form.
- The confirmation email is double opt-in for the full scan command.
- Marketing consent is stored separately from transactional eligibility.
- Bounces, complaints, and rejects suppress future transactional sends to that recipient hash.

## Monitoring Controls

Production uses an SES configuration set for transactional sends. SES publishes send, delivery, bounce, complaint, reject, rendering-failure, and delivery-delay events to SNS, then SQS. The `claude-analyzer-email-events` worker consumes those messages and stores bounded delivery telemetry.

Stored event data is limited to:

- hashed recipient identity
- event type
- SES message id
- bounded detail such as `permanent` bounce type or complaint feedback category
- timestamp

Message bodies and raw email addresses are not stored in delivery-event records. Raw email addresses are already present only in the user-requested unlock record needed to send the transactional messages.

## Suppression

Two suppression layers are enabled:

- SES account-level suppression for `BOUNCE` and `COMPLAINT`.
- App-level suppression before send for `bounce`, `complaint`, and `reject` records.

If the app-level guard finds a suppressed recipient hash, it blocks the send before calling SES and returns a conflict to the unlock flow.

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
