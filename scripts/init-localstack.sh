#!/usr/bin/env bash
set -euo pipefail

PROJECT="${COMPOSE_PROJECT_NAME:-agent-log-analyzer-aws}"
COMPOSE=(docker compose -p "$PROJECT" -f docker-compose.aws.yml)

"${COMPOSE[@]}" up -d localstack

for _ in $(seq 1 60); do
  if curl -fsS http://127.0.0.1:4566/_localstack/health >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

awslocal() {
  "${COMPOSE[@]}" exec -T localstack awslocal "$@"
}

awslocal s3 mb s3://claude-analyzer-uploads >/dev/null 2>&1 || true
awslocal s3 mb s3://claude-analyzer-reports >/dev/null 2>&1 || true
awslocal sqs create-queue --queue-name claude-analyzer-jobs >/dev/null

if ! awslocal dynamodb describe-table --table-name claude-analyzer-jobs >/dev/null 2>&1; then
  awslocal dynamodb create-table \
    --table-name claude-analyzer-jobs \
    --attribute-definitions AttributeName=id,AttributeType=S \
    --key-schema AttributeName=id,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST >/dev/null
fi

echo "localstack resources ready"

