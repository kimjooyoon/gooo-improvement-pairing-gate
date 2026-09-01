package gate

import "encoding/json"

type ActivitySpec struct {
	Name       string
	InputType  string
	OutputType string
	Computes   string
}

type EdgeSpec struct {
	From      string
	To        string
	ValueType string
}

type ArtifactSpec struct {
	Role   string
	File   string
	Schema string
}

type CaseSpec struct {
	ID     string
	Status string
}

type UnknownSpec struct {
	Stage         string
	Step          string
	Reason        string
	UnknownClass  string
	NextOperation string
	BlockedBy     string
}

type ClaimSpec struct {
	Status string
	Reason string
}

type MetaCode struct {
	Path             string
	Digest           string
	Package          string
	Namespace        string
	Operation        string
	Version          string
	IdentityFields   []string
	MeasurementTypes map[string]string
	Directions       map[string]string
	Precedence       []string
	UnknownFields    []string
	EvidenceAlgorithm string
	EvidenceInput    string
	Contradictions   []string
	ExcludedPaths    []string
	RuntimeStages    []string
	Metrics          []string
	Activities       []ActivitySpec
	Bindings         map[string]string
	Edges            []EdgeSpec
	Artifacts        map[string]ArtifactSpec
	Cases            []CaseSpec
	UnknownCases     map[string]UnknownSpec
	Claims           map[string]ClaimSpec
}

type FixtureFile struct {
	Schema string      `json:"schema"`
	Cases  []CaseInput `json:"cases"`
}

type CaseInput struct {
	ID               string         `json:"id"`
	Before           Observation    `json:"before"`
	After            Observation    `json:"after"`
	ClaimedDirection string         `json:"claimed_direction"`
	EvidenceDigest   string         `json:"evidence_digest"`
}

type Observation struct {
	Value    json.RawMessage  `json:"value"`
	Identity map[string]string `json:"identity"`
}

type UnknownRecord struct {
	Stage         string   `json:"stage"`
	Step          string   `json:"step"`
	Reason        string   `json:"reason"`
	UnknownClass  string   `json:"unknown_class"`
	NextOperation string   `json:"next_operation"`
	BlockedBy     []string `json:"blocked_by"`
}

type Comparison struct {
	CaseID                 string            `json:"case_id"`
	Identity               map[string]string `json:"identity"`
	IdentityComplete       bool              `json:"identity_complete"`
	IdentityMatch          bool              `json:"identity_match"`
	BeforeInteger          *int64            `json:"before_integer"`
	AfterInteger           *int64            `json:"after_integer"`
	DeltaInteger           *int64            `json:"delta_integer"`
	Direction              string            `json:"direction"`
	ClaimedDirection       string            `json:"claimed_direction,omitempty"`
	EvidenceDigest         string            `json:"evidence_digest,omitempty"`
	ExpectedEvidenceDigest string            `json:"expected_evidence_digest,omitempty"`
	Status                 string            `json:"status"`
	Reasons                []string          `json:"reasons"`
}

type CaseDecision struct {
	CaseID       string         `json:"case_id"`
	Status       string         `json:"status"`
	Unknown      *UnknownRecord `json:"unknown,omitempty"`
	Contradictions []string     `json:"contradictions,omitempty"`
}

type Inventory struct {
	GoFiles         int64 `json:"go_files"`
	GoLines         int64 `json:"go_lines"`
	GoooFiles       int64 `json:"gooo_files"`
	GoooLines       int64 `json:"gooo_lines"`
	Files           int64 `json:"files"`
	RegularFiles    int64 `json:"regular_files"`
	DescendantDirs  int64 `json:"descendant_dirs"`
}

type PairingManifest struct {
	Schema                 string            `json:"schema"`
	Operation              string            `json:"operation"`
	MetacodeDigest         string            `json:"metacode_digest"`
	IdentityFields         []string          `json:"identity_fields"`
	MeasurementTypes       map[string]string `json:"measurement_types"`
	StatusPrecedence       []string          `json:"status_precedence"`
	Denominator            int               `json:"denominator"`
	ExpectedStatusCounts   map[string]int    `json:"expected_status_counts"`
	CanonicalCases         []CaseSpec        `json:"canonical_cases"`
	ActivityBindings       map[string]string `json:"activity_bindings"`
	Inventory              Inventory         `json:"inventory"`
	ExcludedPaths          []string          `json:"excluded_paths"`
	OutputFiles            []string          `json:"output_files"`
	InputRepositoryMutated bool              `json:"input_repository_mutated"`
	ProductRepositoryWrites int64            `json:"product_repository_writes"`
}

type ComparisonReceipt struct {
	Schema      string       `json:"schema"`
	Operation   string       `json:"operation"`
	Denominator int          `json:"denominator"`
	Comparisons []Comparison `json:"comparisons"`
}

type DecisionReceipt struct {
	Schema                 string         `json:"schema"`
	Operation              string         `json:"operation"`
	Denominator            int            `json:"denominator"`
	ExpectedStatusCounts   map[string]int `json:"expected_status_counts"`
	ObservedStatusCounts   map[string]int `json:"observed_status_counts"`
	RunDecision            string         `json:"run_decision"`
	LanguageUtilityStatus  string         `json:"language_utility_status"`
	LanguageUtilityReason  string         `json:"language_utility_reason"`
	Decisions              []CaseDecision `json:"decisions"`
}

type runResult struct {
	Manifest     PairingManifest
	Comparisons  []Comparison
	Decisions    []CaseDecision
	Decision     DecisionReceipt
	Events       []map[string]any
	Report       string
}
