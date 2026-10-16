package parser

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func readEnc(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", "encodings", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func codes(fs []*ParseError) []string {
	out := []string{}
	for _, f := range fs {
		out = append(out, f.Code)
	}
	sort.Strings(out)
	return out
}

func TestDecodeTable(t *testing.T) {
	cases := []struct {
		file     string
		detected string
		reason   string // "" = not asserted
		codes    []string
		name     string
	}{
		{"ansel.ged", "ANSEL", "header CHAR", []string{CodeEncodingNotUTF8}, "René /Dupré/"},
		{"ansi.ged", "Windows-1252", "", []string{CodeEncodingNotUTF8}, "René /Dupré/"},
		{"utf16le-bom.ged", "UTF-16LE", "byte order mark", []string{CodeEncodingNotUTF8}, "René /Dupré/"},
		{"utf16be-nobom.ged", "UTF-16BE", "byte pattern", []string{CodeEncodingNotUTF8}, "René /Dupré/"},
		{"mislabeled-utf8-as-ansel.ged", "UTF-8", "bytes are valid UTF-8", []string{CodeEncodingMislabeled}, "René /Dupré/"},
		{"utf8.ged", "UTF-8", "header CHAR", []string{}, "René /Dupré/"},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			out, info, fs := Decode(readEnc(t, c.file))
			if info.Detected != c.detected {
				t.Errorf("Detected = %q, want %q", info.Detected, c.detected)
			}
			if c.reason != "" && info.Reason != c.reason {
				t.Errorf("Reason = %q, want %q", info.Reason, c.reason)
			}
			if got := codes(fs); !reflect.DeepEqual(got, c.codes) {
				t.Errorf("codes = %v, want %v", got, c.codes)
			}
			s := string(out)
			if !strings.Contains(s, "1 NAME "+c.name) || !strings.Contains(s, "Zürich") {
				t.Errorf("decoded text wrong: %q", s)
			}
			if strings.HasPrefix(s, "\xEF\xBB\xBF") {
				t.Error("BOM left in output")
			}
		})
	}
}

func TestDecodeUndeclaredLatin1(t *testing.T) {
	out, info, fs := Decode(readEnc(t, "undeclared-latin1.ged"))
	if info.Detected != "Windows-1252" || info.Declared != "" || info.Reason != "bytes are not valid UTF-8" {
		t.Errorf("info = %+v", info)
	}
	want := []string{CodeEncodingMislabeled, CodeEncodingNotUTF8, CodeEncodingUndeclared}
	if got := codes(fs); !reflect.DeepEqual(got, want) {
		t.Errorf("codes = %v, want %v", got, want)
	}
	if !strings.Contains(string(out), "René /Dupré/") {
		t.Errorf("not decoded: %q", out)
	}
	for _, f := range fs {
		if f.Line != 1 {
			t.Errorf("%s line = %d, want 1 (no CHAR record)", f.Code, f.Line)
		}
		if _, ok := f.Details["declared"]; ok {
			t.Errorf("%s has declared detail for a missing CHAR", f.Code)
		}
		if f.Details["encoding"] != "Windows-1252" {
			t.Errorf("%s encoding detail = %q", f.Code, f.Details["encoding"])
		}
	}
}

func TestDecodeBOMBeatsChar(t *testing.T) {
	out, info, fs := Decode(readEnc(t, "bom-utf8-char-ansel.ged"))
	if info.Detected != "UTF-8" || info.Reason != "byte order mark" || info.Declared != "ANSEL" {
		t.Errorf("info = %+v", info)
	}
	if got := codes(fs); !reflect.DeepEqual(got, []string{CodeEncodingMislabeled}) {
		t.Errorf("codes = %v", got)
	}
	if fs[0].Line != 4 || fs[0].Details["declared"] != "ANSEL" {
		t.Errorf("finding = line %d %v", fs[0].Line, fs[0].Details)
	}
	if !strings.Contains(string(out), "Zürich") || strings.HasPrefix(string(out), "\xEF\xBB\xBF") {
		t.Errorf("bad output %q", out)
	}
}

func TestDecodeASCII(t *testing.T) {
	_, info, fs := Decode([]byte("0 HEAD\n1 CHAR ASCII\n0 TRLR\n"))
	if info.Detected != "ASCII" || len(fs) != 0 {
		t.Errorf("info=%+v fs=%v", info, codes(fs))
	}
}

func TestDecodeCodePages(t *testing.T) {
	// CP437: 0x82 = é ; MacRoman: 0x8E = é
	_, info, _ := Decode([]byte("0 HEAD\n1 CHAR IBMPC\n0 @I1@ INDI\n1 NAME Ren\x82\n"))
	if info.Detected != "CP437" {
		t.Errorf("IBMPC: %+v", info)
	}
	out, info, _ := Decode([]byte("0 HEAD\n1 CHAR MACINTOSH\n0 @I1@ INDI\n1 NAME Ren\x8E\n"))
	if info.Detected != "MacRoman" || !strings.Contains(string(out), "René") {
		t.Errorf("MACINTOSH: %+v %q", info, out)
	}
}

func TestDecodeBadBytes(t *testing.T) {
	in := []byte("0 HEAD\n1 CHAR ANSEL\n0 @I1@ INDI\n1 NAME A\xFFB\n1 NOTE \xFF\xFF\n")
	out, _, fs := Decode(in)
	var bad *ParseError
	for _, f := range fs {
		if f.Code == CodeEncodingBadBytes {
			bad = f
		}
	}
	if bad == nil {
		t.Fatalf("no ENCODING_BAD_BYTES in %v", codes(fs))
	}
	if bad.Severity != "warning" || bad.Line != 4 || bad.Details["count"] != "3" {
		t.Errorf("bad = line %d sev %s %v", bad.Line, bad.Severity, bad.Details)
	}
	if !strings.Contains(string(out), "A�B") {
		t.Errorf("output %q", out)
	}
}

func TestDecodeSeverities(t *testing.T) {
	want := map[string]string{
		CodeEncodingNotUTF8: "hint", CodeEncodingMislabeled: "warning",
		CodeEncodingUndeclared: "hint", CodeEncodingBadBytes: "warning",
		CodeEncodingStrayBytes: "warning",
	}
	for code, sev := range want {
		if got := newParseError(code, 1, "x", "").Severity; got != sev {
			t.Errorf("%s severity = %q, want %q", code, got, sev)
		}
	}
}

func TestParseSetsEncodingAndLineCount(t *testing.T) {
	b := readEnc(t, "ansel.ged")
	doc, warnings, err := Parse(strings.NewReader(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Encoding.Detected != "ANSEL" {
		t.Errorf("Encoding = %+v", doc.Encoding)
	}
	if want := strings.Count(string(b), "\r\n"); doc.LineCount != want {
		t.Errorf("LineCount = %d, want %d", doc.LineCount, want)
	}
	if got := codes(warnings); !reflect.DeepEqual(got, []string{CodeEncodingNotUTF8}) {
		t.Errorf("warnings = %v", got)
	}
	if warnings[0].Code != CodeEncodingNotUTF8 {
		t.Error("decode findings should come first")
	}
}

func TestDecodeCharAfterLinesBeforeHead(t *testing.T) {
	out, info, fs := Decode(readEnc(t, "ansel-junk-before-head.ged"))
	if info.Detected != "ANSEL" || info.Declared != "ANSEL" || info.Reason != "header CHAR" {
		t.Errorf("info = %+v", info)
	}
	if !strings.Contains(string(out), "René /Dupré/") {
		t.Errorf("name not decoded: %q", out)
	}
	if got := codes(fs); !reflect.DeepEqual(got, []string{CodeEncodingNotUTF8}) {
		t.Fatalf("codes = %v", got)
	}
	if fs[0].Line != 6 { // two junk lines, then HEAD on 3, GEDC 4, VERS 5, CHAR 6
		t.Errorf("CHAR line = %d, want 6", fs[0].Line)
	}
}

func TestDecodeCharScanStopsAtNextRecord(t *testing.T) {
	raw := []byte("0 HEAD\r\n1 GEDC\r\n2 VERS 5.5.1\r\n0 @I1@ INDI\r\n1 CHAR ANSEL\r\n1 NAME A /B/\r\n0 TRLR\r\n")
	_, info, fs := Decode(raw)
	if got := codes(fs); !reflect.DeepEqual(got, []string{CodeEncodingUndeclared}) {
		t.Errorf("codes = %v", got)
	}
	if info.Declared != "" {
		t.Errorf("declared = %q", info.Declared)
	}
}

func TestDecodeCharCapCountsFromHead(t *testing.T) {
	junk := strings.Repeat("junk\n", 60)
	_, _, fs := Decode([]byte(junk + "0 HEAD\n1 CHAR UTF-8\n0 TRLR\n"))
	if len(fs) != 0 {
		t.Errorf("codes = %v", codes(fs))
	}
}

func longLineANSEL(inHead bool) []byte {
	long := strings.Repeat("x", 1<<20+10) + "\r\n"
	body := "1 NAME Ren\xe2e /Dupr\xe2e/\r\n0 TRLR\r\n"
	if inHead {
		return []byte("0 HEAD\r\n" + long + "1 CHAR ANSEL\r\n0 @I1@ INDI\r\n" + body)
	}
	return []byte(long + "0 HEAD\r\n1 CHAR ANSEL\r\n0 @I1@ INDI\r\n" + body)
}

func TestDecodeCharAfterVeryLongLine(t *testing.T) {
	for _, inHead := range []bool{false, true} {
		out, info, fs := Decode(longLineANSEL(inHead))
		if info.Detected != "ANSEL" || info.Declared != "ANSEL" || info.Reason != "header CHAR" {
			t.Errorf("inHead=%v info = %+v", inHead, info)
		}
		if got := codes(fs); !reflect.DeepEqual(got, []string{CodeEncodingNotUTF8}) {
			t.Errorf("inHead=%v codes = %v", inHead, got)
		}
		if !strings.Contains(string(out), "René /Dupré/") {
			t.Errorf("inHead=%v name not decoded", inHead)
		}
	}
}

func TestFindHeadLineAfterVeryLongLine(t *testing.T) {
	if got := findHeadLine(longLineANSEL(false)); got != 2 {
		t.Errorf("findHeadLine = %d, want 2", got)
	}
}

// ISO-8859-1 and its spellings, LATIN1, WINDOWS-1252 and CP1252 are ANSI:
// read as Windows-1252, with no MISLABELED when the bytes agree.
func TestDecodeLatin1Aliases(t *testing.T) {
	for _, name := range []string{"ISO-8859-1", "iso8859-1", "ISO_8859-1", "Latin1", "LATIN-1", "windows-1252", "Cp1252", "ANSI"} {
		t.Run(name, func(t *testing.T) {
			raw := []byte("0 HEAD\r\n1 CHAR " + name + "\r\n0 @I1@ INDI\r\n1 NAME Ren\xe9 /Dupr\xe9/\r\n0 TRLR\r\n")
			out, info, fs := Decode(raw)
			if info.Detected != "Windows-1252" || info.Declared != name || info.Reason != "header CHAR" {
				t.Errorf("info = %+v", info)
			}
			if got := codes(fs); !reflect.DeepEqual(got, []string{CodeEncodingNotUTF8}) {
				t.Errorf("codes = %v", got)
			}
			if !strings.Contains(string(out), "1 NAME René /Dupré/") {
				t.Errorf("out = %q", out)
			}
		})
	}
}

// A Latin-1 alias over bytes that are valid UTF-8 is mislabeled, like ANSI.
func TestDecodeLatin1AliasOnUTF8(t *testing.T) {
	_, info, fs := Decode([]byte("0 HEAD\n1 CHAR ISO-8859-1\n0 @I1@ INDI\n1 NAME René /Dupré/\n0 TRLR\n"))
	if info.Detected != "UTF-8" || !reflect.DeepEqual(codes(fs), []string{CodeEncodingMislabeled}) {
		t.Errorf("info = %+v codes = %v", info, codes(fs))
	}
}
