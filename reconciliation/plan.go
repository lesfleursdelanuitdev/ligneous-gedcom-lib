package reconciliation

import (
	"fmt"
	"time"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/enricher"
)

// BuildMergePlan compares two enriched snapshots and returns a declarative merge plan.
// It is deterministic for a given pair of documents and Options.
func BuildMergePlan(left, right *enricher.EnrichedDocument, opt *Options) *MergePlan {
	if left == nil || right == nil {
		return &MergePlan{Version: 2, ScoringProfileID: coalesceProfile(opt)}
	}
	profile := coalesceProfile(opt)
	delta := maxBirthYearDelta(opt)

	plan := &MergePlan{
		Version:          2,
		ScoringProfileID: profile,
		GeneratedAt:      time.Now().UTC().Format(time.RFC3339),
		SafeAutoMerges:   nil, // strict-by-default: none without future policy layer
		PossibleMatches:  nil,
	}

	rightByID := individualByID(right)
	leftByXref := individualByXref(left)

	matchedLeft := make(map[string]bool)   // individual primary key: id or xref
	matchedRight := make(map[string]bool)

	alignIndividual := func(l, r *enricher.EnrichedIndividual, sc MatchScorecard, conf AlignmentConfidence) {
		plan.Alignments.Individuals = append(plan.Alignments.Individuals, ProposedAlignment{
			Kind:       "individual",
			LeftID:     l.ID,
			RightID:    r.ID,
			Confidence: conf,
			Scorecard:  sc,
		})
		keyL := indiKey(l)
		keyR := indiKey(r)
		matchedLeft[keyL] = true
		matchedRight[keyR] = true
		plan.IndividualDiffs = append(plan.IndividualDiffs, diffIndividuals(left, right, l, r))
		plan.Conflicts = append(plan.Conflicts, individualConflicts(left, right, l, r, profile, delta)...)
	}

	// Stage 1: stable id
	for i := range left.Individuals {
		l := &left.Individuals[i]
		if l.ID == "" {
			continue
		}
		r := rightByID[l.ID]
		if r == nil {
			continue
		}
		alignIndividual(l, r, scorecardStableID(profile), ConfidenceCertain)
	}

	// Stage 2: xref when not already matched and not contradictory ids
	for i := range right.Individuals {
		r := &right.Individuals[i]
		if matchedRight[indiKey(r)] {
			continue
		}
		rx := normXref(r.Xref)
		if rx == "" {
			continue
		}
		l := leftByXref[rx]
		if l == nil || matchedLeft[indiKey(l)] {
			continue
		}
		if l.ID != "" && r.ID != "" && l.ID != r.ID {
			plan.PossibleMatches = append(plan.PossibleMatches, xrefCollisionMatch(l, r, profile))
			matchedLeft[indiKey(l)] = true
			matchedRight[indiKey(r)] = true
			continue
		}
		alignIndividual(l, r, scorecardXref(profile), ConfidenceHigh)
	}

	// Families: id then xref
	rightFamID := familyByID(right)
	leftFamXref := familyByXref(left)
	matchedFamL := make(map[string]bool)
	matchedFamR := make(map[string]bool)

	alignFamily := func(l, r *enricher.EnrichedFamily, sc MatchScorecard, conf AlignmentConfidence) {
		plan.Alignments.Families = append(plan.Alignments.Families, ProposedAlignment{
			Kind:       "family",
			LeftID:     l.ID,
			RightID:    r.ID,
			Confidence: conf,
			Scorecard:  sc,
		})
		matchedFamL[famKey(l)] = true
		matchedFamR[famKey(r)] = true
		plan.FamilyDiffs = append(plan.FamilyDiffs, diffFamilies(left, right, l, r))
	}

	for i := range left.Families {
		lf := &left.Families[i]
		if lf.ID == "" {
			continue
		}
		rf := rightFamID[lf.ID]
		if rf == nil {
			continue
		}
		alignFamily(lf, rf, scorecardStableID(profile), ConfidenceCertain)
	}
	for i := range right.Families {
		rf := &right.Families[i]
		if matchedFamR[famKey(rf)] {
			continue
		}
		x := normXref(rf.Xref)
		if x == "" {
			continue
		}
		lf := leftFamXref[x]
		if lf == nil || matchedFamL[famKey(lf)] {
			continue
		}
		if lf.ID != "" && rf.ID != "" && lf.ID != rf.ID {
			continue
		}
		alignFamily(lf, rf, scorecardXref(profile), ConfidenceHigh)
	}

	var rightSoftHints map[string][]MatchScorecard
	if opt != nil && opt.EnableSoftIndividualMatching {
		rightSoftHints = runSoftIndividualMatching(left, right, opt, plan, matchedLeft, matchedRight, alignIndividual)
	}

	// Unresolved: individuals on each side not matched
	for i := range left.Individuals {
		l := &left.Individuals[i]
		if matchedLeft[indiKey(l)] {
			continue
		}
		plan.Unresolved = append(plan.Unresolved, UnresolvedEntity{
			Side:       "left",
			EntityType: "individual",
			EntityID:   l.ID,
			Xref:       l.Xref,
			Reason:     "no_candidate",
		})
	}
	for i := range right.Individuals {
		r := &right.Individuals[i]
		if matchedRight[indiKey(r)] {
			continue
		}
		u := UnresolvedEntity{
			Side:       "right",
			EntityType: "individual",
			EntityID:   r.ID,
			Xref:       r.Xref,
			Reason:     "no_candidate",
		}
		if rightSoftHints != nil {
			if h, ok := rightSoftHints[indiKey(r)]; ok && len(h) > 0 {
				u.Reason = "no_alignment_soft_hints"
				u.Hints = h
			}
		}
		plan.Unresolved = append(plan.Unresolved, u)
	}
	for i := range left.Families {
		f := &left.Families[i]
		if matchedFamL[famKey(f)] {
			continue
		}
		plan.Unresolved = append(plan.Unresolved, UnresolvedEntity{
			Side:       "left",
			EntityType: "family",
			EntityID:   f.ID,
			Xref:       f.Xref,
			Reason:     "no_candidate",
		})
	}
	for i := range right.Families {
		f := &right.Families[i]
		if matchedFamR[famKey(f)] {
			continue
		}
		plan.Unresolved = append(plan.Unresolved, UnresolvedEntity{
			Side:       "right",
			EntityType: "family",
			EntityID:   f.ID,
			Xref:       f.Xref,
			Reason:     "no_candidate",
		})
	}

	return plan
}

func coalesceProfile(opt *Options) string {
	if opt == nil {
		return DefaultScoringProfileID
	}
	return opt.scoringProfile()
}

func maxBirthYearDelta(opt *Options) int {
	if opt == nil {
		return 1
	}
	return opt.maxBirthYearDelta()
}

func indiKey(l *enricher.EnrichedIndividual) string {
	if l.ID != "" {
		return "id:" + l.ID
	}
	return "xref:" + normXref(l.Xref)
}

func famKey(f *enricher.EnrichedFamily) string {
	if f.ID != "" {
		return "id:" + f.ID
	}
	return "xref:" + normXref(f.Xref)
}

func xrefCollisionMatch(l, r *enricher.EnrichedIndividual, profile string) PossibleMatch {
	return PossibleMatch{
		Kind:           "individual",
		ResolutionHint: "review_xref_uuid_mismatch",
		Candidates: []AlignmentCandidateEntry{
			{
				LeftID:  l.ID,
				RightID: r.ID,
				Scorecard: MatchScorecard{
					Score:             0.4,
					Stage:             StageXref,
					ScoringProfileID:  profile,
					Reasons:           []string{"same xref but different stable ids — possible duplicate or bad import"},
					Evidence:          []Evidence{{Code: "XREF_UUID_MISMATCH", Weight: 0.4, Detail: map[string]any{"xref": normXref(l.Xref)}}},
				},
			},
		},
	}
}

func diffIndividuals(left, right *enricher.EnrichedDocument, l, r *enricher.EnrichedIndividual) IndividualDiffSummary {
	summary := IndividualDiffSummary{
		LeftID:         l.ID,
		RightID:        r.ID,
		FullNameEqual:  l.FullName == r.FullName,
		SexEqual:       l.Sex == r.Sex,
		EventKeysLeft:  individualEventFingerprints(left, l.Xref),
		EventKeysRight: individualEventFingerprints(right, r.Xref),
	}
	oly, ory := multisetDiff(summary.EventKeysLeft, summary.EventKeysRight)
	summary.EventKeysOnlyLeft = oly
	summary.EventKeysOnlyRight = ory

	ly, lok := yearAt(left, l.BirthDateIndex)
	ry, rok := yearAt(right, r.BirthDateIndex)
	if lok && rok {
		eq := ly == ry
		summary.BirthYearEqual = &eq
	}
	ldy, ldok := yearAt(left, l.DeathDateIndex)
	rdy, rdok := yearAt(right, r.DeathDateIndex)
	if ldok && rdok {
		eq := ldy == rdy
		summary.DeathYearEqual = &eq
	}

	summary.NoteKeysLeft = individualNoteFingerprints(left, l.Xref)
	summary.NoteKeysRight = individualNoteFingerprints(right, r.Xref)
	summary.NoteKeysOnlyLeft, summary.NoteKeysOnlyRight = multisetDiff(summary.NoteKeysLeft, summary.NoteKeysRight)

	summary.MediaKeysLeft = individualMediaFingerprints(left, l.Xref)
	summary.MediaKeysRight = individualMediaFingerprints(right, r.Xref)
	summary.MediaKeysOnlyLeft, summary.MediaKeysOnlyRight = multisetDiff(summary.MediaKeysLeft, summary.MediaKeysRight)

	summary.SourceKeysLeft = individualSourceFingerprints(left, l.Xref)
	summary.SourceKeysRight = individualSourceFingerprints(right, r.Xref)
	summary.SourceKeysOnlyLeft, summary.SourceKeysOnlyRight = multisetDiff(summary.SourceKeysLeft, summary.SourceKeysRight)

	return summary
}

func diffFamilies(left, right *enricher.EnrichedDocument, l, r *enricher.EnrichedFamily) FamilyDiffSummary {
	s := FamilyDiffSummary{
		LeftID:             l.ID,
		RightID:            r.ID,
		HusbandXrefEqual:   normXref(l.HusbandXref) == normXref(r.HusbandXref),
		WifeXrefEqual:      normXref(l.WifeXref) == normXref(r.WifeXref),
		ChildrenCountLeft:  l.ChildrenCount,
		ChildrenCountRight: r.ChildrenCount,
	}
	cl := childXrefsForFamily(left, l.Xref)
	cr := childXrefsForFamily(right, r.Xref)
	onlyL, onlyR := multisetDiff(cl, cr)
	s.ChildXrefOnlyLeft = onlyL
	s.ChildXrefOnlyRight = onlyR
	return s
}

func individualConflicts(left, right *enricher.EnrichedDocument, l, r *enricher.EnrichedIndividual, profile string, maxDelta int) []MergeConflict {
	var out []MergeConflict
	ly, lok := yearAt(left, l.BirthDateIndex)
	ry, rok := yearAt(right, r.BirthDateIndex)
	if lok && rok && abs(ly-ry) > maxDelta {
		out = append(out, MergeConflict{
			ID:        fmt.Sprintf("birth-year-%s-%s", indiKey(l), indiKey(r)),
			Kind:      "individual",
			LeftRef:   map[string]any{"id": l.ID, "xref": l.Xref},
			RightRef:  map[string]any{"id": r.ID, "xref": r.Xref},
			FieldPath: "birthDate.year",
			LeftValue: ly,
			RightValue: ry,
			Severity:  "blocking",
			Explain: MatchScorecard{
				Score:             0,
				Stage:             StageStableID,
				ScoringProfileID:  profile,
				Reasons:           []string{"birth years differ beyond configured tolerance for aligned individuals"},
				Evidence:          []Evidence{{Code: "BIRTH_YEAR_MISMATCH", Weight: 1, Detail: map[string]any{"maxDelta": maxDelta}}},
			},
		})
	}
	if l.Sex != "" && r.Sex != "" && l.Sex != r.Sex {
		out = append(out, MergeConflict{
			ID:        fmt.Sprintf("sex-%s-%s", indiKey(l), indiKey(r)),
			Kind:      "individual",
			LeftRef:   map[string]any{"id": l.ID, "xref": l.Xref},
			RightRef:  map[string]any{"id": r.ID, "xref": r.Xref},
			FieldPath: "sex",
			LeftValue: l.Sex,
			RightValue: r.Sex,
			Severity:  "warning",
			Explain: MatchScorecard{
				Score:            0,
				Stage:            StageStableID,
				ScoringProfileID: profile,
				Reasons:          []string{"sex values differ"},
				Evidence:         []Evidence{{Code: "SEX_MISMATCH", Weight: 0.5}},
			},
		})
	}
	if l.FullName != r.FullName {
		out = append(out, MergeConflict{
			ID:        fmt.Sprintf("fullname-%s-%s", indiKey(l), indiKey(r)),
			Kind:      "individual",
			LeftRef:   map[string]any{"id": l.ID, "xref": l.Xref},
			RightRef:  map[string]any{"id": r.ID, "xref": r.Xref},
			FieldPath: "fullName",
			LeftValue: l.FullName,
			RightValue: r.FullName,
			Severity:  "warning",
			Explain: MatchScorecard{
				Score:            0,
				Stage:            StageStableID,
				ScoringProfileID: profile,
				Reasons:          []string{"display full name strings differ"},
				Evidence:         []Evidence{{Code: "FULL_NAME_MISMATCH", Weight: 0.3}},
			},
		})
	}
	return out
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
