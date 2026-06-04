#!/usr/bin/env bash
#
# check-readonly.sh — Constitution V / FR-019 structural guard.
#
# Fails if any S3 mutation reaches the codebase outside internal/storage/:
#   1. importing aws-sdk-go-v2/service/s3 outside internal/storage/ (only it may), or
#   2. referencing a mutating S3 operation symbol (Put*/Delete*/Create*/Copy*/Upload*
#      /Restore*/Write*) anywhere outside internal/storage/.
#
# Read-only is structural: the storage interface exposes no mutating method, and this
# guard backstops it in CI so a write API can never appear by accident.

set -euo pipefail

cd "$(dirname "$0")/.."

fail=0

# Go files outside internal/storage/, excluding test/vendor noise.
mapfile -t files < <(
  find . -name '*.go' \
    -not -path './internal/storage/*' \
    -not -path './vendor/*' \
    -not -path './.git/*' \
    | sort
)

if [ ${#files[@]} -eq 0 ]; then
  echo "check-readonly: no Go files outside internal/storage/ yet — OK"
  exit 0
fi

# 1. Forbidden import of the S3 service package.
if grep -nE '"github\.com/aws/aws-sdk-go-v2/service/s3' "${files[@]}" 2>/dev/null; then
  echo "ERROR: aws-sdk-go-v2/service/s3 imported outside internal/storage/ (Constitution V)" >&2
  fail=1
fi

# 2. Forbidden mutating S3 operation symbols.
#    A mutation verb directly fused to an S3 entity in ONE identifier — e.g.
#    PutObject, CreateBucket, DeleteObjects, CopyObject, UploadPart. This avoids
#    false positives like cache.Put(Key{Bucket: ...}) where the words are separate.
verbs='Put|Delete|Create|Copy|Upload|Restore|Write'
entities='Object|Bucket|MultipartUpload|Part|Tagging|Acl|Policy|Cors|Lifecycle|Replication|Encryption|Versioning|Website|Notification'
mutating="\\b(${verbs})(${entities})[A-Za-z]*\\b"
if grep -nE "$mutating" "${files[@]}" 2>/dev/null; then
  echo "ERROR: mutating S3 symbol found outside internal/storage/ (FR-019)" >&2
  fail=1
fi

if [ "$fail" -ne 0 ]; then
  echo "check-readonly: FAILED — read-only invariant violated" >&2
  exit 1
fi

echo "check-readonly: PASS — no S3 mutations outside internal/storage/"
