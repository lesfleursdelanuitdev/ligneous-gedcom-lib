package repair

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// Review round 3 probes, with hand-written expected bytes (TestDataKindsKeepContent
// rebuilds its expectation from the Changes, so it cannot catch a wrong
// removal on its own).

const headSubm = "0 HEAD\n1 GEDC\n2 VERS 5.5.1\n1 CHAR UTF-8\n1 SUBM @U1@\n0 @U1@ SUBM\n1 NAME Me\n"

// Only pointer tags carry links: text with @ in it (CONT/CONC, NOTE text),
// custom tags and non-pointer tags stay; every pointer tag to a missing
// record goes.
var pointerTagsGED = headSubm +
	"0 @I1@ INDI\n1 NAME A /B/\n" +
	"1 NOTE see\n2 CONT @I9@\n2 CONC @X9@\n" + // text, not links
	"1 _EMAIL @foo@\n1 _LINK @Z9@\n" + // custom tags
	"1 RESN @x@\n" + // not a pointer tag
	"1 NOTE @N9@\n1 ALIA @I8@\n1 ANCI @U8@\n1 DESI @U7@\n1 SUBM @U6@\n1 OBJE @M9@\n1 SOUR @S9@\n" +
	"1 FAMS @F9@\n1 FAMC @F8@\n1 ASSO @I7@\n2 RELA x\n" +
	"1 BIRT\n2 PLAC @home@\n2 NOTE @N8@\n" +
	"0 @F1@ FAM\n1 HUSB @I6@\n1 WIFE @I5@\n1 CHIL @I4@\n1 CHIL @I1@\n" +
	"0 @S1@ SOUR\n1 REPO @R9@\n" +
	"0 @N1@ NOTE text\n1 CONT @I3@\n1 CONC @I2@\n" +
	"0 TRLR\n"

func TestBrokenLinksOnlyPointerTags(t *testing.T) {
	in := pointerTagsGED
	want := headSubm +
		"0 @I1@ INDI\n1 NAME A /B/\n" +
		"1 NOTE see\n2 CONT @I9@\n2 CONC @X9@\n" +
		"1 _EMAIL @foo@\n1 _LINK @Z9@\n" +
		"1 RESN @x@\n" +
		"1 BIRT\n2 PLAC @home@\n" +
		"0 @F1@ FAM\n1 CHIL @I1@\n" +
		"0 @S1@ SOUR\n" +
		"0 @N1@ NOTE text\n1 CONT @I3@\n1 CONC @I2@\n" +
		"0 TRLR\n"
	res, err := RepairWithOptions([]byte(in), Options{Fixes: []Fix{FixBrokenLinks}})
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Data) != want {
		t.Errorf("Data:\n%s\nwant\n%s", res.Data, want)
	}
	if len(res.Changes) != 15 {
		t.Errorf("%d Changes: %v", len(res.Changes), res.Changes)
	}
}

// @VOID@ (GEDCOM 7) is a deliberate null pointer: never broken.
const voidGED = headSubm + "0 @I1@ INDI\n1 FAMC @VOID@\n2 PEDI birth\n1 BIRT\n2 SOUR @VOID@\n3 PAGE p. 4\n0 TRLR\n"

func TestBrokenLinksVoid(t *testing.T) {
	in := voidGED
	res, err := RepairWithOptions([]byte(in), Options{Fixes: []Fix{FixBrokenLinks}})
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Data) != in || len(res.Changes) != 0 {
		t.Errorf("Changes %v", res.Changes)
	}
	for _, a := range res.Available {
		if a.Fix == FixBrokenLinks {
			t.Errorf("broken-links offered: %+v", a)
		}
	}
}

// brokenFamGED: @F1@'s only member is a broken HUSB, @F2@'s only content
// is a broken NOTE (and CHAN); @F3@ has a real member.
const brokenFamGED = headSubm +
	"0 @I1@ INDI\n1 NAME A /B/\n1 FAMS @F1@\n1 FAMS @F2@\n1 FAMS @F3@\n" + // 8-12
	"0 @F1@ FAM\n1 HUSB @I9@\n" + // 13-14
	"0 @F2@ FAM\n1 NOTE @N9@\n1 CHAN\n2 DATE 1 JAN 2000\n" + // 15-18
	"0 @F3@ FAM\n1 HUSB @I1@\n1 NOTE @N9@\n" + // 19-21
	"0 TRLR\n" // 22

// With broken-links chosen too, empty-families counts broken links as
// gone: one run removes both families and their FAMS lines.
func TestBrokenLinksThenEmptyFamiliesInOneRun(t *testing.T) {
	opt := Options{Fixes: []Fix{FixEmptyFamilies, FixBrokenLinks}}
	res, err := RepairWithOptions([]byte(brokenFamGED), opt)
	if err != nil {
		t.Fatal(err)
	}
	want := headSubm + "0 @I1@ INDI\n1 NAME A /B/\n1 FAMS @F3@\n0 @F3@ FAM\n1 HUSB @I1@\n0 TRLR\n"
	if string(res.Data) != want {
		t.Errorf("Data:\n%s\nwant\n%s", res.Data, want)
	}
	wantChanges := []Change{
		{Line: 10, What: "removed a link to the removed family @F1@", Fix: FixEmptyFamilies},
		{Line: 11, What: "removed a link to the removed family @F2@", Fix: FixEmptyFamilies},
		{Line: 13, What: "removed an empty family @F1@", Fix: FixEmptyFamilies, Removed: []string{"1 HUSB @I9@"}},
		{Line: 15, What: "removed an empty family @F2@", Fix: FixEmptyFamilies, Removed: []string{"1 NOTE @N9@", "1 CHAN", "2 DATE 1 JAN 2000"}},
		{Line: 21, What: "removed a link to note @N9@, which doesn't exist", Fix: FixBrokenLinks},
	}
	if !reflect.DeepEqual(res.Changes, wantChanges) {
		t.Errorf("Changes\n got  %+v\n want %+v", res.Changes, wantChanges)
	}
	again, _ := RepairWithOptions(res.Data, opt)
	if len(again.Changes) != 0 {
		t.Errorf("second run %v", again.Changes)
	}
	// Alone, empty-families finds no empty family, and says why.
	alone, _ := RepairWithOptions([]byte(brokenFamGED), Options{Fixes: []Fix{FixEmptyFamilies}})
	if len(alone.Changes) != 0 {
		t.Errorf("alone: %v", alone.Changes)
	}
	if want := []FixInfo{{Fix: FixEmptyFamilies, Lines: 8, Reason: "needs broken-links too", Kind: KindData}}; !reflect.DeepEqual(alone.Skipped, want) {
		t.Errorf("alone: Skipped %+v", alone.Skipped)
	}
	if !reflect.DeepEqual(alone.Available, res.Available) {
		t.Errorf("Available depends on the selection:\n%+v\n%+v", alone.Available, res.Available)
	}
}

// "has its own information" only when no family qualifies.
func TestEmptyFamiliesReasonOnlyWhenNoneQualifies(t *testing.T) {
	res, _ := RepairWithOptions([]byte(emptyGED), Options{Fixes: []Fix{FixEmptyFamilies}})
	if !slices.Contains(res.Available, FixInfo{Fix: FixEmptyFamilies, Lines: 9, Kind: KindData}) {
		t.Errorf("Available %+v", res.Available)
	}
}

func TestCleanName(t *testing.T) {
	for in, want := range map[string]string{
		" Mona\t\tGon\u200bsalves\x07  x \n": "Mona Gonsalves x",
		"\ufeffRené\u200c \u200d Dupré\r\n":  "René Dupré",
		"a\u00a0\u00a0b":                     "a b",
		"\t\n\x07\u200b":                     "",
	} {
		if got := cleanName(in); got != want {
			t.Errorf("cleanName(%q) = %q, want %q", in, got, want)
		}
	}
}

// Every removed line is recorded: a blank line inside a unit counts and is
// listed as "".
const blankUnitGED = headSubm + "0 @I1@ INDI\n1 FAMC @F9@\n\n2 PEDI birth\n1 NAME A /B/\n0 TRLR\n"

func TestRemovedUnitCountsBlankLines(t *testing.T) {
	in := blankUnitGED
	for _, fixes := range [][]Fix{{FixBrokenLinks}, {FixBrokenLinks, FixBlankLines}} {
		res, err := RepairWithOptions([]byte(in), Options{Fixes: fixes})
		if err != nil {
			t.Fatal(err)
		}
		want := []Change{{Line: 9, What: "removed a link to family @F9@, which doesn't exist (and 2 lines under it)", Fix: FixBrokenLinks, Removed: []string{"", "2 PEDI birth"}}}
		if !reflect.DeepEqual(res.Changes, want) {
			t.Errorf("%v: Changes %+v", fixes, res.Changes)
		}
		if string(res.Data) != headSubm+"0 @I1@ INDI\n1 NAME A /B/\n0 TRLR\n" {
			t.Errorf("%v: Data %q", fixes, res.Data)
		}
	}
}

// An empty "1 SUBM" line in HEAD is filled in, not doubled.
const emptySubmGED = "0 HEAD\n1 GEDC\n2 VERS 5.5.1\n1 SUBM\n1 CHAR UTF-8\n0 TRLR\n"

func TestSubmitterFillsEmptySubm(t *testing.T) {
	in := emptySubmGED
	res, err := RepairWithOptions([]byte(in), Options{Fixes: []Fix{FixSubmitter}, SubmitterName: "Mona"})
	if err != nil {
		t.Fatal(err)
	}
	want := "0 HEAD\n1 GEDC\n2 VERS 5.5.1\n1 SUBM @SUBM1@\n1 CHAR UTF-8\n0 @SUBM1@ SUBM\n1 NAME Mona\n0 TRLR\n"
	if string(res.Data) != want {
		t.Errorf("Data:\n%q\nwant\n%q", res.Data, want)
	}
	assertChanges(t, res.Changes, []lw{
		{4, "added you as the person who made this file (Mona)"},
		{6, "added a submitter record for Mona"},
	})
	if strings.Count(string(res.Data), "1 SUBM") != 1 {
		t.Error("two SUBM lines")
	}
	if want := []FixInfo{{Fix: FixSubmitter, Lines: 3, Kind: KindData}}; !reflect.DeepEqual(res.Applied, want) {
		t.Errorf("Applied %+v", res.Applied)
	}
}

// Re-review: when every broken link sits inside a family that
// empty-families removes, broken-links is applied (with the family), not
// skipped.
var brokenInsideFamily = map[string]struct {
	in      string
	applied []FixInfo
}{
	"broken HUSB": {headSubm + "0 @I1@ INDI\n1 NAME A /B/\n1 FAMS @F1@\n0 @F1@ FAM\n1 HUSB @I9@\n0 TRLR\n",
		[]FixInfo{{Fix: FixBrokenLinks, Lines: 1, Kind: KindData}, {Fix: FixEmptyFamilies, Lines: 3, Kind: KindData}}},
	"broken SOUR with PAGE": {headSubm + "0 @F1@ FAM\n1 SOUR @S9@\n2 PAGE 4\n1 CHAN\n0 TRLR\n",
		[]FixInfo{{Fix: FixBrokenLinks, Lines: 2, Kind: KindData}, {Fix: FixEmptyFamilies, Lines: 4, Kind: KindData}}},
	"broken NOTE under FAMS, broken WIFE": {headSubm + "0 @I1@ INDI\n1 FAMS @F1@\n2 NOTE @N9@\n0 @F1@ FAM\n1 WIFE @I9@\n0 TRLR\n",
		[]FixInfo{{Fix: FixBrokenLinks, Lines: 2, Kind: KindData}, {Fix: FixEmptyFamilies, Lines: 4, Kind: KindData}}},
}

func TestBrokenLinksInsideRemovedFamily(t *testing.T) {
	for name, c := range brokenInsideFamily {
		for _, fixes := range [][]Fix{{FixBrokenLinks, FixEmptyFamilies}, {FixBrokenLinks, FixEmptyFamilies, FixEncoding}} {
			t.Run(name, func(t *testing.T) {
				res, err := RepairWithOptions([]byte(c.in), Options{Fixes: fixes})
				if err != nil {
					t.Fatal(err)
				}
				var applied []FixInfo
				for _, a := range res.Applied {
					if a.Kind == KindData {
						applied = append(applied, a)
					}
				}
				if !reflect.DeepEqual(applied, c.applied) {
					t.Errorf("%v: Applied %+v, want %+v", fixes, res.Applied, c.applied)
				}
				if len(res.Skipped) != 0 {
					t.Errorf("%v: Skipped %+v", fixes, res.Skipped)
				}
				if strings.Contains(string(res.Data), "@F1@") || strings.Contains(string(res.Data), "@I9@") {
					t.Errorf("%v: Data %q", fixes, res.Data)
				}
			})
		}
	}
}
