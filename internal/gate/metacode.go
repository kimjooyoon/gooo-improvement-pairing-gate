package gate

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func LoadMetaCode(path string) (MetaCode, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return MetaCode{}, err
	}
	meta := MetaCode{
		Path:             path,
		Digest:           digestBytes(raw),
		MeasurementTypes: map[string]string{},
		Directions:       map[string]string{},
		Bindings:         map[string]string{},
		Artifacts:        map[string]ArtifactSpec{},
		UnknownCases:     map[string]UnknownSpec{},
		Claims:           map[string]ClaimSpec{},
	}
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields, err := tokenize(line)
		if err != nil {
			return MetaCode{}, fmt.Errorf("%s:%d: %w", path, lineNumber, err)
		}
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "gooo":
			if len(fields) != 3 {
				return MetaCode{}, fmt.Errorf("%s:%d: malformed gooo declaration", path, lineNumber)
			}
			meta.Operation, meta.Version = fields[1], fields[2]
		case "package":
			meta.Package, err = oneValue(fields, "package")
		case "namespace":
			meta.Namespace, err = oneValue(fields, "namespace")
		case "operation":
			meta.Operation, err = oneValue(fields, "operation")
		case "identity-field":
			err = appendOne(&meta.IdentityFields, fields, "identity-field")
		case "measurement-field":
			var key, value string
			key, value, err = pairAfter(fields, "measurement-field")
			if err == nil {
				if _, exists := meta.MeasurementTypes[key]; exists {
					err = fmt.Errorf("duplicate measurement field %q", key)
				} else {
					meta.MeasurementTypes[key] = value
				}
			}
		case "direction":
			var key, value string
			key, value, err = pairAfter(fields, "direction")
			if err == nil {
				if _, exists := meta.Directions[key]; exists {
					err = fmt.Errorf("duplicate direction %q", key)
				} else {
					meta.Directions[key] = value
				}
			}
		case "status-precedence":
			if len(fields) < 2 {
				err = fmt.Errorf("status precedence is empty")
			} else {
				meta.Precedence = append([]string(nil), fields[1:]...)
			}
		case "unknown-field":
			err = appendOne(&meta.UnknownFields, fields, "unknown-field")
		case "evidence-digest":
			options, parseErr := pairs(fields, 1, "evidence-digest")
			err = parseErr
			if err == nil {
				meta.EvidenceAlgorithm = options["algorithm"]
				meta.EvidenceInput = options["input"]
			}
		case "contradiction":
			err = appendOne(&meta.Contradictions, fields, "contradiction")
		case "inventory-exclude":
			err = appendOne(&meta.ExcludedPaths, fields, "inventory-exclude")
		case "runtime-stage":
			err = appendOne(&meta.RuntimeStages, fields, "runtime-stage")
		case "metric":
			err = appendOne(&meta.Metrics, fields, "metric")
		case "artifact":
			if len(fields) != 6 || fields[2] != "file" || fields[4] != "schema" {
				err = fmt.Errorf("malformed artifact declaration")
				break
			}
			role := fields[1]
			if _, exists := meta.Artifacts[role]; exists {
				err = fmt.Errorf("duplicate artifact role %q", role)
				break
			}
			meta.Artifacts[role] = ArtifactSpec{Role: role, File: fields[3], Schema: fields[5]}
		case "claim":
			var key string
			var options map[string]string
			if len(fields) < 4 || fields[1] == "" {
				err = fmt.Errorf("claim expects a name and options")
				break
			}
			key = fields[1]
			options, err = pairs(fields, 2, "claim")
			if err == nil {
				if _, exists := meta.Claims[key]; exists {
					err = fmt.Errorf("duplicate claim %q", key)
				} else {
					meta.Claims[key] = ClaimSpec{Status: options["status"], Reason: options["reason"]}
				}
			}
		case "activity":
			if len(fields) != 8 || fields[2] != "input" || fields[4] != "output" || fields[6] != "computes" {
				err = fmt.Errorf("malformed activity declaration")
				break
			}
			meta.Activities = append(meta.Activities, ActivitySpec{Name: fields[1], InputType: fields[3], OutputType: fields[5], Computes: fields[7]})
		case "binding":
			if len(fields) != 4 || fields[2] != "role" {
				err = fmt.Errorf("malformed binding declaration")
				break
			}
			if _, exists := meta.Bindings[fields[1]]; exists {
				err = fmt.Errorf("duplicate binding for %q", fields[1])
			} else {
				meta.Bindings[fields[1]] = fields[3]
			}
		case "edge":
			if len(fields) != 6 || fields[2] != "->" || fields[4] != "type" {
				err = fmt.Errorf("malformed edge declaration")
			} else {
				meta.Edges = append(meta.Edges, EdgeSpec{From: fields[1], To: fields[3], ValueType: fields[5]})
			}
		case "case":
			if len(fields) != 3 {
				err = fmt.Errorf("malformed case declaration")
			} else {
				meta.Cases = append(meta.Cases, CaseSpec{ID: fields[1], Status: fields[2]})
			}
		case "unknown-case":
			var key string
			var options map[string]string
			if len(fields) < 4 || fields[1] == "" {
				err = fmt.Errorf("unknown-case expects a name and options")
				break
			}
			key = fields[1]
			options, err = pairs(fields, 2, "unknown-case")
			if err == nil {
				if _, exists := meta.UnknownCases[key]; exists {
					err = fmt.Errorf("duplicate unknown case %q", key)
				} else {
					meta.UnknownCases[key] = UnknownSpec{Stage: options["stage"], Step: options["step"], Reason: options["reason"], UnknownClass: options["unknown_class"], NextOperation: options["next_operation"], BlockedBy: options["blocked_by"]}
				}
			}
		default:
			err = fmt.Errorf("unknown directive %q", fields[0])
		}
		if err != nil {
			return MetaCode{}, fmt.Errorf("%s:%d: %w", path, lineNumber, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return MetaCode{}, err
	}
	if err := meta.Validate(); err != nil {
		return MetaCode{}, err
	}
	return meta, nil
}

func (m MetaCode) Validate() error {
	if m.Package == "" || m.Namespace == "" || m.Operation == "" || m.Version == "" {
		return fmt.Errorf("metacode identity is incomplete")
	}
	expectedIdentity := []string{"scenario_id", "source_digest", "contract_digest", "fixture_digest", "toolchain_digest", "runner_identity"}
	if !sameStrings(m.IdentityFields, expectedIdentity) {
		return fmt.Errorf("identity tuple must contain the exact six declared fields")
	}
	if len(m.Activities) != 8 || len(m.Edges) != 7 || len(m.Bindings) != 8 {
		return fmt.Errorf("released graph must contain eight activities, seven edges, and eight bindings")
	}
	activityNames := map[string]bool{}
	for _, activity := range m.Activities {
		if activity.Name == "" || activity.InputType == "" || activity.OutputType == "" || activity.Computes == "" || activityNames[activity.Name] {
			return fmt.Errorf("activity declarations must be unique and complete")
		}
		activityNames[activity.Name] = true
		if m.Bindings[activity.Name] == "" {
			return fmt.Errorf("activity %q is not bound exactly once", activity.Name)
		}
	}
	for _, edge := range m.Edges {
		if !activityNames[edge.From] || !activityNames[edge.To] || edge.ValueType == "" {
			return fmt.Errorf("edge references an undeclared activity")
		}
	}
	for activity := range m.Bindings {
		if !activityNames[activity] {
			return fmt.Errorf("binding references an undeclared activity")
		}
	}
	if !sameStrings(m.Precedence, []string{"REFUTED", "UNKNOWN", "CLOSED"}) {
		return fmt.Errorf("status precedence must be REFUTED > UNKNOWN > CLOSED")
	}
	if !sameStrings(m.UnknownFields, []string{"stage", "step", "reason", "unknown_class", "next_operation", "blocked_by"}) {
		return fmt.Errorf("UNKNOWN must declare the six required causality fields")
	}
	expectedMeasurements := map[string]string{"before": "integer", "after": "integer", "delta": "integer", "direction": "enum"}
	if len(m.MeasurementTypes) != len(expectedMeasurements) {
		return fmt.Errorf("measurement contract is incomplete")
	}
	for key, value := range expectedMeasurements {
		if m.MeasurementTypes[key] != value {
			return fmt.Errorf("measurement %q is not declared as %q", key, value)
		}
	}
	if m.Directions["positive"] == "" || m.Directions["zero"] == "" || m.Directions["negative"] == "" || m.Directions["unknown"] == "" {
		return fmt.Errorf("direction contract is incomplete")
	}
	if m.EvidenceAlgorithm != "sha256" || m.EvidenceInput != "identity_and_values" {
		return fmt.Errorf("evidence digest contract is incomplete")
	}
	if !contains(m.Contradictions, "direction_reversal") || !contains(m.Contradictions, "evidence_digest") {
		return fmt.Errorf("contradiction contract is incomplete")
	}
	if !contains(m.ExcludedPaths, "README.md") || !contains(m.ExcludedPaths, ".git") || !contains(m.ExcludedPaths, "cache") || !contains(m.ExcludedPaths, "vendor") || !contains(m.ExcludedPaths, "generated") {
		return fmt.Errorf("inventory exclusions are incomplete")
	}
	if !sameStrings(m.RuntimeStages, []string{"compile", "build", "test", "conformance", "integration"}) {
		return fmt.Errorf("runtime stages must be compile, build, test, conformance, integration")
	}
	for _, metric := range []string{"wall_ms", "peak_rss_kib", "tests_total", "tests_selected", "tests_executed", "tests_reused", "tests_failed", "tests_unknown", "go_files", "go_lines", "gooo_files", "gooo_lines", "files", "regular_files", "descendant_dirs"} {
		if !contains(m.Metrics, metric) {
			return fmt.Errorf("metric %q is missing", metric)
		}
	}
	if len(m.Artifacts) != 5 {
		return fmt.Errorf("exactly five generated artifacts are required")
	}
	roles := []string{"manifest", "events", "comparison", "decision", "report"}
	files := map[string]bool{}
	for _, role := range roles {
		artifact, ok := m.Artifacts[role]
		if !ok || artifact.File == "" || artifact.Schema == "" || files[artifact.File] {
			return fmt.Errorf("artifact schema is incomplete")
		}
		files[artifact.File] = true
	}
	if len(m.Cases) != 9 || len(m.UnknownCases) != 4 {
		return fmt.Errorf("exactly nine canonical cases and four UNKNOWN causes are required")
	}
	caseIDs := map[string]bool{}
	counts := map[string]int{}
	for _, canonical := range m.Cases {
		if canonical.ID == "" || caseIDs[canonical.ID] || !contains(m.Precedence, canonical.Status) {
			return fmt.Errorf("canonical cases must have unique declared statuses")
		}
		caseIDs[canonical.ID] = true
		counts[canonical.Status]++
	}
	if counts["CLOSED"] != 3 || counts["UNKNOWN"] != 4 || counts["REFUTED"] != 2 {
		return fmt.Errorf("canonical case counts must be CLOSED 3, UNKNOWN 4, REFUTED 2")
	}
	for id, unknown := range m.UnknownCases {
		if !caseIDs[id] || unknown.Stage == "" || unknown.Step == "" || unknown.Reason == "" || unknown.UnknownClass == "" || unknown.NextOperation == "" || unknown.BlockedBy == "" {
			return fmt.Errorf("UNKNOWN case %q is missing one of the six causality fields", id)
		}
	}
	claim, ok := m.Claims["language_utility"]
	if !ok || claim.Status != "UNKNOWN" || claim.Reason == "" {
		return fmt.Errorf("whole-language utility must remain UNKNOWN without external evidence")
	}
	return nil
}

func tokenize(line string) ([]string, error) {
	var fields []string
	for index := 0; index < len(line); {
		for index < len(line) && (line[index] == ' ' || line[index] == '\t') {
			index++
		}
		if index == len(line) {
			break
		}
		if line[index] == '"' {
			start := index
			index++
			escaped := false
			for index < len(line) {
				if line[index] == '"' && !escaped {
					index++
					break
				}
				if line[index] == '\\' && !escaped {
					escaped = true
				} else {
					escaped = false
				}
				index++
			}
			if index > len(line) || line[index-1] != '"' {
				return nil, fmt.Errorf("unterminated quoted value")
			}
			value, err := strconv.Unquote(line[start:index])
			if err != nil {
				return nil, fmt.Errorf("invalid quoted value: %w", err)
			}
			fields = append(fields, value)
			continue
		}
		start := index
		for index < len(line) && line[index] != ' ' && line[index] != '\t' {
			index++
		}
		fields = append(fields, line[start:index])
	}
	return fields, nil
}

func oneValue(fields []string, directive string) (string, error) {
	if len(fields) != 2 || fields[1] == "" {
		return "", fmt.Errorf("%s expects one value", directive)
	}
	return fields[1], nil
}

func appendOne(target *[]string, fields []string, directive string) error {
	value, err := oneValue(fields, directive)
	if err != nil {
		return err
	}
	*target = append(*target, value)
	return nil
}

func pairAfter(fields []string, directive string) (string, string, error) {
	if len(fields) != 3 || fields[1] == "" || fields[2] == "" {
		return "", "", fmt.Errorf("%s expects a key and value", directive)
	}
	return fields[1], fields[2], nil
}

func pairs(fields []string, start int, directive string) (map[string]string, error) {
	if start > len(fields) || (len(fields)-start)%2 != 0 {
		return nil, fmt.Errorf("%s options must be key/value pairs", directive)
	}
	options := map[string]string{}
	for index := start; index < len(fields); index += 2 {
		key, value := fields[index], fields[index+1]
		if key == "" || value == "" || options[key] != "" {
			return nil, fmt.Errorf("%s has an invalid or duplicate option", directive)
		}
		options[key] = value
	}
	return options, nil
}

func sameStrings(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index := range actual {
		if actual[index] != expected[index] {
			return false
		}
	}
	return true
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func digestBytes(raw []byte) string {
	digest := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(digest[:])
}
