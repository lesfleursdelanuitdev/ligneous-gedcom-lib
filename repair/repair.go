// Package repair fixes the format problems of a GEDCOM file that have one safe
// fix, working line by line on the decoded text. It never re-exports from the
// parsed tree, so unknown tags and records survive untouched.
//
// The fixes:
//   - convert the file to UTF-8, remove any byte order mark, and make the header say
//     "1 CHAR UTF-8", adding that line after the GEDC structure if missing;
//     in a UTF-8 file with a few stray bytes only those bytes are converted
//     (one Change per line), every line where decoding wrote U+FFFD gets
//     a Change too, and so does a line whose last UTF-8 character was cut
//     in two by a CONC split (the character is joined into one);
//   - turn bare "\r" line endings into "\r\n", keeping "\n" and "\r\n" as they are;
//   - remove blank lines;
//   - remove spaces and tabs before the level number (only on lines that
//     have a level and a tag; other lines are left untouched);
//   - split lines over 255 bytes with CONC (see below);
//   - add "0 TRLR" when the file has no trailer.
//
// Lines before "0 HEAD" are kept as they are (only their line endings change).
// Links, dates, names, the order of lines, xrefs, level jumps and unknown
// records are never changed; their findings stay in Result.Left.
//
// Splitting. A long line is split only when the enricher joins its CONC lines
// back into the same text, so the repaired file reads exactly like the
// original. In practice that is NOTE text:
//   - a NOTE line, when the note has no CONT lines (the enricher appends all
//     CONT lines before all CONC lines), does not point to a note record
//     ("@N1@"), is not under FAMC (that note is read without its CONC lines),
//     and, below level 0, does not end in whitespace (inline notes are trimmed);
//   - a CONT line of such a note, when it has no children, no CONT line
//     follows it and no CONC line precedes it;
//   - a CONC line of such a note, when it has no children.
//
// In every case the continuation level must stay within the parser's default
// MaxNestingDepth (100), and no cut sits next to a Unicode space. The message
// "split a N-character line" counts characters (runes); the 255 limit is bytes.
//
// TEXT, PAGE, TITL, PUBL, AUTH and ABBR are not split: the enricher reads only
// their first line, so a split would cut the text short. Their LINE_TOO_LONG
// stays in Result.Left.
package repair

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/validator"
)

// Change is one repair. Line is the 1-based line number in the original file;
// 0 means the whole file.
type Change struct {
	Line int    `json:"line"`
	What string `json:"what"`
}

// Result is the repaired file, the changes made, and the codes of the
// problems that remain.
type Result struct {
	Data    []byte   `json:"-"` // UTF-8, no byte order mark
	Changes []Change `json:"changes"`
	Left    []string `json:"left"` // sorted, deduplicated finding codes
}

// maxLine is the GEDCOM limit for a line, in bytes, without its terminator.
const maxLine = 255

// maxDepth mirrors parser.DefaultOptions().MaxNestingDepth: deeper lines are
// skipped by the parser, so they are never part of the tree.
const maxDepth = 100

// What strings.
const (
	whatCharset    = "set the character set to UTF-8"
	whatCRLF       = "changed line endings to CRLF"
	whatBlank      = "removed a blank line"
	whatWhitespace = "removed spaces before the level number"
	whatTrailer    = "added the end-of-file marker (0 TRLR)"
	whatBOM        = "removed the byte order mark"
	whatStray      = "converted %d stray Windows-1252 characters to UTF-8"
	whatUnreadable = "replaced %d unreadable characters with \uFFFD"
	whatJoined     = "joined a character split across lines %d and %d"
	whatReadUTF8   = "read %d characters as UTF-8 (the header said %s)"
)

// srcLine is one line of the decoded original.
type srcLine struct {
	text string // without terminator
	term string // "\r\n", "\n", "\r" or "" (last line)
}

// node is the tokenised form of a line at or after HEAD that the parser
// keeps, with its place in the tree.
type node struct {
	ok         bool
	level      int
	tag        string
	valueStart int // byte offset of the value in the cleaned text
	parent     int // index of the parent line, -1 for level 0 or a dropped line
	children   []int
}

type outLine struct {
	text string
	term string
}

// Repair decodes data, applies the fixes described in the package comment
// and returns the result. It fails only when data holds no GEDCOM line.
func Repair(data []byte) (*Result, error) {
	text, info, _, fixes := parser.DecodeReport(data)
	src := splitLines(text)
	n := len(src)

	head := 0
	for i, l := range src {
		if f := strings.Fields(l.text); len(f) >= 2 && f[0] == "0" && f[1] == "HEAD" {
			head = i
			break
		}
	}
	hasHead := head < n && isHead(src[head].text)

	changes := []Change{}
	if info.Detected != "UTF-8" && info.Detected != "ASCII" {
		changes = append(changes, Change{0, "converted from " + info.Detected + " to UTF-8"})
	}
	if hasBOM(data) {
		changes = append(changes, Change{0, whatBOM})
	}
	// What decoding did to single lines: stray bytes in a UTF-8 file read as
	// Windows-1252, unreadable bytes written as U+FFFD, characters split
	// across a CONC line joined, and lines read as UTF-8 against the header.
	unreadable, readUTF8 := false, false
	for _, f := range fixes {
		if f.UTF8 > 0 { // the header named a legacy encoding
			readUTF8 = true
			changes = append(changes, Change{f.Line, fmt.Sprintf(whatReadUTF8, f.UTF8, info.Declared)})
		}
		if f.Stray > 0 {
			changes = append(changes, Change{f.Line, fmt.Sprintf(whatStray, f.Stray)})
		}
		if f.Unreadable > 0 {
			unreadable = true
			changes = append(changes, Change{f.Line, fmt.Sprintf(whatUnreadable, f.Unreadable)})
		}
		if f.Joined > 0 { // the CONC line is the next one
			changes = append(changes, Change{f.Line, fmt.Sprintf(whatJoined, f.Line, f.Line+1)})
		}
	}

	// Clean each line from HEAD on: drop blank lines, strip leading whitespace
	// from GEDCOM lines.
	clean := make([]string, n)
	removed := make([]bool, n)
	nodes := make([]node, n)
	stack := []int{}
	anyNode := false
	for i := head; i < n; i++ {
		t := src[i].text
		if strings.TrimSpace(t) == "" {
			removed[i] = true
			changes = append(changes, Change{i + 1, whatBlank})
			continue
		}
		// Strip leading whitespace only from a GEDCOM line (level and tag);
		// any other line is kept exactly as it is.
		s := strings.TrimLeft(t, " \t")
		nd := tokenise(s)
		nd.parent = -1
		if !nd.ok {
			clean[i] = t
			nodes[i] = nd
			continue
		}
		if s != t {
			t = s
			changes = append(changes, Change{i + 1, whatWhitespace})
		}
		clean[i] = t
		anyNode = true
		if nd.level == 0 {
			stack = append(stack[:0], i)
		} else if len(stack) > 0 {
			for len(stack) > 1 && nodes[stack[len(stack)-1]].level >= nd.level {
				stack = stack[:len(stack)-1]
			}
			nd.parent = stack[len(stack)-1]
			nodes[nd.parent].children = append(nodes[nd.parent].children, i)
			stack = append(stack, i)
		}
		nodes[i] = nd
	}
	if !anyNode {
		return nil, errors.New("the file has no GEDCOM lines")
	}

	// CHAR: rewrite it in place, or plan to insert it.
	replace := map[int]string{}
	charAfter := -1
	if hasHead {
		charLine := -1
		for _, c := range nodes[head].children {
			if nodes[c].level == 1 && nodes[c].tag == "CHAR" {
				charLine = c
				break
			}
		}
		switch {
		case charLine >= 0:
			if value(clean[charLine], nodes[charLine]) != "UTF-8" || info.Detected != "UTF-8" {
				replace[charLine] = "1 CHAR UTF-8"
				changes = append(changes, Change{charLine + 1, whatCharset})
			}
		default:
			charAfter = head
			for _, c := range nodes[head].children {
				if nodes[c].level == 1 && nodes[c].tag == "GEDC" {
					charAfter = lastDescendant(nodes, c)
					break
				}
			}
			changes = append(changes, Change{head + 1, whatCharset})
		}
	}

	// Long lines.
	splits := map[int][]string{}
	for i := head; i < n; i++ {
		nd := nodes[i]
		if !nd.ok || len(clean[i]) <= maxLine || !canSplit(nodes, clean, i) {
			continue
		}
		contLevel := nd.level + 1
		if nd.tag == "CONT" || nd.tag == "CONC" {
			contLevel = nd.level
		}
		if contLevel > maxDepth {
			continue // the parser would drop the CONC lines
		}
		contPrefix := strconv.Itoa(contLevel) + " CONC "
		prefix := clean[i][:nd.valueStart]
		parts := splitValue(clean[i][nd.valueStart:], maxLine-len(prefix), maxLine-len(contPrefix))
		if parts == nil {
			continue
		}
		lines := []string{prefix + parts[0]}
		for _, p := range parts[1:] {
			lines = append(lines, contPrefix+p)
		}
		splits[i] = lines
		changes = append(changes, Change{i + 1,
			fmt.Sprintf("split a %d-character line into %d lines", utf8.RuneCountInString(src[i].text), len(lines))})
	}

	hasTrailer := false
	for i := head; i < n; i++ {
		if nodes[i].ok && nodes[i].level == 0 && nodes[i].tag == "TRLR" {
			hasTrailer = true
		}
	}

	// Emit.
	common := commonTerm(src)
	out := make([]outLine, 0, n+2)
	crChanged := false
	for i, l := range src {
		term := l.term
		if term == "\r" {
			term = "\r\n"
		}
		switch {
		case i < head:
			out = append(out, outLine{l.text, term})
		case removed[i]:
			continue
		case splits[i] != nil:
			for _, s := range splits[i] {
				out = append(out, outLine{s, term})
			}
		case replace[i] != "":
			out = append(out, outLine{replace[i], term})
		default:
			out = append(out, outLine{clean[i], term})
		}
		if l.term == "\r" {
			crChanged = true
		}
		if i == charAfter {
			out = append(out, outLine{"1 CHAR UTF-8", common})
		}
	}
	if !hasTrailer {
		out = append(out, outLine{"0 TRLR", common})
		changes = append(changes, Change{n + 1, whatTrailer})
	}
	if crChanged {
		changes = append(changes, Change{0, whatCRLF})
	}

	var buf bytes.Buffer
	for k, o := range out {
		buf.WriteString(o.text)
		if o.term == "" && k < len(out)-1 {
			o.term = common
		}
		buf.WriteString(o.term)
	}

	sort.SliceStable(changes, func(a, b int) bool { return changes[a].Line < changes[b].Line })
	res := &Result{Data: buf.Bytes(), Changes: changes}
	var extra []string
	if unreadable {
		extra = append(extra, parser.CodeEncodingBadBytes)
	}
	if readUTF8 {
		extra = append(extra, parser.CodeEncodingMislabeled)
	}
	res.Left = leftCodes(res.Data, extra...)
	return res, nil
}

// hasBOM reports whether data starts with a UTF-8 or UTF-16 byte order mark,
// which parser.Decode strips.
func hasBOM(data []byte) bool {
	return bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) ||
		bytes.HasPrefix(data, []byte{0xFF, 0xFE}) || bytes.HasPrefix(data, []byte{0xFE, 0xFF})
}

func isHead(s string) bool {
	f := strings.Fields(s)
	return len(f) >= 2 && f[0] == "0" && f[1] == "HEAD"
}

// splitLines splits text like the parser's scanner: at "\r\n", "\n" or a
// lone "\r", keeping each terminator.
func splitLines(b []byte) []srcLine {
	var out []srcLine
	for len(b) > 0 {
		i := bytes.IndexAny(b, "\r\n")
		if i < 0 {
			out = append(out, srcLine{string(b), ""})
			break
		}
		k := 1
		if b[i] == '\r' && i+1 < len(b) && b[i+1] == '\n' {
			k = 2
		}
		out = append(out, srcLine{string(b[:i]), string(b[i : i+k])})
		b = b[i+k:]
	}
	return out
}

// commonTerm is the line ending used for added lines: "\n" when most lines
// end with it, "\r\n" otherwise (bare "\r" counts as "\r\n").
func commonTerm(src []srcLine) string {
	lf, crlf := 0, 0
	for _, l := range src {
		switch l.term {
		case "\n":
			lf++
		case "\r\n", "\r":
			crlf++
		}
	}
	if lf > crlf {
		return "\n"
	}
	return "\r\n"
}

func isSep(c byte) bool { return c == ' ' || c == '\t' }

// tokenise reads a line (without leading whitespace) the way the parser does:
// level SP+ [xref SP+] tag [SP value].
func tokenise(t string) node {
	i := 0
	for i < len(t) && !isSep(t[i]) {
		i++
	}
	levelTok := t[:i]
	for i < len(t) && isSep(t[i]) {
		i++
	}
	if i >= len(t) {
		return node{}
	}
	level, err := strconv.Atoi(levelTok)
	if err != nil || level < 0 || level > maxDepth {
		return node{}
	}
	start := i
	for i < len(t) && !isSep(t[i]) {
		i++
	}
	tok := t[start:i]
	if len(tok) > 2 && tok[0] == '@' && tok[len(tok)-1] == '@' {
		for i < len(t) && isSep(t[i]) {
			i++
		}
		if i >= len(t) {
			return node{}
		}
		start = i
		for i < len(t) && !isSep(t[i]) {
			i++
		}
		tok = t[start:i]
	}
	vs := len(t)
	if i < len(t) {
		vs = i + 1
	}
	return node{ok: true, level: level, tag: tok, valueStart: vs}
}

func value(t string, nd node) string { return t[nd.valueStart:] }

func lastDescendant(nodes []node, i int) int {
	for len(nodes[i].children) > 0 {
		i = nodes[i].children[len(nodes[i].children)-1]
	}
	return i
}

// canSplit reports whether line i may be split with CONC without changing
// what the enricher reads. See the package comment.
func canSplit(nodes []node, clean []string, i int) bool {
	nd := nodes[i]
	switch nd.tag {
	case "NOTE":
		return noteSplittable(nodes, clean, i)
	case "CONT", "CONC":
		p := nd.parent
		if p < 0 || nodes[p].tag != "NOTE" || len(nd.children) > 0 || !noteContinuable(nodes, p) {
			return false
		}
		if nd.tag == "CONC" {
			return true
		}
		before := true
		for _, s := range nodes[p].children {
			switch {
			case s == i:
				before = false
			case before && nodes[s].tag == "CONC":
				return false
			case !before && nodes[s].tag == "CONT":
				return false
			}
		}
		return true
	}
	return false
}

// noteContinuable reports whether the enricher joins the CONC lines of the
// NOTE at line p in order: the note is not under FAMC.
func noteContinuable(nodes []node, p int) bool {
	gp := nodes[p].parent
	return gp < 0 || nodes[gp].tag != "FAMC"
}

// noteSplittable reports whether a long NOTE line may get CONC lines.
func noteSplittable(nodes []node, clean []string, i int) bool {
	nd := nodes[i]
	if !noteContinuable(nodes, i) {
		return false
	}
	for _, c := range nd.children {
		if nodes[c].tag == "CONT" {
			return false
		}
	}
	v := value(clean[i], nd)
	if strings.HasPrefix(strings.TrimSpace(v), "@") {
		return false
	}
	if nd.level > 0 {
		r, _ := utf8.DecodeLastRuneInString(v)
		if unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// splitValue cuts v into a first piece of at most first bytes and further
// pieces of at most rest bytes. Each cut falls on a rune boundary with no
// Unicode space (unicode.IsSpace: also U+00A0, U+3000) on either side, since
// the enricher trims the first piece of an inline note. It returns nil when
// no such cut exists.
func splitValue(v string, first, rest int) []string {
	var parts []string
	limit := first
	for len(v) > limit {
		c := cutPoint(v, limit)
		if c <= 0 {
			return nil
		}
		parts = append(parts, v[:c])
		v = v[c:]
		limit = rest
	}
	return append(parts, v)
}

func cutPoint(v string, limit int) int {
	for c := limit; c > 0; c-- {
		if !utf8.RuneStart(v[c]) {
			continue
		}
		before, _ := utf8.DecodeLastRuneInString(v[:c])
		at, _ := utf8.DecodeRuneInString(v[c:])
		if !unicode.IsSpace(before) && !unicode.IsSpace(at) {
			return c
		}
	}
	return 0
}

// leftCodes re-checks the repaired data: a lenient parse plus validation with
// date checks, plus the extra codes. The caller passes ENCODING_BAD_BYTES
// when decoding wrote U+FFFD (unreadable), since the replacement characters
// are valid UTF-8 in the repaired file, and ENCODING_MISLABELED when a file
// whose header named a legacy encoding was read as UTF-8, since that is a
// guess the repaired header no longer shows. Stray bytes converted in a UTF-8 file are fixed, so they
// leave nothing behind.
func leftCodes(data []byte, extra ...string) []string {
	set := map[string]bool{}
	for _, c := range extra {
		set[c] = true
	}
	opts := parser.DefaultOptions()
	opts.Lenient = true
	doc, findings, err := parser.ParseWithOptions(bytes.NewReader(data), opts)
	for _, f := range findings {
		set[f.Code] = true
	}
	var pe *parser.ParseError
	if errors.As(err, &pe) {
		set[pe.Code] = true
	}
	if doc != nil {
		vo := validator.DefaultOptions()
		vo.DateConsistency = true
		for _, e := range validator.ValidateWithOptions(doc, vo) {
			set[e.Code] = true
		}
	}
	left := make([]string, 0, len(set))
	for c := range set {
		if c != "" {
			left = append(left, c)
		}
	}
	sort.Strings(left)
	return left
}
