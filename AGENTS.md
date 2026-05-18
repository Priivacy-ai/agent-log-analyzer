## Spec Kitty SaaS Testing On This Computer

- On this computer, when running `spec-kitty` commands that use SaaS, tracker, or sync flows for testing, always set `SPEC_KITTY_ENABLE_SAAS_SYNC=1`.
- The purpose of this machine-level rule is to ensure CLI sync and tracker data flows to the Spec Kitty SaaS dev deployment used for testing, currently `https://spec-kitty-dev.fly.dev/`.
- Do not assume the flag is optional on this machine during dev testing. If a command path touches hosted auth, tracker, or sync behavior, use the env var unless the user explicitly says not to.
- This is a local testing rule for the CLI on this computer. It does not mean tracker itself has a rollout system, and it does not justify keeping rollout gating inside `spec-kitty-tracker`.

## AWS Deployment Profile

- Use the `claude-analyzer-prod` AWS profile for production infrastructure work.
- Default deployment region is `us-east-1`.
- Prefer setting the environment before Terraform/AWS commands:

```sh
export AWS_PROFILE=claude-analyzer-prod
export AWS_REGION=us-east-1
terraform -chdir=infra/aws plan
```

- One-off equivalent:

```sh
AWS_PROFILE=claude-analyzer-prod terraform -chdir=infra/aws plan
```

- Do not paste AWS access keys or secret access keys into chat, docs, commits, or logs.
- The local `.env` may contain profile/region selectors only. It must not contain credentials.
- The profile may exist before it has sufficient IAM permissions. Verify identity and permissions before applying infrastructure.
