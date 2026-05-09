package reconciliation

import (
	"bytes"
	"encoding/json"
)

// MergeReconcileOptionsFromJSON overlays a JSON "options" object onto DefaultOptions().
// Omitted keys keep defaults; null uses the JSON decoder's behavior (pointer fields stay unset).
// Use pointer fields in wire JSON so explicit false for booleans is preserved.
func MergeReconcileOptionsFromJSON(raw json.RawMessage) (*Options, error) {
	base := DefaultOptions()
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return base, nil
	}
	var patch struct {
		ScoringProfileID             *string  `json:"scoringProfileId"`
		MaxBirthYearDelta            *int     `json:"maxBirthYearDelta"`
		EnableSoftIndividualMatching *bool    `json:"enableSoftIndividualMatching"`
		SoftMinAlignScore            *float64 `json:"softMinAlignScore"`
		SoftMinHintScore             *float64 `json:"softMinHintScore"`
		MaxSoftComparisonsPerSide    *int     `json:"maxSoftComparisonsPerSide"`
		UseHungarianAssignment       *bool    `json:"useHungarianAssignment"`
		HungarianMaxMatrix           *int     `json:"hungarianMaxMatrix"`
	}
	if err := json.Unmarshal(trimmed, &patch); err != nil {
		return nil, err
	}
	if patch.ScoringProfileID != nil {
		base.ScoringProfileID = *patch.ScoringProfileID
	}
	if patch.MaxBirthYearDelta != nil {
		base.MaxBirthYearDelta = *patch.MaxBirthYearDelta
	}
	if patch.EnableSoftIndividualMatching != nil {
		base.EnableSoftIndividualMatching = *patch.EnableSoftIndividualMatching
	}
	if patch.SoftMinAlignScore != nil {
		base.SoftMinAlignScore = *patch.SoftMinAlignScore
	}
	if patch.SoftMinHintScore != nil {
		base.SoftMinHintScore = *patch.SoftMinHintScore
	}
	if patch.MaxSoftComparisonsPerSide != nil {
		base.MaxSoftComparisonsPerSide = *patch.MaxSoftComparisonsPerSide
	}
	if patch.UseHungarianAssignment != nil {
		base.UseHungarianAssignment = *patch.UseHungarianAssignment
	}
	if patch.HungarianMaxMatrix != nil {
		base.HungarianMaxMatrix = *patch.HungarianMaxMatrix
	}
	return base, nil
}
