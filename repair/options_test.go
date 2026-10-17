package repair

import (
	"bytes"
	"os"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
)

var allFixes = []Fix{FixEncoding, FixBOM, FixLineEndings, FixBlankLines, FixLeadingSpace, FixLongLines, FixTrailer}

func TestFixValues(t *testing.T) {
	want := []string{"encoding", "bom", "line-endings", "blank-lines", "leading-space", "long-lines", "trailer",
		"broken-links", "empty-families", "submitter"}
	for i, f := range Fixes() {
		if string(f) != want[i] {
			t.Errorf("fix %d = %q, want %q", i, f, want[i])
		}
	}
	if !reflect.DeepEqual(Fixes()[:len(allFixes)], allFixes) || len(Fixes()) != len(want) {
		t.Errorf("Fixes() = %v", Fixes())
	}
}

// fixOfWhat is the kind each What belongs to, by its prefix.
func fixOfWhat(what string) Fix {
	for _, p := range []struct {
		prefix string
		fix    Fix
	}{
		{"converted from ", FixEncoding},
		{"set the character set", FixEncoding},
		{"converted ", FixEncoding}, // stray Windows-1252 characters
		{"replaced ", FixEncoding},
		{"joined a character", FixEncoding},
		{"read ", FixEncoding},
		{"removed the byte order mark", FixBOM}, // FixEncoding for UTF-16, see below
		{"changed line endings", FixLineEndings},
		{"removed a blank line", FixBlankLines},
		{"removed spaces before", FixLeadingSpace},
		{"split a ", FixLongLines},
		{"added the end-of-file marker", FixTrailer},
	} {
		if strings.HasPrefix(what, p.prefix) {
			return p.fix
		}
	}
	return ""
}

// extraInputs are in-memory files for the subset tests, beside the samples.
func extraInputs() map[string][]byte {
	return map[string][]byte{
		"ansel messy":            []byte(anselMessy),
		"ansel long":             anselLong(),
		"ansel blank in join":    []byte("0 HEAD\r\n1 CHAR ANSEL\r\n0 @I1@ INDI\r\n1 NOTE Ren\xE2\r\n\r\n2 CONC e and more\r\n0 TRLR\r\n"),
		"ansel space in join":    []byte("0 HEAD\r\n1 CHAR ANSEL\r\n0 @I1@ INDI\r\n1 NOTE Ren\xE2\r\n  2 CONC e and more\r\n0 TRLR\r\n"),
		"mixed blank in join":    []byte("0 HEAD\r\n1 CHAR UTF-8\r\n0 @I1@ INDI\r\n1 NAME Ren\xC3\xA9e /Dupr\xC3\xA9/\r\n1 NOTE Jean \xC3\r\n\r\n2 CONC \x89mile\r\n0 TRLR\r\n"),
		"mixed cr in join":       []byte("0 HEAD\r1 CHAR UTF-8\r0 @I1@ INDI\r1 NAME Ren\xC3\xA9e /Dupr\xC3\xA9/\r1 NOTE Jean \xC3\r  2 CONC \x89mile\r0 TRLR\r"),
		"bom stray char ansel":   []byte("\xEF\xBB\xBF0 HEAD\n1 CHAR ANSEL\n0 @I1@ INDI\n1 NAME Ren\xC3\xA9e\n1 NOTE caf\xE9\n0 TRLR\n"),
		"bom utf8 messy":         []byte("\xEF\xBB\xBF0 HEAD\r1 CHAR UTF-8\r\r  0 @I1@ INDI\r1 NAME Ren\xC3\xA9e\r1 NOTE " + strings.Repeat("ab ", 100) + "z\r"),
		"utf16 messy":            utf16Messy(),
		"cp1252 nbsp blank line": []byte("0 HEAD\r\n1 CHAR ANSI\r\n0 @I1@ INDI\r\n1 NAME Ren\xE9e\r\n\xA0\r\n1 SEX F\r\n  1 NOTE x\r\n0 TRLR\r\n"),
	}
}

// anselMessy has ANSEL accents (é = E2 65, Ü = E8 55), blank lines,
// leading spaces, and no trailer.
const anselMessy = "0 HEAD\r\n1 GEDC\r\n2 VERS 5.5.1\r\n1 CHAR ANSEL\r\n\r\n0 @I1@ INDI\r\n  1 NAME Ren\xE2e /Dupr\xE2e/\r\n\r\n 1 BIRT\r\n\t2 PLAC Z\xE8urich\r\n"

func anselLong() []byte {
	return []byte("0 HEAD\r\n1 CHAR ANSEL\r\n0 @I1@ INDI\r\n1 NOTE " + strings.Repeat("Ren\xE2e ", 50) + "x\r\n\r\n0 TRLR\r\n")
}

func utf16Messy() []byte {
	b := []byte{0xFF, 0xFE}
	for _, u := range utf16.Encode([]rune("0 HEAD\r\n1 CHAR UNICODE\r\n\r\n  0 @I1@ INDI\r\n1 NAME René /Dupré/\r\n")) {
		b = append(b, byte(u), byte(u>>8))
	}
	return b
}

// subsetInputs is every sample file, the unreadable inputs and the extras.
func subsetInputs(t *testing.T) map[string][]byte {
	in := goldenInputs(t)
	for name, raw := range extraInputs() {
		in["extra/"+name] = raw
	}
	for name, raw := range round1Inputs() {
		in["round1/"+name] = raw
	}
	return in
}

type selection struct {
	name  string
	fixes []Fix
}

// selections is each single kind and all but each kind.
func selections() []selection {
	var out []selection
	for _, f := range allFixes {
		out = append(out, selection{"only " + string(f), []Fix{f}})
		but := slices.DeleteFunc(slices.Clone(allFixes), func(g Fix) bool { return g == f })
		out = append(out, selection{"all but " + string(f), but})
	}
	return out
}

func sortedNames(m map[string][]byte) []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Every Change carries the kind its What belongs to. Removing a UTF-16 byte
// order mark is part of the conversion.
func TestEveryChangeHasItsFix(t *testing.T) {
	inputs := subsetInputs(t)
	for _, name := range sortedNames(inputs) {
		raw := inputs[name]
		res, err := Repair(raw)
		if err != nil {
			continue
		}
		utf16 := bytes.HasPrefix(raw, []byte{0xFF, 0xFE}) || bytes.HasPrefix(raw, []byte{0xFE, 0xFF})
		for _, c := range res.Changes {
			want := fixOfWhat(c.What)
			if want == FixBOM && utf16 {
				want = FixEncoding
			}
			if want == "" || c.Fix != want {
				t.Errorf("%s: %+v has Fix %q, want %q", name, c, c.Fix, want)
			}
		}
	}
}

func stripBOM(b []byte) []byte { return bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF}) }

// subsequence reports the first line of want that is not found, in order, in
// got. A want term of "*" matches any terminator.
func subsequence(want, got []fileLine, withTerm bool) (fileLine, bool) {
	j := 0
	for _, w := range want {
		for j < len(got) && (got[j].text != w.text || (withTerm && w.term != "*" && got[j].term != w.term)) {
			j++
		}
		if j == len(got) {
			return w, false
		}
		j++
	}
	return fileLine{}, true
}

func TestSubsetsKeepSafetyRules(t *testing.T) {
	inputs := subsetInputs(t)
	for _, name := range sortedNames(inputs) {
		raw := inputs[name]
		full, err := RepairWithOptions(raw, Options{})
		if err != nil {
			continue // no GEDCOM lines; covered by TestRepairEmpty
		}
		before := enrichedJSON(t, raw)
		sels := selections()
		if isBig(raw) { // one raw-byte and one converting subset
			sels = sels[:2] // only encoding, all but encoding
		}
		for _, sel := range sels {
			t.Run(name+"/"+sel.name, func(t *testing.T) {
				opt := Options{Fixes: sel.fixes}
				res, err := RepairWithOptions(raw, opt)
				if err != nil {
					t.Fatal(err)
				}
				// The content is unchanged.
				if after := enrichedJSON(t, res.Data); after != before {
					t.Errorf("enriched content changed (changes %v)\n%s", res.Changes, firstDiff(before, after))
				}
				// Every Change carries a selected Fix.
				changed := map[int]bool{}
				crlf := false
				for _, c := range res.Changes {
					if !slices.Contains(sel.fixes, c.Fix) {
						t.Errorf("Change %+v is not in the selection", c)
					}
					changed[c.Line] = true
					crlf = crlf || c.Fix == FixLineEndings
				}
				// Lines with no Change are byte-identical: those of the
				// original bytes, or of the decoded text after a conversion.
				conv := slices.Contains(sel.fixes, FixEncoding)
				var orig []fileLine
				if conv {
					decoded, _, _ := parser.Decode(raw)
					orig = lines(decoded)
				} else {
					orig = lines(stripBOM(raw))
				}
				var keep []fileLine
				for i, l := range orig {
					if changed[i+1] {
						continue
					}
					if i == len(orig)-1 {
						l.term = "*" // may get a terminator before an added TRLR
					}
					keep = append(keep, l)
				}
				out := lines(stripBOM(res.Data))
				if miss, ok := subsequence(keep, out, !crlf); !ok {
					t.Errorf("unchanged line %q (%q) not kept", miss.text, miss.term)
				}
				// Not converted: the bytes stay the original encoding, so the
				// BOM stays unless removed.
				if !conv && bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}) != bytes.HasPrefix(res.Data, []byte{0xEF, 0xBB, 0xBF}) &&
					!slices.Contains(sel.fixes, FixBOM) {
					t.Error("BOM changed without FixBOM")
				}
				// A second run with the same options changes nothing.
				again, err := RepairWithOptions(res.Data, opt)
				if err != nil {
					t.Fatal(err)
				}
				if len(again.Changes) != 0 || !bytes.Equal(again.Data, res.Data) {
					t.Errorf("second run changed %v", again.Changes)
				}
				// Available does not depend on the selection.
				if !reflect.DeepEqual(res.Available, full.Available) {
					t.Errorf("Available %+v, with all fixes %+v", res.Available, full.Available)
				}
				// No Changes means the same bytes.
				if len(res.Changes) == 0 && !bytes.Equal(res.Data, raw) {
					t.Error("no changes but different bytes")
				}
			})
		}
	}
}

// Available lists every kind the file could use, in catalogue order, with
// the same counts whatever was selected.
func TestAvailable(t *testing.T) {
	cases := []struct {
		name string
		raw  []byte
		want []FixInfo
	}{
		{"ansel messy", []byte(anselMessy), []FixInfo{
			{FixEncoding, 3, false, "", kindOf(FixEncoding), nil}, // CHAR and the two lines with accents
			{FixBlankLines, 2, false, "", kindOf(FixBlankLines), nil},
			{FixLeadingSpace, 3, false, "", kindOf(FixLeadingSpace), nil},
			{FixTrailer, 1, false, "", kindOf(FixTrailer), nil},
			{FixSubmitter, 3, false, "", KindData, nil}, // 5.5.1 and no SUBM in HEAD
		}},
		{"ansel long", anselLong(), []FixInfo{
			{FixEncoding, 2, false, "", kindOf(FixEncoding), nil},
			{FixBlankLines, 1, false, "", kindOf(FixBlankLines), nil},
			{FixLongLines, 1, false, reasonNeedsUTF8, kindOf(FixLongLines), nil},
		}},
		{"utf16 messy", utf16Messy(), []FixInfo{
			{FixEncoding, 5, true, reasonUTF16, kindOf(FixEncoding), nil}, // every line
			{FixBlankLines, 1, false, reasonNeedsUTF8, kindOf(FixBlankLines), nil},
			{FixLeadingSpace, 1, false, reasonNeedsUTF8, kindOf(FixLeadingSpace), nil},
			{FixTrailer, 1, false, reasonNeedsUTF8, kindOf(FixTrailer), nil},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, sel := range append(selections(), selection{"none", []Fix{}}) {
				res, err := RepairWithOptions(c.raw, Options{Fixes: sel.fixes})
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(res.Available, c.want) {
					t.Errorf("%s: Available\n got  %+v\n want %+v", sel.name, res.Available, c.want)
				}
			}
		})
	}
}

// Without FixEncoding an ANSEL file keeps its ANSEL bytes; the blank lines and
// leading spaces go.
func TestANSELWithoutEncoding(t *testing.T) {
	res, err := RepairWithOptions([]byte(anselMessy), Options{Fixes: []Fix{FixBlankLines, FixLeadingSpace}})
	if err != nil {
		t.Fatal(err)
	}
	want := "0 HEAD\r\n1 GEDC\r\n2 VERS 5.5.1\r\n1 CHAR ANSEL\r\n0 @I1@ INDI\r\n1 NAME Ren\xE2e /Dupr\xE2e/\r\n1 BIRT\r\n2 PLAC Z\xE8urich\r\n"
	if string(res.Data) != want {
		t.Errorf("Data:\n%q\nwant\n%q", res.Data, want)
	}
	text, info, _ := parser.Decode(res.Data)
	if info.Detected != "ANSEL" {
		t.Errorf("repaired file reads as %s", info.Detected)
	}
	wantText := "0 HEAD\r\n1 GEDC\r\n2 VERS 5.5.1\r\n1 CHAR ANSEL\r\n0 @I1@ INDI\r\n1 NAME René /Dupré/\r\n1 BIRT\r\n2 PLAC Zürich\r\n"
	if string(text) != wantText {
		t.Errorf("decoded:\n%q\nwant\n%q", text, wantText)
	}
	assertChanges(t, res.Changes, []lw{
		{5, "removed a blank line"},
		{7, "removed spaces before the level number"},
		{8, "removed a blank line"},
		{9, "removed spaces before the level number"},
		{10, "removed spaces before the level number"},
	})
	assertLeftHas(t, res, "ENCODING_NOT_UTF8", true)
	assertLeftHas(t, res, "MISSING_TRAILER", true)
	assertLeftHas(t, res, "EMPTY_LINE", false)
}

// A UTF-16 file without FixEncoding is returned as it is.
func TestUTF16WithoutEncoding(t *testing.T) {
	raw := utf16Messy()
	all := slices.DeleteFunc(slices.Clone(allFixes), func(f Fix) bool { return f == FixEncoding })
	res, err := RepairWithOptions(raw, Options{Fixes: all})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(res.Data, raw) || len(res.Changes) != 0 {
		t.Errorf("Data changed or Changes %v", res.Changes)
	}
	if len(res.Available) == 0 || !reflect.DeepEqual(res.Available[0], FixInfo{FixEncoding, 5, true, "this file is UTF-16, so it has to be converted first", kindOf(FixEncoding), nil}) {
		t.Errorf("Available %+v", res.Available)
	}
	for _, a := range res.Available {
		if a.Fix == FixBOM {
			t.Error("the UTF-16 byte order mark is offered on its own")
		}
	}
	// With FixEncoding the UTF-16 mark goes with the conversion.
	res, _ = RepairWithOptions(raw, Options{Fixes: []Fix{FixEncoding}})
	if bytes.HasPrefix(res.Data, []byte{0xFF, 0xFE}) || bytes.HasPrefix(res.Data, []byte{0xEF, 0xBB, 0xBF}) {
		t.Error("byte order mark kept")
	}
	for _, c := range res.Changes {
		if c.Fix != FixEncoding {
			t.Errorf("Change %+v", c)
		}
	}
}

// FixLongLines without FixEncoding is skipped on an ANSEL file.
func TestLongLinesNeedUTF8(t *testing.T) {
	raw := anselLong()
	res, err := RepairWithOptions(raw, Options{Fixes: []Fix{FixLongLines}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(res.Data, raw) || len(res.Changes) != 0 {
		t.Errorf("Changes %v", res.Changes)
	}
	assertLeftHas(t, res, "LINE_TOO_LONG", true)
	found := false
	for _, a := range res.Available {
		if a.Fix == FixLongLines {
			found = true
			if a.Reason != "needs the conversion to UTF-8" || a.Required {
				t.Errorf("long-lines %+v", a)
			}
		}
	}
	if !found {
		t.Error("long-lines not listed")
	}
	// With the conversion it is split.
	res, _ = RepairWithOptions(raw, Options{Fixes: []Fix{FixEncoding, FixLongLines}})
	assertLeftHas(t, res, "LINE_TOO_LONG", false)
}

// A UTF-8 file with a byte order mark keeps it unless FixBOM is chosen, and
// may have its long lines split without a conversion.
func TestUTF8BOMKeptUnlessChosen(t *testing.T) {
	raw := extraInputs()["bom utf8 messy"]
	res, err := RepairWithOptions(raw, Options{Fixes: []Fix{FixEncoding, FixLongLines, FixTrailer}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(res.Data, []byte{0xEF, 0xBB, 0xBF}) {
		t.Error("BOM removed without FixBOM")
	}
	assertLeftHas(t, res, "LINE_TOO_LONG", false)
	res, _ = RepairWithOptions(raw, Options{Fixes: []Fix{FixBOM}})
	if !bytes.Equal(res.Data, raw[3:]) {
		t.Errorf("Data %q", res.Data)
	}
	assertChanges(t, res.Changes, []lw{{0, "removed the byte order mark"}})
}

// A BOM that overrules a legacy CHAR cannot go on its own: without it the
// file would be read as ANSEL.
func TestBOMOverrulingCharNeedsConversion(t *testing.T) {
	raw := extraInputs()["bom stray char ansel"]
	res, err := RepairWithOptions(raw, Options{Fixes: []Fix{FixBOM}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(res.Data, raw) || len(res.Changes) != 0 {
		t.Errorf("Changes %v", res.Changes)
	}
	if !containsInfo(res.Available, FixInfo{FixBOM, 1, false, reasonNeedsUTF8, kindOf(FixBOM), nil}) {
		t.Errorf("Available %+v", res.Available)
	}
}

// Without the conversion, a blank line or leading spaces next to a character
// the decoder joins across lines stay, so the file reads the same.
func TestRawEditsKeepJoins(t *testing.T) {
	for _, name := range []string{"ansel blank in join", "ansel space in join", "mixed blank in join", "mixed cr in join"} {
		t.Run(name, func(t *testing.T) {
			raw := extraInputs()[name]
			res, err := RepairWithOptions(raw, Options{Fixes: []Fix{FixBlankLines, FixLeadingSpace, FixLineEndings}})
			if err != nil {
				t.Fatal(err)
			}
			if enrichedJSON(t, raw) != enrichedJSON(t, res.Data) {
				t.Errorf("content changed: %q", res.Data)
			}
			for _, c := range res.Changes {
				if c.Line == 5 || (c.Line == 0 && name != "mixed cr in join") {
					t.Errorf("Change %+v next to the joined character", c)
				}
			}
			if name == "mixed cr in join" && !strings.Contains(string(res.Data), "Jean \xC3\r  2 CONC") {
				t.Errorf("Data %q", res.Data)
			}
		})
	}
}

// Without the conversion a blank line made of U+00A0 (0xA0 in Windows-1252)
// stays, and leading spaces away from non-ASCII line ends go.
func TestRawWindows1252(t *testing.T) {
	raw := extraInputs()["cp1252 nbsp blank line"]
	res, err := RepairWithOptions(raw, Options{Fixes: []Fix{FixBlankLines, FixLeadingSpace}})
	if err != nil {
		t.Fatal(err)
	}
	want := "0 HEAD\r\n1 CHAR ANSI\r\n0 @I1@ INDI\r\n1 NAME Ren\xE9e\r\n\xA0\r\n1 SEX F\r\n1 NOTE x\r\n0 TRLR\r\n"
	if string(res.Data) != want {
		t.Errorf("Data %q", res.Data)
	}
	assertChanges(t, res.Changes, []lw{{7, "removed spaces before the level number"}})
}

func TestUnknownFix(t *testing.T) {
	if _, err := RepairWithOptions([]byte("0 HEAD\n0 TRLR\n"), Options{Fixes: []Fix{"colour"}}); err == nil {
		t.Error("unknown fix accepted")
	}
}

// Options{} and Repair are the same call.
func TestRepairIsAllOptions(t *testing.T) {
	raw, err := os.ReadFile("../testdata/repair/blank-lines.ged")
	if err != nil {
		t.Fatal(err)
	}
	a, _ := Repair(raw)
	b, _ := RepairWithOptions(raw, Options{})
	c, _ := RepairWithOptions(raw, Options{Fixes: allFixes})
	if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(a, c) {
		t.Error("Repair, Options{} and all fixes differ")
	}
}

// containsInfo is slices.Contains for FixInfo (which holds a slice).
func containsInfo(list []FixInfo, want FixInfo) bool {
	for _, f := range list {
		if reflect.DeepEqual(f, want) {
			return true
		}
	}
	return false
}
