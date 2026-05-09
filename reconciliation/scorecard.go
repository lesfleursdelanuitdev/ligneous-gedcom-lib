package reconciliation

func scorecardStableID(profile string) MatchScorecard {
	return MatchScorecard{
		Score:             1.0,
		Stage:             StageStableID,
		ScoringProfileID:  profile,
		Reasons:           []string{"same stable id (uuid) on both snapshots"},
		Evidence:          []Evidence{{Code: "SAME_STABLE_ID", Weight: 1.0}},
	}
}

func scorecardXref(profile string) MatchScorecard {
	return MatchScorecard{
		Score:             0.95,
		Stage:             StageXref,
		ScoringProfileID:  profile,
		Reasons:           []string{"normalized gedcom xref match"},
		Evidence:          []Evidence{{Code: "XREF_EQUIVALENCE", Weight: 0.95}},
	}
}
