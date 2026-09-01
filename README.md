# gooo-improvement-pairing-gate

An independent Gooo gate for self-improvement claims.

The gate accepts an improvement claim only when the before and after observations
form an exact identity match across `scenario_id`, `source_digest`,
`contract_digest`, `fixture_digest`, `toolchain_digest`, and `runner_identity`,
and both measurements are exact integers. It records the integer before value,
integer after value, integer delta, and direction. Missing or mismatched identity
evidence is `UNKNOWN`; regressions and contradictions are `REFUTED` and take
precedence over `UNKNOWN`.

The semantic contract and generated-artifact schema are authoritative in
`.gooo/pairing-gate.gooo`. Go provides only the evaluator, generator, and runtime.
The released graph binds eight activities exactly once and evaluates exactly nine
canonical cases: three `CLOSED`, four `UNKNOWN`, and two `REFUTED`.

## Run

```text
go run ./cmd/gooo-pairing-gate \
  --source . \
  --fixture testdata/canonical-fixtures.json \
  --metacode .gooo/pairing-gate.gooo \
  --output /tmp/gooo-improvement-pairing-gate-output
```

The caller-owned output directory contains exactly five files:

* `pairing-manifest.json`
* `pairing-events.ndjson`
* `comparison-receipt.json`
* `decision-receipt.json`
* `report.md`

The runtime never writes to the input source tree. The authoritative compile,
build, test, conformance, integration, and resource measurements run in GitHub
Actions. Failed runs are retained as CI artifacts and classified as operational
or semantic failures.

## Release process

Changes are introduced through a pull request into protected `main`. The
post-main workflow must pass before the release workflow creates an annotated
`v0.1.0` tag, creates a draft release, uploads the five-file bundle and its
checksum, publishes the release, and verifies the public release API response.
