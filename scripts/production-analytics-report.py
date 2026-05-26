#!/usr/bin/env python3
"""Generate production website analytics + non-owner email report.

Reads the production AWS backend directly:
- S3 report bucket: usage/events/ and analytics/events/
- DynamoDB jobs table: email_unlock, email_event, email_suppression records

The report intentionally masks email addresses. By default, all
@robshouse.net addresses are treated as owner addresses and excluded.
"""

from __future__ import annotations

import argparse
import collections
import concurrent.futures
import datetime as dt
import hashlib
import json
import re
import sys
from typing import Any

try:
    import boto3
    from boto3.dynamodb.conditions import Attr
except ImportError as exc:  # pragma: no cover - environment dependency check.
    print("boto3 is required: python3 -m pip install boto3", file=sys.stderr)
    raise SystemExit(2) from exc


DEFAULT_PROFILE = "claude-analyzer-prod"
DEFAULT_REGION = "us-east-1"
DEFAULT_BUCKET = "claude-analyzer-prod-reports-984d05ff"
DEFAULT_TABLE = "claude-analyzer-prod-jobs"
DEFAULT_OWN_EMAILS = {"robert@spec-kitty.ai"}
DEFAULT_OWN_DOMAINS = {"robshouse.net"}
SHA256_HEX = re.compile(r"^[0-9a-f]{64}$")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Generate a production website analytics and non-owner email report."
    )
    parser.add_argument("--profile", default=DEFAULT_PROFILE, help="AWS profile name.")
    parser.add_argument("--region", default=DEFAULT_REGION, help="AWS region.")
    parser.add_argument("--bucket", default=DEFAULT_BUCKET, help="Production report S3 bucket.")
    parser.add_argument("--table", default=DEFAULT_TABLE, help="Production DynamoDB job table.")
    parser.add_argument("--days", type=int, default=90, help="UTC lookback window.")
    parser.add_argument(
        "--own-email",
        action="append",
        default=sorted(DEFAULT_OWN_EMAILS),
        help="Exact owner email to exclude. Repeatable.",
    )
    parser.add_argument(
        "--own-domain",
        action="append",
        default=sorted(DEFAULT_OWN_DOMAINS),
        help="Owner email domain to exclude, without @. Repeatable.",
    )
    parser.add_argument("--previous-json", help="Previous JSON report used to compute new emails.")
    parser.add_argument("--new-email-since", help="UTC timestamp; emails first seen at or after it are new.")
    parser.add_argument("--show-emails", action="store_true", help="Show full non-owner emails in local output.")
    parser.add_argument("--json", action="store_true", help="Emit JSON instead of Markdown.")
    return parser.parse_args()


def utc_now() -> dt.datetime:
    return dt.datetime.now(dt.UTC)


def parse_time(raw: str | None) -> dt.datetime | None:
    if not raw:
        return None
    try:
        return dt.datetime.fromisoformat(raw.replace("Z", "+00:00"))
    except ValueError:
        return None


def iso_date(raw: str | None) -> str:
    parsed = parse_time(raw)
    if parsed is None:
        return "unknown"
    return parsed.astimezone(dt.UTC).date().isoformat()


def key_date(key: str) -> str | None:
    match = re.search(r"date=(\d{4}-\d{2}-\d{2})", key)
    if match:
        return match.group(1)
    return None


def in_window(timestamp: str | None, since: dt.datetime) -> bool:
    parsed = parse_time(timestamp)
    if parsed is None:
        return True
    return parsed.astimezone(dt.UTC) >= since


def key_in_window(key: str, since_date: dt.date) -> bool:
    found = key_date(key)
    if found is None:
        return True
    return dt.date.fromisoformat(found) >= since_date


def list_keys(s3: Any, bucket: str, prefix: str, since_date: dt.date) -> list[str]:
    keys: list[str] = []
    paginator = s3.get_paginator("list_objects_v2")
    for page in paginator.paginate(Bucket=bucket, Prefix=prefix):
        for obj in page.get("Contents", []):
            key = obj["Key"]
            if key_in_window(key, since_date):
                keys.append(key)
    return keys


def read_jsonl_object(s3: Any, bucket: str, key: str) -> list[dict[str, Any]]:
    obj = s3.get_object(Bucket=bucket, Key=key)
    body = obj["Body"].read().decode("utf-8").strip()
    rows: list[dict[str, Any]] = []
    object_date = key_date(key)
    for line in body.splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            row = json.loads(line)
        except json.JSONDecodeError:
            continue
        if object_date:
            row["_object_date"] = object_date
        rows.append(row)
    return rows


def load_jsonl_prefix(s3: Any, bucket: str, prefix: str, since_date: dt.date) -> tuple[list[dict[str, Any]], collections.Counter[str]]:
    keys = list_keys(s3, bucket, prefix, since_date)
    events: list[dict[str, Any]] = []
    daily = collections.Counter(key_date(key) or "unknown" for key in keys)
    with concurrent.futures.ThreadPoolExecutor(max_workers=24) as pool:
        futures = [pool.submit(read_jsonl_object, s3, bucket, key) for key in keys]
        for future in concurrent.futures.as_completed(futures):
            events.extend(future.result())
    return events, daily


def scan_record_type(table: Any, record_type: str) -> list[dict[str, Any]]:
    items: list[dict[str, Any]] = []
    kwargs = {"FilterExpression": Attr("record_type").eq(record_type)}
    while True:
        response = table.scan(**kwargs)
        items.extend(response.get("Items", []))
        if "LastEvaluatedKey" not in response:
            return items
        kwargs["ExclusiveStartKey"] = response["LastEvaluatedKey"]


def normalize_email(email: str | None) -> str:
    return (email or "").strip().lower()


def hash_email(email: str) -> str:
    return hashlib.sha256(normalize_email(email).encode("utf-8")).hexdigest()


def is_owner_email(email: str, own_emails: set[str], own_domains: set[str]) -> bool:
    email = normalize_email(email)
    if email in own_emails:
        return True
    if "@" not in email:
        return False
    domain = email.rsplit("@", 1)[1]
    return domain in own_domains


def mask_email(email: str) -> str:
    email = normalize_email(email)
    if "@" not in email:
        return email or "unknown"
    local, domain = email.split("@", 1)
    if len(local) <= 1:
        masked = "*"
    elif len(local) == 2:
        masked = f"{local[0]}*"
    else:
        masked = f"{local[0]}***{local[-1]}"
    return f"{masked}@{domain}"


def email_label(email: str, show_emails: bool) -> str:
    email = normalize_email(email)
    if show_emails:
        return email
    return mask_email(email)


def counter_top(counter: collections.Counter[str], limit: int = 10) -> list[tuple[str, int]]:
    return counter.most_common(limit)


def summarize_delivery_for_hash(
    email_hash: str,
    events_by_hash: dict[str, list[dict[str, Any]]],
    suppressions_by_hash: dict[str, dict[str, Any]],
) -> dict[str, Any]:
    events = events_by_hash.get(email_hash, [])
    types = collections.Counter(item.get("type") or "unknown" for item in events)
    details = collections.Counter(item.get("detail") or "" for item in events if item.get("detail"))
    latest = None
    latest_type = None
    latest_detail = None
    for event in events:
        created_at = parse_time(event.get("created_at"))
        if created_at is None:
            continue
        if latest is None or created_at >= latest:
            latest = created_at
            latest_type = event.get("type") or "unknown"
            latest_detail = event.get("detail") or ""

    suppression = suppressions_by_hash.get(email_hash)
    if suppression:
        status = "provider_rejected" if suppression.get("reason") == "reject" else "suppressed"
        detail = suppression.get("reason") or latest_detail or ""
        usable = False
    elif any(types.get(kind, 0) for kind in ("bounce", "complaint", "reject")):
        status = "provider_rejected"
        detail = latest_detail or next(iter(details), "")
        usable = False
    elif types.get("send_failure", 0):
        detail = latest_detail or next(iter(details), "")
        rejected_details = ("suppressed_recipient", "recipient_rejected", "message_rejected")
        status = "provider_rejected" if any(part in detail for part in rejected_details) else "delivery_failed"
        usable = False
    elif types.get("rendering_failure", 0):
        status = "delivery_failed"
        detail = latest_detail or "rendering_failure"
        usable = False
    elif types.get("delivery", 0):
        status = "delivered"
        detail = latest_detail or ""
        usable = True
    elif types.get("send", 0):
        status = "sent"
        detail = latest_detail or ""
        usable = True
    else:
        status = "no_delivery_signal"
        detail = ""
        usable = True

    return {
        "delivery_status": status,
        "delivery_usable": usable,
        "delivery_detail": detail,
        "latest_delivery_event_type": latest_type,
        "latest_delivery_event_at": latest.replace(microsecond=0).isoformat() if latest else None,
        "delivery_event_counts": counter_top(types, 8),
    }


def summarize_usage(events: list[dict[str, Any]]) -> dict[str, Any]:
    request_status = collections.Counter()
    by_day = collections.Counter()
    by_path = collections.Counter()
    by_status = collections.Counter()
    by_browser = collections.Counter()
    by_os = collections.Counter()
    by_device = collections.Counter()
    by_language = collections.Counter()
    by_region = collections.Counter()
    by_referrer = collections.Counter()
    by_utm_source = collections.Counter()
    by_auth_surface = collections.Counter()
    client_hashes: set[str] = set()
    bot_requests = 0

    for event in events:
        by_day[iso_date(event.get("timestamp"))] += 1
        by_path[event.get("path") or "unknown"] += 1
        status = int(event.get("status") or 0)
        by_status[str(status or "unknown")] += 1
        if status >= 500:
            request_status["server_error"] += 1
        elif status >= 400:
            request_status["client_error"] += 1
        elif status >= 300:
            request_status["redirect"] += 1
        else:
            request_status["success"] += 1
        by_browser[event.get("browser") or "unknown"] += 1
        by_os[event.get("operating_system") or "unknown"] += 1
        by_device[event.get("device_class") or "unknown"] += 1
        by_language[event.get("language") or "unknown"] += 1
        by_region[event.get("region") or "unknown"] += 1
        by_referrer[event.get("referrer_host") or "unknown"] += 1
        by_auth_surface[event.get("auth_surface") or "unknown"] += 1
        if event.get("client_hash"):
            client_hashes.add(event["client_hash"])
        if event.get("bot"):
            bot_requests += 1
        utm = event.get("utm") or {}
        if utm.get("utm_source"):
            by_utm_source[utm["utm_source"]] += 1

    return {
        "total": len(events),
        "success": request_status["success"],
        "redirect": request_status["redirect"],
        "client_error": request_status["client_error"],
        "server_error": request_status["server_error"],
        "unique_client_hashes": len(client_hashes),
        "bot_requests": bot_requests,
        "daily": dict(sorted(by_day.items())),
        "top_paths": counter_top(by_path, 12),
        "statuses": counter_top(by_status, 8),
        "auth_surfaces": counter_top(by_auth_surface, 8),
        "top_referrers": [(k, v) for k, v in counter_top(by_referrer, 10) if k != "unknown"],
        "utm_sources": counter_top(by_utm_source, 10),
        "browsers": counter_top(by_browser, 8),
        "operating_systems": counter_top(by_os, 8),
        "devices": counter_top(by_device, 8),
        "languages": counter_top(by_language, 8),
        "regions": counter_top(by_region, 8),
    }


def product_event_hashes(event: dict[str, Any]) -> list[str]:
    hashes: list[str] = []
    seen: set[str] = set()
    for item in event.get("analyzed_log_hashes") or []:
        if not isinstance(item, dict):
            continue
        value = (item.get("content_hash_sha256") or "").strip().lower()
        if not SHA256_HEX.match(value) or value in seen:
            continue
        seen.add(value)
        hashes.append(value)
    return hashes


def increment_product_counters(event: dict[str, Any], weight: int, counters: dict[str, collections.Counter[str]]) -> None:
    counters["scan_types"][event.get("scan_type") or "unknown"] += weight
    ecosystem = event.get("ecosystem") or {}
    counters["clients"][ecosystem.get("client") or "unknown"] += weight
    counters["score_buckets"][event.get("score_bucket") or "unknown"] += weight
    counters["waste_buckets"][event.get("waste_bucket") or "unknown"] += weight
    tooling = ecosystem.get("tooling_utilization") or {}
    counters["mcp_warning_bands"][(tooling.get("mcp") or {}).get("warning_band") or "unknown"] += weight
    counters["skill_warning_bands"][(tooling.get("skill") or {}).get("warning_band") or "unknown"] += weight
    for fingerprint in ecosystem.get("workflow_fingerprints") or []:
        if fingerprint.get("id"):
            counters["sdd_tools"][fingerprint["id"]] += weight
    recommendation = event.get("recommendation") or {}
    for slot_name in ("primary", "secondary"):
        slot = recommendation.get(slot_name) or {}
        if slot.get("class"):
            counters["recommendation_classes"][slot["class"]] += weight
        if slot.get("tool_id"):
            counters["recommendation_tools"][slot["tool_id"]] += weight


def summarize_product(events: list[dict[str, Any]], daily_from_keys: collections.Counter[str]) -> dict[str, Any]:
    scan_types = collections.Counter()
    clients = collections.Counter()
    score_buckets = collections.Counter()
    waste_buckets = collections.Counter()
    mcp_warning_bands = collections.Counter()
    skill_warning_bands = collections.Counter()
    sdd_tools = collections.Counter()
    recommendation_classes = collections.Counter()
    recommendation_tools = collections.Counter()
    counters = {
        "scan_types": scan_types,
        "clients": clients,
        "score_buckets": score_buckets,
        "waste_buckets": waste_buckets,
        "mcp_warning_bands": mcp_warning_bands,
        "skill_warning_bands": skill_warning_bands,
        "sdd_tools": sdd_tools,
        "recommendation_classes": recommendation_classes,
        "recommendation_tools": recommendation_tools,
    }
    daily = collections.Counter()
    seen_hashes: set[str] = set()
    hash_refs = 0
    duplicate_hash_refs = 0
    events_with_hashes = 0
    events_without_hashes = 0
    deduped_count = 0

    for event in events:
        object_day = event.get("_object_date") or "unknown"
        hashes = product_event_hashes(event)
        if not hashes:
            events_without_hashes += 1
            deduped_count += 1
            daily[object_day] += 1
            increment_product_counters(event, 1, counters)
            continue

        events_with_hashes += 1
        new_hash_count = 0
        for content_hash in hashes:
            hash_refs += 1
            if content_hash in seen_hashes:
                duplicate_hash_refs += 1
                continue
            seen_hashes.add(content_hash)
            new_hash_count += 1
        if new_hash_count == 0:
            continue
        deduped_count += new_hash_count
        daily[object_day] += new_hash_count
        increment_product_counters(event, new_hash_count, counters)

    daily_counts = daily if daily else daily_from_keys
    return {
        "total_report_events": deduped_count,
        "raw_report_events": len(events),
        "events_with_hashes": events_with_hashes,
        "events_without_hashes": events_without_hashes,
        "analyzed_log_hash_refs": hash_refs,
        "unique_analyzed_log_hashes": len(seen_hashes),
        "duplicate_analyzed_log_hash_refs": duplicate_hash_refs,
        "dedupe_mode": "content_hash_sha256_with_legacy_event_fallback",
        "daily": dict(sorted(daily_counts.items())),
        "scan_types": counter_top(scan_types, 8),
        "clients": counter_top(clients, 8),
        "score_buckets": counter_top(score_buckets, 8),
        "waste_buckets": counter_top(waste_buckets, 8),
        "mcp_warning_bands": counter_top(mcp_warning_bands, 8),
        "skill_warning_bands": counter_top(skill_warning_bands, 8),
        "sdd_tools": counter_top(sdd_tools, 10),
        "recommendation_classes": counter_top(recommendation_classes, 10),
        "recommendation_tools": counter_top(recommendation_tools, 10),
    }


def summarize_emails(
    unlocks: list[dict[str, Any]],
    email_events: list[dict[str, Any]],
    suppressions: list[dict[str, Any]],
    since: dt.datetime,
    own_emails: set[str],
    own_domains: set[str],
    show_emails: bool,
    new_email_since: dt.datetime | None,
    previous_email_labels: set[str],
) -> dict[str, Any]:
    recent_unlocks = [item for item in unlocks if in_window(item.get("created_at"), since)]
    non_owner: list[dict[str, Any]] = []
    owner: list[dict[str, Any]] = []
    for item in recent_unlocks:
        email = normalize_email(item.get("email"))
        if is_owner_email(email, own_emails, own_domains):
            owner.append(item)
        else:
            non_owner.append(item)

    by_day = collections.Counter(iso_date(item.get("created_at")) for item in non_owner)
    status = collections.Counter(item.get("status") or "unknown" for item in non_owner)
    marketing = collections.Counter(
        "marketing_opt_in" if item.get("marketing_opt_in") else "no_marketing_opt_in"
        for item in non_owner
    )
    domains = collections.Counter()
    unique_emails: list[str] = []
    seen: set[str] = set()
    by_email: dict[str, dict[str, Any]] = {}
    for item in non_owner:
        email = normalize_email(item.get("email"))
        if "@" in email:
            domains[email.rsplit("@", 1)[1]] += 1
        if email and email not in seen:
            seen.add(email)
            unique_emails.append(email)
        if email:
            created_at = parse_time(item.get("created_at"))
            current = by_email.setdefault(
                email,
                {
                    "marketing_opt_in": False,
                    "first_seen_at": created_at,
                    "latest_seen_at": created_at,
                    "status": item.get("status") or "unknown",
                },
            )
            current["marketing_opt_in"] = current["marketing_opt_in"] or bool(item.get("marketing_opt_in"))
            if created_at is not None:
                if current["first_seen_at"] is None or created_at < current["first_seen_at"]:
                    current["first_seen_at"] = created_at
                if current["latest_seen_at"] is None or created_at >= current["latest_seen_at"]:
                    current["latest_seen_at"] = created_at
                    current["status"] = item.get("status") or "unknown"

    recent_events = [item for item in email_events if in_window(item.get("created_at"), since)]
    delivery_types = collections.Counter(item.get("type") or "unknown" for item in recent_events)
    delivery_daily = collections.Counter(iso_date(item.get("created_at")) for item in recent_events)
    events_by_hash: dict[str, list[dict[str, Any]]] = collections.defaultdict(list)
    for event in recent_events:
        if event.get("email_hash"):
            events_by_hash[event["email_hash"]].append(event)

    recent_suppressions = [
        item
        for item in suppressions
        if in_window(item.get("updated_at") or item.get("suppressed_at"), since)
    ]
    suppressions_by_hash = {
        item["email_hash"]: item for item in recent_suppressions if item.get("email_hash")
    }

    unique_email_marketing = []
    new_email_marketing = []
    new_usable_email_marketing = []
    new_rejected_email_marketing = []
    previous_email_marketing = []
    new_email_daily = collections.Counter()
    new_usable_email_daily = collections.Counter()
    new_rejected_email_daily = collections.Counter()
    delivery_statuses = collections.Counter()
    for email in sorted(unique_emails):
        item = by_email[email]
        first_seen_at = item["first_seen_at"]
        delivery = summarize_delivery_for_hash(hash_email(email), events_by_hash, suppressions_by_hash)
        delivery_statuses[delivery["delivery_status"]] += 1
        row = {
            "email": email_label(email, show_emails),
            "marketing_opt_in": item["marketing_opt_in"],
            "status": item["status"],
            "first_seen_at": first_seen_at.replace(microsecond=0).isoformat() if first_seen_at else None,
            **delivery,
        }
        unique_email_marketing.append(row)
        was_in_previous = bool(
            previous_email_labels
            and (email in previous_email_labels or mask_email(email) in previous_email_labels)
        )
        if was_in_previous:
            previous_email_marketing.append(row)
        is_new = bool(previous_email_labels and not was_in_previous)
        if new_email_since is not None and first_seen_at is not None and first_seen_at >= new_email_since:
            is_new = True
        if is_new:
            new_email_marketing.append(row)
            if first_seen_at is not None:
                new_email_daily[iso_date(first_seen_at.isoformat())] += 1
                if delivery["delivery_usable"]:
                    new_usable_email_daily[iso_date(first_seen_at.isoformat())] += 1
                else:
                    new_rejected_email_daily[iso_date(first_seen_at.isoformat())] += 1
            if delivery["delivery_usable"]:
                new_usable_email_marketing.append(row)
            else:
                new_rejected_email_marketing.append(row)

    new_basis = []
    if previous_email_labels:
        new_basis.append("previous_json")
    if new_email_since is not None:
        new_basis.append(new_email_since.replace(microsecond=0).isoformat())

    return {
        "total_email_unlock_records": len(recent_unlocks),
        "non_owner_records": len(non_owner),
        "owner_records_excluded": len(owner),
        "unique_non_owner_emails": len(seen),
        "daily": dict(sorted(by_day.items())),
        "status_counts": counter_top(status, 8),
        "marketing_counts": counter_top(marketing, 4),
        "top_domains": counter_top(domains, 12),
        "delivery_status_counts": counter_top(delivery_statuses, 8),
        "masked_unique_emails": [mask_email(email) for email in sorted(unique_emails)],
        "masked_unique_email_marketing": [
            {
                "email": mask_email(email),
                "marketing_opt_in": by_email[email]["marketing_opt_in"],
            }
            for email in sorted(unique_emails)
        ],
        "unique_email_marketing": unique_email_marketing,
        "new_email_basis": new_basis,
        "new_unique_email_marketing": new_email_marketing,
        "new_unique_email_count": len(new_email_marketing),
        "new_unique_email_daily": dict(sorted(new_email_daily.items())),
        "new_usable_email_marketing": new_usable_email_marketing,
        "new_usable_email_count": len(new_usable_email_marketing),
        "new_usable_email_daily": dict(sorted(new_usable_email_daily.items())),
        "new_rejected_email_marketing": new_rejected_email_marketing,
        "new_rejected_email_count": len(new_rejected_email_marketing),
        "new_rejected_email_daily": dict(sorted(new_rejected_email_daily.items())),
        "previous_unique_email_marketing": previous_email_marketing,
        "delivery_hashed": {
            "events": len(recent_events),
            "daily": dict(sorted(delivery_daily.items())),
            "types": counter_top(delivery_types, 8),
            "suppressions": len(recent_suppressions),
        },
    }


def previous_email_labels(path: str | None) -> set[str]:
    if not path:
        return set()
    with open(path, "r", encoding="utf-8") as handle:
        report = json.load(handle)
    emails = report.get("emails_not_owner") or {}
    labels = set()
    for key in ("unique_email_marketing", "masked_unique_email_marketing"):
        for item in emails.get(key) or []:
            if item.get("email"):
                labels.add(normalize_email(item["email"]))
    for email in emails.get("masked_unique_emails") or []:
        labels.add(normalize_email(email))
    return labels


def build_report(args: argparse.Namespace) -> dict[str, Any]:
    now = utc_now()
    since = now - dt.timedelta(days=args.days)
    since_date = since.date()
    own_emails = {normalize_email(email) for email in args.own_email}
    own_domains = {domain.strip().lower().lstrip("@") for domain in args.own_domain}
    email_since = parse_time(args.new_email_since)
    if args.new_email_since and email_since is None:
        raise SystemExit(f"invalid --new-email-since timestamp: {args.new_email_since}")
    previous_labels = previous_email_labels(args.previous_json)

    session = boto3.Session(profile_name=args.profile, region_name=args.region)
    s3 = session.client("s3")
    table = session.resource("dynamodb").Table(args.table)

    usage_events, _usage_daily_from_keys = load_jsonl_prefix(
        s3, args.bucket, "usage/events/", since_date
    )
    usage_events = [
        event for event in usage_events if in_window(event.get("timestamp"), since)
    ]
    product_events, product_daily_from_keys = load_jsonl_prefix(
        s3, args.bucket, "analytics/events/", since_date
    )
    unlocks = scan_record_type(table, "email_unlock")
    email_events = scan_record_type(table, "email_event")
    suppressions = scan_record_type(table, "email_suppression")

    usage = summarize_usage(usage_events)
    product = summarize_product(product_events, product_daily_from_keys)
    emails = summarize_emails(
        unlocks,
        email_events,
        suppressions,
        since,
        own_emails,
        own_domains,
        args.show_emails,
        email_since,
        previous_labels,
    )

    all_days = sorted(
        set(usage["daily"])
        | set(product["daily"])
        | set(emails["daily"])
        | set(emails["delivery_hashed"]["daily"])
        | set(emails["new_unique_email_daily"])
        | set(emails["new_usable_email_daily"])
        | set(emails["new_rejected_email_daily"])
    )
    daily = [
        {
            "date": day,
            "new_non_owner_emails": emails["new_unique_email_daily"].get(day, 0),
            "new_usable_non_owner_emails": emails["new_usable_email_daily"].get(day, 0),
            "new_rejected_non_owner_emails": emails["new_rejected_email_daily"].get(day, 0),
            "website_requests": usage["daily"].get(day, 0),
            "product_analytics_events": product["daily"].get(day, 0),
            "non_owner_email_unlocks": emails["daily"].get(day, 0),
            "hashed_delivery_events": emails["delivery_hashed"]["daily"].get(day, 0),
        }
        for day in all_days
    ]

    return {
        "generated_at_utc": now.replace(microsecond=0).isoformat(),
        "since_utc": since.replace(microsecond=0).isoformat(),
        "source": {
            "profile": args.profile,
            "region": args.region,
            "bucket": args.bucket,
            "table": args.table,
        },
        "owner_filter": {
            "emails": sorted(own_emails),
            "domains": sorted(own_domains),
        },
        "coverage": {
            "usage_first_day": min(usage["daily"]) if usage["daily"] else None,
            "usage_last_day": max(usage["daily"]) if usage["daily"] else None,
            "usage_events": usage["total"],
            "product_analytics_first_day": min(product["daily"]) if product["daily"] else None,
            "product_analytics_last_day": max(product["daily"]) if product["daily"] else None,
            "product_analytics_events": product["total_report_events"],
            "product_analytics_raw_events": product["raw_report_events"],
            "product_analytics_events_with_hashes": product["events_with_hashes"],
            "product_analytics_events_without_hashes": product["events_without_hashes"],
            "product_analytics_unique_file_hashes": product["unique_analyzed_log_hashes"],
            "product_analytics_duplicate_file_hash_refs": product["duplicate_analyzed_log_hash_refs"],
            "email_unlock_records": emails["total_email_unlock_records"],
            "non_owner_email_unlock_records": emails["non_owner_records"],
            "new_non_owner_emails": emails["new_unique_email_count"],
            "new_usable_non_owner_emails": emails["new_usable_email_count"],
            "new_rejected_non_owner_emails": emails["new_rejected_email_count"],
            "owner_email_unlock_records_excluded": emails["owner_records_excluded"],
            "email_delivery_events_hashed": emails["delivery_hashed"]["events"],
            "email_suppressions_hashed": emails["delivery_hashed"]["suppressions"],
        },
        "top_line_metrics": {
            "new_usable_non_owner_emails": emails["new_usable_email_count"],
            "new_rejected_non_owner_emails": emails["new_rejected_email_count"],
            "new_non_owner_email_submissions": emails["new_unique_email_count"],
            "new_email_basis": emails["new_email_basis"],
            "new_usable_emails": emails["new_usable_email_marketing"],
            "new_rejected_emails": emails["new_rejected_email_marketing"],
            "new_email_submissions": emails["new_unique_email_marketing"],
        },
        "website_requests": usage,
        "product_analytics": product,
        "emails_not_owner": emails,
        "time_scale_daily": daily,
    }


def markdown_pairs(rows: list[tuple[str, int]]) -> str:
    if not rows:
        return "_none_"
    return ", ".join(f"`{key}` {value}" for key, value in rows)


def markdown_email_marketing(rows: list[dict[str, Any]]) -> str:
    if not rows:
        return "_none_"
    rendered = []
    for row in rows:
        opt_in = "yes" if row.get("marketing_opt_in") else "no"
        status = row.get("status")
        first_seen = row.get("first_seen_at")
        suffix = f" {opt_in}"
        if status:
            suffix += f", {status}"
        if first_seen:
            suffix += f", first seen {first_seen}"
        delivery_status = row.get("delivery_status")
        if delivery_status:
            suffix += f", delivery {delivery_status}"
        delivery_detail = row.get("delivery_detail")
        if delivery_detail:
            suffix += f" ({delivery_detail})"
        rendered.append(f"`{row['email']}`{suffix}")
    return ", ".join(rendered)


def markdown_new_email_basis(basis: list[str]) -> str:
    if not basis:
        return "no previous report or new-email timestamp supplied"
    return ", ".join(f"`{item}`" for item in basis)


def render_markdown(report: dict[str, Any]) -> str:
    coverage = report["coverage"]
    website = report["website_requests"]
    product = report["product_analytics"]
    emails = report["emails_not_owner"]
    owner_filter = report["owner_filter"]

    lines = [
        "# Production Website Analytics Report",
        "",
        f"Generated UTC: `{report['generated_at_utc']}`",
        f"Window start UTC: `{report['since_utc']}`",
        f"Source: S3 `{report['source']['bucket']}` + DynamoDB `{report['source']['table']}`",
        "",
        "Owner email filter:",
        f"- exact emails: {', '.join(f'`{email}`' for email in owner_filter['emails']) or '_none_'}",
        f"- domains: {', '.join(f'`@{domain}`' for domain in owner_filter['domains']) or '_none_'}",
        "",
        "## New Emails",
        "",
        f"- New usable emails: `{emails['new_usable_email_count']:,}`.",
        f"- New rejected/failed submissions: `{emails['new_rejected_email_count']:,}`.",
        f"- New email submissions: `{emails['new_unique_email_count']:,}`.",
        f"- Basis: {markdown_new_email_basis(emails['new_email_basis'])}.",
        f"- Usable emails: {markdown_email_marketing(emails['new_usable_email_marketing'])}.",
        f"- Rejected/failed emails: {markdown_email_marketing(emails['new_rejected_email_marketing'])}.",
        "",
        "## Daily Time Scale",
        "",
        "| Date | New usable emails | New rejected emails | Website requests | Product analytics deduped | Non-owner email unlocks | Hashed email delivery events |",
        "|---|---:|---:|---:|---:|---:|---:|",
    ]
    for row in report["time_scale_daily"]:
        lines.append(
            "| {date} | {new_usable_non_owner_emails:,} | {new_rejected_non_owner_emails:,} | {website_requests:,} | {product_analytics_events:,} | {non_owner_email_unlocks:,} | {hashed_delivery_events:,} |".format(
                **row
            )
        )

    lines.extend(
        [
            "",
            "## Coverage",
            "",
            f"- New usable emails: `{coverage['new_usable_non_owner_emails']:,}`; rejected/failed new submissions: `{coverage['new_rejected_non_owner_emails']:,}`.",
            f"- Website usage: `{coverage['usage_events']:,}` events from `{coverage['usage_first_day']}` to `{coverage['usage_last_day']}`.",
            f"- Product analytics: `{coverage['product_analytics_events']:,}` deduped analyzed files/reports from `{coverage['product_analytics_first_day']}` to `{coverage['product_analytics_last_day']}`.",
            f"- Product analytics raw: `{coverage['product_analytics_raw_events']:,}` report events; `{coverage['product_analytics_unique_file_hashes']:,}` unique file hashes; `{coverage['product_analytics_duplicate_file_hash_refs']:,}` duplicate file-hash refs; `{coverage['product_analytics_events_without_hashes']:,}` legacy events without hashes.",
            f"- Email unlocks: `{coverage['email_unlock_records']:,}` records; `{coverage['owner_email_unlock_records_excluded']:,}` owner records excluded; `{coverage['non_owner_email_unlock_records']:,}` non-owner records.",
            f"- Hashed delivery telemetry: `{coverage['email_delivery_events_hashed']:,}` events; `{coverage['email_suppressions_hashed']:,}` suppressions.",
            "",
            "## Website Requests",
            "",
            f"- Total: `{website['total']:,}`; success `{website['success']:,}`, redirects `{website['redirect']:,}`, client errors `{website['client_error']:,}`, server errors `{website['server_error']:,}`.",
            f"- Unique client hashes: `{website['unique_client_hashes']:,}`; bot requests: `{website['bot_requests']:,}`.",
            f"- Top paths: {markdown_pairs(website['top_paths'])}.",
            f"- Referrers: {markdown_pairs(website['top_referrers'])}.",
            f"- Browsers: {markdown_pairs(website['browsers'])}.",
            f"- Regions: {markdown_pairs(website['regions'])}.",
            "",
            "## Product Analytics",
            "",
            f"- Deduped analyzed files/reports: `{product['total_report_events']:,}`.",
            f"- Raw report events: `{product['raw_report_events']:,}`.",
            f"- File-hash dedupe: `{product['unique_analyzed_log_hashes']:,}` unique hashes, `{product['duplicate_analyzed_log_hash_refs']:,}` duplicate refs, `{product['events_with_hashes']:,}` events with hashes, `{product['events_without_hashes']:,}` legacy events without hashes.",
            f"- Scan types: {markdown_pairs(product['scan_types'])}.",
            f"- Clients: {markdown_pairs(product['clients'])}.",
            f"- SDD tools: {markdown_pairs(product['sdd_tools'])}.",
            f"- Recommendation classes: {markdown_pairs(product['recommendation_classes'])}.",
            f"- Recommendation tools: {markdown_pairs(product['recommendation_tools'])}.",
            "",
            "## Emails Not Owner",
            "",
            f"- New usable emails: `{emails['new_usable_email_count']:,}`; {markdown_email_marketing(emails['new_usable_email_marketing'])}.",
            f"- New rejected/failed submissions: `{emails['new_rejected_email_count']:,}`; {markdown_email_marketing(emails['new_rejected_email_marketing'])}.",
            f"- Records: `{emails['non_owner_records']:,}`; unique emails: `{emails['unique_non_owner_emails']:,}`.",
            f"- Status: {markdown_pairs(emails['status_counts'])}.",
            f"- Marketing: {markdown_pairs(emails['marketing_counts'])}.",
            f"- Delivery status: {markdown_pairs(emails['delivery_status_counts'])}.",
            f"- Domains: {markdown_pairs(emails['top_domains'])}.",
            f"- Masked emails: {', '.join(f'`{email}`' for email in emails['masked_unique_emails']) or '_none_'}.",
            f"- Unique emails with marketing opt-in: {markdown_email_marketing(emails['unique_email_marketing'])}.",
            f"- Emails already present in previous report: {markdown_email_marketing(emails['previous_unique_email_marketing'])}.",
            "",
            "## Hashed Email Delivery",
            "",
            f"- Events: `{emails['delivery_hashed']['events']:,}`.",
            f"- Types: {markdown_pairs(emails['delivery_hashed']['types'])}.",
            f"- Suppressions: `{emails['delivery_hashed']['suppressions']:,}`.",
        ]
    )
    return "\n".join(lines) + "\n"


def main() -> int:
    args = parse_args()
    report = build_report(args)
    if args.json:
        print(json.dumps(report, indent=2))
    else:
        print(render_markdown(report), end="")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
