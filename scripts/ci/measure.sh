#!/usr/bin/env bash
set -u

metrics=${1:?metrics path is required}
log=${2:?log path is required}
shift 2

tmp=$(mktemp)
start_ms=$(date +%s%3N)
/usr/bin/time -f '%M' -o "$tmp" "$@" >"$log" 2>&1
status=$?
end_ms=$(date +%s%3N)
peak=$(awk 'NR == 1 {print $1}' "$tmp")
rm -f "$tmp"
if [[ -z "$peak" ]]; then
  peak=0
fi
wall_ms=$((end_ms - start_ms))
if (( wall_ms < 0 )); then
  wall_ms=0
fi
jq -n --argjson exit_code "$status" --argjson wall_ms "$wall_ms" --argjson peak_rss_kib "$peak" \
  '{exit_code: $exit_code, wall_ms: $wall_ms, peak_rss_kib: $peak_rss_kib}' >"$metrics"
exit "$status"
