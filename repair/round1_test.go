package repair

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// Review probe 1: a UTF-8 character split across a blank line, in a file
// read as Windows-1252 (no CHAR, or CHAR ANSI). Removing the blank line
// would let the decoder join the character.
func probeSplitAcrossBlank(char string) []byte {
	return []byte("0 HEAD\n" + char + "0 @I1@ INDI\n  1 SEX M\n1 NOTE abc\xC3\n\n2 CONC \xA9def\n")
}

// Review probe 2: CHAR ANSEL more than 50 lines into HEAD, so the decoder
// does not see it; removing blank lines in HEAD would bring it into view.
func probeCharOutOfWindow() []byte {
	return []byte("0 HEAD\n" + strings.Repeat("1 NOTE x\n", 48) + "\n\n\n1 CHAR ANSEL\n0 @I1@ INDI\n  1 NAME Ren\xE2e\n")
}

func round1Inputs() map[string][]byte {
	return map[string][]byte{
		"probe split no CHAR":      probeSplitAcrossBlank(""),
		"probe split CHAR ANSI":    probeSplitAcrossBlank("1 CHAR ANSI\n"),
		"probe CHAR out of window": probeCharOutOfWindow(),
	}
}

// One unsafe edit no longer drops the others: the trailer and the leading
// spaces are fixed, and the blank lines are reported as skipped.
func TestProbesKeepTheSafeFixes(t *testing.T) {
	sel := []Fix{FixBlankLines, FixLeadingSpace, FixTrailer}
	for name, raw := range round1Inputs() {
		t.Run(name, func(t *testing.T) {
			res, err := RepairWithOptions(raw, Options{Fixes: sel})
			if err != nil {
				t.Fatal(err)
			}
			if a, b := enrichedJSON(t, raw), enrichedJSON(t, res.Data); a != b {
				t.Fatalf("content changed\n%s", firstDiff(a, b))
			}
			kinds := map[Fix]bool{}
			for _, c := range res.Changes {
				kinds[c.Fix] = true
			}
			if !kinds[FixTrailer] || !kinds[FixLeadingSpace] || kinds[FixBlankLines] {
				t.Errorf("Changes %+v", res.Changes)
			}
			want := FixInfo{Fix: FixBlankLines, Lines: len(touchedBlank(raw)), Reason: reasonReadChange, Kind: KindFormat}
			if !slices.Contains(res.Skipped, want) {
				t.Errorf("Skipped %+v, want %+v in it", res.Skipped, want)
			}
		})
	}
}

// touchedBlank is the blank-line Changes of a repair with every fix.
func touchedBlank(raw []byte) []Change {
	res, _ := Repair(raw)
	var out []Change
	for _, c := range res.Changes {
		if c.Fix == FixBlankLines {
			out = append(out, c)
		}
	}
	return out
}

// When the file still reads differently, the adjacency-changing kinds go
// first; when even that is not enough, everything is skipped, and said so.
func TestRereadRetry(t *testing.T) {
	saved := rereadOK
	defer func() { rereadOK = saved }()
	raw := []byte(anselMessy)
	sel := Options{Fixes: []Fix{FixBlankLines, FixLeadingSpace, FixTrailer}}

	calls := 0
	rereadOK = func(out, want []byte) bool { calls++; return calls > 1 }
	res, err := RepairWithOptions(raw, sel)
	if err != nil {
		t.Fatal(err)
	}
	assertChanges(t, res.Changes, []lw{{11, "added the end-of-file marker (0 TRLR)"}})
	wantSkipped := []FixInfo{{FixBlankLines, 2, false, reasonReadChange, kindOf(FixBlankLines)}, {FixLeadingSpace, 3, false, reasonReadChange, kindOf(FixLeadingSpace)}}
	if !reflect.DeepEqual(res.Skipped, wantSkipped) {
		t.Errorf("Skipped %+v", res.Skipped)
	}
	if want := []FixInfo{{Fix: FixTrailer, Lines: 1, Kind: KindFormat}}; !reflect.DeepEqual(res.Applied, want) {
		t.Errorf("Applied %+v", res.Applied)
	}

	rereadOK = func(out, want []byte) bool { return false }
	res, _ = RepairWithOptions(raw, sel)
	if !bytes.Equal(res.Data, raw) || len(res.Changes) != 0 || len(res.Applied) != 0 {
		t.Errorf("Changes %v Applied %v", res.Changes, res.Applied)
	}
	wantSkipped = append(wantSkipped, FixInfo{FixTrailer, 1, false, reasonReadChange, kindOf(FixTrailer)})
	if !reflect.DeepEqual(res.Skipped, wantSkipped) {
		t.Errorf("Skipped %+v", res.Skipped)
	}
}

// Every chosen kind the file could use either changes something or is in
// Skipped with a reason. Applied counts what really changed.
func TestChosenKindsAreAppliedOrSkipped(t *testing.T) {
	inputs := subsetInputs(t)
	for _, name := range sortedNames(inputs) {
		raw := inputs[name]
		if _, err := Repair(raw); err != nil {
			continue
		}
		for _, sel := range append(selections(), selection{"all", allFixes}, selection{"none", []Fix{}}) {
			res, err := RepairWithOptions(raw, Options{Fixes: sel.fixes})
			if err != nil {
				t.Fatal(err)
			}
			lines := lineSets{}
			for _, c := range res.Changes {
				lines.mark(c.Fix, max(c.Line, 1))
			}
			applied := map[Fix]int{}
			for _, a := range res.Applied {
				applied[a.Fix] = a.Lines
				if a.Lines <= 0 || !slices.Contains(sel.fixes, a.Fix) || len(lines[a.Fix]) == 0 {
					t.Errorf("%s/%s: Applied %+v", name, sel.name, a)
				}
			}
			skipped := map[Fix]string{}
			for _, s := range res.Skipped {
				skipped[s.Fix] = s.Reason
				if s.Reason == "" || !slices.Contains(sel.fixes, s.Fix) || applied[s.Fix] > 0 {
					t.Errorf("%s/%s: Skipped %+v", name, sel.name, s)
				}
			}
			for f := range lines {
				if applied[f] == 0 {
					t.Errorf("%s/%s: Changes of %s but not in Applied", name, sel.name, f)
				}
			}
			for _, av := range res.Available {
				if !slices.Contains(sel.fixes, av.Fix) {
					continue
				}
				if applied[av.Fix] == 0 && skipped[av.Fix] == "" {
					t.Errorf("%s/%s: %s chosen and available but neither applied nor skipped", name, sel.name, av.Fix)
				}
			}
		}
	}
}

func TestAppliedCounts(t *testing.T) {
	res, _ := RepairWithOptions([]byte(anselMessy), Options{Fixes: []Fix{FixBlankLines}})
	if want := []FixInfo{{Fix: FixBlankLines, Lines: 2, Kind: KindFormat}}; !reflect.DeepEqual(res.Applied, want) || len(res.Skipped) != 0 {
		t.Errorf("Applied %+v Skipped %+v", res.Applied, res.Skipped)
	}
	// The U+00A0 blank line is not removed: 0 lines, and skipped.
	res, _ = RepairWithOptions(extraInputs()["cp1252 nbsp blank line"], Options{Fixes: []Fix{FixBlankLines, FixLeadingSpace}})
	if want := []FixInfo{{Fix: FixLeadingSpace, Lines: 1, Kind: KindFormat}}; !reflect.DeepEqual(res.Applied, want) {
		t.Errorf("Applied %+v", res.Applied)
	}
	if want := []FixInfo{{FixBlankLines, 1, false, reasonReadChange, kindOf(FixBlankLines)}}; !reflect.DeepEqual(res.Skipped, want) {
		t.Errorf("Skipped %+v", res.Skipped)
	}
	// Skipped for want of the conversion.
	res, _ = RepairWithOptions(anselLong(), Options{Fixes: []Fix{FixLongLines}})
	if want := []FixInfo{{FixLongLines, 1, false, reasonNeedsUTF8, kindOf(FixLongLines)}}; !reflect.DeepEqual(res.Skipped, want) {
		t.Errorf("Skipped %+v", res.Skipped)
	}
	res, _ = RepairWithOptions(utf16Messy(), Options{Fixes: []Fix{FixBlankLines, FixTrailer}})
	if want := []FixInfo{{FixBlankLines, 1, false, reasonNeedsUTF8, kindOf(FixBlankLines)}, {FixTrailer, 1, false, reasonNeedsUTF8, kindOf(FixTrailer)}}; !reflect.DeepEqual(res.Skipped, want) {
		t.Errorf("Skipped %+v", res.Skipped)
	}
}

// The line-endings message counts the endings when some bare CRs stay.
func TestPartialLineEndingsMessage(t *testing.T) {
	res, err := RepairWithOptions(extraInputs()["mixed cr in join"], Options{Fixes: []Fix{FixLineEndings}})
	if err != nil {
		t.Fatal(err)
	}
	assertChanges(t, res.Changes, []lw{{0, "changed 6 line endings to CRLF"}})
	if want := []FixInfo{{Fix: FixLineEndings, Lines: 6, Kind: KindFormat}}; !reflect.DeepEqual(res.Applied, want) {
		t.Errorf("Applied %+v", res.Applied)
	}
	// Every bare CR changed: the v0.2.1 wording.
	res, _ = RepairWithOptions(extraInputs()["bom utf8 messy"], Options{Fixes: []Fix{FixLineEndings}})
	assertChanges(t, res.Changes, []lw{{0, "changed line endings to CRLF"}})
}

func TestOptionsJSON(t *testing.T) {
	var o Options
	if err := json.Unmarshal([]byte(`{}`), &o); err != nil || o.Fixes != nil {
		t.Errorf("{} gives %#v, %v", o, err)
	}
	if err := json.Unmarshal([]byte(`{"fixes":[]}`), &o); err != nil || o.Fixes == nil || len(o.Fixes) != 0 {
		t.Errorf(`{"fixes":[]} gives %#v, %v`, o, err)
	}
	res, _ := RepairWithOptions([]byte(anselMessy), o)
	if len(res.Changes) != 0 {
		t.Errorf("no fixes chosen but %v", res.Changes)
	}
	b, _ := json.Marshal(Options{Fixes: []Fix{FixBOM}})
	if string(b) != `{"fixes":["bom"]}` {
		t.Errorf("%s", b)
	}
	b, _ = json.Marshal(res)
	for _, k := range []string{`"available":`, `"applied":[]`, `"skipped":[]`, `"changes":[]`} {
		if !strings.Contains(string(b), k) {
			t.Errorf("%s lacks %s", b, k)
		}
	}
}
