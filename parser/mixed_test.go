package parser

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func badBytesFinding(t *testing.T, fs []*ParseError) *ParseError {
	t.Helper()
	for _, f := range fs {
		if f.Code == CodeEncodingBadBytes {
			return f
		}
	}
	t.Fatalf("no ENCODING_BAD_BYTES in %v", codes(fs))
	return nil
}

func strayFinding(t *testing.T, fs []*ParseError) *ParseError {
	t.Helper()
	for _, f := range fs {
		if f.Code == CodeEncodingStrayBytes {
			return f
		}
	}
	t.Fatalf("no ENCODING_STRAY_BYTES in %v", codes(fs))
	return nil
}

// A UTF-8 file with one stray Latin-1 byte: decoded as UTF-8, only the stray
// byte read as Windows-1252, reported as ENCODING_STRAY_BYTES on its line.
func TestDecodeMixedUTF8Latin1(t *testing.T) {
	out, info, fs := Decode(readEnc(t, "mixed-utf8-latin1.ged"))
	want := "0 HEAD\r\n1 GEDC\r\n2 VERS 5.5.1\r\n1 CHAR UTF-8\r\n0 @I1@ INDI\r\n" +
		"1 NAME Renée /Dupré/\r\n1 BIRT\r\n2 PLAC Zürich\r\n1 NOTE Café au lait\r\n0 TRLR\r\n"
	if string(out) != want {
		t.Errorf("decoded:\n%q\nwant\n%q", out, want)
	}
	if info.Detected != "UTF-8" || info.Declared != "UTF-8" {
		t.Errorf("info = %+v", info)
	}
	if got := codes(fs); !reflect.DeepEqual(got, []string{CodeEncodingStrayBytes}) {
		t.Fatalf("codes = %v", got)
	}
	f := fs[0]
	if f.Severity != "warning" || f.Line != 9 || f.Details["count"] != "1" || f.Details["encoding"] != "UTF-8" {
		t.Errorf("finding = line %d sev %s %v", f.Line, f.Severity, f.Details)
	}
}

// The same after a UTF-8 byte order mark.
func TestDecodeMixedAfterBOM(t *testing.T) {
	raw := append([]byte("\xEF\xBB\xBF"), readEnc(t, "mixed-utf8-latin1.ged")...)
	out, info, fs := Decode(raw)
	if !strings.Contains(string(out), "1 NAME Renée /Dupré/\r\n") || !strings.Contains(string(out), "1 NOTE Café au lait\r\n") {
		t.Errorf("decoded: %q", out)
	}
	if info.Detected != "UTF-8" || info.Reason != "byte order mark" {
		t.Errorf("info = %+v", info)
	}
	if f := strayFinding(t, fs); f.Line != 9 || f.Details["count"] != "1" {
		t.Errorf("finding = line %d %v", f.Line, f.Details)
	}
}

// Stray bytes on two lines: one ENCODING_STRAY_BYTES (warning) on the first
// affected line with the total count and the encoding; no ENCODING_BAD_BYTES,
// since nothing became U+FFFD.
func TestDecodeStrayBytesFinding(t *testing.T) {
	_, _, fs := Decode([]byte("0 HEAD\n1 CHAR UTF-8\n1 NOTE ü ö ä é\n1 NOTE \xe9\xe8\n1 NOTE ok\n1 NOTE \xe0\n0 TRLR\n"))
	if got := codes(fs); !reflect.DeepEqual(got, []string{CodeEncodingStrayBytes}) {
		t.Fatalf("codes = %v", got)
	}
	f := fs[0]
	if f.Severity != "warning" || f.Line != 4 || f.Details["count"] != "3" || f.Details["encoding"] != "UTF-8" {
		t.Errorf("finding = line %d sev %s %v", f.Line, f.Severity, f.Details)
	}
}

// A file that is Windows-1252 throughout (no valid multi-byte UTF-8) still
// takes the whole-file path, with MISLABELED.
func TestDecodePureWindows1252WholeFile(t *testing.T) {
	out, info, fs := Decode([]byte("0 HEAD\n1 CHAR UTF-8\n0 @I1@ INDI\n1 NAME Ren\xe9e /Dupr\xe9/\n1 NOTE \xe9t\xe9\n0 TRLR\n"))
	if info.Detected != "Windows-1252" {
		t.Errorf("info = %+v", info)
	}
	if got := codes(fs); !reflect.DeepEqual(got, []string{CodeEncodingMislabeled, CodeEncodingNotUTF8}) {
		t.Errorf("codes = %v", got)
	}
	if !strings.Contains(string(out), "1 NAME Renée /Dupré/\n1 NOTE été\n") {
		t.Errorf("decoded: %q", out)
	}
}

// More invalid bytes than valid multi-byte sequences: whole-file Windows-1252.
func TestDecodeMostlyWindows1252WholeFile(t *testing.T) {
	_, info, fs := Decode([]byte("0 HEAD\n1 CHAR UTF-8\n0 @I1@ INDI\n1 NAME Ren\xe9e /Dupr\xe9/\n1 NOTE \xc3\xa9\n0 TRLR\n"))
	if info.Detected != "Windows-1252" || !reflect.DeepEqual(codes(fs), []string{CodeEncodingMislabeled, CodeEncodingNotUTF8}) {
		t.Errorf("info = %+v codes = %v", info, codes(fs))
	}
}

// An ANSEL combining mark at the end of a line carries over to the first
// letter of a CONC continuation.
func TestDecodeANSELMarkAcrossConc(t *testing.T) {
	raw := "0 HEAD\r\n1 CHAR ANSEL\r\n0 @I1@ INDI\r\n1 NOTE Ren\xE2\r\n2 CONC e and more\r\n0 TRLR\r\n"
	out, _, fs := Decode([]byte(raw))
	want := "0 HEAD\r\n1 CHAR ANSEL\r\n0 @I1@ INDI\r\n1 NOTE Ren\r\n2 CONC é and more\r\n0 TRLR\r\n"
	if string(out) != want {
		t.Errorf("decoded:\n%q\nwant\n%q", out, want)
	}
	if got := codes(fs); !reflect.DeepEqual(got, []string{CodeEncodingNotUTF8}) {
		t.Errorf("codes = %v", got)
	}
}

// At the end of a line that is not followed by CONC, the mark becomes U+FFFD
// and is reported on its line.
func TestDecodeANSELMarkAtLineEnd(t *testing.T) {
	raw := "0 HEAD\r\n1 CHAR ANSEL\r\n0 @I1@ INDI\r\n1 NOTE Ren\xE2\r\n1 SEX M\r\n1 NOTE x\xE2\r\n2 CONT e\r\n0 TRLR\r\n"
	out, _, fs := Decode([]byte(raw))
	want := "0 HEAD\r\n1 CHAR ANSEL\r\n0 @I1@ INDI\r\n1 NOTE Ren�\r\n1 SEX M\r\n1 NOTE x�\r\n2 CONT e\r\n0 TRLR\r\n"
	if string(out) != want {
		t.Errorf("decoded:\n%q\nwant\n%q", out, want)
	}
	if f := badBytesFinding(t, fs); f.Line != 4 || f.Details["count"] != "2" {
		t.Errorf("finding = line %d %v", f.Line, f.Details)
	}
}

func utf16le(s string) []byte {
	var b []byte
	for _, r := range s {
		b = append(b, byte(r), byte(r>>8))
	}
	return b
}

func TestDecodeReportLineFixes(t *testing.T) {
	loneSurrogate := append(append([]byte{0xFF, 0xFE}, utf16le("0 HEAD\r\n1 CHAR UNICODE\r\n1 NOTE a")...),
		0x00, 0xD8) // a lone high surrogate
	loneSurrogate = append(loneSurrogate, utf16le("b �\r\n0 TRLR\r\n")...) // the U+FFFD is the file's own
	cases := []struct {
		name string
		raw  []byte
		want []LineFix
		text string // a line the decoded text must contain
	}{
		{"mixed", readEnc(t, "mixed-utf8-latin1.ged"), []LineFix{{Line: 9, Stray: 1}}, "1 NOTE Café au lait\r\n"},
		{"mixed two lines", []byte("0 HEAD\n1 NOTE ü ö ä é\n1 NOTE \xe9\xe8\n1 NOTE ok\n1 NOTE \xe0\n0 TRLR\n"),
			[]LineFix{{Line: 3, Stray: 2}, {Line: 5, Stray: 1}}, "1 NOTE éè\n1 NOTE ok\n1 NOTE à\n"},
		{"BOM and a stray byte", []byte("\xEF\xBB\xBF0 HEAD\n1 CHAR UTF-8\n0 @I1@ INDI\n1 NAME A\xFF /B/\n0 TRLR\n"),
			[]LineFix{{Line: 4, Unreadable: 1}}, "1 NAME A� /B/\n"},
		{"UTF-16 lone surrogate", loneSurrogate, []LineFix{{Line: 3, Unreadable: 1}}, "1 NOTE a�b �\r\n"},
		{"UTF-16 odd length", append([]byte{0xFF, 0xFE}, append(utf16le("0 HEAD\r\n0 TRLR\r\n"), 'x')...),
			[]LineFix{{Line: 3, Unreadable: 1}}, "0 TRLR\r\n�"},
		{"ANSEL", []byte("0 HEAD\r\n1 CHAR ANSEL\r\n1 NOTE \xFF\xE2\r\n1 NOTE x\xE2\r\n2 CONC e\r\n"),
			[]LineFix{{Line: 3, Unreadable: 2}}, "1 NOTE ��\r\n1 NOTE x\r\n2 CONC é\r\n"},
		{"clean UTF-8", readEnc(t, "utf8.ged"), nil, "Zürich"},
		{"UTF-8 with its own U+FFFD", []byte("0 HEAD\n1 NOTE �\n0 TRLR\n"), nil, "1 NOTE �\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, _, _, fixes := DecodeReport(c.raw)
			if !reflect.DeepEqual(fixes, c.want) {
				t.Errorf("fixes = %+v, want %+v", fixes, c.want)
			}
			if !strings.Contains(string(out), c.text) {
				t.Errorf("decoded %q lacks %q", out, c.text)
			}
		})
	}
}

// 0x81 is undefined in Windows-1252. In a mixed file it is unreadable, not a
// stray conversion: U+FFFD, ENCODING_BAD_BYTES, and no ENCODING_STRAY_BYTES.
func TestDecodeMixedUndefinedWindows1252Byte(t *testing.T) {
	raw := []byte("0 HEAD\n1 CHAR UTF-8\n0 @I1@ INDI\n1 NAME Renée /Dupré/\n1 NOTE a\x81b\n0 TRLR\n")
	out, info, fs, fixes := DecodeReport(raw)
	if !strings.Contains(string(out), "1 NOTE a�b\n") || info.Detected != "UTF-8" {
		t.Errorf("decoded %q, info %+v", out, info)
	}
	if got := codes(fs); !reflect.DeepEqual(got, []string{CodeEncodingBadBytes}) {
		t.Fatalf("codes = %v", got)
	}
	if f := fs[0]; f.Line != 5 || f.Details["count"] != "1" {
		t.Errorf("finding = line %d %v", f.Line, f.Details)
	}
	if want := []LineFix{{Line: 5, Unreadable: 1}}; !reflect.DeepEqual(fixes, want) {
		t.Errorf("fixes = %+v, want %+v", fixes, want)
	}
}

// A stray byte and an undefined one in the same mixed file: each is counted
// in its own finding and on its own line.
func TestDecodeMixedStrayAndUndefined(t *testing.T) {
	raw := []byte("0 HEAD\n1 CHAR UTF-8\n1 NOTE ü ö ä ë\n1 NOTE \xe9\x8d\n1 NOTE \x9d\n0 TRLR\n")
	out, _, fs, fixes := DecodeReport(raw)
	if !strings.Contains(string(out), "1 NOTE é�\n1 NOTE �\n") {
		t.Errorf("decoded %q", out)
	}
	if f := strayFinding(t, fs); f.Line != 4 || f.Details["count"] != "1" {
		t.Errorf("stray = line %d %v", f.Line, f.Details)
	}
	if f := badBytesFinding(t, fs); f.Line != 4 || f.Details["count"] != "2" {
		t.Errorf("bad = line %d %v", f.Line, f.Details)
	}
	if want := []LineFix{{Line: 4, Stray: 1, Unreadable: 1}, {Line: 5, Unreadable: 1}}; !reflect.DeepEqual(fixes, want) {
		t.Errorf("fixes = %+v, want %+v", fixes, want)
	}
}

// The same undefined byte in a CHAR ANSI file read as Windows-1252 throughout.
func TestDecodeANSIUndefinedByte(t *testing.T) {
	for _, b := range []byte{0x81, 0x8D, 0x8F, 0x90, 0x9D} {
		raw := []byte("0 HEAD\n1 CHAR ANSI\n0 @I1@ INDI\n1 NAME Ren\xe9e\n1 NOTE a" + string([]byte{b}) + "b\n0 TRLR\n")
		out, info, fs, fixes := DecodeReport(raw)
		if !strings.Contains(string(out), "1 NAME Renée\n1 NOTE a�b\n") || info.Detected != "Windows-1252" {
			t.Errorf("%#x: decoded %q, info %+v", b, out, info)
		}
		if f := badBytesFinding(t, fs); f.Line != 5 || f.Details["count"] != "1" {
			t.Errorf("%#x: finding = line %d %v", b, f.Line, f.Details)
		}
		if want := []LineFix{{Line: 5, Unreadable: 1}}; !reflect.DeepEqual(fixes, want) {
			t.Errorf("%#x: fixes = %+v, want %+v", b, fixes, want)
		}
	}
}

// An ANSEL mark at the end of a line carries over only to a real CONC
// continuation of that line: CONC one level deeper, or CONC at the same
// level when the line is itself CONC or CONT. Otherwise it is stranded.
func TestDecodeANSELMarkConcLevel(t *testing.T) {
	const head = "0 HEAD\r\n1 CHAR ANSEL\r\n0 @I1@ INDI\r\n"
	cases := []struct {
		name, in, want string
		bad            int // U+FFFD written, on line 4
	}{
		{"sibling CONC strands", "1 NOTE Ren\xE2\r\n1 CONC e\r\n", "1 NOTE Ren�\r\n1 CONC e\r\n", 1},
		{"CONC two levels down strands", "1 NOTE Ren\xE2\r\n2 SOUR x\r\n3 CONC e\r\n", "1 NOTE Ren�\r\n2 SOUR x\r\n3 CONC e\r\n", 1},
		{"CONC three levels after NOTE strands", "1 NOTE Ren\xE2\r\n3 CONC e\r\n", "1 NOTE Ren�\r\n3 CONC e\r\n", 1},
		{"CONC one level down carries", "1 NOTE Ren\xE2\r\n2 CONC e\r\n", "1 NOTE Ren\r\n2 CONC é\r\n", 0},
		{"CONC after CONC carries", "1 NOTE x\r\n2 CONC Ren\xE2\r\n2 CONC e\r\n", "1 NOTE x\r\n2 CONC Ren\r\n2 CONC é\r\n", 0},
		{"CONC after CONT carries", "1 NOTE x\r\n2 CONT Ren\xE2\r\n2 CONC e\r\n", "1 NOTE x\r\n2 CONT Ren\r\n2 CONC é\r\n", 0},
		{"CONC one level below a CONC carries", "1 NOTE x\r\n2 CONC Ren\xE2\r\n3 CONC e\r\n", "1 NOTE x\r\n2 CONC Ren\r\n3 CONC é\r\n", 0},
		{"CONC two levels below a CONC strands", "1 NOTE x\r\n2 CONC Ren\xE2\r\n4 CONC e\r\n", "1 NOTE x\r\n2 CONC Ren�\r\n4 CONC e\r\n", 1},
		{"record with xref carries", "0 @N1@ NOTE Ren\xE2\r\n1 CONC e\r\n", "0 @N1@ NOTE Ren\r\n1 CONC é\r\n", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, _, fs := Decode([]byte(head + c.in + "0 TRLR\r\n"))
			if want := head + c.want + "0 TRLR\r\n"; string(out) != want {
				t.Errorf("decoded:\n%q\nwant\n%q", out, want)
			}
			var bad *ParseError
			for _, f := range fs {
				if f.Code == CodeEncodingBadBytes {
					bad = f
				}
			}
			switch {
			case c.bad == 0 && bad != nil:
				t.Errorf("unexpected %v", bad.Details)
			case c.bad > 0 && (bad == nil || bad.Line < 4 || bad.Details["count"] != strconv.Itoa(c.bad)):
				t.Errorf("want ENCODING_BAD_BYTES count %d, got %+v", c.bad, bad)
			}
		})
	}
}
