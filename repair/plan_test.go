package repair

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
)

// withKind is opt.Fixes (nil: every format kind) plus k.
func withKind(opt Options, k Fix) []Fix {
	base := opt.Fixes
	if base == nil {
		base = slices.Clone(allFixes)
	}
	if slices.Contains(base, k) {
		return slices.Clone(base)
	}
	return append(slices.Clone(base), k)
}

// unitsOf turns a repair's Changes of kind k into the Units Plan must give.
// Lines: the removed lines for data kinds (Removed), and for the others the
// lines added or removed, read back from the repaired file.
func unitsOf(t *testing.T, res *Result, k Fix) []Unit {
	t.Helper()
	out := []Unit{}
	for _, c := range res.Changes {
		if c.Fix == k {
			out = append(out, Unit{Line: c.Line, What: c.What, Lines: c.Removed})
		}
	}
	return out
}

// planSelections: the default, nothing, each kind alone, every data kind,
// and everything.
func planSelections() []Options {
	out := []Options{{}, {Fixes: []Fix{}}}
	for _, f := range Fixes() {
		out = append(out, Options{Fixes: []Fix{f}})
	}
	out = append(out, Options{Fixes: slices.Clone(dataKinds)}, Options{Fixes: Fixes()})
	for i := range out {
		out[i].SubmitterName = "Test Person"
	}
	return out
}

func optName(o Options) string {
	if o.Fixes == nil {
		return "default"
	}
	return fmt.Sprint(o.Fixes)
}

// For every input and selection: Available is RepairWithOptions's, Units
// has an entry for each available kind and no other, and each kind's Units
// are exactly the Changes (Line, What, Removed) that RepairWithOptions makes
// for opt.Fixes plus that kind.
func TestPlanMatchesRepair(t *testing.T) {
	inputs := dataInputs(t)
	for _, name := range sortedNames(inputs) {
		raw := inputs[name]
		if _, err := Repair(raw); err != nil {
			continue
		}
		sels := planSelections()
		if isBig(raw) {
			sels = []Options{{SubmitterName: "Test Person"}, {Fixes: []Fix{}, SubmitterName: "Test Person"}}
		}
		for _, opt := range sels {
			pr, err := Plan(raw, opt)
			if err != nil {
				t.Fatalf("%s/%s: %v", name, optName(opt), err)
			}
			res, _ := RepairWithOptions(raw, opt)
			if !reflect.DeepEqual(pr.Available, res.Available) {
				t.Errorf("%s/%s: Available\n plan   %+v\n repair %+v", name, optName(opt), pr.Available, res.Available)
			}
			for k := range pr.Units {
				found := false
				for _, a := range pr.Available {
					found = found || a.Fix == k
				}
				if !found {
					t.Errorf("%s/%s: Units for %s, which is not available", name, optName(opt), k)
				}
			}
			for _, av := range pr.Available {
				k := av.Fix
				got, ok := pr.Units[k]
				if !ok {
					t.Errorf("%s/%s: no Units for available %s", name, optName(opt), k)
					continue
				}
				want, _ := RepairWithOptions(raw, Options{Fixes: withKind(opt, k), SubmitterName: opt.SubmitterName})
				checkUnits(t, fmt.Sprintf("%s/%s/%s", name, optName(opt), k), got, want, k)
			}
		}
	}
}

// checkUnits compares Line and What with the Changes, Lines with Removed
// for data removals, and for the others with the lines in the result.
func checkUnits(t *testing.T, what string, got []Unit, res *Result, k Fix) {
	t.Helper()
	want := unitsOf(t, res, k)
	if len(got) != len(want) {
		t.Errorf("%s: %d units, the repair made %d changes\n got  %+v\n want %+v", what, len(got), len(want), got, want)
		return
	}
	decoded, _, _ := parser.Decode(res.Data)
	text := string(decoded)
	for i := range got {
		g, w := got[i], want[i]
		if g.Line != w.Line || g.What != w.What {
			t.Errorf("%s: unit %d is %d %q, the change %d %q", what, i, g.Line, g.What, w.Line, w.What)
			continue
		}
		switch {
		case kindOf(k) == KindData && !strings.HasPrefix(w.What, "added "):
			if !slices.Equal(g.Lines, w.Lines) {
				t.Errorf("%s: unit %d Lines %q, Removed %q", what, i, g.Lines, w.Lines)
			}
		case k == FixBlankLines:
			if !slices.Equal(g.Lines, []string{""}) {
				t.Errorf("%s: blank unit Lines %q", what, g.Lines)
			}
		default:
			for _, l := range g.Lines { // added lines are in the result
				if !strings.Contains(text, l+"\n") && !strings.Contains(text, l+"\r") {
					t.Errorf("%s: unit %d line %q not in the result", what, i, l)
				}
			}
		}
	}
}

// The reviewer's cases, with exact units.
func TestPlanFixtures(t *testing.T) {
	head := "0 HEAD\n1 GEDC\n2 VERS 5.5.1\n1 CHAR UTF-8\n1 SUBM @U1@\n0 @U1@ SUBM\n1 NAME Me\n"
	cases := []struct {
		name  string
		in    string
		opt   []Fix
		kind  Fix
		units []Unit
		needs []Fix
	}{
		{"blank line inside a FAM with a wife", head + "0 @I1@ INDI\n1 FAMS @F1@\n0 @F1@ FAM\n\n1 WIFE @I1@\n0 TRLR\n",
			nil, FixEmptyFamilies, nil, nil},
		{"_FAM pointer to an empty family", head + "0 @I1@ INDI\n1 _FAM @F1@\n0 @F1@ FAM\n0 TRLR\n",
			nil, FixEmptyFamilies, nil, nil},
		{"duplicate FAM xref", head + "0 @F1@ FAM\n0 @F1@ FAM\n1 HUSB @U1@\n0 TRLR\n",
			nil, FixEmptyFamilies, nil, nil},
		{"broken OBJE/SUBM/REPO inside a FAM, broken-links chosen", head + "0 @I1@ INDI\n1 FAMS @F1@\n0 @F1@ FAM\n1 OBJE @M9@\n1 SUBM @U9@\n1 REPO @R9@\n0 TRLR\n",
			[]Fix{FixBrokenLinks}, FixEmptyFamilies, []Unit{
				{Line: 9, What: "removed a link to the removed family @F1@"},
				{Line: 10, What: "removed an empty family @F1@", Lines: []string{"1 OBJE @M9@", "1 SUBM @U9@", "1 REPO @R9@"}},
			}, []Fix{FixBrokenLinks}},
		{"broken OBJE/SUBM/REPO inside a FAM, alone", head + "0 @I1@ INDI\n1 FAMS @F1@\n0 @F1@ FAM\n1 OBJE @M9@\n1 SUBM @U9@\n1 REPO @R9@\n0 TRLR\n",
			[]Fix{}, FixEmptyFamilies, []Unit{}, []Fix{FixBrokenLinks}},
		{"broken links listed once, with their family", head + "0 @I1@ INDI\n1 FAMS @F1@\n0 @F1@ FAM\n1 OBJE @M9@\n1 SUBM @U9@\n1 REPO @R9@\n0 TRLR\n",
			[]Fix{FixEmptyFamilies}, FixBrokenLinks, []Unit{}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pr, err := Plan([]byte(c.in), Options{Fixes: c.opt})
			if err != nil {
				t.Fatal(err)
			}
			var av *FixInfo
			for i := range pr.Available {
				if pr.Available[i].Fix == c.kind {
					av = &pr.Available[i]
				}
			}
			if c.units == nil {
				if av != nil || pr.Units[c.kind] != nil {
					t.Errorf("%s offered: %+v %+v", c.kind, av, pr.Units[c.kind])
				}
				return
			}
			if av == nil {
				t.Fatalf("%s not available: %+v", c.kind, pr.Available)
			}
			if !slices.Equal(av.Needs, c.needs) {
				t.Errorf("Needs %v, want %v", av.Needs, c.needs)
			}
			if got := pr.Units[c.kind]; !reflect.DeepEqual(got, c.units) {
				t.Errorf("Units\n got  %+v\n want %+v", got, c.units)
			}
		})
	}
}

// Plan never writes the file or re-checks it.
func TestPlanNeverEmits(t *testing.T) {
	emits, lefts := 0, 0
	onEmit, onLeftCodes = func() { emits++ }, func() { lefts++ }
	defer func() { onEmit, onLeftCodes = nil, nil }()
	for name, raw := range dataInputs(t) {
		if _, err := Plan(raw, Options{Fixes: Fixes(), SubmitterName: "x"}); err != nil {
			continue
		}
		if emits+lefts != 0 {
			t.Fatalf("%s: Plan emitted %d times, ran leftCodes %d times", name, emits, lefts)
		}
	}
	if _, err := Repair([]byte(brokenGED)); err != nil || emits == 0 || lefts == 0 {
		t.Fatalf("the hooks do not fire (emit %d, leftCodes %d)", emits, lefts)
	}
}

// partialFamGED: @F1@ is empty as it stands; @F2@ only once its broken HUSB
// is removed.
const partialFamGED = "0 HEAD\n1 GEDC\n2 VERS 5.5.1\n1 CHAR UTF-8\n1 SUBM @U1@\n0 @U1@ SUBM\n1 NAME Me\n" +
	"0 @F1@ FAM\n0 @F2@ FAM\n1 HUSB @I9@\n0 TRLR\n"

// empty-families alone does part of its work: Applied has the real count,
// Skipped says the rest needs broken-links.
func TestEmptyFamiliesPartialSkip(t *testing.T) {
	res, err := RepairWithOptions([]byte(partialFamGED), Options{Fixes: []Fix{FixEmptyFamilies}})
	if err != nil {
		t.Fatal(err)
	}
	if want := []FixInfo{{Fix: FixEmptyFamilies, Lines: 1, Kind: KindData}}; !reflect.DeepEqual(res.Applied, want) {
		t.Errorf("Applied %+v", res.Applied)
	}
	if want := []FixInfo{{Fix: FixEmptyFamilies, Lines: 2, Reason: "some families need “Remove links to records that don't exist” too", Kind: KindData}}; !reflect.DeepEqual(res.Skipped, want) {
		t.Errorf("Skipped %+v", res.Skipped)
	}
	full, _ := RepairWithOptions([]byte(partialFamGED), Options{Fixes: []Fix{FixEmptyFamilies, FixBrokenLinks}})
	if len(full.Skipped) != 0 {
		t.Errorf("with broken-links: Skipped %+v", full.Skipped)
	}
	for _, a := range res.Available {
		if a.Fix == FixEmptyFamilies && !slices.Equal(a.Needs, []Fix{FixBrokenLinks}) {
			t.Errorf("Needs %v", a.Needs)
		}
	}
}
