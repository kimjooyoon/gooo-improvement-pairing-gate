package gate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCanonicalContractAndCases(t *testing.T) {
	root := filepath.Join("..", "..")
	meta, err := LoadMetaCode(filepath.Join(root, ".gooo", "pairing-gate.gooo"))
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := loadFixture(filepath.Join(root, "testdata", "canonical-fixtures.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateFixture(meta, fixture); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, canonical := range meta.Cases {
		input := fixtureCase(fixture, canonical.ID)
		comparison, _, err := Evaluate(meta, input)
		if err != nil {
			t.Fatal(err)
		}
		counts[comparison.Status]++
		if comparison.Status != canonical.Status {
			t.Fatalf("case %s: got %s, want %s", canonical.ID, comparison.Status, canonical.Status)
		}
	}
	if counts["CLOSED"] != 3 || counts["UNKNOWN"] != 4 || counts["REFUTED"] != 2 {
		t.Fatalf("unexpected status counts: %#v", counts)
	}
}

func TestContradictionPrecedesUnknown(t *testing.T) {
	root := filepath.Join("..", "..")
	meta, err := LoadMetaCode(filepath.Join(root, ".gooo", "pairing-gate.gooo"))
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := loadFixture(filepath.Join(root, "testdata", "canonical-fixtures.json"))
	if err != nil {
		t.Fatal(err)
	}
	input := fixtureCase(fixture, "evidence-digest-contradiction")
	input.Before.Identity["fixture_digest"] = ""
	input.After.Identity["fixture_digest"] = ""
	comparison, decision, err := Evaluate(meta, input)
	if err != nil {
		t.Fatal(err)
	}
	if comparison.Status != "REFUTED" || decision.Unknown != nil {
		t.Fatalf("contradiction did not take precedence: comparison=%#v decision=%#v", comparison, decision)
	}
}

func TestRunWritesExactlyFiveCallerOwnedArtifacts(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	err = Run(Options{
		Source:   root,
		Fixture:  filepath.Join(root, "testdata", "canonical-fixtures.json"),
		Metacode: filepath.Join(root, ".gooo", "pairing-gate.gooo"),
		Output:   output,
	})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(output)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"pairing-manifest.json":   true,
		"pairing-events.ndjson":   true,
		"comparison-receipt.json": true,
		"decision-receipt.json":   true,
		"report.md":               true,
	}
	if len(entries) != len(want) {
		t.Fatalf("got %d artifacts, want %d", len(entries), len(want))
	}
	for _, entry := range entries {
		if !want[entry.Name()] {
			t.Fatalf("unexpected artifact %q", entry.Name())
		}
	}
	var receipt DecisionReceipt
	raw, err := os.ReadFile(filepath.Join(output, "decision-receipt.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Denominator != 9 || receipt.ObservedStatusCounts["CLOSED"] != 3 || receipt.ObservedStatusCounts["UNKNOWN"] != 4 || receipt.ObservedStatusCounts["REFUTED"] != 2 {
		t.Fatalf("unexpected receipt: %#v", receipt)
	}
}

func fixtureCase(fixture FixtureFile, id string) CaseInput {
	for _, input := range fixture.Cases {
		if input.ID == id {
			return input
		}
	}
	return CaseInput{}
}
