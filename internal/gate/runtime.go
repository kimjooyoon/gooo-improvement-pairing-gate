package gate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Options struct {
	Source   string
	Fixture  string
	Metacode string
	Output   string
}

func Run(options Options) error {
	if options.Source == "" || options.Fixture == "" || options.Metacode == "" || options.Output == "" {
		return fmt.Errorf("source, fixture, metacode, and output are required")
	}
	source, err := filepath.Abs(options.Source)
	if err != nil {
		return err
	}
	output, err := filepath.Abs(options.Output)
	if err != nil {
		return err
	}
	if source == output || isWithin(source, output) || isWithin(output, source) {
		return fmt.Errorf("caller-owned output must be separate from the input source")
	}
	meta, err := LoadMetaCode(options.Metacode)
	if err != nil {
		return err
	}
	fixture, err := loadFixture(options.Fixture)
	if err != nil {
		return err
	}
	if err := validateFixture(meta, fixture); err != nil {
		return err
	}
	inventory, err := inventorySource(source, meta.ExcludedPaths)
	if err != nil {
		return err
	}
	inputs := make(map[string]CaseInput, len(fixture.Cases))
	for _, input := range fixture.Cases {
		inputs[input.ID] = input
	}

	// The denominator is captured once before evaluation and is never derived from
	// an observed status count during this run.
	denominator := len(meta.Cases)
	comparisons := make([]Comparison, 0, denominator)
	decisions := make([]CaseDecision, 0, denominator)
	observed := map[string]int{"CLOSED": 0, "UNKNOWN": 0, "REFUTED": 0}
	for _, canonical := range meta.Cases {
		comparison, decision, evalErr := Evaluate(meta, inputs[canonical.ID])
		if evalErr != nil {
			return evalErr
		}
		if comparison.Status != canonical.Status {
			return fmt.Errorf("case %q evaluated as %s, contract requires %s", canonical.ID, comparison.Status, canonical.Status)
		}
		comparisons = append(comparisons, comparison)
		decisions = append(decisions, decision)
		observed[decision.Status]++
	}

	expected := expectedCounts(meta.Cases)
	runDecision := reduceStatus(meta, observed)
	claim := meta.Claims["language_utility"]
	manifestFiles := artifactFiles(meta)
	result := runResult{
		Manifest: PairingManifest{
			Schema:                  meta.Artifacts["manifest"].Schema,
			Operation:               meta.Operation,
			MetacodeDigest:          meta.Digest,
			IdentityFields:          append([]string(nil), meta.IdentityFields...),
			MeasurementTypes:        cloneStringMap(meta.MeasurementTypes),
			StatusPrecedence:        append([]string(nil), meta.Precedence...),
			Denominator:             denominator,
			ExpectedStatusCounts:    expected,
			CanonicalCases:          append([]CaseSpec(nil), meta.Cases...),
			ActivityBindings:        cloneStringMap(meta.Bindings),
			Inventory:               inventory,
			ExcludedPaths:           append([]string(nil), meta.ExcludedPaths...),
			OutputFiles:             manifestFiles,
			InputRepositoryMutated:  false,
			ProductRepositoryWrites: 0,
		},
		Comparisons: comparisons,
		Decisions:   decisions,
		Decision: DecisionReceipt{
			Schema:                meta.Artifacts["decision"].Schema,
			Operation:             meta.Operation,
			Denominator:           denominator,
			ExpectedStatusCounts:  expected,
			ObservedStatusCounts:  observed,
			RunDecision:           runDecision,
			LanguageUtilityStatus: claim.Status,
			LanguageUtilityReason: claim.Reason,
			Decisions:             decisions,
		},
		Events: buildEvents(meta, denominator, comparisons, observed, runDecision),
	}
	result.Report = buildReport(meta, result)
	return writeOutput(output, meta, result)
}

func loadFixture(path string) (FixtureFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return FixtureFile{}, err
	}
	var fixture FixtureFile
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fixture); err != nil {
		return FixtureFile{}, fmt.Errorf("decode fixture: %w", err)
	}
	if fixture.Schema == "" {
		return FixtureFile{}, fmt.Errorf("fixture schema is required")
	}
	return fixture, nil
}

func validateFixture(meta MetaCode, fixture FixtureFile) error {
	if len(fixture.Cases) != len(meta.Cases) {
		return fmt.Errorf("fixture denominator must remain exactly %d", len(meta.Cases))
	}
	seen := map[string]bool{}
	for _, input := range fixture.Cases {
		if input.ID == "" || seen[input.ID] {
			return fmt.Errorf("fixture case ids must be unique")
		}
		seen[input.ID] = true
	}
	for _, canonical := range meta.Cases {
		if !seen[canonical.ID] {
			return fmt.Errorf("fixture is missing canonical case %q", canonical.ID)
		}
	}
	return nil
}

func inventorySource(root string, exclusions []string) (Inventory, error) {
	directories := map[string]bool{}
	for _, exclusion := range exclusions {
		if exclusion != "README.md" {
			directories[exclusion] = true
		}
	}
	var inventory Inventory
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if directories[entry.Name()] {
				return filepath.SkipDir
			}
			inventory.DescendantDirs++
			return nil
		}
		if relative == "README.md" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		inventory.Files++
		inventory.RegularFiles++
		switch filepath.Ext(entry.Name()) {
		case ".go":
			inventory.GoFiles++
			lines, err := physicalLines(path)
			if err != nil {
				return err
			}
			inventory.GoLines += lines
		case ".gooo":
			inventory.GoooFiles++
			lines, err := physicalLines(path)
			if err != nil {
				return err
			}
			inventory.GoooLines += lines
		}
		return nil
	})
	return inventory, err
}

func physicalLines(path string) (int64, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(raw) == 0 {
		return 0, nil
	}
	lines := int64(bytes.Count(raw, []byte{'\n'}))
	if raw[len(raw)-1] != '\n' {
		lines++
	}
	return lines, nil
}

func expectedCounts(cases []CaseSpec) map[string]int {
	counts := map[string]int{}
	for _, canonical := range cases {
		counts[canonical.Status]++
	}
	return counts
}

func reduceStatus(meta MetaCode, counts map[string]int) string {
	for _, status := range meta.Precedence {
		if counts[status] > 0 {
			return status
		}
	}
	return "UNKNOWN"
}

func buildEvents(meta MetaCode, denominator int, comparisons []Comparison, counts map[string]int, decision string) []map[string]any {
	schema := meta.Artifacts["events"].Schema
	events := []map[string]any{{"schema": schema, "event": "run_started", "denominator": denominator, "denominator_fixed": true}}
	for ordinal, activity := range meta.Activities {
		events = append(events, map[string]any{"schema": schema, "event": "activity_bound", "ordinal": ordinal + 1, "activity": activity.Name, "binding": meta.Bindings[activity.Name]})
	}
	for _, comparison := range comparisons {
		events = append(events, map[string]any{
			"schema": schema, "event": "case_evaluated", "case_id": comparison.CaseID, "status": comparison.Status,
			"before_integer": comparison.BeforeInteger, "after_integer": comparison.AfterInteger, "delta_integer": comparison.DeltaInteger,
			"direction": comparison.Direction, "identity_complete": comparison.IdentityComplete, "identity_match": comparison.IdentityMatch,
		})
	}
	events = append(events, map[string]any{"schema": schema, "event": "run_completed", "denominator": denominator, "status_counts": counts, "decision": decision})
	return events
}

func buildReport(meta MetaCode, result runResult) string {
	var report strings.Builder
	report.WriteString("# Gooo improvement pairing gate report\n\n")
	report.WriteString("This report records exact before/after integer pairs only. No score or percentage is used.\n\n")
	fmt.Fprintf(&report, "- Operation: `%s`\n- Denominator (fixed for this run): `%d`\n- Run decision: `%s`\n- Whole-language utility: `%s` — %s\n\n", meta.Operation, result.Decision.Denominator, result.Decision.RunDecision, result.Decision.LanguageUtilityStatus, result.Decision.LanguageUtilityReason)
	report.WriteString("## Status counts\n\n")
	fmt.Fprintf(&report, "- CLOSED: `%d`\n- UNKNOWN: `%d`\n- REFUTED: `%d`\n\n", result.Decision.ObservedStatusCounts["CLOSED"], result.Decision.ObservedStatusCounts["UNKNOWN"], result.Decision.ObservedStatusCounts["REFUTED"])
	report.WriteString("## Exact comparisons\n\n| case | status | before_integer | after_integer | delta_integer | direction |\n|---|---|---:|---:|---:|---|\n")
	for _, comparison := range result.Comparisons {
		before, after, delta := "null", "null", "null"
		if comparison.BeforeInteger != nil {
			before = fmt.Sprintf("%d", *comparison.BeforeInteger)
		}
		if comparison.AfterInteger != nil {
			after = fmt.Sprintf("%d", *comparison.AfterInteger)
		}
		if comparison.DeltaInteger != nil {
			delta = fmt.Sprintf("%d", *comparison.DeltaInteger)
		}
		fmt.Fprintf(&report, "| `%s` | `%s` | `%s` | `%s` | `%s` | `%s` |\n", comparison.CaseID, comparison.Status, before, after, delta, comparison.Direction)
	}
	report.WriteString("\n## UNKNOWN causality\n\n")
	for _, decision := range result.Decisions {
		if decision.Unknown == nil {
			continue
		}
		unknown := decision.Unknown
		fmt.Fprintf(&report, "- `%s`: stage `%s`, step `%s`, reason `%s`, unknown_class `%s`, next_operation `%s`, blocked_by `%s`\n", decision.CaseID, unknown.Stage, unknown.Step, unknown.Reason, unknown.UnknownClass, unknown.NextOperation, strings.Join(unknown.BlockedBy, ","))
	}
	report.WriteString("\n## Input inventory\n\n")
	fmt.Fprintf(&report, "- Go files / physical lines: `%d` / `%d`\n- Gooo files / physical lines: `%d` / `%d`\n- Files / regular files / descendant directories: `%d` / `%d` / `%d`\n- Input repository mutated: `%t`\n", result.Manifest.Inventory.GoFiles, result.Manifest.Inventory.GoLines, result.Manifest.Inventory.GoooFiles, result.Manifest.Inventory.GoooLines, result.Manifest.Inventory.Files, result.Manifest.Inventory.RegularFiles, result.Manifest.Inventory.DescendantDirs, result.Manifest.InputRepositoryMutated)
	return report.String()
}

func writeOutput(output string, meta MetaCode, result runResult) error {
	if err := os.MkdirAll(output, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(output)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("caller-owned output directory must be empty")
	}
	comparison := ComparisonReceipt{Schema: meta.Artifacts["comparison"].Schema, Operation: meta.Operation, Denominator: result.Decision.Denominator, Comparisons: result.Comparisons}
	values := map[string]any{
		meta.Artifacts["manifest"].File:     result.Manifest,
		meta.Artifacts["comparison"].File:   comparison,
		meta.Artifacts["decision"].File:     result.Decision,
		meta.Artifacts["report"].File:       result.Report,
	}
	for file, value := range values {
		if file == meta.Artifacts["report"].File {
			if err := os.WriteFile(filepath.Join(output, file), []byte(value.(string)), 0o644); err != nil {
				return err
			}
			continue
		}
		raw, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		raw = append(raw, '\n')
		if err := os.WriteFile(filepath.Join(output, file), raw, 0o644); err != nil {
			return err
		}
	}
	eventPath := filepath.Join(output, meta.Artifacts["events"].File)
	eventFile, err := os.OpenFile(eventPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	for _, event := range result.Events {
		raw, marshalErr := json.Marshal(event)
		if marshalErr != nil {
			eventFile.Close()
			return marshalErr
		}
		if _, writeErr := eventFile.Write(append(raw, '\n')); writeErr != nil {
			eventFile.Close()
			return writeErr
		}
	}
	if err := eventFile.Close(); err != nil {
		return err
	}

	actual, err := os.ReadDir(output)
	if err != nil {
		return err
	}
	if len(actual) != len(meta.Artifacts) {
		return fmt.Errorf("generated output count is %d, expected %d", len(actual), len(meta.Artifacts))
	}
	return nil
}

func artifactFiles(meta MetaCode) []string {
	roles := []string{"manifest", "events", "comparison", "decision", "report"}
	files := make([]string, 0, len(roles))
	for _, role := range roles {
		files = append(files, meta.Artifacts[role].File)
	}
	return files
}

func cloneStringMap(values map[string]string) map[string]string {
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

func isWithin(path, parent string) bool {
	relative, err := filepath.Rel(parent, path)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}
