package reconciliation

import (
	"strings"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/enricher"
)

// scoreSoftIndividualPair returns a combined 0–1 score and assigned stage (3–5) for explainability.
func scoreSoftIndividualPair(
	left, right *enricher.EnrichedDocument,
	l, r *enricher.EnrichedIndividual,
	profile string,
) MatchScorecard {
	nameS := nameSimilarity(l, r)
	byL, bokL := yearAt(left, l.BirthDateIndex)
	byR, bokR := yearAt(right, r.BirthDateIndex)
	birthS := birthYearSimilarity(byL, bokL, byR, bokR)

	fpL := individualNeighborFingerprints(left, l)
	fpR := individualNeighborFingerprints(right, r)
	famS := jaccardMultisets(fpL, fpR)

	combined := 0.38*nameS + 0.37*birthS + 0.25*famS

	var stage MatchStage
	switch {
	case combined >= 0.78 && famS >= 0.28:
		stage = StageFamily
	case combined >= 0.88 && nameS >= 0.92 && birthS >= 0.85:
		stage = StageNameDate
	default:
		stage = StageFuzzy
	}

	reasons := []string{}
	ev := []Evidence{
		{Code: "NAME_SIMILARITY", Weight: nameS, Detail: map[string]any{"left": l.FullNameLower, "right": r.FullNameLower}},
		{Code: "BIRTH_YEAR_SIMILARITY", Weight: birthS},
		{Code: "FAMILY_CONTEXT_JACCARD", Weight: famS, Detail: map[string]any{"leftNeighbors": len(fpL), "rightNeighbors": len(fpR)}},
	}
	if stage == StageNameDate {
		reasons = append(reasons, "strong name and birth agreement")
	}
	if stage == StageFamily {
		reasons = append(reasons, "neighbor fingerprint multiset overlap")
	}
	if stage == StageFuzzy {
		reasons = append(reasons, "weak or partial agreement — requires review")
	}

	return MatchScorecard{
		Score:             combined,
		Stage:             stage,
		ScoringProfileID: profile,
		Reasons:           reasons,
		Evidence:          ev,
	}
}

func nameSimilarity(l, r *enricher.EnrichedIndividual) float64 {
	a := strings.TrimSpace(strings.ToLower(l.FullNameLower))
	b := strings.TrimSpace(strings.ToLower(r.FullNameLower))
	if a == "" && b == "" {
		return 0.5
	}
	if a == b {
		return 1.0
	}
	if a != "" && b != "" && (strings.Contains(a, b) || strings.Contains(b, a)) {
		return 0.88
	}
	ls := strings.TrimSpace(strings.ToLower(l.PrimarySurnameLower))
	rs := strings.TrimSpace(strings.ToLower(r.PrimarySurnameLower))
	if ls != "" && rs != "" && ls == rs {
		// Same primary surname: token overlap on full name lower
		tokA := strings.Fields(a)
		tokB := strings.Fields(b)
		if len(tokA) > 0 && len(tokB) > 0 {
			overlap := tokenOverlapRatio(tokA, tokB)
			return 0.55 + 0.35*overlap
		}
		return 0.65
	}
	return 0.35
}

func tokenOverlapRatio(a, b []string) float64 {
	set := make(map[string]struct{})
	for _, t := range a {
		if t != "" {
			set[t] = struct{}{}
		}
	}
	if len(set) == 0 {
		return 0
	}
	hit := 0
	for _, t := range b {
		if _, ok := set[t]; ok {
			hit++
		}
	}
	den := len(a)
	if len(b) > den {
		den = len(b)
	}
	if den == 0 {
		return 0
	}
	return float64(hit) / float64(den)
}

func birthYearSimilarity(yL int, okL bool, yR int, okR bool) float64 {
	switch {
	case okL && okR:
		d := abs(yL - yR)
		if d == 0 {
			return 1.0
		}
		if d <= 2 {
			return 0.92
		}
		if d <= 10 {
			return 0.75 - 0.04*float64(d-3)
		}
		if d <= 30 {
			return 0.35
		}
		return 0.15
	case okL || okR:
		return 0.55 // one side missing — neutral leaning weak match
	default:
		return 0.45
	}
}
