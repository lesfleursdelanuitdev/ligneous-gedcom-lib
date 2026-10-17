package repair

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

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
	out := texts(lines(decoded))
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
			// Added lines (CHAR, trailer, submitter, a split's pieces) are
			// in the result exactly, in order, one after another.
			if len(g.Lines) > 0 && !hasRun(out, g.Lines) {
				t.Errorf("%s: unit %d lines %q are not a run of the result", what, i, g.Lines)
			}
			var pieces int
			if _, err := fmt.Sscanf(g.What, "split a %d-character line into %d lines", new(int), &pieces); err == nil && pieces != len(g.Lines) {
				t.Errorf("%s: unit %d has %d pieces for %q", what, i, len(g.Lines), g.What)
			}
			switch {
			case g.What == whatCharset && !slices.Equal(g.Lines, []string{"1 CHAR UTF-8"}),
				k == FixTrailer && !slices.Equal(g.Lines, []string{"0 TRLR"}),
				k == FixSubmitter && strings.HasPrefix(g.What, "added you") && (len(g.Lines) != 1 || !strings.HasPrefix(g.Lines[0], "1 SUBM @SUBM")),
				k == FixSubmitter && strings.HasPrefix(g.What, "added a submitter") && (len(g.Lines) != 2 || !strings.HasPrefix(g.Lines[1], "1 NAME ")):
				t.Errorf("%s: unit %d Lines %q for %q", what, i, g.Lines, g.What)
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

// Review: a UTF-8 file with a byte order mark, no CHAR and a stray byte. A
// non-ASCII name cannot be written without the conversion (the result would
// read differently), so the submitter is skipped up front, and Plan agrees.
const bomStrayGED = "\xEF\xBB\xBF0 HEAD\n1 GEDC\n2 VERS 5.5.1\n0 @I1@ INDI\n1 NAME Ren\xE9\n0 TRLR\n"

func TestSubmitterNonASCIINeedsConversionWhenBytesDiffer(t *testing.T) {
	for _, name := range []string{"René", "李"} {
		opt := Options{Fixes: []Fix{FixSubmitter}, SubmitterName: name}
		res, err := RepairWithOptions([]byte(bomStrayGED), opt)
		if err != nil {
			t.Fatal(err)
		}
		if want := []FixInfo{{Fix: FixSubmitter, Lines: 3, Reason: "needs the conversion to UTF-8", Kind: KindData}}; !reflect.DeepEqual(res.Skipped, want) {
			t.Errorf("%s: Skipped %+v", name, res.Skipped)
		}
		pr, _ := Plan([]byte(bomStrayGED), Options{Fixes: []Fix{}, SubmitterName: name})
		if u := pr.Units[FixSubmitter]; len(u) != 0 {
			t.Errorf("%s: Plan units %+v", name, u)
		}
		// With the conversion it is written.
		res, _ = RepairWithOptions([]byte(bomStrayGED), Options{Fixes: []Fix{FixSubmitter, FixEncoding}, SubmitterName: name})
		if !strings.Contains(string(res.Data), "1 NAME "+name) {
			t.Errorf("%s: converted: %q", name, res.Data)
		}
	}
	// An ASCII name needs nothing.
	res, _ := RepairWithOptions([]byte(bomStrayGED), Options{Fixes: []Fix{FixSubmitter}, SubmitterName: "Mona"})
	if len(res.Skipped) != 0 || !strings.Contains(string(res.Data), "1 NAME Mona") {
		t.Errorf("ASCII name: Skipped %+v", res.Skipped)
	}
}

// guardFamGED (ANSI): @F1@ is empty; @F2@ is empty once its broken HUSB
// goes, but its FAMS line follows a line ending in a non-ASCII byte, so the
// guard keeps it whatever is chosen. Its lines are skipped because of the
// guard, not because broken-links is missing.
const guardFamGED = "0 HEAD\n1 GEDC\n2 VERS 5.5.1\n1 CHAR ANSI\n1 SUBM @U1@\n0 @U1@ SUBM\n1 NAME Me\n" + // 1-7
	"0 @I1@ INDI\n1 FAMS @F1@\n1 NAME Ren\xE9\n1 FAMS @F2@\n" + // 8-11
	"0 @F1@ FAM\n0 @F2@ FAM\n1 HUSB @I9@\n0 TRLR\n" // 12-15

func TestEmptyFamiliesGuardNotBlamedOnBrokenLinks(t *testing.T) {
	for _, fixes := range [][]Fix{{FixEmptyFamilies}, {FixEmptyFamilies, FixBrokenLinks}} {
		res, err := RepairWithOptions([]byte(guardFamGED), Options{Fixes: fixes})
		if err != nil {
			t.Fatal(err)
		}
		var applied, skipped []FixInfo
		for _, a := range res.Applied {
			if a.Fix == FixEmptyFamilies {
				applied = append(applied, a)
			}
		}
		for _, s := range res.Skipped {
			if s.Fix == FixEmptyFamilies {
				skipped = append(skipped, s)
			}
		}
		if want := []FixInfo{{Fix: FixEmptyFamilies, Lines: 2, Kind: KindData}}; !reflect.DeepEqual(applied, want) {
			t.Errorf("%v: Applied %+v", fixes, applied)
		}
		if want := []FixInfo{{Fix: FixEmptyFamilies, Lines: 3, Reason: "skipped: it would change how the file is read", Kind: KindData}}; !reflect.DeepEqual(skipped, want) {
			t.Errorf("%v: Skipped %+v", fixes, skipped)
		}
	}
}

// Picking a free @SUBMn@ is linear: 20,000 "@SUBMk@" values in use.
func TestSubmitterXrefPickIsFast(t *testing.T) {
	var b strings.Builder
	b.WriteString("0 HEAD\n1 GEDC\n2 VERS 5.5.1\n1 CHAR UTF-8\n0 @I1@ INDI\n")
	for k := 1; k <= 20000; k++ {
		fmt.Fprintf(&b, "1 _X @SUBM%d@\n1 NOTE %s\n", k, strings.Repeat("x", 12))
	}
	b.WriteString("0 TRLR\n")
	in := []byte(b.String())
	start := time.Now()
	res, err := RepairWithOptions(in, Options{Fixes: []Fix{FixSubmitter}, SubmitterName: "Mona"})
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("took %v for %d bytes", d, len(in))
	}
	if !strings.Contains(string(res.Data), "0 @SUBM20001@ SUBM\n") {
		t.Errorf("xref not @SUBM20001@: %v", res.Changes)
	}
}

// hasRun reports whether want occurs in out as consecutive lines.
func hasRun(out, want []string) bool {
	for i := 0; i+len(want) <= len(out); i++ {
		if slices.Equal(out[i:i+len(want)], want) {
			return true
		}
	}
	return false
}
