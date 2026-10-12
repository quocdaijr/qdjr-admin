#!/usr/bin/env bash
# Dump the production database and upload it to a private R2 bucket.
#
# Run by .github/workflows/backup.yml every night; can also be run by hand.
# pg_dump comes from the postgres:18 image so no local Postgres install is
# needed and the client is never older than the server.
#
# Required env:
#   BACKUP_DATABASE_URL   Neon DIRECT connection string (no "-pooler")
#   R2_ACCOUNT_ID         Cloudflare account id
#   R2_BACKUP_BUCKET      private bucket, e.g. qdjr-backups (never the public media bucket)
#   AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY   R2 token scoped to that bucket
# Optional env:
#   BACKUP_KEEP           how many dumps to keep (default 30)
#   S3_ENDPOINT           override the R2 endpoint (local tests use MinIO)
#   PG_IMAGE              default postgres:18-alpine
#   PG_DOCKER_NETWORK     docker network for pg_dump (default host)
#   AWS                   aws CLI command (default "aws")
set -euo pipefail

: "${BACKUP_DATABASE_URL:?BACKUP_DATABASE_URL is required}"
: "${R2_BACKUP_BUCKET:?R2_BACKUP_BUCKET is required}"
: "${AWS_ACCESS_KEY_ID:?AWS_ACCESS_KEY_ID is required}"
: "${AWS_SECRET_ACCESS_KEY:?AWS_SECRET_ACCESS_KEY is required}"
if [ -z "${S3_ENDPOINT:-}" ]; then
  : "${R2_ACCOUNT_ID:?R2_ACCOUNT_ID is required}"
  S3_ENDPOINT="https://${R2_ACCOUNT_ID}.r2.cloudflarestorage.com"
fi
BACKUP_KEEP="${BACKUP_KEEP:-30}"
PG_IMAGE="${PG_IMAGE:-postgres:18-alpine}"
PG_DOCKER_NETWORK="${PG_DOCKER_NETWORK:-host}"
AWS="${AWS:-aws}"

# R2 does not implement the newer default CRC checksums of aws-cli v2.
export AWS_DEFAULT_REGION=auto
export AWS_REQUEST_CHECKSUM_CALCULATION=when_required
export AWS_RESPONSE_CHECKSUM_VALIDATION=when_required
s3() { $AWS --endpoint-url "$S3_ENDPOINT" "$@"; }

workdir="$(mktemp -d "${TMPDIR:-/tmp}/qdjr-backup.XXXXXX")"
trap 'rm -rf "$workdir"' EXIT
stamp="$(date -u +%Y-%m-%dT%H%M%SZ)"
file="$workdir/qdjr-$stamp.dump"
key="db/qdjr-$stamp.dump"

echo "dumping database..."
# Custom format: compressed, and pg_restore can restore selectively.
docker run --rm --network "$PG_DOCKER_NETWORK" -e PGCONNECT_TIMEOUT=60 "$PG_IMAGE" \
  pg_dump --format=custom --no-owner --no-privileges "$BACKUP_DATABASE_URL" > "$file"

echo "verifying dump..."
toc="$(docker run --rm -i "$PG_IMAGE" pg_restore --list < "$file")"
for table in posts users goose_db_version; do
  if ! grep -qE "TABLE (DATA )?public $table " <<<"$toc"; then
    echo "dump is missing table public.$table, refusing to upload" >&2
    exit 1
  fi
done
size="$(wc -c < "$file" | tr -d ' ')"
echo "dump ok ($size bytes)"

echo "uploading s3://$R2_BACKUP_BUCKET/$key ..."
# Streamed via stdin so the CLI needs no access to the file path.
s3 s3 cp - "s3://$R2_BACKUP_BUCKET/$key" --only-show-errors < "$file"

echo "pruning to the newest $BACKUP_KEEP dumps..."
# Keys sort chronologically because the timestamp is ISO-8601 UTC.
keys=()
while IFS= read -r k; do keys+=("$k"); done < <(s3 s3api list-objects-v2 --bucket "$R2_BACKUP_BUCKET" \
  --prefix db/ --query 'Contents[].Key' --output text | tr '\t' '\n' | grep -E '^db/qdjr-.*\.dump$' | sort)
excess=$(( ${#keys[@]} - BACKUP_KEEP ))
for (( i = 0; i < excess; i++ )); do
  s3 s3 rm "s3://$R2_BACKUP_BUCKET/${keys[$i]}" --only-show-errors
  echo "deleted ${keys[$i]}"
done
echo "done: $key (${#keys[@]} dumps before pruning, keep $BACKUP_KEEP)"
