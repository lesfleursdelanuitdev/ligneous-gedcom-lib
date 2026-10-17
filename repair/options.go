package repair

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
)

// Fix is one kind of repair. Every Change carries the kind it belongs to.
type Fix string

// The kinds of repair, in catalogue order (the order of Result.Available).
const (
	// FixEncoding converts the file to UTF-8 and makes the header say
	// "1 CHAR UTF-8". It covers what decoding does to single lines too:
	// stray bytes converted, unreadable bytes replaced, lines read as UTF-8
	// against the header, and characters split across a CONC line joined.
	// For a UTF-16 file it also removes the byte order mark.
	FixEncoding Fix = "encoding"
	// FixBOM removes a UTF-8 byte order mark.
	FixBOM Fix = "bom"
	// FixLineEndings turns bare "\r" line endings into "\r\n".
	FixLineEndings Fix = "line-endings"
	// FixBlankLines removes blank lines from HEAD on.
	FixBlankLines Fix = "blank-lines"
	// FixLeadingSpace removes spaces and tabs before the level number.
	FixLeadingSpace Fix = "leading-space"
	// FixLongLines splits lines over 255 bytes with CONC.
	FixLongLines Fix = "long-lines"
	// FixTrailer adds "0 TRLR" when the file has none.
	FixTrailer Fix = "trailer"
)

var catalogue = []Fix{FixEncoding, FixBOM, FixLineEndings, FixBlankLines, FixLeadingSpace, FixLongLines, FixTrailer}

// Fixes returns every kind of repair, in catalogue order.
func Fixes() []Fix { return append([]Fix(nil), catalogue...) }

// The reasons in FixInfo.
const (
	reasonUTF16     = "this file is UTF-16, so it has to be converted first"
	reasonNeedsUTF8 = "needs the conversion to UTF-8"
)

// Options chooses the kinds of repair. A nil Fixes means all of them; an
// empty, non-nil Fixes means none.
type Options struct {
	Fixes []Fix
}

// FixInfo describes a kind of repair the file could use, whether or not it
// was chosen.
type FixInfo struct {
	Fix   Fix `json:"fix"`
	Lines int `json:"lines"` // lines of the original file it changes (with every fix chosen)
	// Required: nothing else can be done unless this fix is chosen too.
	Required bool `json:"required"`
	// Reason says why the fix is required, or why it is skipped unless
	// FixEncoding is chosen too ("needs the conversion to UTF-8").
	Reason string `json:"reason,omitempty"`
}

type fixSet map[Fix]bool

// Repair applies every fix: RepairWithOptions(data, Options{}).
func Repair(data []byte) (*Result, error) {
	return RepairWithOptions(data, Options{})
}

// RepairWithOptions decodes data, applies the chosen fixes and returns the
// result. Result.Available lists every kind of fix the file could use, the
// same whatever was chosen. It fails when data holds no GEDCOM line or
// Options names an unknown fix.
//
// Without FixEncoding the bytes stay in the original encoding: the other
// fixes work on the original lines, which every supported 8-bit encoding
// writes the same way (level digits, spaces, CR/LF, "0 TRLR"). Then:
//   - a UTF-16 file is returned unchanged (FixEncoding is Required);
//   - FixLongLines is skipped unless the original bytes are already the
//     UTF-8 text (Reason "needs the conversion to UTF-8");
//   - FixBOM is skipped when the byte order mark decides how the file is
//     read (a legacy CHAR with bytes that are not plain UTF-8);
//   - in an ANSEL file, or a UTF-8 file with bytes the decoder had to fix,
//     a blank line, a leading-space removal or a line ending is left alone
//     where it touches a line ending in a non-ASCII byte, because the
//     decoder joins a character split across such a line and the next CONC
//     line;
//   - a blank line holding non-ASCII bytes (U+00A0 in Windows-1252, say)
//     stays, since removing it could change how the encoding is detected;
//   - the result is decoded again and must read exactly like the original
//     with the same edits; if it does not, the file is returned unchanged.
func RepairWithOptions(data []byte, opt Options) (*Result, error) {
	sel, err := selected(opt)
	if err != nil {
		return nil, err
	}
	a, err := analyse(data)
	if err != nil {
		return nil, err
	}
	full, touched := a.run(everyFix())
	res := full
	if len(sel) != len(catalogue) {
		res, _ = a.run(sel)
	}
	res.Available = a.available(touched)
	return res, nil
}

func everyFix() fixSet {
	s := fixSet{}
	for _, f := range catalogue {
		s[f] = true
	}
	return s
}

func selected(opt Options) (fixSet, error) {
	if opt.Fixes == nil {
		return everyFix(), nil
	}
	s := fixSet{}
	for _, f := range opt.Fixes {
		known := false
		for _, c := range catalogue {
			known = known || c == f
		}
		if !known {
			return nil, fmt.Errorf("unknown fix %q", f)
		}
		s[f] = true
	}
	return s, nil
}

// analysis is what the fixes need to know about the file, the same whatever
// fixes are chosen. Every decision is taken on the decoded text.
type analysis struct {
	data         []byte
	body         []byte // data without a UTF-8 byte order mark
	bomUTF8      bool
	bomUTF16     bool
	utf16        bool
	detected     string
	declared     string
	fixes        []parser.LineFix
	src          []srcLine // the decoded lines
	raw          []srcLine // the original lines of body; nil for UTF-16 or when they do not line up with src
	same         bool      // the decoded text is body, byte for byte
	bomNeedsConv bool      // without its byte order mark the file would read differently
	head         int
	hasHead      bool
	blank        []bool // from HEAD on
	ws           []int  // leading spaces and tabs of a GEDCOM line
	clean        []string
	nodes        []node
	hasTrailer   bool
	charLine     int  // the HEAD.CHAR line, or -1
	charRewrite  bool // that line has to say UTF-8
	charAfter    int  // where to insert CHAR when there is none, or -1
}

func analyse(data []byte) (*analysis, error) {
	text, info, _, fixes := parser.DecodeReport(data)
	a := &analysis{
		data: data, body: data, detected: info.Detected, declared: info.Declared, fixes: fixes,
		src: splitLines(text), charLine: -1, charAfter: -1,
	}
	a.bomUTF8 = bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	a.bomUTF16 = hasBOM(data) && !a.bomUTF8
	a.utf16 = strings.HasPrefix(info.Detected, "UTF-16")
	if a.bomUTF8 {
		a.body = data[3:]
		alone, _, _ := parser.Decode(a.body)
		a.bomNeedsConv = !bytes.Equal(alone, text)
	}
	if !a.utf16 {
		if raw := splitLines(a.body); len(raw) == len(a.src) {
			a.raw = raw
			a.same = bytes.Equal(text, a.body)
		}
	}
	n := len(a.src)
	for i, l := range a.src {
		if f := strings.Fields(l.text); len(f) >= 2 && f[0] == "0" && f[1] == "HEAD" {
			a.head = i
			break
		}
	}
	a.hasHead = a.head < n && isHead(a.src[a.head].text)

	// Read each line from HEAD on into the tree, as the parser does.
	a.blank = make([]bool, n)
	a.ws = make([]int, n)
	a.clean = make([]string, n)
	a.nodes = make([]node, n)
	stack := []int{}
	anyNode := false
	for i := a.head; i < n; i++ {
		t := a.src[i].text
		if strings.TrimSpace(t) == "" {
			a.blank[i] = true
			continue
		}
		// Leading whitespace counts only on a GEDCOM line (level and tag);
		// any other line is kept exactly as it is.
		s := strings.TrimLeft(t, " \t")
		nd := tokenise(s)
		nd.parent = -1
		if !nd.ok {
			a.clean[i] = t
			a.nodes[i] = nd
			continue
		}
		a.ws[i] = len(t) - len(s)
		a.clean[i] = s
		anyNode = true
		if nd.level == 0 {
			stack = append(stack[:0], i)
		} else if len(stack) > 0 {
			for len(stack) > 1 && a.nodes[stack[len(stack)-1]].level >= nd.level {
				stack = stack[:len(stack)-1]
			}
			nd.parent = stack[len(stack)-1]
			a.nodes[nd.parent].children = append(a.nodes[nd.parent].children, i)
			stack = append(stack, i)
		}
		a.nodes[i] = nd
	}
	if !anyNode {
		return nil, errors.New("the file has no GEDCOM lines")
	}

	// CHAR: rewrite it in place, or insert it after the GEDC structure.
	if a.hasHead {
		for _, c := range a.nodes[a.head].children {
			if a.nodes[c].level == 1 && a.nodes[c].tag == "CHAR" {
				a.charLine = c
				break
			}
		}
		if a.charLine >= 0 {
			a.charRewrite = value(a.clean[a.charLine], a.nodes[a.charLine]) != "UTF-8" || info.Detected != "UTF-8"
		} else {
			a.charAfter = a.head
			for _, c := range a.nodes[a.head].children {
				if a.nodes[c].level == 1 && a.nodes[c].tag == "GEDC" {
					a.charAfter = lastDescendant(a.nodes, c)
					break
				}
			}
		}
	}
	for i := a.head; i < n; i++ {
		if a.nodes[i].ok && a.nodes[i].level == 0 && a.nodes[i].tag == "TRLR" {
			a.hasTrailer = true
		}
	}
	return a, nil
}

// lineSets holds, per kind, the 1-based original lines it changed.
type lineSets map[Fix]map[int]bool

func (s lineSets) mark(f Fix, line int) {
	if s[f] == nil {
		s[f] = map[int]bool{}
	}
	s[f][line] = true
}

// plan is what one run does to each line.
type plan struct {
	removed   []bool
	strip     []bool
	fixCR     []bool
	splits    map[int][]string
	replace   map[int]string
	charAfter int
	trailer   bool
	common    string
}

// run applies the fixes in sel and returns the result (without Available)
// and the original lines each kind changed.
func (a *analysis) run(sel fixSet) (*Result, lineSets) {
	conv := sel[FixEncoding]
	if !conv && a.raw == nil {
		return a.unchanged(), lineSets{}
	}
	src := a.src
	// Protect, without the conversion, the lines the decoder may join: an
	// ANSEL mark or a split UTF-8 character at the end of a line belongs to
	// the next CONC line. The other 8-bit encodings read byte by byte.
	guard := false
	if !conv {
		src = a.raw
		guard = !a.same && (a.detected == "ANSEL" || a.detected == "UTF-8")
	}
	n := len(src)
	touched := lineSets{}
	changes := []Change{}
	add := func(line int, what string, f Fix) {
		changes = append(changes, Change{Line: line, What: what, Fix: f})
		if line > 0 {
			touched.mark(f, line)
		}
	}

	unreadable, readUTF8 := false, false
	if conv {
		if a.detected != "UTF-8" && a.detected != "ASCII" {
			add(0, "converted from "+a.detected+" to UTF-8", FixEncoding)
			for i := range a.src {
				if a.raw == nil || a.raw[i].text != a.src[i].text {
					touched.mark(FixEncoding, i+1)
				}
			}
		}
		if a.bomUTF16 {
			add(0, whatBOM, FixEncoding)
		}
	}
	keepBOM := a.bomUTF8
	if a.bomUTF8 && sel[FixBOM] && (conv || !a.bomNeedsConv) {
		add(0, whatBOM, FixBOM)
		touched.mark(FixBOM, 1)
		keepBOM = false
	}
	if conv {
		// What decoding did to single lines: stray bytes in a UTF-8 file read
		// as Windows-1252, unreadable bytes written as U+FFFD, characters
		// split across a CONC line joined, and lines read as UTF-8 against
		// the header.
		for _, f := range a.fixes {
			if f.UTF8 > 0 { // the header named a legacy encoding
				readUTF8 = true
				add(f.Line, fmt.Sprintf(whatReadUTF8, f.UTF8, a.declared), FixEncoding)
			}
			if f.Stray > 0 {
				add(f.Line, fmt.Sprintf(whatStray, f.Stray), FixEncoding)
			}
			if f.Unreadable > 0 {
				unreadable = true
				add(f.Line, fmt.Sprintf(whatUnreadable, f.Unreadable), FixEncoding)
			}
			if f.Joined > 0 { // the CONC line is the next one
				add(f.Line, fmt.Sprintf(whatJoined, f.Line, f.Line+1), FixEncoding)
			}
		}
	}

	p := plan{
		removed: make([]bool, n), strip: make([]bool, n), fixCR: make([]bool, n),
		splits: map[int][]string{}, replace: map[int]string{}, charAfter: -1,
	}
	for i := a.head; i < n; i++ {
		if a.blank[i] {
			if sel[FixBlankLines] && (conv || asciiBlank(a.raw[i].text)) && (!guard || !a.prevHigh(i)) {
				p.removed[i] = true
				add(i+1, whatBlank, FixBlankLines)
			}
			continue
		}
		if a.ws[i] > 0 && sel[FixLeadingSpace] && (!guard || a.rawStripOK(i)) {
			p.strip[i] = true
			add(i+1, whatWhitespace, FixLeadingSpace)
		}
	}

	if conv && a.hasHead {
		switch {
		case a.charLine >= 0:
			if a.charRewrite {
				p.replace[a.charLine] = "1 CHAR UTF-8"
				add(a.charLine+1, whatCharset, FixEncoding)
			}
		default:
			p.charAfter = a.charAfter
			add(a.head+1, whatCharset, FixEncoding)
		}
	}

	// Long lines, only where the output is UTF-8.
	if sel[FixLongLines] && (conv || a.same) {
		for i := a.head; i < n; i++ {
			nd := a.nodes[i]
			if !nd.ok {
				continue
			}
			t := p.text(src, a.ws, i)
			if len(t) <= maxLine || !canSplit(a.nodes, a.clean, i) {
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
			vs := len(t) - len(a.clean[i]) + nd.valueStart // leading spaces kept
			prefix := t[:vs]
			parts := splitValue(t[vs:], maxLine-len(prefix), maxLine-len(contPrefix))
			if parts == nil {
				continue
			}
			lines := []string{prefix + parts[0]}
			for _, s := range parts[1:] {
				lines = append(lines, contPrefix+s)
			}
			p.splits[i] = lines
			add(i+1, fmt.Sprintf("split a %d-character line into %d lines", utf8.RuneCountInString(a.src[i].text), len(lines)),
				FixLongLines)
		}
	}

	crChanged := false
	for i, l := range src {
		if l.term == "\r" && sel[FixLineEndings] && !p.removed[i] && (!guard || !highEnd(a.raw[i].text)) {
			p.fixCR[i] = true
			crChanged = true
			touched.mark(FixLineEndings, i+1)
		}
	}
	p.common = commonTerm(src, sel[FixLineEndings])
	if sel[FixTrailer] && !a.hasTrailer {
		p.trailer = true
		add(n+1, whatTrailer, FixTrailer)
	}
	if crChanged {
		add(0, whatCRLF, FixLineEndings)
	}

	out := p.emit(src, a.ws, a.head)
	if keepBOM {
		out = append([]byte{0xEF, 0xBB, 0xBF}, out...)
	}
	if !conv {
		// The file is read again with its own encoding: it must read like
		// the decoded original with the same edits.
		again, _, _ := parser.Decode(out)
		if !bytes.Equal(again, p.emit(a.src, a.ws, a.head)) {
			return a.unchanged(), lineSets{}
		}
	}

	sort.SliceStable(changes, func(x, y int) bool { return changes[x].Line < changes[y].Line })
	res := &Result{Data: out, Changes: changes}
	var extra []string
	if unreadable {
		extra = append(extra, parser.CodeEncodingBadBytes)
	}
	if readUTF8 {
		extra = append(extra, parser.CodeEncodingMislabeled)
	}
	res.Left = leftCodes(res.Data, extra...)
	return res, touched
}

// unchanged is the result that keeps the file as it is.
func (a *analysis) unchanged() *Result {
	data := bytes.Clone(a.data)
	return &Result{Data: data, Changes: []Change{}, Left: leftCodes(data)}
}

// text is line i of lines as the plan leaves it before splitting.
func (p *plan) text(lines []srcLine, ws []int, i int) string {
	if p.strip[i] {
		return lines[i].text[ws[i]:]
	}
	return lines[i].text
}

// emit writes lines with the plan applied.
func (p *plan) emit(lines []srcLine, ws []int, head int) []byte {
	out := make([]outLine, 0, len(lines)+2)
	for i, l := range lines {
		term := l.term
		if p.fixCR[i] {
			term = "\r\n"
		}
		switch {
		case i < head:
			out = append(out, outLine{l.text, term})
		case p.removed[i]:
			continue
		case p.splits[i] != nil:
			for _, s := range p.splits[i] {
				out = append(out, outLine{s, term})
			}
		case p.replace[i] != "":
			out = append(out, outLine{p.replace[i], term})
		default:
			out = append(out, outLine{p.text(lines, ws, i), term})
		}
		if i == p.charAfter {
			out = append(out, outLine{"1 CHAR UTF-8", p.common})
		}
	}
	if p.trailer {
		out = append(out, outLine{"0 TRLR", p.common})
	}
	var buf bytes.Buffer
	for k, o := range out {
		buf.WriteString(o.text)
		if o.term == "" && k < len(out)-1 {
			o.term = p.common
		}
		buf.WriteString(o.term)
	}
	return buf.Bytes()
}

// highEnd reports whether a line ends in a non-ASCII byte: the decoder may
// join it with the next line (a UTF-8 character or an ANSEL mark split
// across a CONC line).
func highEnd(s string) bool { return s != "" && s[len(s)-1] >= utf8.RuneSelf }

func asciiBlank(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return strings.TrimSpace(s) == ""
}

// prevHigh reports whether the line before i, skipping blank lines, ends in
// a non-ASCII byte.
func (a *analysis) prevHigh(i int) bool {
	j := i - 1
	for j >= 0 && asciiBlank(a.raw[j].text) {
		j--
	}
	return j >= 0 && highEnd(a.raw[j].text)
}

// rawStripOK reports whether the original line i has the same leading
// whitespace as the decoded line and can lose it without changing how the
// decoder reads the file.
func (a *analysis) rawStripOK(i int) bool {
	t := a.raw[i].text
	return len(t)-len(strings.TrimLeft(t, " \t")) == a.ws[i] && !highEnd(t) && !a.prevHigh(i)
}

// available lists the kinds the file could use, from the lines a run with
// every fix changed.
func (a *analysis) available(touched lineSets) []FixInfo {
	out := []FixInfo{}
	for _, f := range catalogue {
		info := FixInfo{Fix: f, Lines: len(touched[f])}
		switch f {
		case FixEncoding:
			if a.utf16 {
				info.Required, info.Reason = true, reasonUTF16
			}
		case FixBOM:
			if a.bomNeedsConv {
				info.Reason = reasonNeedsUTF8
			}
		case FixLongLines:
			if !a.same {
				info.Reason = reasonNeedsUTF8
			}
		}
		if info.Lines == 0 && !info.Required {
			continue
		}
		out = append(out, info)
	}
	return out
}
