#!/usr/bin/env bash
set -euo pipefail

AWS_PROFILE="${AWS_PROFILE:-claude-analyzer-prod}"
AWS_REGION="${AWS_REGION:-us-east-1}"
PLATFORM="${PLATFORM:-linux/amd64}"
if [ -z "${IMAGE_TAG:-}" ]; then
  git_sha="$(git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)"
  IMAGE_TAG="deploy-$(date -u +%Y%m%d%H%M%S)-${git_sha}"
fi
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

retry() {
  local attempts="$1"
  shift
  local attempt=1
  local delay=5
  while true; do
    if "$@"; then
      return 0
    fi
    if [ "$attempt" -ge "$attempts" ]; then
      return 1
    fi
    echo "command failed; retrying in ${delay}s ($attempt/$attempts): $*" >&2
    sleep "$delay"
    attempt=$((attempt + 1))
    delay=$((delay * 2))
  done
}

ecr_login() {
  local password
  password="$(AWS_PROFILE="$AWS_PROFILE" AWS_REGION="$AWS_REGION" aws ecr get-login-password --region "$AWS_REGION")"
  printf '%s' "$password" | docker login --username AWS --password-stdin "$registry" >/dev/null
}

ecr_repo="$(AWS_PROFILE="$AWS_PROFILE" AWS_REGION="$AWS_REGION" terraform -chdir=infra/aws output -raw ecr_repository_url)"
image="${ecr_repo}:${IMAGE_TAG}"
registry="${ecr_repo%/*}"

echo "deploy target: $image"
echo "required platform: $PLATFORM"
echo "immutable tag: $IMAGE_TAG"

retry 3 ecr_login

retry 2 docker buildx build --platform "$PLATFORM" -t "$image" --push .

remote_image="$(docker buildx imagetools inspect "$image" --format '{{json .Image}}')"
remote_platform="$(printf '%s' "$remote_image" | python3 -c 'import json,sys; data=json.load(sys.stdin); print("%s/%s" % (data.get("os", ""), data.get("architecture", "")))')"
if [ "$remote_platform" != "$PLATFORM" ]; then
  echo "refusing ECS update: remote image platform is $remote_platform, expected $PLATFORM" >&2
  exit 66
fi

echo "verified image platform: $remote_platform"

image_digest="$(AWS_PROFILE="$AWS_PROFILE" AWS_REGION="$AWS_REGION" aws ecr describe-images \
  --repository-name "${ecr_repo##*/}" \
  --image-ids imageTag="$IMAGE_TAG" \
  --query 'imageDetails[0].imageDigest' \
  --output text)"

if [ -z "$image_digest" ] || [ "$image_digest" = "None" ]; then
  echo "refusing ECS update: ECR did not return a digest for $image" >&2
  exit 67
fi

immutable_image="${ecr_repo}@${image_digest}"
echo "verified image digest: $image_digest"
echo "deploying immutable image: $immutable_image"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

for service in $SERVICES; do
  current_task_definition="$(AWS_PROFILE="$AWS_PROFILE" AWS_REGION="$AWS_REGION" aws ecs describe-services \
    --cluster "$CLUSTER" \
    --services "$service" \
    --query 'services[0].taskDefinition' \
    --output text)"

  if [ -z "$current_task_definition" ] || [ "$current_task_definition" = "None" ]; then
    echo "could not resolve current task definition for $service" >&2
    exit 68
  fi

  described="$tmpdir/${service}-task-definition.json"
  register_input="$tmpdir/${service}-register-task-definition.json"

  AWS_PROFILE="$AWS_PROFILE" AWS_REGION="$AWS_REGION" aws ecs describe-task-definition \
    --task-definition "$current_task_definition" \
    --query 'taskDefinition' \
    --output json > "$described"

  python3 - "$described" "$register_input" "$immutable_image" <<'PY'
import json
import sys

source_path, output_path, image = sys.argv[1:]
with open(source_path, "r", encoding="utf-8") as handle:
    task_definition = json.load(handle)

allowed = [
    "family",
    "taskRoleArn",
    "executionRoleArn",
    "networkMode",
    "containerDefinitions",
    "volumes",
    "placementConstraints",
    "requiresCompatibilities",
    "cpu",
    "memory",
    "pidMode",
    "ipcMode",
    "proxyConfiguration",
    "inferenceAccelerators",
    "ephemeralStorage",
    "runtimePlatform",
]

payload = {key: task_definition[key] for key in allowed if key in task_definition and task_definition[key] not in (None, [])}
for container in payload["containerDefinitions"]:
    container["image"] = image

with open(output_path, "w", encoding="utf-8") as handle:
    json.dump(payload, handle, separators=(",", ":"))
PY

  new_task_definition="$(AWS_PROFILE="$AWS_PROFILE" AWS_REGION="$AWS_REGION" aws ecs register-task-definition \
    --cli-input-json "file://$register_input" \
    --query 'taskDefinition.taskDefinitionArn' \
    --output text)"

  echo "updating $service to $new_task_definition"
  AWS_PROFILE="$AWS_PROFILE" AWS_REGION="$AWS_REGION" aws ecs update-service \
    --cluster "$CLUSTER" \
    --service "$service" \
    --task-definition "$new_task_definition" >/dev/null
done

AWS_PROFILE="$AWS_PROFILE" AWS_REGION="$AWS_REGION" aws ecs wait services-stable \
  --cluster "$CLUSTER" \
  --services $SERVICES

echo "deploy stable: $immutable_image ($PLATFORM)"
