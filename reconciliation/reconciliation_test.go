package reconciliation

import (
	"testing"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/enricher"
)

func TestBuildMergePlan_StableIDAlignment(t *testing.T) {
	left := &enricher.EnrichedDocument{
		Individuals: []enricher.EnrichedIndividual{
			{ID: "u1", Xref: "@I1@", FullName: "John Smith", Sex: "M", BirthDateIndex: 0},
		},
		Dates: []enricher.ParsedDate{
			{Original: "1880", Year: 1880},
		},
	}
	right := &enricher.EnrichedDocument{
		Individuals: []enricher.EnrichedIndividual{
			{ID: "u1", Xref: "@I1@", FullName: "John Smith", Sex: "M", BirthDateIndex: 0},
		},
		Dates: []enricher.ParsedDate{
			{Original: "1880", Year: 1880},
		},
	}
	plan := BuildMergePlan(left, right, nil)
	if len(plan.Alignments.Individuals) != 1 {
		t.Fatalf("individual alignments: got %d want 1", len(plan.Alignments.Individuals))
	}
	if plan.Alignments.Individuals[0].Scorecard.Stage != StageStableID {
		t.Fatalf("stage: got %v want stage1", plan.Alignments.Individuals[0].Scorecard.Stage)
	}
	if len(plan.IndividualDiffs) != 1 || !plan.IndividualDiffs[0].FullNameEqual {
		t.Fatalf("diff summary: %+v", plan.IndividualDiffs)
	}
}

func TestBuildMergePlan_BirthYearConflict(t *testing.T) {
	left := &enricher.EnrichedDocument{
		Individuals: []enricher.EnrichedIndividual{
			{ID: "u1", Xref: "@I1@", FullName: "John Smith", BirthDateIndex: 0},
		},
		Dates: []enricher.ParsedDate{{Original: "1880", Year: 1880}},
	}
	right := &enricher.EnrichedDocument{
		Individuals: []enricher.EnrichedIndividual{
			{ID: "u1", Xref: "@I1@", FullName: "John Smith", BirthDateIndex: 0},
		},
		Dates: []enricher.ParsedDate{{Original: "1900", Year: 1900}},
	}
	plan := BuildMergePlan(left, right, &Options{MaxBirthYearDelta: 1})
	var found bool
	for _, c := range plan.Conflicts {
		if c.FieldPath == "birthDate.year" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected birth year conflict, got %+v", plan.Conflicts)
	}
}

func TestBuildMergePlan_XrefAlignmentWithoutID(t *testing.T) {
	left := &enricher.EnrichedDocument{
		Individuals: []enricher.EnrichedIndividual{
			{Xref: "@I1@", FullName: "Jane Doe", Sex: "F"},
		},
	}
	right := &enricher.EnrichedDocument{
		Individuals: []enricher.EnrichedIndividual{
			{Xref: "@I1@", FullName: "Jane Doe", Sex: "F"},
		},
	}
	plan := BuildMergePlan(left, right, nil)
	if len(plan.Alignments.Individuals) != 1 {
		t.Fatalf("xref alignments: got %d want 1", len(plan.Alignments.Individuals))
	}
	if plan.Alignments.Individuals[0].Scorecard.Stage != StageXref {
		t.Fatalf("expected xref stage")
	}
}

func TestBuildMergePlan_XrefUUIDMismatchPossibleMatch(t *testing.T) {
	left := &enricher.EnrichedDocument{
		Individuals: []enricher.EnrichedIndividual{
			{ID: "a", Xref: "@I1@", FullName: "A"},
		},
	}
	right := &enricher.EnrichedDocument{
		Individuals: []enricher.EnrichedIndividual{
			{ID: "b", Xref: "@I1@", FullName: "B"},
		},
	}
	plan := BuildMergePlan(left, right, nil)
	if len(plan.PossibleMatches) != 1 {
		t.Fatalf("possible matches: got %d want 1", len(plan.PossibleMatches))
	}
	if len(plan.Alignments.Individuals) != 0 {
		t.Fatalf("should not auto-align contradictory ids")
	}
}

func TestBuildMergePlan_SoftMatchDifferentXref(t *testing.T) {
	left := &enricher.EnrichedDocument{
		Individuals: []enricher.EnrichedIndividual{
			{Xref: "@I1@", FullName: "John Smith", FullNameLower: "john smith", PrimarySurnameLower: "smith", BirthDateIndex: 0},
			{Xref: "@P1@", FullName: "Mary Smith", PrimarySurnameLower: "smith", BirthDateIndex: 1},
		},
		Dates: []enricher.ParsedDate{
			{Original: "1880", Year: 1880},
			{Original: "1850", Year: 1850},
		},
		ParentChild: []enricher.ParentChildEdge{
			{ParentXref: "@P1@", ChildXref: "@I1@"},
		},
	}
	right := &enricher.EnrichedDocument{
		Individuals: []enricher.EnrichedIndividual{
			{Xref: "@I9@", FullName: "John Smith", FullNameLower: "john smith", PrimarySurnameLower: "smith", BirthDateIndex: 0},
			{Xref: "@P9@", FullName: "Mary Smith", PrimarySurnameLower: "smith", BirthDateIndex: 1},
		},
		Dates: []enricher.ParsedDate{
			{Original: "1880", Year: 1880},
			{Original: "1850", Year: 1850},
		},
		ParentChild: []enricher.ParentChildEdge{
			{ParentXref: "@P9@", ChildXref: "@I9@"},
		},
	}
	opt := &Options{
		EnableSoftIndividualMatching: true,
		SoftMinAlignScore:            0.70,
		MaxSoftComparisonsPerSide:    80,
	}
	plan := BuildMergePlan(left, right, opt)
	if len(plan.Alignments.Individuals) < 1 {
		t.Fatalf("expected soft alignment, got %+v", plan.Alignments.Individuals)
	}
	var sawSoft bool
	for _, a := range plan.Alignments.Individuals {
		if a.Scorecard.Stage >= StageNameDate {
			sawSoft = true
		}
	}
	if !sawSoft {
		t.Fatalf("expected at least one stage>=3 alignment, got %+v", plan.Alignments.Individuals)
	}
}

func TestNewReconciliationSession(t *testing.T) {
	plan := &MergePlan{Version: 2, ScoringProfileID: "t"}
	s := NewReconciliationSession(plan)
	if s == nil || s.ID == "" || s.MergePlan != plan || s.Status != "draft" {
		t.Fatalf("session: %+v", s)
	}
	s.WithInputSummary(3, 5)
	if s.InputSummary.LeftIndividualCount != 3 || s.InputSummary.RightIndividualCount != 5 {
		t.Fatalf("summary: %+v", s.InputSummary)
	}
}

func TestIndividualDiff_AttachmentFingerprints(t *testing.T) {
	left := &enricher.EnrichedDocument{
		Individuals: []enricher.EnrichedIndividual{
			{ID: "u1", Xref: "@I1@", FullName: "John"},
		},
		Notes: []enricher.EnrichedNote{{Content: "note-a"}},
		IndividualNotes: []enricher.IndividualNoteLink{{IndividualXref: "@I1@", NoteIndex: 0}},
	}
	right := &enricher.EnrichedDocument{
		Individuals: []enricher.EnrichedIndividual{
			{ID: "u1", Xref: "@I1@", FullName: "John"},
		},
		Notes: []enricher.EnrichedNote{{Content: "note-b"}},
		IndividualNotes: []enricher.IndividualNoteLink{{IndividualXref: "@I1@", NoteIndex: 0}},
	}
	plan := BuildMergePlan(left, right, nil)
	if len(plan.IndividualDiffs) != 1 {
		t.Fatalf("diffs: %d", len(plan.IndividualDiffs))
	}
	d := plan.IndividualDiffs[0]
	if len(d.NoteKeysOnlyLeft) == 0 && len(d.NoteKeysOnlyRight) == 0 {
		t.Fatalf("expected note multiset diff, got left=%v right=%v", d.NoteKeysLeft, d.NoteKeysRight)
	}
}
