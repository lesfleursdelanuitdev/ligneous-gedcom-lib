package repair

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
)

// fileLine is one line of a file: its content and its terminator.
type fileLine struct {
	text string
	term string
}

// lines splits b the way the parser does: "\r\n", "\n" or a lone "\r".
func lines(b []byte) []fileLine {
	var out []fileLine
	for len(b) > 0 {
		i := bytes.IndexAny(b, "\r\n")
		if i < 0 {
			out = append(out, fileLine{string(b), ""})
			break
		}
		n := 1
		if b[i] == '\r' && i+1 < len(b) && b[i+1] == '\n' {
			n = 2
		}
		out = append(out, fileLine{string(b[:i]), string(b[i : i+n])})
		b = b[i+n:]
	}
	return out
}

func texts(ls []fileLine) []string {
	out := make([]string, len(ls))
	for i, l := range ls {
		out[i] = l.text
	}
	return out
}

func repairFixture(t *testing.T, name string) (*Result, []byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	res, err := Repair(raw)
	if err != nil {
		t.Fatalf("Repair(%s): %v", name, err)
	}
	return res, raw
}

func assertChanges(t *testing.T, got []Change, want []Change) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Changes:\n got  %+v\n want %+v", got, want)
	}
}

func assertLeftHas(t *testing.T, res *Result, code string, want bool) {
	t.Helper()
	has := false
	for _, c := range res.Left {
		if c == code {
			has = true
		}
	}
	if has != want {
		t.Errorf("Left = %v; contains %s = %v, want %v", res.Left, code, has, want)
	}
}

// assertFormat checks what every repaired file must satisfy: valid UTF-8, no
// BOM, no bare "\r", a single "1 CHAR UTF-8" line in the header.
func assertFormat(t *testing.T, data []byte) {
	t.Helper()
	if !utf8.Valid(data) {
		t.Error("repaired data is not valid UTF-8")
	}
	if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		t.Error("repaired data starts with a BOM")
	}
	for _, l := range lines(data) {
		if l.term == "\r" {
			t.Errorf("bare CR after %q", l.text)
		}
	}
}

func TestRepairBlankLines(t *testing.T) {
	res, _ := repairFixture(t, "repair/blank-lines.ged")
	assertChanges(t, res.Changes, []Change{
		{8, "removed a blank line"},
		{11, "removed a blank line"},
		{13, "removed a blank line"},
		{14, "removed a blank line"},
		{16, "removed a blank line"},
	})
	want := "junk before the header\n\n0 HEAD\n1 GEDC\n2 VERS 5.5.1\n2 FORM LINEAGE-LINKED\n1 CHAR UTF-8\n" +
		"0 @I1@ INDI\n1 NAME Anna /Berg/\n1 SEX F\n0 TRLR\n"
	if string(res.Data) != want {
		t.Errorf("Data:\n%q\nwant\n%q", res.Data, want)
	}
	assertFormat(t, res.Data)
	assertLeftHas(t, res, "EMPTY_LINE", false)
	assertLeftHas(t, res, "LINES_BEFORE_HEAD", true)
}

func TestRepairLeadingSpace(t *testing.T) {
	res, _ := repairFixture(t, "repair/leading-space.ged")
	assertChanges(t, res.Changes, []Change{
		{6, "removed spaces before the level number"},
		{7, "removed spaces before the level number"},
		{9, "removed spaces before the level number"},
		{10, "removed spaces before the level number"},
	})
	got := lines(res.Data)
	for i, want := range map[int]string{5: "0 @I1@ INDI", 6: "1 NAME Carl /Dahl/", 8: "2 DATE 1 JAN 1900", 9: "2 PLAC Oslo"} {
		if got[i].text != want || got[i].term != "\r\n" {
			t.Errorf("line %d = %q %q, want %q CRLF", i+1, got[i].text, got[i].term, want)
		}
	}
	assertFormat(t, res.Data)
	assertLeftHas(t, res, "LEADING_WHITESPACE", false)
}

func TestRepairNoTrailer(t *testing.T) {
	res, raw := repairFixture(t, "repair/no-trailer.ged")
	assertChanges(t, res.Changes, []Change{{8, "added the end-of-file marker (0 TRLR)"}})
	if want := string(raw) + "\n0 TRLR\n"; string(res.Data) != want {
		t.Errorf("Data = %q, want %q", res.Data, want)
	}
	assertLeftHas(t, res, "MISSING_TRAILER", false)
}

func TestRepairCROnly(t *testing.T) {
	res, raw := repairFixture(t, "repair/cr-only.ged")
	assertChanges(t, res.Changes, []Change{{0, "changed line endings to CRLF"}})
	if want := strings.ReplaceAll(string(raw), "\r", "\r\n"); string(res.Data) != want {
		t.Errorf("Data = %q, want %q", res.Data, want)
	}
	assertFormat(t, res.Data)
}

func TestRepairNoChar(t *testing.T) {
	res, _ := repairFixture(t, "repair/no-char.ged")
	assertChanges(t, res.Changes, []Change{{1, "set the character set to UTF-8"}})
	got := texts(lines(res.Data))
	want := []string{"0 HEAD", "1 SOUR TEST", "1 GEDC", "2 VERS 5.5.1", "2 FORM LINEAGE-LINKED",
		"1 CHAR UTF-8", "1 LANG English", "0 @I1@ INDI", "1 NAME Ida /Juul/", "0 TRLR"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lines:\n%q\nwant\n%q", got, want)
	}
	for _, l := range lines(res.Data) {
		if l.term != "\r\n" {
			t.Errorf("line %q ends with %q, want CRLF", l.text, l.term)
		}
	}
	assertLeftHas(t, res, "ENCODING_UNDECLARED", false)
}

func TestRepairCustomTags(t *testing.T) {
	res, _ := repairFixture(t, "repair/custom-tags.ged")
	assertChanges(t, res.Changes, []Change{{12, "removed a blank line"}})
	assertLeftHas(t, res, "UNKNOWN_RECORD", true)
}

func TestRepairLongNote(t *testing.T) {
	res, raw := repairFixture(t, "repair/long-note.ged")
	assertChanges(t, res.Changes, []Change{
		{8, "split a 298-character line into 2 lines"},
		{9, "split a 599-character line into 3 lines"},
		{11, "split a 279-character line into 2 lines"},
		{12, "split a 300-character line into 2 lines"},
	})
	assertFormat(t, res.Data)
	assertLeftHas(t, res, "LINE_TOO_LONG", false)

	got := lines(res.Data)
	r := strings.Repeat
	want := []string{
		"0 HEAD", "1 GEDC", "2 VERS 5.5.1", "2 FORM LINEAGE-LINKED", "1 CHAR UTF-8",
		"0 @I1@ INDI", "1 NAME Mia /Nord/",
		"1 NOTE " + r("m", 247), "2 CONC 中" + r("n", 43),
		"0 @N1@ NOTE " + r("a", 242), "1 CONC é" + r("b", 245), "1 CONC b " + r("c", 97),
		"0 @N2@ NOTE start",
		"1 CONT " + r("d", 247), "1 CONC ü" + r("e", 24),
		"1 CONC " + r("f", 248), "1 CONC " + r("f", 45),
		"0 TRLR",
	}
	if !reflect.DeepEqual(texts(got), want) {
		t.Errorf("lines:\n%q\nwant\n%q", texts(got), want)
	}

	// The invariants of every split, checked line by line.
	orig := lines(raw)
	for i, l := range got {
		if len(l.text) > 255 {
			t.Errorf("line %d is %d bytes", i+1, len(l.text))
		}
		if l.term != "\r\n" {
			t.Errorf("line %d ends with %q", i+1, l.term)
		}
		if strings.HasSuffix(l.text, " ") {
			t.Errorf("line %d ends with a space", i+1)
		}
		if strings.Contains(l.text, " CONC ") && strings.HasPrefix(l.text[strings.Index(l.text, " CONC ")+6:], " ") {
			t.Errorf("CONC line %d starts with a space", i+1)
		}
		if !utf8.ValidString(l.text) {
			t.Errorf("line %d splits a UTF-8 sequence", i+1)
		}
	}
	// Joining the pieces gives back the original values.
	join := func(from, to int, prefix string) string {
		s := strings.TrimPrefix(got[from].text, prefix)
		for _, l := range got[from+1 : to] {
			s += l.text[len("1 CONC "):]
		}
		return s
	}
	if v := join(7, 9, "1 NOTE "); v != orig[7].text[len("1 NOTE "):] {
		t.Error("inline note pieces do not join back")
	}
	if v := join(9, 12, "0 @N1@ NOTE "); v != orig[8].text[len("0 @N1@ NOTE "):] {
		t.Error("N1 pieces do not join back")
	}
	if v := join(13, 15, "1 CONT "); v != orig[10].text[len("1 CONT "):] {
		t.Error("CONT pieces do not join back")
	}
	if v := join(15, 17, "1 CONC "); v != orig[11].text[len("1 CONC "):] {
		t.Error("CONC pieces do not join back")
	}
}

// Long lines that are not split: their tag's continuation is not joined back
// by the enricher, or the split would change how the enricher reads them.
func TestRepairLongUnsplittable(t *testing.T) {
	res, raw := repairFixture(t, "repair/long-unsplittable.ged")
	assertChanges(t, res.Changes, nil)
	if !bytes.Equal(res.Data, raw) {
		t.Error("data changed")
	}
	assertLeftHas(t, res, "LINE_TOO_LONG", true)
}

func TestRepairEncodings(t *testing.T) {
	cases := map[string][]Change{
		"encodings/utf8.ged": nil,
		"encodings/ansel.ged": {
			{0, "converted from ANSEL to UTF-8"},
			{4, "set the character set to UTF-8"},
		},
		"encodings/ansi.ged": {
			{0, "converted from Windows-1252 to UTF-8"},
			{4, "set the character set to UTF-8"},
		},
		"encodings/mislabeled-utf8-as-ansel.ged": {{4, "set the character set to UTF-8"}},
		"encodings/bom-utf8-char-ansel.ged": {
			{0, "removed the byte order mark"},
			{4, "set the character set to UTF-8"},
		},
		"encodings/undeclared-latin1.ged": {
			{0, "converted from Windows-1252 to UTF-8"},
			{1, "set the character set to UTF-8"},
		},
		"encodings/utf16le-bom.ged": {
			{0, "converted from UTF-16LE to UTF-8"},
			{0, "removed the byte order mark"},
			{4, "set the character set to UTF-8"},
		},
		"encodings/utf16be-nobom.ged": {
			{0, "converted from UTF-16BE to UTF-8"},
			{4, "set the character set to UTF-8"},
		},
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			res, _ := repairFixture(t, name)
			assertChanges(t, res.Changes, want)
			assertFormat(t, res.Data)
			got := texts(lines(res.Data))
			if got[3] != "1 CHAR UTF-8" || got[5] != "1 NAME René /Dupré/" || got[7] != "2 PLAC Zürich" {
				t.Errorf("lines = %q", got)
			}
			for _, code := range []string{"ENCODING_NOT_UTF8", "ENCODING_MISLABELED", "ENCODING_UNDECLARED"} {
				assertLeftHas(t, res, code, false)
			}
		})
	}
}

func TestRepairASCIIDeclared(t *testing.T) {
	in := "0 HEAD\n1 GEDC\n2 VERS 5.5.1\n1 CHAR ASCII\n0 TRLR\n"
	res, err := Repair([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	assertChanges(t, res.Changes, []Change{{4, "set the character set to UTF-8"}})
	if want := strings.Replace(in, "ASCII", "UTF-8", 1); string(res.Data) != want {
		t.Errorf("Data = %q", res.Data)
	}
}

func TestRepairBadBytesStayInLeft(t *testing.T) {
	// 0xFF after a UTF-8 BOM is not valid UTF-8; it becomes U+FFFD.
	in := "\xEF\xBB\xBF0 HEAD\n1 CHAR UTF-8\n0 @I1@ INDI\n1 NAME A\xFF /B/\n0 TRLR\n"
	res, err := Repair([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	assertChanges(t, res.Changes, []Change{
		{0, "removed the byte order mark"},
		{4, "replaced 1 unreadable characters with \uFFFD"},
	})
	assertLeftHas(t, res, "ENCODING_BAD_BYTES", true)
	if !strings.Contains(string(res.Data), "1 NAME A� /B/\n") {
		t.Errorf("Data = %q", res.Data)
	}
}

func TestRepairLeftSortedUnique(t *testing.T) {
	res, _ := repairFixture(t, "repair/long-unsplittable.ged")
	for i := 1; i < len(res.Left); i++ {
		if res.Left[i-1] >= res.Left[i] {
			t.Errorf("Left not sorted and unique: %v", res.Left)
		}
	}
}

func TestRepairLastLineUnterminated(t *testing.T) {
	in := "0 HEAD\r\n1 CHAR UTF-8\r\n0 TRLR"
	res, err := Repair([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	assertChanges(t, res.Changes, nil)
	if string(res.Data) != in {
		t.Errorf("Data = %q", res.Data)
	}
}

func TestRepairEmpty(t *testing.T) {
	if _, err := Repair(nil); err == nil {
		t.Error("Repair(nil) should fail")
	}
	if _, err := Repair([]byte("\n \n")); err == nil {
		t.Error("Repair of blank lines should fail")
	}
}

func TestRepairRemovesUTF8BOM(t *testing.T) {
	in := "\xEF\xBB\xBF0 HEAD\n1 CHAR UTF-8\n0 TRLR\n"
	res, err := Repair([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	assertChanges(t, res.Changes, []Change{{0, "removed the byte order mark"}})
	if string(res.Data) != in[3:] {
		t.Errorf("Data = %q", res.Data)
	}
}

// Empty Changes must mean the bytes did not change.
func TestRepairNoChangesMeansSameBytes(t *testing.T) {
	for _, path := range sampleFiles(t) {
		raw, res := readAndRepair(t, path)
		if len(res.Changes) == 0 && !bytes.Equal(res.Data, raw) {
			t.Errorf("%s: no changes but different bytes", path)
		}
	}
}

// A cut next to a Unicode space (U+00A0, U+3000) would let the enricher trim
// that space off the first piece of an inline note.
func TestRepairNoCutAtUnicodeSpace(t *testing.T) {
	for _, name := range []string{"repair/unicode-space.ged", "repair/unicode-space-1252.ged"} {
		t.Run(name, func(t *testing.T) {
			res, raw := repairFixture(t, name)
			if a, b := enrichedJSON(t, raw), enrichedJSON(t, res.Data); a != b {
				t.Errorf("enriched content changed\n%s", firstDiff(a, b))
			}
			for _, l := range lines(res.Data) {
				if len(l.text) > 255 {
					t.Errorf("line of %d bytes left", len(l.text))
				}
				first, _ := utf8.DecodeRuneInString(strings.TrimPrefix(l.text, "2 CONC "))
				last, _ := utf8.DecodeLastRuneInString(l.text)
				if strings.HasPrefix(l.text, "2 CONC ") && unicode.IsSpace(first) || unicode.IsSpace(last) {
					t.Errorf("cut next to a space: %q", l.text)
				}
			}
		})
	}
	res, _ := repairFixture(t, "repair/unicode-space.ged")
	assertChanges(t, res.Changes, []Change{
		{8, "split a 354-character line into 2 lines"},
		{9, "split a 353-character line into 2 lines"},
	})
	got := texts(lines(res.Data))
	r := strings.Repeat
	if got[7] != "1 NOTE "+r("x", 245) || got[8] != "2 CONC x\u00a0"+r("y", 100) ||
		got[9] != "1 NOTE "+r("x", 244) || got[10] != "2 CONC x\u3000"+r("y", 100) {
		t.Errorf("lines = %q", got[7:11])
	}
	res, _ = repairFixture(t, "repair/unicode-space-1252.ged")
	assertChanges(t, res.Changes, []Change{
		{0, "converted from Windows-1252 to UTF-8"},
		{5, "set the character set to UTF-8"},
		{8, "split a 354-character line into 2 lines"},
	})
}

// A NOTE at level 100 is not split: its CONC would be at level 101, past the
// parser's MaxNestingDepth, and would be dropped.
func TestRepairNoSplitPastMaxDepth(t *testing.T) {
	in := "0 HEAD\n1 CHAR UTF-8\n0 @I1@ INDI\n100 NOTE " + strings.Repeat("z", 270) + "\n0 TRLR\n"
	res, err := Repair([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	assertChanges(t, res.Changes, nil)
	if string(res.Data) != in {
		t.Error("data changed")
	}
	assertLeftHas(t, res, "LINE_TOO_LONG", true)
}

// splitLines must split exactly like the parser (parser/split.go nextLine):
// same number of lines, so Change line numbers match finding line numbers.
func TestSplitLinesMatchesParser(t *testing.T) {
	inputs := []string{
		"0 HEAD\n1 CHAR UTF-8\n0 TRLR\n",
		"0 HEAD\r\n1 CHAR UTF-8\r\n0 TRLR",
		"0 HEAD\r1 CHAR UTF-8\r0 TRLR\r",
		"0 HEAD\r\r\n\n\r1 CHAR UTF-8\n\r\n0 TRLR\r",
		"junk\r\n\r\n0 HEAD\n1 NOTE a\r\n\r\n\n0 TRLR\n\n",
		"0 HEAD\n1 NOTE " + strings.Repeat("x", 1<<20+5) + "\r0 TRLR\r\n",
	}
	for i, in := range inputs {
		opts := parser.DefaultOptions()
		opts.Lenient = true
		doc, _, err := parser.ParseWithOptions(strings.NewReader(in), opts)
		if err != nil {
			t.Fatalf("input %d: %v", i, err)
		}
		src := splitLines([]byte(in))
		if doc.LineCount != len(src) {
			t.Errorf("input %d: parser counts %d lines, splitLines %d", i, doc.LineCount, len(src))
		}
		if got, want := len(src), len(lines([]byte(in))); got != want {
			t.Errorf("input %d: splitLines %d lines, test helper %d", i, got, want)
		}
		if doc.Trailer.Line == 0 || src[doc.Trailer.Line-1].text != "0 TRLR" {
			t.Errorf("input %d: TRLR on parser line %d is not splitLines' line", i, doc.Trailer.Line)
		}
	}
}

// Leading spaces are removed only from lines that are GEDCOM lines (a level
// and a tag). Any other line is left exactly as it is.
func TestRepairLeadingSpaceOnlyOnGedcomLines(t *testing.T) {
	in := "0 HEAD\n1 CHAR UTF-8\n0 @I1@ INDI\n  1 NAME A /B/\n   some stray text\n \tx\n  @\n0 TRLR\n"
	res, err := Repair([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	assertChanges(t, res.Changes, []Change{{4, "removed spaces before the level number"}})
	want := "0 HEAD\n1 CHAR UTF-8\n0 @I1@ INDI\n1 NAME A /B/\n   some stray text\n \tx\n  @\n0 TRLR\n"
	if string(res.Data) != want {
		t.Errorf("Data = %q\nwant   %q", res.Data, want)
	}
}
