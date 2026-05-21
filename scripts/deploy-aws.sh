#!/usr/bin/env bash
set -euo pipefail

AWS_PROFILE="${AWS_PROFILE:-claude-analyzer-prod}"
AWS_REGION="${AWS_REGION:-us-east-1}"
PLATFORM="${PLATFORM:-linux/amd64}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
CLUSTER="${CLUSTER:-claude-analyzer-prod}"
SERVICES="${SERVICES:-claude-analyzer-prod-api claude-analyzer-prod-worker claude-analyzer-prod-email-events}"

if [ "$PLATFORM" != "linux/amd64" ]; then
  echo "refusing deploy: Fargate production expects linux/amd64, got PLATFORM=$PLATFORM" >&2
  exit 64
fi

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 127
  fi
}

require_command aws
require_command docker
require_command terraform
require_command python3

ecr_repo="$(AWS_PROFILE="$AWS_PROFILE" AWS_REGION="$AWS_REGION" terraform -chdir=infra/aws output -raw ecr_repository_url)"
image="${ecr_repo}:${IMAGE_TAG}"
registry="${ecr_repo%/*}"

echo "deploy target: $image"
echo "required platform: $PLATFORM"

AWS_PROFILE="$AWS_PROFILE" AWS_REGION="$AWS_REGION" aws ecr get-login-password --region "$AWS_REGION" \
  | docker login --username AWS --password-stdin "$registry" >/dev/null

docker buildx build --platform "$PLATFORM" --load -t "$image" .

local_platform="$(docker image inspect "$image" --format '{{.Os}}/{{.Architecture}}')"
if [ "$local_platform" != "$PLATFORM" ]; then
  echo "refusing push: local image platform is $local_platform, expected $PLATFORM" >&2
  exit 65
fi

docker push "$image"

remote_manifest="$(docker manifest inspect --verbose "$image")"
remote_platform="$(printf '%s' "$remote_manifest" | python3 -c 'import json,sys; data=json.load(sys.stdin); p=(data.get("Descriptor") or {}).get("platform") or {}; print(f"{p.get(\"os\", \"\")}/{p.get(\"architecture\", \"\")}")')"
if [ "$remote_platform" != "$PLATFORM" ]; then
  echo "refusing ECS update: remote image platform is $remote_platform, expected $PLATFORM" >&2
  exit 66
fi

echo "verified image platform: $remote_platform"

for service in $SERVICES; do
  AWS_PROFILE="$AWS_PROFILE" AWS_REGION="$AWS_REGION" aws ecs update-service \
    --cluster "$CLUSTER" \
    --service "$service" \
    --force-new-deployment >/dev/null
done

AWS_PROFILE="$AWS_PROFILE" AWS_REGION="$AWS_REGION" aws ecs wait services-stable \
  --cluster "$CLUSTER" \
  --services $SERVICES

echo "deploy stable: $image ($PLATFORM)"
