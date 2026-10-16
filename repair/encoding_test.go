package repair

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
)

// A UTF-8 file with one stray Latin-1 byte: the repaired file is the expected
// UTF-8 text, the stray byte's line has its own Change, and there is no
// file-level "converted from Windows-1252".
func TestRepairMixedUTF8Latin1(t *testing.T) {
	res, _ := repairFixture(t, "encodings/mixed-utf8-latin1.ged")
	want := "0 HEAD\r\n1 GEDC\r\n2 VERS 5.5.1\r\n1 CHAR UTF-8\r\n0 @I1@ INDI\r\n" +
		"1 NAME Renée /Dupré/\r\n1 BIRT\r\n2 PLAC Zürich\r\n1 NOTE Café au lait\r\n0 TRLR\r\n"
	if string(res.Data) != want {
		t.Errorf("Data:\n%q\nwant\n%q", res.Data, want)
	}
	assertChanges(t, res.Changes, []Change{{9, "converted 1 stray Windows-1252 characters to UTF-8"}})
	for _, code := range []string{"ENCODING_NOT_UTF8", "ENCODING_MISLABELED", "ENCODING_UNDECLARED", "ENCODING_BAD_BYTES"} {
		assertLeftHas(t, res, code, false)
	}
	got := enrichedJSON(t, res.Data)
	for _, s := range []string{`"Renée /Dupré/"`, `"Café au lait"`, `"Zürich"`} {
		if !strings.Contains(got, s) {
			t.Errorf("enriched output lacks %s", s)
		}
	}
	if strings.Contains(got, "Ã") {
		t.Error("enriched output has mojibake")
	}
}

// An ANSEL accent at the end of a NOTE line belongs to the CONC line's first
// letter: the repaired note reads "René and more".
func TestRepairANSELMarkAcrossConc(t *testing.T) {
	in := "0 HEAD\r\n1 GEDC\r\n2 VERS 5.5.1\r\n1 CHAR ANSEL\r\n0 @I1@ INDI\r\n1 NAME A /B/\r\n1 NOTE Ren\xE2\r\n2 CONC e and more\r\n0 TRLR\r\n"
	res, err := Repair([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	want := "0 HEAD\r\n1 GEDC\r\n2 VERS 5.5.1\r\n1 CHAR UTF-8\r\n0 @I1@ INDI\r\n1 NAME A /B/\r\n1 NOTE Ren\r\n2 CONC é and more\r\n0 TRLR\r\n"
	if string(res.Data) != want {
		t.Errorf("Data:\n%q\nwant\n%q", res.Data, want)
	}
	assertChanges(t, res.Changes, []Change{{0, "converted from ANSEL to UTF-8"}, {4, "set the character set to UTF-8"}})
	if got := enrichedJSON(t, res.Data); !strings.Contains(got, `"René and more"`) {
		t.Errorf("enriched note is not \"René and more\":\n%s", got)
	}
}

func utf16LE(s string) []byte {
	var b []byte
	for _, u := range utf16.Encode([]rune(s)) {
		b = append(b, byte(u), byte(u>>8))
	}
	return b
}

// unreadableInputs are files whose decoding writes U+FFFD.
func unreadableInputs() map[string][]byte {
	surrogate := append([]byte{0xFF, 0xFE}, utf16LE("0 HEAD\r\n1 CHAR UNICODE\r\n0 @I1@ INDI\r\n1 NOTE a")...)
	surrogate = append(surrogate, 0x00, 0xDC) // lone low surrogate
	surrogate = append(surrogate, utf16LE("b\r\n0 TRLR\r\n")...)
	return map[string][]byte{
		"BOM and stray byte":             []byte("\xEF\xBB\xBF0 HEAD\n1 CHAR UTF-8\n0 @I1@ INDI\n1 NAME A\xFF /B/\n0 TRLR\n"),
		"UTF-16 lone surrogate":          surrogate,
		"ANSEL undefined byte":           []byte("0 HEAD\r\n1 CHAR ANSEL\r\n0 @I1@ INDI\r\n1 NAME A\xFF\xFF /B/\r\n1 NOTE x\xFFy\xFF\r\n0 TRLR\r\n"),
		"ANSEL stranded mark":            []byte("0 HEAD\r\n1 CHAR ANSEL\r\n0 @I1@ INDI\r\n1 NOTE Ren\xE2\r\n1 SEX M\r\n0 TRLR\r\n"),
		"ANSEL mark before sibling CONC": []byte("0 HEAD\r\n1 CHAR ANSEL\r\n0 @I1@ INDI\r\n1 NOTE Ren\xE2\r\n1 CONC e\r\n0 TRLR\r\n"),
		"ANSEL mark before deep CONC":    []byte("0 HEAD\r\n1 CHAR ANSEL\r\n0 @I1@ INDI\r\n1 NOTE Ren\xE2\r\n3 CONC e\r\n0 TRLR\r\n"),
		"ANSEL mark at EOF":              []byte("0 HEAD\r\n1 CHAR ANSEL\r\n0 @I1@ INDI\r\n1 NOTE Ren\xE2"),
		"mixed undefined byte":           []byte("0 HEAD\n1 CHAR UTF-8\n0 @I1@ INDI\n1 NAME Renée /Dupré/\n1 NOTE a\x81b\n0 TRLR\n"),
		"ANSI undefined byte":            []byte("0 HEAD\n1 CHAR ANSI\n0 @I1@ INDI\n1 NAME Ren\xe9e\n1 NOTE a\x81b\n0 TRLR\n"),
		"ANSEL long line with bad":       []byte("0 HEAD\r\n1 CHAR ANSEL\r\n0 @I1@ INDI\r\n1 NOTE " + strings.Repeat("x", 300) + "\xFF\r\n0 TRLR\r\n"),
	}
}

func TestRepairUnreadableChanges(t *testing.T) {
	want := map[string][]Change{
		"BOM and stray byte": {{0, "removed the byte order mark"}, {4, "replaced 1 unreadable characters with �"}},
		"UTF-16 lone surrogate": {{0, "converted from UTF-16LE to UTF-8"}, {0, "removed the byte order mark"},
			{2, "set the character set to UTF-8"}, {4, "replaced 1 unreadable characters with �"}},
		"ANSEL undefined byte": {{0, "converted from ANSEL to UTF-8"}, {2, "set the character set to UTF-8"},
			{4, "replaced 2 unreadable characters with �"}, {5, "replaced 2 unreadable characters with �"}},
		"ANSEL stranded mark": {{0, "converted from ANSEL to UTF-8"}, {2, "set the character set to UTF-8"},
			{4, "replaced 1 unreadable characters with �"}},
		"ANSEL mark before sibling CONC": {{0, "converted from ANSEL to UTF-8"}, {2, "set the character set to UTF-8"},
			{4, "replaced 1 unreadable characters with �"}},
		"ANSEL mark before deep CONC": {{0, "converted from ANSEL to UTF-8"}, {2, "set the character set to UTF-8"},
			{4, "replaced 1 unreadable characters with �"}},
		"mixed undefined byte": {{5, "replaced 1 unreadable characters with �"}},
		"ANSI undefined byte": {{0, "converted from Windows-1252 to UTF-8"}, {2, "set the character set to UTF-8"},
			{5, "replaced 1 unreadable characters with �"}},
	}
	for name, raw := range unreadableInputs() {
		w, ok := want[name]
		if !ok {
			continue
		}
		t.Run(name, func(t *testing.T) {
			res, err := Repair(raw)
			if err != nil {
				t.Fatal(err)
			}
			assertChanges(t, res.Changes, w)
			assertLeftHas(t, res, "ENCODING_BAD_BYTES", true)
		})
	}
}

// Every U+FFFD that decoding wrote into the repaired file is covered by a
// "replaced N unreadable characters" Change on its original line, and N is
// the number of U+FFFD on that line.
func TestRepairEveryReplacementCharacterHasChange(t *testing.T) {
	inputs := unreadableInputs()
	for _, path := range sampleFiles(t) {
		raw, _ := readAndRepair(t, path)
		inputs[filepath.Base(path)] = raw
	}
	for name, raw := range inputs {
		t.Run(name, func(t *testing.T) {
			res, err := Repair(raw)
			if err != nil {
				t.Fatal(err)
			}
			decoded, _, _, fixes := parser.DecodeReport(raw)
			own := strings.Count(string(raw), "�") // U+FFFD already in a UTF-8 file
			if own > 0 {
				t.Skip("the file has its own U+FFFD")
			}
			if a, b := strings.Count(string(res.Data), "�"), strings.Count(string(decoded), "�"); a != b {
				t.Errorf("repaired file has %d U+FFFD, decoded %d", a, b)
			}
			byLine := map[int]int{}
			for _, f := range fixes {
				byLine[f.Line] = f.Unreadable
			}
			for i, l := range lines(decoded) {
				n := strings.Count(l.text, "�")
				if byLine[i+1] != n {
					t.Errorf("line %d has %d U+FFFD, decoder reports %d", i+1, n, byLine[i+1])
				}
				what := "replaced " + strconv.Itoa(n) + " unreadable characters with �"
				found := false
				for _, c := range res.Changes {
					if c.Line == i+1 && c.What == what {
						found = true
					} else if c.Line == i+1 && strings.HasPrefix(c.What, "replaced ") && n == 0 {
						t.Errorf("line %d: %q but the line has no U+FFFD", i+1, c.What)
					}
				}
				if n > 0 && !found {
					t.Errorf("line %d has %d U+FFFD but no Change %q (changes %v)", i+1, n, what, res.Changes)
				}
			}
		})
	}
}

// An undefined Windows-1252 byte reads as U+FFFD in the repaired file, in a
// mixed file and in a CHAR ANSI file alike, and is never called "stray".
func TestRepairUndefinedWindows1252Byte(t *testing.T) {
	in := unreadableInputs()
	for _, name := range []string{"mixed undefined byte", "ANSI undefined byte"} {
		t.Run(name, func(t *testing.T) {
			res, err := Repair(in[name])
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(res.Data), "1 NAME Renée") || !strings.Contains(string(res.Data), "1 NOTE a\uFFFDb\n") {
				t.Errorf("Data %q", res.Data)
			}
			for _, c := range res.Changes {
				if strings.Contains(c.What, "stray") {
					t.Errorf("Change %v calls the byte stray", c)
				}
			}
		})
	}
}
