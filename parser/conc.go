package parser

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
