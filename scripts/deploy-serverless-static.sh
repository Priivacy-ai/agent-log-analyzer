#!/usr/bin/env bash
set -euo pipefail

AWS_PROFILE="${AWS_PROFILE-claude-analyzer-prod}"
AWS_REGION="${AWS_REGION:-us-east-1}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

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

cd "$ROOT"
npm run build:web
mkdir -p web-dist/docs
rsync -a --delete docs/ web-dist/docs/

bucket="$(terraform_cmd -chdir=infra/aws output -raw static_bucket)"
cloudfront_domain="$(terraform_cmd -chdir=infra/aws output -raw cloudfront_domain_name)"
distribution_id="$(aws_cmd cloudfront list-distributions --query "DistributionList.Items[?DomainName=='${cloudfront_domain}'].Id | [0]" --output text)"

aws_cmd s3 sync web-dist "s3://$bucket" \
  --delete \
  --cache-control 'public,max-age=300'

if [ -n "$distribution_id" ] && [ "$distribution_id" != "None" ]; then
  aws_cmd cloudfront create-invalidation \
    --distribution-id "$distribution_id" \
    --paths '/*' >/dev/null
fi

printf 'deployed static site to s3://%s\n' "$bucket"
