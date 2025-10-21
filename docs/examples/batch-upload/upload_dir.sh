#!/usr/bin/env bash
set -euo pipefail

# Required env vars:
#   BASE_URL   e.g. http://localhost:8080
#   CLUSTER    e.g. demo
#   TOKEN      bearer token string
#   NAMESPACE  e.g. default
#   POD        e.g. nginx-abc123
#   CONTAINER  e.g. nginx
#   TARGET_DIR absolute directory path inside the container (e.g. /tmp)
#   LOCAL_DIR  local directory with files to upload

required=(BASE_URL CLUSTER TOKEN NAMESPACE POD CONTAINER TARGET_DIR LOCAL_DIR)
for v in "${required[@]}"; do
  if [[ -z "${!v:-}" ]]; then
    echo "Missing env var: $v" >&2
    exit 1
  fi
done

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required. Install jq and retry." >&2
  exit 1
fi

endpoint="$BASE_URL/k8s/cluster/$CLUSTER/file/upload"

success=0
failed=0
failed_list=()

shopt -s nullglob
files=("$LOCAL_DIR"/*)
count=${#files[@]}
if (( count == 0 )); then
  echo "No files in $LOCAL_DIR"
  exit 0
fi

echo "Starting upload of $count file(s) from $LOCAL_DIR to $TARGET_DIR"

for f in "${files[@]}"; do
  if [[ ! -f "$f" ]]; then
    continue
  fi
  fname="$(basename "$f")"
  pod_path="$TARGET_DIR/$fname"

  echo "Uploading: $fname -> $pod_path"

  resp="$(curl -sS -X POST \
    -H "Authorization: Bearer $TOKEN" \
    -F "containerName=$CONTAINER" \
    -F "namespace=$NAMESPACE" \
    -F "podName=$POD" \
    -F "path=$pod_path" \
    -F "fileName=$fname" \
    -F "file=@$f" \
    "$endpoint")" || true

  status="$(echo "$resp" | jq -r '.data.file.status // empty')"
  errorMsg="$(echo "$resp" | jq -r '.data.file.error // empty')"

  if [[ "$status" == "done" ]]; then
    echo "OK: $fname"
    ((success++))
  else
    echo "FAIL: $fname ${errorMsg:+- $errorMsg}"
    ((failed++))
    failed_list+=("$fname")
  fi
done

echo
echo "Summary: success=$success failed=$failed"
if (( failed > 0 )); then
  printf 'Failed files: %s\n' "${failed_list[@]}"
  exit 1
fi


