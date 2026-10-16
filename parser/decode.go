package parser

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/gedcom"
)

// charKind is the class of encoding a HEAD.CHAR value names.
type charKind int

const (
	kindNone charKind = iota // no CHAR line
	kindUnknown
	kindUTF8
	kindUTF16 // UNICODE
	kindASCII
	kindANSEL
	kindANSI
	kindCP437
	kindMac
)

func classifyChar(declared string) charKind {
	switch strings.ToUpper(strings.TrimSpace(declared)) {
	case "":
		return kindNone
	case "UTF-8", "UTF8":
		return kindUTF8
	case "UNICODE", "UTF-16":
		return kindUTF16
	case "ASCII", "US-ASCII":
		return kindASCII
	case "ANSEL":
		return kindANSEL
	case "ANSI", "WINDOWS-1252", "CP1252",
		"ISO-8859-1", "ISO8859-1", "ISO_8859-1", "LATIN1", "LATIN-1":
		// ISO-8859-1 is read as its superset Windows-1252, as browsers do.
		return kindANSI
	case "IBMPC", "IBM WINDOWS", "IBM-PC", "IBM":
		return kindCP437
	case "MACINTOSH":
		return kindMac
	}
	return kindUnknown
}

func (k charKind) legacy() bool { return k >= kindANSEL }

// scanChar reads the HEAD.CHAR value from the first 50 lines of the HEAD
// record in b. HEAD is the first "0 HEAD" line (the same rule as the parser's
// findHeadLine), so lines before it are skipped but still counted; with no
// "0 HEAD" the scan starts at line 1. The scan ends at the next level-0 line.
// The CHAR line is ASCII in every supported encoding, so b can still be in its
// legacy encoding. It returns the value as written and the 1-based line number
// in b.
func scanChar(b []byte) (value string, line int) {
	head, _ := findHead(b)
	n := 0
	for pos := 0; pos < len(b); {
		var ln []byte
		ln, pos = nextLine(b, pos)
		n++
		if n < head {
			continue
		}
		if n-head >= 50 {
			break
		}
		f := bytes.Fields(ln)
		if len(f) >= 2 && string(f[0]) == "0" && n > head {
			break // the header ended
		}
		if len(f) >= 2 && string(f[0]) == "1" && string(bytes.ToUpper(f[1])) == "CHAR" {
			i := bytes.Index(bytes.ToUpper(ln), []byte("CHAR")) + 4
			return string(bytes.TrimSpace(ln[i:])), n
		}
	}
	return "", 0
}

// lineAt returns the 1-based line number of byte offset idx in b.
func lineAt(b []byte, idx int) int {
	if idx > len(b) {
		idx = len(b)
	}
	n := 1
	for i := 0; i < idx; i++ {
		if b[i] == '\n' || (b[i] == '\r' && (i+1 >= len(b) || b[i+1] != '\n')) {
			n++
		}
	}
	return n
}

// Decode turns the raw bytes of a GEDCOM file into UTF-8. It checks, in order,
// a byte order mark, the UTF-16 byte pattern and HEAD.CHAR, then checks the
// choice against the bytes. A file meant to be UTF-8 (by its byte order mark,
// its CHAR or no CHAR) that has a few invalid bytes is still read as UTF-8
// when its valid multi-byte sequences outnumber the invalid bytes; each
// invalid byte is then read as Windows-1252 (ENCODING_BAD_BYTES). Otherwise
// such a file is read as Windows-1252 throughout (ENCODING_MISLABELED).
// Decode never fails: bytes that cannot be decoded become U+FFFD and are
// reported in the findings.
func Decode(raw []byte) (utf8Bytes []byte, info gedcom.EncodingInfo, findings []*ParseError) {
	utf8Bytes, info, findings, _ = DecodeReport(raw)
	return utf8Bytes, info, findings
}

// LineFix says what Decode did to one line beyond converting its encoding.
type LineFix struct {
	Line int // 1-based line number in the file
	// Stray is the number of bytes that were not valid UTF-8 in a file that
	// is otherwise UTF-8, each read as Windows-1252.
	Stray int
	// Unreadable is the number of U+FFFD characters Decode wrote because it
	// could not read the bytes (U+FFFD already in the file is not counted).
	Unreadable int
}

// DecodeReport is Decode plus, in line order, every line where a stray byte
// was converted or an unreadable character was replaced with U+FFFD.
func DecodeReport(raw []byte) (utf8Bytes []byte, info gedcom.EncodingInfo, findings []*ParseError, fixes []LineFix) {
	var d decoder
	utf8Bytes, info, findings = d.run(raw)
	for _, f := range findings { // Detected is final only now
		f.Details["encoding"] = info.Detected
	}
	if len(d.strayAt) > 0 || len(d.fffdAt) > 0 {
		fixes = lineFixes(utf8Bytes, d.strayAt, d.fffdAt)
	}
	return utf8Bytes, info, findings, fixes
}

type decoder struct {
	info     gedcom.EncodingInfo
	findings []*ParseError
	line     int // line of the CHAR record, or 1
	// Byte offsets in the output, in increasing order, of each character
	// converted from a stray byte and of each U+FFFD written by the decoder.
	strayAt, fffdAt []int
}

// lineFixes turns output offsets into per-line counts, in line order.
func lineFixes(text []byte, strayAt, fffdAt []int) []LineFix {
	var fixes []LineFix
	line, i, s, f := 1, 0, 0, 0
	add := func(stray bool) {
		if len(fixes) == 0 || fixes[len(fixes)-1].Line != line {
			fixes = append(fixes, LineFix{Line: line})
		}
		if stray {
			fixes[len(fixes)-1].Stray++
		} else {
			fixes[len(fixes)-1].Unreadable++
		}
	}
	for s < len(strayAt) || f < len(fffdAt) {
		next, stray := 0, false
		switch {
		case f >= len(fffdAt) || (s < len(strayAt) && strayAt[s] < fffdAt[f]):
			next, stray = strayAt[s], true
			s++
		default:
			next = fffdAt[f]
			f++
		}
		for ; i < next && i < len(text); i++ {
			if text[i] == '\n' || (text[i] == '\r' && (i+1 >= len(text) || text[i+1] != '\n')) {
				line++
			}
		}
		add(stray)
	}
	return fixes
}

// fffdOffsets returns the offset of every U+FFFD in text.
func fffdOffsets(text []byte) []int {
	var at []int
	for i := 0; ; {
		k := bytes.Index(text[i:], []byte("\uFFFD"))
		if k < 0 {
			return at
		}
		at = append(at, i+k)
		i += k + 3
	}
}

func (d *decoder) add(code, msg string, extra map[string]string) {
	f := newParseError(code, d.line, msg, "")
	f.Details = map[string]string{"encoding": d.info.Detected}
	if d.info.Declared != "" {
		f.Details["declared"] = d.info.Declared
	}
	for k, v := range extra {
		f.Details[k] = v
	}
	d.findings = append(d.findings, f)
}

func (d *decoder) setChar(value string, line int) charKind {
	d.info.Declared = value
	d.line = 1
	if line > 0 {
		d.line = line
	}
	return classifyChar(value)
}

// badBytes reports n unreadable bytes; the finding sits on the line of the
// first U+FFFD the decoder wrote.
func (d *decoder) badBytes(text []byte, n int) {
	if n == 0 {
		return
	}
	line := 1
	if len(d.fffdAt) > 0 {
		line = lineAt(text, d.fffdAt[0])
	}
	d.badBytesFinding(line, n, fmt.Sprintf("%d byte(s) could not be decoded as %s", n, d.info.Detected))
}

func (d *decoder) badBytesFinding(line, n int, msg string) {
	f := newParseError(CodeEncodingBadBytes, line, msg, "")
	f.Details = map[string]string{"encoding": d.info.Detected, "count": strconv.Itoa(n)}
	if d.info.Declared != "" {
		f.Details["declared"] = d.info.Declared
	}
	d.findings = append(d.findings, f)
}

func (d *decoder) run(raw []byte) ([]byte, gedcom.EncodingInfo, []*ParseError) {
	switch {
	case bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}):
		return d.bomUTF8(raw[3:])
	case bytes.HasPrefix(raw, []byte{0xFF, 0xFE}):
		return d.utf16(raw[2:], false, "byte order mark")
	case bytes.HasPrefix(raw, []byte{0xFE, 0xFF}):
		return d.utf16(raw[2:], true, "byte order mark")
	case len(raw) >= 4 && raw[0] == '0' && raw[1] == 0 && raw[2] == ' ' && raw[3] == 0:
		return d.utf16(raw, false, "byte pattern")
	case len(raw) >= 4 && raw[0] == 0 && raw[1] == '0' && raw[2] == 0 && raw[3] == ' ':
		return d.utf16(raw, true, "byte pattern")
	}

	kind := d.setChar(scanChar(raw))
	if kind == kindNone {
		d.info.Reason = "no CHAR in header"
	} else {
		d.info.Reason = "header CHAR"
	}

	if kind.legacy() {
		if utf8.Valid(raw) && hasHigh(raw) {
			d.info.Detected, d.info.Reason = "UTF-8", "bytes are valid UTF-8"
			d.add(CodeEncodingMislabeled, "header says CHAR "+d.info.Declared+" but the bytes are valid UTF-8", nil)
			return raw, d.info, d.findings
		}
		return d.legacy(raw, kind)
	}

	if kind == kindNone {
		d.info.Detected = "UTF-8" // for the Details of the undeclared finding
		d.add(CodeEncodingUndeclared, "the header has no CHAR line", nil)
	}
	if utf8.Valid(raw) {
		d.info.Detected = "UTF-8"
		if kind == kindASCII && !hasHigh(raw) {
			d.info.Detected = "ASCII"
		}
		return raw, d.info, d.findings
	}
	if multi, invalid := utf8Counts(raw); multi > invalid {
		// Mostly UTF-8 with a few stray bytes: keep the UTF-8.
		d.info.Detected = "UTF-8"
		return d.mixed(raw), d.info, d.findings
	}
	// Not UTF-8 although declared (or assumed) to be.
	d.info.Detected, d.info.Reason = "Windows-1252", "bytes are not valid UTF-8"
	d.add(CodeEncodingMislabeled, "the bytes are not valid UTF-8; read as Windows-1252", nil)
	return d.legacy(raw, kindANSI)
}

func hasHigh(b []byte) bool {
	for _, c := range b {
		if c >= 0x80 {
			return true
		}
	}
	return false
}

// bomUTF8 handles a file with a UTF-8 byte order mark: the mark wins over CHAR.
func (d *decoder) bomUTF8(body []byte) ([]byte, gedcom.EncodingInfo, []*ParseError) {
	kind := d.setChar(scanChar(body))
	d.info.Detected, d.info.Reason = "UTF-8", "byte order mark"
	switch {
	case kind == kindNone:
		d.add(CodeEncodingUndeclared, "the header has no CHAR line", nil)
	case kind.legacy():
		d.add(CodeEncodingMislabeled, "header says CHAR "+d.info.Declared+" but the file starts with a UTF-8 byte order mark", nil)
	}
	if utf8.Valid(body) {
		return body, d.info, d.findings
	}
	if multi, invalid := utf8Counts(body); multi > invalid {
		return d.mixed(body), d.info, d.findings
	}
	// Each run of invalid bytes becomes one U+FFFD.
	out := make([]byte, 0, len(body)+8)
	n := 0
	for i := 0; i < len(body); {
		r, size := utf8.DecodeRune(body[i:])
		if r == utf8.RuneError && size == 1 {
			n++
			d.fffdAt = append(d.fffdAt, len(out))
			out = append(out, "\uFFFD"...)
			for i += size; i < len(body); i += size {
				if r, size = utf8.DecodeRune(body[i:]); !(r == utf8.RuneError && size == 1) {
					break
				}
			}
			continue
		}
		out = append(out, body[i:i+size]...)
		i += size
	}
	d.badBytes(out, n)
	return out, d.info, d.findings
}

// utf8Counts returns the number of valid multi-byte UTF-8 sequences in b and
// the number of bytes that are not valid UTF-8.
func utf8Counts(b []byte) (multi, invalid int) {
	for i := 0; i < len(b); {
		if b[i] < utf8.RuneSelf {
			i++
			continue
		}
		r, size := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && size == 1 {
			invalid++
		} else {
			multi++
		}
		i += size
	}
	return multi, invalid
}

// mixed decodes b as UTF-8 and reads each byte that is not valid UTF-8 as
// Windows-1252. It reports ENCODING_BAD_BYTES (count = bytes converted) on
// the first affected line.
func (d *decoder) mixed(b []byte) []byte {
	out := make([]byte, 0, len(b)+16)
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && size == 1 {
			d.strayAt = append(d.strayAt, len(out))
			out = utf8.AppendRune(out, charmap.Windows1252.DecodeByte(b[i]))
			i++
			continue
		}
		out = append(out, b[i:i+size]...)
		i += size
	}
	n := len(d.strayAt)
	d.badBytesFinding(lineAt(out, d.strayAt[0]), n,
		fmt.Sprintf("%d byte(s) are not valid UTF-8; read as Windows-1252", n))
	return out
}

func (d *decoder) utf16(b []byte, be bool, reason string) ([]byte, gedcom.EncodingInfo, []*ParseError) {
	d.info.Detected, d.info.Reason = "UTF-16LE", reason
	if be {
		d.info.Detected = "UTF-16BE"
	}
	unit := func(i int) rune {
		if be {
			return rune(b[2*i])<<8 | rune(b[2*i+1])
		}
		return rune(b[2*i+1])<<8 | rune(b[2*i])
	}
	n := len(b) / 2
	out := make([]byte, 0, n+n/2)
	bad := 0
	for i := 0; i < n; i++ {
		r := unit(i)
		if utf16.IsSurrogate(r) {
			if i+1 < n {
				if p := utf16.DecodeRune(r, unit(i+1)); p != utf8.RuneError {
					out = utf8.AppendRune(out, p)
					i++
					continue
				}
			}
			// A lone surrogate.
			bad++
			d.fffdAt = append(d.fffdAt, len(out))
			r = utf8.RuneError
		}
		out = utf8.AppendRune(out, r)
	}
	if len(b)%2 == 1 {
		bad++
		d.fffdAt = append(d.fffdAt, len(out))
		out = append(out, "\uFFFD"...)
	}
	kind := d.setChar(scanChar(out))
	d.add(CodeEncodingNotUTF8, d.info.Detected+" is not UTF-8; it will be converted", nil)
	switch {
	case kind == kindNone:
		d.add(CodeEncodingUndeclared, "the header has no CHAR line", nil)
	case kind != kindUTF16:
		d.add(CodeEncodingMislabeled, "header says CHAR "+d.info.Declared+" but the file is "+d.info.Detected, nil)
	}
	d.badBytes(out, bad)
	return out, d.info, d.findings
}

// legacy converts single-byte encodings to UTF-8 and reports NOT_UTF8.
func (d *decoder) legacy(raw []byte, kind charKind) ([]byte, gedcom.EncodingInfo, []*ParseError) {
	var out []byte
	bad := 0
	switch kind {
	case kindANSEL:
		d.info.Detected = "ANSEL"
		var s string
		s, bad = decodeANSEL(raw)
		out = []byte(s)
		if bad > 0 { // ANSEL has no U+FFFD of its own: every one is ours
			d.fffdAt = fffdOffsets(out)
		}
	case kindCP437:
		d.info.Detected = "CP437"
		out = decodeCharmap(charmap.CodePage437, raw)
	case kindMac:
		d.info.Detected = "MacRoman"
		out = decodeCharmap(charmap.Macintosh, raw)
	default:
		d.info.Detected = "Windows-1252"
		out = decodeCharmap(charmap.Windows1252, raw)
	}
	d.add(CodeEncodingNotUTF8, d.info.Detected+" is not UTF-8; it will be converted", nil)
	d.badBytes(out, bad)
	return out, d.info, d.findings
}

func decodeCharmap(cm *charmap.Charmap, raw []byte) []byte {
	out, err := cm.NewDecoder().Bytes(raw)
	if err != nil {
		return raw
	}
	return out
}
