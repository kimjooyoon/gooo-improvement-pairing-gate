#!/usr/bin/env bash
set -u

evidence=${1:?evidence directory is required}

stage_names=(compile build test conformance integration)
stage_json='[]'
overall="success"
failure_cause="none"
failed_stages='[]'
for stage in "${stage_names[@]}"; do
  metrics="$evidence/$stage.json"
  if [[ -f "$metrics" ]]; then
    stage_json=$(jq --arg stage "$stage" --slurpfile measurement "$metrics" '. + [{stage: $stage, wall_ms: $measurement[0].wall_ms, peak_rss_kib: $measurement[0].peak_rss_kib, exit_code: $measurement[0].exit_code}]' <<<"$stage_json")
    if [[ "$(jq -r '.exit_code' "$metrics")" != "0" ]]; then
      overall="failure"
      failed_stages=$(jq --arg stage "$stage" '. + [$stage]' <<<"$failed_stages")
    fi
  else
    overall="failure"
    failure_cause="operational"
    failed_stages=$(jq --arg stage "$stage" '. + [$stage]' <<<"$failed_stages")
  fi
done

if [[ "$overall" == "failure" && "$failure_cause" == "none" ]]; then
  failure_cause="operational"
  for stage in conformance integration test; do
    if [[ -f "$evidence/$stage.log" ]] && grep -Eq 'case .* evaluated as|status counts|canonical|UNKNOWN|REFUTED' "$evidence/$stage.log"; then
      failure_cause="semantic"
      break
    fi
  done
fi

test_total=0
test_selected=0
test_executed=0
test_reused=0
test_failed=0
test_unknown=0
test_results="$evidence/test-results.json"
if [[ -f "$test_results" ]]; then
  test_total=$(jq -s '[.[] | select(.Action == "run" and .Test != null)] | length' "$test_results" 2>/dev/null || echo 0)
  test_selected=$test_total
  test_executed=$(jq -s '[.[] | select((.Action == "pass" or .Action == "fail") and .Test != null)] | length' "$test_results" 2>/dev/null || echo 0)
  test_reused=0
  test_failed=$(jq -s '[.[] | select(.Action == "fail" and .Test != null)] | length' "$test_results" 2>/dev/null || echo 0)
fi

jq -n \
  --arg schema 'gooo/improvement-pairing-gate/github-actions-evidence/v1' \
  --arg authority 'GITHUB_ACTIONS' \
  --arg go_version '1.27' \
  --arg outcome "$overall" \
  --arg cause "$failure_cause" \
  --argjson stages "$stage_json" \
  --argjson failed_stages "$failed_stages" \
  --argjson tests_total "$test_total" \
  --argjson tests_selected "$test_selected" \
  --argjson tests_executed "$test_executed" \
  --argjson tests_reused "$test_reused" \
  --argjson tests_failed "$test_failed" \
  --argjson tests_unknown "$test_unknown" \
  '{schema: $schema, authority: $authority, go_version: $go_version, outcome: $outcome, failure_cause: $cause, failed_stages: $failed_stages, failed_run_preserved: true, stages: $stages, tests: {total: $tests_total, selected: $tests_selected, executed: $tests_executed, reused: $tests_reused, failed: $tests_failed, unknown: $tests_unknown}}' >"$evidence/ci-evidence.json"

exit 0
