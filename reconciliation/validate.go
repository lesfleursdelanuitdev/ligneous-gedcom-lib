package reconciliation

// ValidateMergePlan performs structural checks before persistence or apply workflows.
func ValidateMergePlan(plan *MergePlan) []string {
	if plan == nil {
		return []string{"mergePlan is nil"}
	}
	var out []string
	if plan.Version < 1 || plan.Version > 2 {
		out = append(out, "unsupported mergePlan.version")
	}
	if plan.ScoringProfileID == "" {
		out = append(out, "missing scoringProfileId")
	}
	return out
}
