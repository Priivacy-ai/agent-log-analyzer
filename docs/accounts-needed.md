# Accounts Needed

Required for production launch:

- AWS account, S3, CloudFront, WAF, SQS, DynamoDB, ECS Fargate, IAM, CloudWatch
- GitHub organization/repo admin access
- AWS SES production access and verified sender/domain for future transactional receipts/support emails
- Stripe account for Checkout and paid plugin downloads
- Anthropic API account for optional interpretation layer
- Domain registrar/DNS provider, preferably Route 53 or Cloudflare
- Backup email provider for transactional receipts/support, for example Mailgun, Postmark, or Resend
- Error/uptime monitoring, for example Sentry plus Better Stack or equivalent

Optional but useful:

- Product analytics provider that supports strict event schemas, or warehouse-only analytics
- Legal/privacy review account/vendor
- Load testing cloud runners if local network is insufficient
- HN/ProductHunt launch tracking accounts

Do not create cloud infrastructure until the Docker-local runthrough and CI gates pass.
