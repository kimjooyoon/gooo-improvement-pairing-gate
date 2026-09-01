#!/usr/bin/env bash
set -euo pipefail

binary=${1:?binary is required}
output=${2:?output is required}

before_status=$(git status --porcelain=v1)
mkdir -p "$output"
"$binary" --source . --fixture testdata/canonical-fixtures.json --metacode .gooo/pairing-gate.gooo --output "$output"
after_status=$(git status --porcelain=v1)
test "$before_status" = "$after_status"
bash scripts/conformance.sh "$binary" "$output"
