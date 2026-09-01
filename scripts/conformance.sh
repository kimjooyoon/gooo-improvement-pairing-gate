#!/usr/bin/env bash
set -euo pipefail

binary=${1:?binary is required}
output=${2:?output is required}

test -x "$binary"
test -d "$output"
test "$(find "$output" -maxdepth 1 -type f | wc -l | tr -d ' ')" = "5"
test "$(find "$output" -maxdepth 1 -type d | wc -l | tr -d ' ')" = "1"

for file in pairing-manifest.json pairing-events.ndjson comparison-receipt.json decision-receipt.json report.md; do
  test -f "$output/$file"
done

jq -e '
  .denominator == 9 and
  .identity_fields == ["scenario_id", "source_digest", "contract_digest", "fixture_digest", "toolchain_digest", "runner_identity"] and
  .input_repository_mutated == false and
  .product_repository_writes == 0 and
  (.output_files | length) == 5
' "$output/pairing-manifest.json" >/dev/null

jq -e '
  .denominator == 9 and
  .observed_status_counts.CLOSED == 3 and
  .observed_status_counts.UNKNOWN == 4 and
  .observed_status_counts.REFUTED == 2 and
  .language_utility_status == "UNKNOWN"
' "$output/decision-receipt.json" >/dev/null

jq -s -e '
  (map(select(.event == "activity_bound")) | length) == 8 and
  (map(select(.event == "activity_bound") | .activity) | unique | length) == 8 and
  (map(select(.event == "case_evaluated")) | length) == 9
' "$output/pairing-events.ndjson" >/dev/null
