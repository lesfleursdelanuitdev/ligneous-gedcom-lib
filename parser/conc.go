package parser

import "unicode/utf8"

// The helpers below look at raw lines before decoding. The level, xref, tag
// and "CONC" are ASCII in every single-byte encoding and in UTF-8.

// lineStart returns the offset in b of the start of the line holding b[i].
func lineStart(b []byte, i int) int {
	for i > 0 && b[i-1] != '\n' && b[i-1] != '\r' {
		i--
	}
	return i
}

// maxLevelDigits bounds the level numbers these helpers read; a longer level
// is never a continuation the parser would join.
const maxLevelDigits = 4

// lineLevelTag reads "[ws] <level> SP+ [@xref@ SP+] <tag>" at the start of ln.
func lineLevelTag(ln []byte) (level int, tag string, ok bool) {
	i := skipBlank(ln, 0)
	level, i, ok = readLevel(ln, i)
	if !ok {
		return 0, "", false
	}
	j := skipBlank(ln, i)
	if j == i {
		return 0, "", false
	}
	if j < len(ln) && ln[j] == '@' {
		k := j + 1
		for k < len(ln) && ln[k] != '@' {
			k++
		}
		if k >= len(ln) {
			return 0, "", false
		}
		i = k + 1
		if j = skipBlank(ln, i); j == i {
			return 0, "", false
		}
	}
	k := j
	for k < len(ln) && ln[k] != ' ' && ln[k] != '\t' {
		k++
	}
	if k == j {
		return 0, "", false
	}
	return level, string(ln[j:k]), true
}

func skipBlank(b []byte, i int) int {
	for i < len(b) && (b[i] == ' ' || b[i] == '\t') {
		i++
	}
	return i
}

func readLevel(b []byte, i int) (level, end int, ok bool) {
	d := i
	for i < len(b) && b[i] >= '0' && b[i] <= '9' {
		if i-d == maxLevelDigits {
			return 0, i, false
		}
		level = level*10 + int(b[i]-'0')
		i++
	}
	return level, i, i > d
}

// concPrefix returns the level of the CONC line at the start of b and the
// length of its "[ws] <level> SP+ CONC SP" prefix, or n = 0 when b does not
// start a CONC line.
func concPrefix(b []byte) (level, n int) {
	level, i, ok := readLevel(b, skipBlank(b, 0))
	if !ok {
		return 0, 0
	}
	sp := i
	i = skipBlank(b, i)
	if i == sp || len(b) < i+5 || string(b[i:i+4]) != "CONC" || (b[i+4] != ' ' && b[i+4] != '\t') {
		return 0, 0
	}
	return level, i + 5
}

// concNext reports whether rest starts with a CONC line that continues the
// line cur: CONC one level below cur, or CONC at cur's level when cur is
// itself a CONC or CONT line. It returns the length of the CONC line's prefix
// up to its value, or 0.
func concNext(cur, rest []byte) int {
	lvl, tag, ok := lineLevelTag(cur)
	if !ok {
		return 0
	}
	next, n := concPrefix(rest)
	if n == 0 {
		return 0
	}
	if next == lvl+1 || (next == lvl && (tag == "CONC" || tag == "CONT")) {
		return n
	}
	return 0
}

// maxLineBytes is the GEDCOM limit for a line, in bytes, without its
// terminator.
const maxLineBytes = 255

// split is a UTF-8 character cut at the end of a line (lead bytes b[i:e])
// and finished by the first rest bytes of the next line's CONC value at v.
type split struct {
	e, v, rest int
	// back: join by moving the rest bytes back onto the first line, which
	// then ends with the whole character. Otherwise the lead bytes move
	// forward to the start of the CONC value.
	back bool
}

// splitRune reports whether b[i], a byte that is not valid UTF-8 where it
// stands, starts a UTF-8 character cut at the end of its line and finished
// by the first bytes of the next line's value, that line being a CONC
// continuation of this one (concNext). Some exporters cut CONC lines by
// bytes. The character is joined back onto the first line when that line
// stays within 255 bytes; otherwise forward onto the CONC line when no
// whitespace precedes the lead (so the first line does not end in a space a
// reader would trim); otherwise it is not joined (ok is false) and the bytes
// are read as stray or unreadable.
func splitRune(b []byte, i int) (s split, ok bool) {
	var need int
	switch c := b[i]; {
	case c >= 0xC2 && c <= 0xDF:
		need = 2
	case c >= 0xE0 && c <= 0xEF:
		need = 3
	case c >= 0xF0 && c <= 0xF4:
		need = 4
	default:
		return s, false
	}
	e := i + 1
	for e < len(b) && e-i < need && b[e] >= 0x80 && b[e] <= 0xBF {
		e++
	}
	if e >= len(b) || e-i >= need || (b[e] != '\r' && b[e] != '\n') {
		return s, false
	}
	next := e + 1
	if b[e] == '\r' && next < len(b) && b[next] == '\n' {
		next++
	}
	start := lineStart(b, i)
	n := concNext(b[start:i], b[next:])
	if n == 0 {
		return s, false
	}
	v, rest := next+n, need-(e-i)
	if v+rest > len(b) {
		return s, false
	}
	var buf [utf8.UTFMax]byte
	k := copy(buf[:], b[i:e])
	copy(buf[k:], b[v:v+rest])
	if r, size := utf8.DecodeRune(buf[:need]); r == utf8.RuneError || size != need {
		return s, false
	}
	s = split{e: e, v: v, rest: rest, back: e-start+rest <= maxLineBytes}
	if !s.back && i > start && (b[i-1] == ' ' || b[i-1] == '\t') {
		return split{}, false
	}
	return s, true
}
