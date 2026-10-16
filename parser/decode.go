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
	case "ANSI", "WINDOWS-1252", "CP1252":
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
// choice against the bytes. It never fails: bytes that cannot be decoded become
// U+FFFD and are reported in the findings.
func Decode(raw []byte) (utf8Bytes []byte, info gedcom.EncodingInfo, findings []*ParseError) {
	var d decoder
	utf8Bytes, info, findings = d.run(raw)
	for _, f := range findings { // Detected is final only now
		f.Details["encoding"] = info.Detected
	}
	return utf8Bytes, info, findings
}

type decoder struct {
	info     gedcom.EncodingInfo
	findings []*ParseError
	line     int // line of the CHAR record, or 1
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

func (d *decoder) badBytes(text []byte, n int) {
	if n == 0 {
		return
	}
	i := bytes.Index(text, []byte("�"))
	line := 1
	if i >= 0 {
		line = lineAt(text, i)
	}
	f := newParseError(CodeEncodingBadBytes, line, fmt.Sprintf("%d byte(s) could not be decoded as %s", n, d.info.Detected), "")
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
	out := bytes.ToValidUTF8(body, []byte("�"))
	n, i := 0, 0
	for i < len(body) {
		r, size := utf8.DecodeRune(body[i:])
		if r == utf8.RuneError && size == 1 {
			n++
			for i += size; i < len(body); i += size {
				if r, size = utf8.DecodeRune(body[i:]); !(r == utf8.RuneError && size == 1) {
					break
				}
			}
			continue
		}
		i += size
	}
	d.badBytes(out, n)
	return out, d.info, d.findings
}

func (d *decoder) utf16(b []byte, be bool, reason string) ([]byte, gedcom.EncodingInfo, []*ParseError) {
	d.info.Detected, d.info.Reason = "UTF-16LE", reason
	if be {
		d.info.Detected = "UTF-16BE"
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		if be {
			u[i] = uint16(b[2*i])<<8 | uint16(b[2*i+1])
		} else {
			u[i] = uint16(b[2*i+1])<<8 | uint16(b[2*i])
		}
	}
	out := []byte(string(utf16.Decode(u)))
	bad := bytes.Count(out, []byte("�"))
	if len(b)%2 == 1 {
		out = append(out, "�"...)
		bad++
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
