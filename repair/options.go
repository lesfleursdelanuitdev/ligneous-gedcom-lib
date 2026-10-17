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

	// The data kinds change the family data, so they run only when named
	// in Options.Fixes.

	// FixBrokenLinks removes pointer lines whose target record is not in the
	// file, with the lines nested under them.
	FixBrokenLinks Fix = "broken-links"
	// FixEmptyFamilies removes FAM records with no members and no data of
	// their own, and the FAMS/FAMC links to them.
	FixEmptyFamilies Fix = "empty-families"
	// FixSubmitter adds a submitter (Options.SubmitterName) when the header
	// has none.
	FixSubmitter Fix = "submitter"
)

// FixInfo.Kind values.
const (
	KindFormat = "format" // changes how the file is written, never what it says
	KindData   = "data"   // changes the family data; never chosen by default
)

var dataKinds = []Fix{FixBrokenLinks, FixEmptyFamilies, FixSubmitter}

// kindOf is KindData for the data kinds and KindFormat for the others.
func kindOf(f Fix) string {
	for _, d := range dataKinds {
		if d == f {
			return KindData
		}
	}
	return KindFormat
}

var formatKinds = []Fix{FixEncoding, FixBOM, FixLineEndings, FixBlankLines, FixLeadingSpace, FixLongLines, FixTrailer}

// catalogue is every kind, format kinds first.
var catalogue = append(append([]Fix(nil), formatKinds...), dataKinds...)

// Fixes returns every kind of repair, in catalogue order: the format kinds,
// then the data kinds.
func Fixes() []Fix { return append([]Fix(nil), catalogue...) }

// The reasons in FixInfo.
const (
	reasonUTF16      = "this file is UTF-16, so it has to be converted first"
	reasonNeedsUTF8  = "needs the conversion to UTF-8"
	reasonReadChange = "skipped: it would change how the file is read"
	reasonFamilyData = "this family has its own information (events, notes or sources)"
	reasonNeedsName  = "needs a name"
	reasonNameLong   = "the name is too long (at most 120 characters)"
)

// rereadOK reports whether out, read again with its own encoding, is want.
// Tests replace it to reach the retry.
var rereadOK = func(out, want []byte) bool {
	again, _, _ := parser.Decode(out)
	return bytes.Equal(again, want)
}

// Options chooses the kinds of repair. A nil Fixes (in JSON: "fixes"
// omitted or null) means every format fix; an empty, non-nil Fixes (in JSON:
// "fixes": []) means none.
type Options struct {
	Fixes []Fix `json:"fixes"`
	// SubmitterName is the name FixSubmitter writes.
	SubmitterName string `json:"submitterName,omitempty"`
}

// FixInfo describes a kind of repair the file could use, whether or not it
// was chosen.
type FixInfo struct {
	Fix Fix `json:"fix"`
	// Lines: in Available, the lines of the original file it changes with
	// every fix applied; in Applied, the lines it really changed.
	Lines int `json:"lines"`
	// Required: nothing else can be done unless this fix is chosen too.
	Required bool `json:"required"`
	// Reason says why the fix is required, or why it is skipped unless
	// FixEncoding is chosen too ("needs the conversion to UTF-8").
	Reason string `json:"reason,omitempty"`
	Kind   string `json:"kind"` // KindFormat or KindData
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
//   - when the bytes are not already the decoded text, a blank line, a
//     leading-space removal or a line ending is left alone where it touches
//     a line ending in a non-ASCII byte, because the decoder may join a
//     character split across such a line and the next CONC line (an ANSEL
//     mark, or UTF-8 that the edit would make valid);
//   - no line is removed or split in HEAD before its CHAR line (or in all
//     of HEAD when the decoder finds no CHAR), since that changes which
//     CHAR line the decoder sees in its 50-line window;
//   - a blank line holding non-ASCII bytes (U+00A0 in Windows-1252, say)
//     stays, since removing it could change how the encoding is detected;
//   - the result is decoded again and must read exactly like the original
//     with the same edits; if it does not, it is made again without
//     FixBlankLines and FixLeadingSpace, and if that fails too the file is
//     returned unchanged.
//
// Result.Applied counts the lines each kind really changed. A chosen kind
// that Available lists but that changed nothing is in Result.Skipped with
// its reason: "needs the conversion to UTF-8", or "skipped: it would
// change how the file is read".
func RepairWithOptions(data []byte, opt Options) (*Result, error) {
	sel, err := selected(opt)
	if err != nil {
		return nil, err
	}
	a, err := analyse(data)
	if err != nil {
		return nil, err
	}
	full := a.plan(formatFixes(), "")
	avail := a.available(full.touched)
	if sameSet(sel, formatFixes()) {
		res, _ := a.finish(full) // converted: nothing to read again
		a.report(res, full, sel, avail, nil)
		return res, nil
	}
	o := a.plan(sel, opt.SubmitterName)
	res, ok := a.finish(o)
	forced := fixSet{}
	if !ok {
		// Blank lines and leading spaces change which lines sit next to
		// each other; try without them before giving up on the rest.
		retry := fixSet{}
		for f := range sel {
			if f == FixBlankLines || f == FixLeadingSpace {
				forced[f] = true
			} else {
				retry[f] = true
			}
		}
		o = a.plan(retry, opt.SubmitterName)
		if res, ok = a.finish(o); !ok {
			o = &outcome{touched: lineSets{}}
			res = a.unchanged()
			forced = sel
		}
	}
	a.report(res, o, sel, avail, forced)
	return res, nil
}

// report fills in Available, Applied and Skipped. A chosen kind the file
// could use that changed no line is skipped, with its reason.
func (a *analysis) report(res *Result, o *outcome, sel fixSet, avail []FixInfo, forced fixSet) {
	res.Available = avail
	res.Applied = []FixInfo{}
	res.Skipped = []FixInfo{}
	for _, f := range catalogue {
		if k := len(o.touched[f]); k > 0 {
			res.Applied = append(res.Applied, FixInfo{Fix: f, Lines: k, Kind: kindOf(f)})
		}
	}
	for _, av := range avail {
		if !sel[av.Fix] || len(o.touched[av.Fix]) > 0 || av.Lines == 0 {
			continue
		}
		reason := reasonReadChange
		switch {
		case forced[av.Fix]:
		case o.skipReason[av.Fix] != "":
			reason = o.skipReason[av.Fix]
		case !o.conv && a.raw == nil,
			!o.conv && av.Fix == FixLongLines && !a.same,
			!o.conv && av.Fix == FixBOM && a.bomNeedsConv:
			reason = reasonNeedsUTF8
		}
		res.Skipped = append(res.Skipped, FixInfo{Fix: av.Fix, Lines: av.Lines, Reason: reason, Kind: av.Kind})
	}
}

// formatFixes is the default selection: every format kind.
func formatFixes() fixSet {
	s := fixSet{}
	for _, f := range formatKinds {
		s[f] = true
	}
	return s
}

func sameSet(a, b fixSet) bool {
	if len(a) != len(b) {
		return false
	}
	for f := range a {
		if !b[f] {
			return false
		}
	}
	return true
}

func selected(opt Options) (fixSet, error) {
	if opt.Fixes == nil {
		return formatFixes(), nil
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
	// charScanEnd: lines from head up to here decide which CHAR line the
	// decoder finds (see parser scanChar); without the conversion none of
	// them is removed or split.
	charScanEnd int
	facts       dataFacts
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
	a.charScanEnd = a.scanEnd()
	a.analyseData()
	return a, nil
}

// scanEnd mirrors the parser's scanChar: it reads at most 50 lines from HEAD
// and stops at the next level-0 line. When it finds a CHAR line, that line
// is the end; otherwise the end of the HEAD record, since removing a line
// there could bring a later CHAR into the window.
func (a *analysis) scanEnd() int {
	n := len(a.src)
	isLevel0 := func(i int) bool {
		f := strings.Fields(a.src[i].text)
		return len(f) >= 2 && f[0] == "0"
	}
	for i, k := a.head, 0; i < n && k < 50; i, k = i+1, k+1 {
		f := strings.Fields(a.src[i].text)
		if i > a.head && isLevel0(i) {
			break
		}
		if len(f) >= 2 && f[0] == "1" && strings.ToUpper(f[1]) == "CHAR" {
			return i
		}
	}
	for i := a.head + 1; i < n; i++ {
		if isLevel0(i) {
			return i
		}
	}
	return n
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
	// Lines the data kinds add: after or before a line, or at the end
	// (before an added trailer).
	insertAfter, insertBefore map[int][]insLine
	appendEnd                 []insLine
}

// insLine is an added line, as written in the file's own encoding (raw) and
// in UTF-8 (dec).
type insLine struct{ raw, dec string }

// outcome is a planned run: what it does to each line, its Changes and the
// original lines each kind changes. finish turns it into a Result.
type outcome struct {
	p                    plan
	conv                 bool // FixEncoding: work on the decoded lines
	src                  []srcLine
	changes              []Change
	touched              lineSets
	keepBOM              bool
	unreadable, readUTF8 bool
	none                 bool // nothing can be done: keep the file
	// skipReason says why a chosen data kind did nothing, when the plan
	// knows (a missing name, say).
	skipReason map[Fix]string
}

// plan decides what the fixes in sel do, without writing anything.
func (a *analysis) plan(sel fixSet, name string) *outcome {
	conv := sel[FixEncoding]
	o := &outcome{conv: conv, touched: lineSets{}, changes: []Change{}, skipReason: map[Fix]string{}}
	if !conv && a.raw == nil {
		o.none = true
		return o
	}
	src := a.src
	// Without the conversion, protect the lines the decoder may join: an
	// ANSEL mark or a split UTF-8 character at the end of a line belongs to
	// the next CONC line, and an edit could also make two stray bytes into
	// one UTF-8 character. Only needed where decoding is not the identity.
	guard := false
	if !conv {
		src = a.raw
		guard = !a.same
	}
	o.src = src
	n := len(src)
	touched := o.touched
	add := func(line int, what string, f Fix) {
		o.changes = append(o.changes, Change{Line: line, What: what, Fix: f})
		if line > 0 {
			touched.mark(f, line)
		}
	}

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
	o.keepBOM = a.bomUTF8
	if a.bomUTF8 && sel[FixBOM] && (conv || !a.bomNeedsConv) {
		add(0, whatBOM, FixBOM)
		touched.mark(FixBOM, 1)
		o.keepBOM = false
	}
	if conv {
		// What decoding did to single lines: stray bytes in a UTF-8 file read
		// as Windows-1252, unreadable bytes written as U+FFFD, characters
		// split across a CONC line joined, and lines read as UTF-8 against
		// the header.
		for _, f := range a.fixes {
			if f.UTF8 > 0 { // the header named a legacy encoding
				o.readUTF8 = true
				add(f.Line, fmt.Sprintf(whatReadUTF8, f.UTF8, a.declared), FixEncoding)
			}
			if f.Stray > 0 {
				add(f.Line, fmt.Sprintf(whatStray, f.Stray), FixEncoding)
			}
			if f.Unreadable > 0 {
				o.unreadable = true
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
		insertAfter: map[int][]insLine{}, insertBefore: map[int][]insLine{},
	}

	// The data kinds come first, so the format kinds see the final lines.
	// p.removed marks every line they remove.
	a.planData(sel, name, o, &p, guard, add)

	for i := a.head; i < n; i++ {
		if p.removed[i] {
			continue
		}
		if a.blank[i] {
			if sel[FixBlankLines] && (conv || asciiBlank(a.raw[i].text) && i >= a.charScanEnd) && (!guard || !a.prevHigh(i)) {
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
			if !nd.ok || p.removed[i] || !conv && i < a.charScanEnd {
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

	crFixed, crLeft := 0, 0
	for i, l := range src {
		if l.term != "\r" || p.removed[i] {
			continue
		}
		if sel[FixLineEndings] && (!guard || !highEnd(a.raw[i].text)) {
			p.fixCR[i] = true
			crFixed++
			touched.mark(FixLineEndings, i+1)
		} else {
			crLeft++
		}
	}
	p.common = commonTerm(src, sel[FixLineEndings])
	if sel[FixTrailer] && !a.hasTrailer {
		p.trailer = true
		add(n+1, whatTrailer, FixTrailer)
	}
	switch {
	case crFixed > 0 && crLeft == 0:
		add(0, whatCRLF, FixLineEndings)
	case crFixed > 0:
		add(0, fmt.Sprintf(whatCRLFSome, crFixed), FixLineEndings)
	}
	o.p = p
	sort.SliceStable(o.changes, func(x, y int) bool { return o.changes[x].Line < o.changes[y].Line })
	return o
}

// finish writes the planned file. Without the conversion the file is read
// again with its own encoding and must read like the decoded original with
// the same edits; ok is false when it does not.
func (a *analysis) finish(o *outcome) (res *Result, ok bool) {
	if o.none {
		return a.unchanged(), true
	}
	out := o.p.emit(o.src, a.ws, a.head, !o.conv)
	if o.keepBOM {
		out = append([]byte{0xEF, 0xBB, 0xBF}, out...)
	}
	if !o.conv && !rereadOK(out, o.p.emit(a.src, a.ws, a.head, false)) {
		return nil, false
	}
	res = &Result{Data: out, Changes: o.changes}
	var extra []string
	if o.unreadable {
		extra = append(extra, parser.CodeEncodingBadBytes)
	}
	if o.readUTF8 {
		extra = append(extra, parser.CodeEncodingMislabeled)
	}
	res.Left = leftCodes(res.Data, extra...)
	return res, true
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
func (p *plan) emit(lines []srcLine, ws []int, head int, raw bool) []byte {
	out := make([]outLine, 0, len(lines)+2)
	ins := func(ls []insLine) {
		for _, l := range ls {
			if raw {
				out = append(out, outLine{l.raw, p.common})
			} else {
				out = append(out, outLine{l.dec, p.common})
			}
		}
	}
	for i, l := range lines {
		ins(p.insertBefore[i])
		term := l.term
		if p.fixCR[i] {
			term = "\r\n"
		}
		switch {
		case i < head:
			out = append(out, outLine{l.text, term})
		case p.removed[i]:
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
		ins(p.insertAfter[i])
	}
	ins(p.appendEnd)
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
		info := FixInfo{Fix: f, Lines: len(touched[f]), Kind: kindOf(f)}
		if info.Kind == KindData {
			info.Lines = a.dataLines(f)
		}
		switch f {
		case FixEncoding:
			if a.utf16 {
				info.Required, info.Reason = true, reasonUTF16
			}
		default:
			if a.raw == nil { // UTF-16: nothing without the conversion
				info.Reason = reasonNeedsUTF8
			}
		case FixBOM:
			if a.bomNeedsConv {
				info.Reason = reasonNeedsUTF8
			}
		case FixLongLines:
			if !a.same {
				info.Reason = reasonNeedsUTF8
			}
		case FixEmptyFamilies:
			if a.facts.famWithData {
				info.Reason = reasonFamilyData
			}
			if a.raw == nil {
				info.Reason = reasonNeedsUTF8
			}
		}
		// An empty-families entry with no family to remove but one that
		// holds data is listed as not offered (Lines 0, with the reason).
		notOffered := f == FixEmptyFamilies && a.facts.famWithData
		if info.Lines == 0 && !info.Required && !notOffered {
			continue
		}
		out = append(out, info)
	}
	return out
}
