package gate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func Evaluate(meta MetaCode, input CaseInput) (Comparison, CaseDecision, error) {
	identity := declaredIdentity(meta, input.Before.Identity)
	identityComplete := completeIdentity(meta, input.Before.Identity) && completeIdentity(meta, input.After.Identity)
	identityMatch := identityComplete && sameIdentity(meta, input.Before.Identity, input.After.Identity)

	before, beforeOK := parseInteger(input.Before.Value)
	after, afterOK := parseInteger(input.After.Value)
	comparison := Comparison{
		CaseID:           input.ID,
		Identity:         identity,
		IdentityComplete: identityComplete,
		IdentityMatch:    identityMatch,
		Direction:        meta.Directions["unknown"],
		ClaimedDirection: input.ClaimedDirection,
		EvidenceDigest:   input.EvidenceDigest,
		Reasons:          []string{},
	}
	if beforeOK {
		comparison.BeforeInteger = int64Pointer(before)
	}
	if afterOK {
		comparison.AfterInteger = int64Pointer(after)
	}

	var expectedDigest string
	if beforeOK && afterOK {
		if !fitsDelta(before, after) {
			comparison.Reasons = append(comparison.Reasons, "delta_overflow")
		} else {
			delta := after - before
			direction := directionFor(meta, delta)
			comparison.DeltaInteger = int64Pointer(delta)
			comparison.Direction = direction
			expectedDigest, _ = expectedEvidenceDigest(meta, input.Before.Identity, before, after, delta, direction)
			comparison.ExpectedEvidenceDigest = expectedDigest
			if input.ClaimedDirection != "" && input.ClaimedDirection != direction {
				comparison.Reasons = append(comparison.Reasons, "direction_reversal")
			}
			if input.EvidenceDigest != "" && input.EvidenceDigest != expectedDigest {
				comparison.Reasons = append(comparison.Reasons, "evidence_digest")
			}
			if direction == meta.Directions["negative"] {
				comparison.Reasons = append(comparison.Reasons, "regression")
			}
		}
	} else {
		if !beforeOK || !afterOK {
			comparison.Reasons = append(comparison.Reasons, "non_integer_measurement")
		}
		if input.EvidenceDigest != "" {
			comparison.Reasons = append(comparison.Reasons, "evidence_digest")
		}
	}

	refuted := hasRefutation(comparison.Reasons)
	status := statusFor(meta, "CLOSED")
	if refuted {
		status = statusFor(meta, "REFUTED")
	} else if !identityComplete || !identityMatch || !beforeOK || !afterOK || comparison.DeltaInteger == nil {
		status = statusFor(meta, "UNKNOWN")
	}
	comparison.Status = status
	if len(comparison.Reasons) == 0 && status == statusFor(meta, "CLOSED") {
		comparison.Reasons = []string{"exact_integer_pair"}
	}

	decision := CaseDecision{CaseID: input.ID, Status: status}
	if status == statusFor(meta, "UNKNOWN") {
		unknown, ok := meta.UnknownCases[input.ID]
		if !ok {
			return Comparison{}, CaseDecision{}, fmt.Errorf("UNKNOWN case %q has no declared causality", input.ID)
		}
		record := UnknownRecord{Stage: unknown.Stage, Step: unknown.Step, Reason: unknown.Reason, UnknownClass: unknown.UnknownClass, NextOperation: unknown.NextOperation, BlockedBy: strings.Split(unknown.BlockedBy, ",")}
		if err := validateUnknown(record); err != nil {
			return Comparison{}, CaseDecision{}, err
		}
		decision.Unknown = &record
	} else if refuted {
		decision.Contradictions = append([]string(nil), comparison.Reasons...)
	}
	return comparison, decision, nil
}

func declaredIdentity(meta MetaCode, values map[string]string) map[string]string {
	identity := make(map[string]string, len(meta.IdentityFields))
	for _, field := range meta.IdentityFields {
		identity[field] = values[field]
	}
	return identity
}

func completeIdentity(meta MetaCode, values map[string]string) bool {
	if len(values) != len(meta.IdentityFields) {
		return false
	}
	for _, field := range meta.IdentityFields {
		if values[field] == "" {
			return false
		}
	}
	return true
}

func sameIdentity(meta MetaCode, before, after map[string]string) bool {
	for _, field := range meta.IdentityFields {
		if before[field] != after[field] {
			return false
		}
	}
	return true
}

func parseInteger(raw json.RawMessage) (int64, bool) {
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return 0, false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return 0, false
	}
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	parsed, err := strconv.ParseInt(string(number), 10, 64)
	return parsed, err == nil
}

func fitsDelta(before, after int64) bool {
	const minInt64 = -1 << 63
	const maxInt64 = 1<<63 - 1
	return !(after > 0 && before < minInt64+after) && !(after < 0 && before > maxInt64+after)
}

func directionFor(meta MetaCode, delta int64) string {
	if delta > 0 {
		return meta.Directions["positive"]
	}
	if delta < 0 {
		return meta.Directions["negative"]
	}
	return meta.Directions["zero"]
}

type evidencePayload struct {
	Identity      map[string]string `json:"identity"`
	BeforeInteger int64             `json:"before_integer"`
	AfterInteger  int64             `json:"after_integer"`
	DeltaInteger  int64             `json:"delta_integer"`
	Direction     string            `json:"direction"`
}

func expectedEvidenceDigest(meta MetaCode, identity map[string]string, before, after, delta int64, direction string) (string, error) {
	if meta.EvidenceAlgorithm != "sha256" || meta.EvidenceInput != "identity_and_values" {
		return "", fmt.Errorf("unsupported evidence digest contract")
	}
	payload, err := json.Marshal(evidencePayload{Identity: declaredIdentity(meta, identity), BeforeInteger: before, AfterInteger: after, DeltaInteger: delta, Direction: direction})
	if err != nil {
		return "", err
	}
	return digestBytes(payload), nil
}

func hasRefutation(reasons []string) bool {
	for _, reason := range reasons {
		if reason == "regression" || reason == "direction_reversal" || reason == "evidence_digest" || reason == "delta_overflow" {
			return true
		}
	}
	return false
}

func statusFor(meta MetaCode, wanted string) string {
	for _, status := range meta.Precedence {
		if status == wanted {
			return status
		}
	}
	return wanted
}

func int64Pointer(value int64) *int64 {
	return &value
}

func validateUnknown(record UnknownRecord) error {
	if record.Stage == "" || record.Step == "" || record.Reason == "" || record.UnknownClass == "" || record.NextOperation == "" || len(record.BlockedBy) == 0 || record.BlockedBy[0] == "" {
		return fmt.Errorf("UNKNOWN record must contain stage, step, reason, unknown_class, next_operation, and blocked_by")
	}
	return nil
}
