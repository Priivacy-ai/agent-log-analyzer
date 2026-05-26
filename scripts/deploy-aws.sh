#!/usr/bin/env bash
set -euo pipefail

AWS_PROFILE="${AWS_PROFILE-claude-analyzer-prod}"
AWS_REGION="${AWS_REGION:-us-east-1}"
PLATFORM="${PLATFORM:-linux/amd64}"
BUILDX_BUILDER="${BUILDX_BUILDER:-claude-analyzer-builder}"
if [ -z "${IMAGE_TAG:-}" ]; then
  git_sha="$(git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)"
  IMAGE_TAG="deploy-$(date -u +%Y%m%d%H%M%S)-${git_sha}"
fi
CLUSTER="${CLUSTER:-claude-analyzer-prod}"
SERVICES="${SERVICES:-claude-analyzer-prod-api claude-analyzer-prod-worker claude-analyzer-prod-email-events}"
DEPLOY_LOCK_TABLE="${DEPLOY_LOCK_TABLE:-claude-analyzer-terraform-locks}"
DEPLOY_LOCK_ID="${DEPLOY_LOCK_ID:-${CLUSTER}/deploy-lock}"
DEPLOY_LOCK_TTL_SECONDS="${DEPLOY_LOCK_TTL_SECONDS:-2700}"
DEPLOY_LOCK_WAIT_SECONDS="${DEPLOY_LOCK_WAIT_SECONDS:-1800}"
DEPLOY_LOCK_POLL_SECONDS="${DEPLOY_LOCK_POLL_SECONDS:-10}"
DEPLOY_LOCK_TOKEN="${DEPLOY_LOCK_TOKEN:-$(python3 -c 'import uuid; print(uuid.uuid4())' 2>/dev/null || uuidgen 2>/dev/null || date -u +%s-$$)}"
DEPLOY_LOCK_HOLDER="${DEPLOY_LOCK_HOLDER:-$(whoami 2>/dev/null || echo unknown)@$(hostname 2>/dev/null || echo unknown):$$}"
DEPLOY_COMMIT="$(git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)"
tmpdir=""
lock_acquired=0

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

cleanup() {
  local rc=$?
  if [ "$lock_acquired" = "1" ]; then
    if ! aws_cmd dynamodb delete-item \
      --table-name "$DEPLOY_LOCK_TABLE" \
      --key '{"LockID":{"S":"'"$DEPLOY_LOCK_ID"'"}}' \
      --condition-expression "Token = :token" \
      --expression-attribute-values '{":token":{"S":"'"$DEPLOY_LOCK_TOKEN"'"}}' >/dev/null 2>&1; then
      echo "warning: could not release deploy lock $DEPLOY_LOCK_ID; it will expire by ExpiresAt" >&2
    fi
  fi
  if [ -n "$tmpdir" ]; then
    rm -rf "$tmpdir"
  fi
  exit "$rc"
}
trap cleanup EXIT

aws_cmd() {
  if [ -n "$AWS_PROFILE" ]; then
    AWS_PROFILE="$AWS_PROFILE" AWS_REGION="$AWS_REGION" aws "$@"
  else
    AWS_REGION="$AWS_REGION" aws "$@"
  fi
}

terraform_cmd() {
  if [ -n "$AWS_PROFILE" ]; then
    AWS_PROFILE="$AWS_PROFILE" AWS_REGION="$AWS_REGION" terraform "$@"
  else
    AWS_REGION="$AWS_REGION" terraform "$@"
  fi
}

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
  password="$(aws_cmd ecr get-login-password --region "$AWS_REGION")"
  printf '%s' "$password" | docker login --username AWS --password-stdin "$registry" >/dev/null
}

acquire_deploy_lock() {
  local started now expires deadline holder_json commit_json tag_json
  started="$(date -u +%s)"
  deadline=$((started + DEPLOY_LOCK_WAIT_SECONDS))
  holder_json="$(printf '%s' "$DEPLOY_LOCK_HOLDER" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))')"
  commit_json="$(printf '%s' "$DEPLOY_COMMIT" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))')"
  tag_json="$(printf '%s' "$IMAGE_TAG" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))')"

  echo "acquiring deploy lock: $DEPLOY_LOCK_TABLE / $DEPLOY_LOCK_ID"
  while true; do
    now="$(date -u +%s)"
    expires=$((now + DEPLOY_LOCK_TTL_SECONDS))
    if aws_cmd dynamodb put-item \
      --table-name "$DEPLOY_LOCK_TABLE" \
      --item '{"LockID":{"S":"'"$DEPLOY_LOCK_ID"'"},"Token":{"S":"'"$DEPLOY_LOCK_TOKEN"'"},"Holder":{"S":'"$holder_json"'},"Commit":{"S":'"$commit_json"'},"ImageTag":{"S":'"$tag_json"'},"CreatedAt":{"N":"'"$now"'"},"ExpiresAt":{"N":"'"$expires"'"}}' \
      --condition-expression "attribute_not_exists(LockID) OR ExpiresAt < :now" \
      --expression-attribute-values '{":now":{"N":"'"$now"'"}}' >/dev/null; then
      lock_acquired=1
      echo "deploy lock acquired by $DEPLOY_LOCK_HOLDER; expires in ${DEPLOY_LOCK_TTL_SECONDS}s"
      return 0
    fi

    echo "deploy lock is held; waiting ${DEPLOY_LOCK_POLL_SECONDS}s" >&2
    aws_cmd dynamodb get-item \
      --table-name "$DEPLOY_LOCK_TABLE" \
      --key '{"LockID":{"S":"'"$DEPLOY_LOCK_ID"'"}}' \
      --projection-expression "Holder,Commit,ImageTag,CreatedAt,ExpiresAt" \
      --output json >&2 || true

    if [ "$now" -ge "$deadline" ]; then
      echo "timed out waiting for deploy lock $DEPLOY_LOCK_ID after ${DEPLOY_LOCK_WAIT_SECONDS}s" >&2
      exit 69
    fi
    sleep "$DEPLOY_LOCK_POLL_SECONDS"
  done
}

wait_for_services_stable() {
  local phase="$1"
  echo "waiting for ECS services to be stable ($phase): $SERVICES"
  aws_cmd ecs wait services-stable \
    --cluster "$CLUSTER" \
    --services $SERVICES
}

verify_services_quiescent() {
  local services_json
  services_json="$(aws_cmd ecs describe-services --cluster "$CLUSTER" --services $SERVICES --output json)"
  python3 -c '
import json
import sys

data = json.load(sys.stdin)
bad = []
for service in data.get("services", []):
    name = service["serviceName"]
    if service.get("pendingCount") != 0:
        bad.append("{}: pendingCount={}".format(name, service.get("pendingCount")))
    if service.get("runningCount") != service.get("desiredCount"):
        bad.append("{}: runningCount={} desiredCount={}".format(name, service.get("runningCount"), service.get("desiredCount")))
    for deployment in service.get("deployments", []):
        rollout = deployment.get("rolloutState")
        status = deployment.get("status")
        desired = deployment.get("desiredCount")
        running = deployment.get("runningCount")
        if rollout == "IN_PROGRESS" or (status != "PRIMARY" and running):
            bad.append(
                "{}: deployment {} status={} rollout={} desired={} running={}".format(
                    name, deployment.get("id"), status, rollout, desired, running
                )
            )
if bad:
    print("ECS services are not quiescent:", file=sys.stderr)
    for item in bad:
        print(f"- {item}", file=sys.stderr)
    sys.exit(1)
' <<< "$services_json"
}

verify_deployed_image() {
  local service service_tasks task_definition task_image tasks_json
  for service in $SERVICES; do
    task_definition="$(aws_cmd ecs describe-services \
      --cluster "$CLUSTER" \
      --services "$service" \
      --query 'services[0].taskDefinition' \
      --output text)"
    task_image="$(aws_cmd ecs describe-task-definition \
      --task-definition "$task_definition" \
      --query 'taskDefinition.containerDefinitions[0].image' \
      --output text)"
    if [ "$task_image" != "$immutable_image" ]; then
      echo "deployed task definition for $service uses $task_image, expected $immutable_image" >&2
      exit 70
    fi
    service_tasks="$(aws_cmd ecs list-tasks \
      --cluster "$CLUSTER" \
      --service-name "$service" \
      --desired-status RUNNING \
      --query 'taskArns[]' \
      --output text)"
    if [ -z "$service_tasks" ]; then
      echo "no running tasks found for $service after stable wait" >&2
      exit 71
    fi
    tasks_json="$(aws_cmd ecs describe-tasks --cluster "$CLUSTER" --tasks $service_tasks --output json)"
    python3 -c '
import json
import sys

service, expected = sys.argv[1:]
data = json.load(sys.stdin)
bad = []
for task in data.get("tasks", []):
    for container in task.get("containers", []):
        image = container.get("image")
        if image != expected:
            bad.append("{} container={} image={}".format(task.get("taskArn"), container.get("name"), image))
if bad:
    print(f"running tasks for {service} are not on expected image {expected}:", file=sys.stderr)
    for item in bad:
        print(f"- {item}", file=sys.stderr)
    sys.exit(1)
' "$service" "$immutable_image" <<< "$tasks_json"
  done
}

ecr_repo="$(terraform_cmd -chdir=infra/aws output -raw ecr_repository_url)"
image="${ecr_repo}:${IMAGE_TAG}"
registry="${ecr_repo%/*}"

echo "deploy target: $image"
echo "required platform: $PLATFORM"
echo "buildx builder: $BUILDX_BUILDER"
echo "immutable tag: $IMAGE_TAG"

acquire_deploy_lock
wait_for_services_stable "preflight"
verify_services_quiescent

retry 3 ecr_login

retry 2 docker buildx build \
  --builder "$BUILDX_BUILDER" \
  --platform "$PLATFORM" \
  --provenance=false \
  --sbom=false \
  -t "$image" \
  --push \
  .

remote_image="$(docker buildx imagetools inspect "$image" --format '{{json .Image}}')"
remote_platform="$(printf '%s' "$remote_image" | python3 -c 'import json,sys; data=json.load(sys.stdin); print("%s/%s" % (data.get("os", ""), data.get("architecture", "")))')"
if [ "$remote_platform" != "$PLATFORM" ]; then
  echo "refusing ECS update: remote image platform is $remote_platform, expected $PLATFORM" >&2
  exit 66
fi

echo "verified image platform: $remote_platform"

image_digest="$(aws_cmd ecr describe-images \
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

for service in $SERVICES; do
  current_task_definition="$(aws_cmd ecs describe-services \
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

  aws_cmd ecs describe-task-definition \
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

  new_task_definition="$(aws_cmd ecs register-task-definition \
    --cli-input-json "file://$register_input" \
    --query 'taskDefinition.taskDefinitionArn' \
    --output text)"

  echo "updating $service to $new_task_definition"
  aws_cmd ecs update-service \
    --cluster "$CLUSTER" \
    --service "$service" \
    --task-definition "$new_task_definition" >/dev/null
done

wait_for_services_stable "post-update"
verify_services_quiescent
verify_deployed_image

echo "deploy stable: $immutable_image ($PLATFORM)"
