package repair

import (
	"bytes"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/validator"
)

// brokenGED has a broken link of every kind, links that are not broken
// (custom record, wrong-type ASSO) and nested lines under some.
const brokenGED = "0 HEAD\n1 GEDC\n2 VERS 5.5.1\n1 CHAR UTF-8\n1 SUBM @U9@\n" + // 1-5
	"0 @I1@ INDI\n1 NAME Ann /Lund/\n" + // 6-7
	"1 FAMC @F9@\n2 PEDI birth\n" + // 8-9
	"1 BIRT\n2 DATE 1 JAN 1900\n2 SOUR @S9@\n3 PAGE p. 12\n3 QUAY 2\n" + // 10-14
	"1 ASSO @I9@\n2 RELA godfather\n" + // 15-16
	"1 ASSO @F1@\n2 RELA wrong type\n" + // 17-18
	"1 NOTE @N9@\n" + // 19
	"1 OBJE @M9@\n" + // 20
	"1 _EVT @X1@\n" + // 21
	"1 SOUR @S1@\n" + // 22
	"0 @F1@ FAM\n1 HUSB @I8@\n1 CHIL @I1@\n" + // 23-25
	"0 @S1@ SOUR\n1 TITL t\n1 REPO @R9@\n2 CALN 7\n" + // 26-29
	"0 @X1@ _EVDEF\n1 NAME custom\n" + // 30-31
	"0 TRLR\n" // 32

func TestBrokenLinks(t *testing.T) {
	res, err := RepairWithOptions([]byte(brokenGED), Options{Fixes: []Fix{FixBrokenLinks}})
	if err != nil {
		t.Fatal(err)
	}
	want := []Change{
		{Line: 5, What: "removed a link to submitter @U9@, which doesn't exist", Fix: FixBrokenLinks},
		{Line: 8, What: "removed a link to family @F9@, which doesn't exist (and 1 lines under it)", Fix: FixBrokenLinks, Removed: []string{"2 PEDI birth"}},
		{Line: 12, What: "removed a link to source @S9@, which doesn't exist (and 2 lines under it)", Fix: FixBrokenLinks, Removed: []string{"3 PAGE p. 12", "3 QUAY 2"}},
		{Line: 15, What: "removed a link to person @I9@, which doesn't exist (and 1 lines under it)", Fix: FixBrokenLinks, Removed: []string{"2 RELA godfather"}},
		{Line: 19, What: "removed a link to note @N9@, which doesn't exist", Fix: FixBrokenLinks},
		{Line: 20, What: "removed a link to media @M9@, which doesn't exist", Fix: FixBrokenLinks},
		{Line: 24, What: "removed a link to person @I8@, which doesn't exist", Fix: FixBrokenLinks},
		{Line: 28, What: "removed a link to repository @R9@, which doesn't exist (and 1 lines under it)", Fix: FixBrokenLinks, Removed: []string{"2 CALN 7"}},
	}
	if !reflect.DeepEqual(res.Changes, want) {
		t.Errorf("Changes\n got  %+v\n want %+v", res.Changes, want)
	}
	gone := map[int]bool{5: true, 8: true, 9: true, 12: true, 13: true, 14: true, 15: true, 16: true, 19: true, 20: true, 24: true, 28: true, 29: true}
	var kept []string
	for i, l := range strings.SplitAfter(brokenGED, "\n") {
		if !gone[i+1] {
			kept = append(kept, l)
		}
	}
	if got := string(res.Data); got != strings.Join(kept, "") {
		t.Errorf("Data:\n%s", got)
	}
	if want := []FixInfo{{Fix: FixBrokenLinks, Lines: 13, Kind: KindData}}; !reflect.DeepEqual(res.Applied, want) {
		t.Errorf("Applied %+v", res.Applied)
	}
	codes := validatorCodes(t, res.Data)
	for c := range codes {
		if strings.HasPrefix(c, "ORPHANED_") && c != "ORPHANED_XREF_OTHER" {
			t.Errorf("%s left", c)
		}
	}
	if !codes["INVALID_ASSOCIATE_TARGET"] {
		t.Error("the wrong-type ASSO was removed")
	}
	if !strings.Contains(string(res.Data), "1 _EVT @X1@\n") {
		t.Error("the link to the custom record was removed")
	}
	again, _ := RepairWithOptions(res.Data, Options{Fixes: []Fix{FixBrokenLinks}})
	if len(again.Changes) != 0 {
		t.Errorf("second run %v", again.Changes)
	}
	assertDataContent(t, []byte(brokenGED), res, true)
}

// Data kinds never run by default, and are listed as data.
func TestDataKindsNotDefault(t *testing.T) {
	res, err := Repair([]byte(brokenGED))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range res.Changes {
		if kindOf(c.Fix) == KindData {
			t.Errorf("default repair made %+v", c)
		}
	}
	found := false
	for _, a := range res.Available {
		if a.Kind != kindOf(a.Fix) || a.Kind == "" {
			t.Errorf("Available %+v has Kind %q", a, a.Kind)
		}
		if a.Fix == FixBrokenLinks {
			found = a.Lines == 13 && a.Kind == KindData
		}
	}
	if !found {
		t.Errorf("broken-links not available as data: %+v", res.Available)
	}
	if !reflect.DeepEqual(Fixes()[:7], allFixes) || !slices.Contains(Fixes(), FixBrokenLinks) {
		t.Errorf("Fixes() = %v", Fixes())
	}
}

// Without the conversion, broken links go and the file stays ANSEL.
func TestBrokenLinksANSEL(t *testing.T) {
	in := "0 HEAD\r\n1 CHAR ANSEL\r\n0 @I1@ INDI\r\n1 NAME Ren\xE2e /B/\r\n1 FAMC @F9@\r\n2 PEDI birth\r\n0 TRLR\r\n"
	res, err := RepairWithOptions([]byte(in), Options{Fixes: []Fix{FixBrokenLinks}})
	if err != nil {
		t.Fatal(err)
	}
	if want := "0 HEAD\r\n1 CHAR ANSEL\r\n0 @I1@ INDI\r\n1 NAME Ren\xE2e /B/\r\n0 TRLR\r\n"; string(res.Data) != want {
		t.Errorf("Data %q", res.Data)
	}
}

// A link next to a character the decoder may join stays, and says why.
func TestBrokenLinksGuard(t *testing.T) {
	in := "0 HEAD\n0 @I1@ INDI\n1 NOTE abc\xC3\n1 FAMC @F9@\n1 CONC \xA9def\n1 SOUR @S9@\n0 TRLR\n"
	res, err := RepairWithOptions([]byte(in), Options{Fixes: []Fix{FixBrokenLinks}})
	if err != nil {
		t.Fatal(err)
	}
	if a, b := enrichedJSON(t, []byte(in)), enrichedJSON(t, res.Data); strings.Contains(a, "©") != strings.Contains(b, "©") {
		t.Errorf("reading changed: %q", res.Data)
	}
	assertChanges(t, res.Changes, []lw{{6, "removed a link to source @S9@, which doesn't exist"}})
}

func validatorCodes(t *testing.T, data []byte) map[string]bool {
	t.Helper()
	opts := parser.DefaultOptions()
	opts.Lenient = true
	doc, _, err := parser.ParseWithOptions(bytes.NewReader(data), opts)
	if err != nil {
		t.Fatal(err)
	}
	set := map[string]bool{}
	for _, e := range validator.Validate(doc) {
		set[e.Code] = true
	}
	return set
}

// levelOf is the level of a GEDCOM line, or -1.
func levelOf(s string) int {
	f := strings.Fields(s)
	if len(f) < 2 {
		return -1
	}
	n, err := strconv.Atoi(f[0])
	if err != nil {
		return -1
	}
	return n
}

// unitEnd is the last line of the unit starting at line i: the line and
// every following line up to the last one deeper than it.
func unitEnd(ls []fileLine, i int) int {
	lvl, end := levelOf(ls[i].text), i
	for j := i + 1; j < len(ls); j++ {
		l := levelOf(ls[j].text)
		if l >= 0 && l <= lvl {
			break
		}
		if l > lvl {
			end = j
		}
	}
	return end
}

// expectedAfterData builds, from the decoded original, the text the data
// Changes describe: each removed unit gone, the submitter lines added.
func expectedAfterData(t *testing.T, raw []byte, res *Result) ([]fileLine, []fileLine) {
	t.Helper()
	decoded, _, _ := parser.Decode(raw)
	orig := lines(decoded)
	drop := map[int]bool{}
	var adds []Change
	for _, c := range res.Changes {
		if kindOf(c.Fix) != KindData {
			continue
		}
		if strings.HasPrefix(c.What, "added ") {
			adds = append(adds, c)
			continue
		}
		end := unitEnd(orig, c.Line-1)
		var nested []string
		for j := c.Line - 1; j <= end; j++ {
			drop[j] = true
			if j > c.Line-1 && strings.TrimSpace(orig[j].text) != "" {
				nested = append(nested, orig[j].text)
			}
		}
		if !slices.Equal(nested, c.Removed) {
			t.Errorf("%+v: the unit's lines are %q", c, nested)
		}
	}
	var want []fileLine
	for i, l := range orig {
		if !drop[i] {
			want = append(want, l)
		}
	}
	_ = adds
	return orig, want
}

// assertDataContent: the result reads exactly like the original with the
// units the Changes name removed (and the submitter added); with data kinds
// only, every other line is the same text.
func assertDataContent(t *testing.T, raw []byte, res *Result, dataOnly bool) {
	t.Helper()
	orig, want := expectedAfterData(t, raw, res)
	got, _, _ := parser.Decode(res.Data)
	gotLines := lines(got)
	var wantText strings.Builder
	for _, l := range want {
		wantText.WriteString(l.text + "\n")
	}
	added := addedIdx(orig, gotLines)
	if dataOnly {
		var g, w []string
		for i, l := range gotLines {
			if !added[i] {
				g = append(g, l.text)
			}
		}
		for _, l := range want {
			w = append(w, l.text)
		}
		if !slices.Equal(g, w) {
			t.Errorf("lines differ from the original minus the removed units:\n got  %q\n want %q", g, w)
		}
	}
	var gotText strings.Builder
	for i, l := range gotLines {
		if !added[i] {
			gotText.WriteString(l.text + "\n")
		}
	}
	if a, b := enrichedJSON(t, []byte(wantText.String())), enrichedJSON(t, []byte(gotText.String())); a != b {
		t.Errorf("enriched content differs beyond the removed units\n%s", firstDiff(a, b))
	}
}

// addedIdx is the index in got of each line the submitter fix added: a
// "1 SUBM @SUBMn@" or "0 @SUBMn@ SUBM" line the original does not have, and
// the NAME line after the record line.
func addedIdx(orig, got []fileLine) map[int]bool {
	had := map[string]bool{}
	for _, l := range orig {
		had[l.text] = true
	}
	set := map[int]bool{}
	for i, l := range got {
		if had[l.text] {
			continue
		}
		if strings.HasPrefix(l.text, "1 SUBM @SUBM") {
			set[i] = true
		}
		if strings.HasPrefix(l.text, "0 @SUBM") && strings.HasSuffix(l.text, "@ SUBM") {
			set[i] = true
			if i+1 < len(got) && strings.HasPrefix(got[i+1].text, "1 NAME ") {
				set[i+1] = true
			}
		}
	}
	return set
}

// dataSelections: each data kind alone, all data kinds, and data kinds with
// each format kind and with all of them.
func dataSelections(kinds []Fix) []selection {
	var out []selection
	for _, k := range kinds {
		out = append(out, selection{"only " + string(k), []Fix{k}})
	}
	out = append(out, selection{"all data", kinds})
	for _, f := range allFixes {
		out = append(out, selection{"all data + " + string(f), append(slices.Clone(kinds), f)})
	}
	out = append(out, selection{"everything", append(slices.Clone(kinds), allFixes...)})
	return out
}

var implementedData = []Fix{FixBrokenLinks}

func dataInputs(t *testing.T) map[string][]byte {
	in := subsetInputs(t)
	in["data/broken"] = []byte(brokenGED)
	return in
}

// Every data selection, on every input: the content is the original minus
// the named units; a second run changes nothing; the order of Fixes does
// not matter; Available does not depend on the selection.
func TestDataKindsKeepContent(t *testing.T) {
	inputs := dataInputs(t)
	names := sortedNames(inputs)
	for _, name := range names {
		raw := inputs[name]
		base, err := Repair(raw)
		if err != nil {
			continue
		}
		for _, sel := range dataSelections(implementedData) {
			opt := Options{Fixes: sel.fixes, SubmitterName: "Test Person"}
			res, err := RepairWithOptions(raw, opt)
			if err != nil {
				t.Fatalf("%s/%s: %v", name, sel.name, err)
			}
			onlyData := true
			for _, f := range sel.fixes {
				onlyData = onlyData && kindOf(f) == KindData
			}
			t.Run(name+"/"+sel.name, func(t *testing.T) {
				assertDataContent(t, raw, res, onlyData)
				again, _ := RepairWithOptions(res.Data, opt)
				if len(again.Changes) != 0 {
					t.Errorf("second run %v", again.Changes)
				}
				rev := slices.Clone(sel.fixes)
				slices.Reverse(rev)
				shuffled, _ := RepairWithOptions(raw, Options{Fixes: rev, SubmitterName: opt.SubmitterName})
				if !bytes.Equal(shuffled.Data, res.Data) || !reflect.DeepEqual(shuffled.Changes, res.Changes) {
					t.Error("the order of Fixes changed the result")
				}
				if !reflect.DeepEqual(res.Available, base.Available) {
					t.Errorf("Available %+v, by default %+v", res.Available, base.Available)
				}
				for _, c := range res.Changes {
					if !slices.Contains(sel.fixes, c.Fix) {
						t.Errorf("%+v not selected", c)
					}
				}
				// No new broken links.
				before, after := orphans(t, raw), orphans(t, res.Data)
				for k := range after {
					if !before[k] {
						t.Errorf("new %s", k)
					}
				}
			})
		}
	}
}

// orphans is the set of "code pointer" for ORPHANED_* findings.
func orphans(t *testing.T, data []byte) map[string]bool {
	t.Helper()
	opts := parser.DefaultOptions()
	opts.Lenient = true
	doc, _, err := parser.ParseWithOptions(bytes.NewReader(data), opts)
	if err != nil || doc == nil {
		return nil
	}
	set := map[string]bool{}
	for _, e := range validator.Validate(doc) {
		if strings.HasPrefix(e.Code, "ORPHANED_") {
			set[e.Code+" "+e.RelatedXref] = true
		}
	}
	return set
}
