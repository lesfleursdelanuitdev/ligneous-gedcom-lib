// Package parser converts GEDCOM files into the canonical GedcomDocument format.
//
// The parser handles character encoding (UTF-8 BOM), line parsing, hierarchical
// structure building, and record classification. It produces a GedcomDocument
// directly — no intermediate types.
//
// Usage:
//
//	doc, warnings, err := parser.Parse(reader)
//	doc, warnings, err := parser.ParseWithOptions(reader, opts)
package parser

import (
	"bufio"
	"bytes"
	"io"
	"strconv"
	"strings"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/gedcom"
)

type rawLine struct {
	level      int
	xref       string
	tag        string
	value      string
	lineNumber int
}

// Parse reads a GEDCOM file and returns a GedcomDocument with default options.
// Returns the document, any non-fatal warnings, and a fatal error if parsing fails.
func Parse(r io.Reader) (*gedcom.GedcomDocument, []*ParseError, error) {
	return ParseWithOptions(r, DefaultOptions())
}

// ParseWithOptions reads a GEDCOM file with custom options.
func ParseWithOptions(r io.Reader, opts *Options) (*gedcom.GedcomDocument, []*ParseError, error) {
	if opts == nil {
		opts = DefaultOptions()
	}

	if opts.Context != nil {
		select {
		case <-opts.Context.Done():
			return nil, nil, opts.Context.Err()
		default:
		}
	}

	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, wrapParseError(CodeReadError, 0, "error reading input", "", err)
	}
	text, info, findings := Decode(raw)
	lines, parsed, lineCount, err := parseLines(text, opts)
	warnings := append(findings, parsed...)
	if err != nil {
		return nil, warnings, err
	}

	if opts.Context != nil {
		select {
		case <-opts.Context.Done():
			return nil, warnings, opts.Context.Err()
		default:
		}
	}

	doc, structural := buildDocument(lines)
	warnings = append(warnings, structural...)
	doc.Encoding = info
	doc.LineCount = lineCount
	return doc, warnings, nil
}

// parseLines tokenizes the input into raw lines.
func withDetails(e *ParseError, kv ...string) *ParseError {
	e.Details = map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		e.Details[kv[i]] = kv[i+1]
	}
	return e
}

// findHeadLine returns the 1-based number of the first "0 HEAD" line, or 1
// when there is none (nothing is skipped then).
func findHeadLine(text []byte) int {
	sc := bufio.NewScanner(bytes.NewReader(text))
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	sc.Split(splitGedcomLines)
	n := 0
	for sc.Scan() {
		n++
		f := strings.Fields(sc.Text())
		if len(f) >= 2 && f[0] == "0" && f[1] == "HEAD" {
			return n
		}
	}
	return 1
}

func parseLines(data []byte, opts *Options) ([]rawLine, []*ParseError, int, error) {
	headLine := findHeadLine(data)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	scanner.Split(splitGedcomLines)
	var lines []rawLine
	var warnings []*ParseError
	lineNum := 0
	prevLevel := -1

	for scanner.Scan() {
		lineNum++
		text := scanner.Text()

		if lineNum < headLine {
			// Before HEAD: not tokenised, so junk never produces BAD_LINE.
			continue
		}
		if lineNum == headLine && headLine > 1 {
			warnings = append(warnings, withDetails(newParseError(CodeLinesBeforeHead, 1,
				"lines before the 0 HEAD line were skipped", ""), "count", strconv.Itoa(headLine-1)))
		}

		if strings.TrimSpace(text) == "" {
			warnings = append(warnings, newParseError(CodeEmptyLine, lineNum, "empty line skipped", text))
			continue
		}

		if len(text) > 255 {
			warnings = append(warnings, newParseError(CodeLineTooLong, lineNum, "line exceeds 255-character GEDCOM limit", text[:80]+"..."))
		}

		rl, warn, err := parseSingleLine(text, lineNum, opts)
		if warn != nil {
			warnings = append(warnings, warn)
		}
		if err != nil {
			if opts.Lenient {
				if pe, ok := err.(*ParseError); ok {
					warnings = append(warnings, pe)
					continue
				}
			}
			return nil, warnings, lineNum, err
		}
		if rl.level > 0 && prevLevel >= 0 && rl.level > prevLevel+1 {
			warnings = append(warnings, withDetails(newParseError(CodeLevelJump, lineNum,
				"level jumps by more than one", text),
				"from", strconv.Itoa(prevLevel), "to", strconv.Itoa(rl.level)))
		}
		prevLevel = rl.level
		lines = append(lines, rl)
	}

	if err := scanner.Err(); err != nil {
		return nil, warnings, lineNum, wrapParseError(CodeReadError, lineNum, "error reading input", "", err)
	}

	return lines, warnings, lineNum, nil
}

func isSep(c byte) bool { return c == ' ' || c == '\t' }

// parseSingleLine tokenises one line by position: [ws] level SP+ [xref SP+] tag [SP value].
// The value is everything after the single space that follows the tag, verbatim.
// The returned warning (leading whitespace) is non-nil even when err is nil.
func parseSingleLine(text string, lineNum int, opts *Options) (rawLine, *ParseError, error) {
	var warn *ParseError
	i := 0
	for i < len(text) && isSep(text[i]) {
		i++
	}
	if i > 0 {
		warn = newParseError(CodeLeadingWhitespace, lineNum, "line starts with whitespace", text)
	}

	// level
	start := i
	for i < len(text) && !isSep(text[i]) {
		i++
	}
	levelTok := text[start:i]
	for i < len(text) && isSep(text[i]) {
		i++
	}
	if i >= len(text) {
		return rawLine{}, warn, newParseError(CodeBadLine, lineNum, "line must have at least level and tag", text)
	}

	level, err := strconv.Atoi(levelTok)
	if err != nil {
		return rawLine{}, warn, wrapParseError(CodeBadLevel, lineNum, "invalid level number", text, err)
	}
	if level < 0 {
		return rawLine{}, warn, newParseError(CodeBadLevel, lineNum, "level cannot be negative", text)
	}
	if opts.MaxNestingDepth > 0 && level > opts.MaxNestingDepth {
		return rawLine{}, warn, newParseError(CodeTooDeep, lineNum, "maximum nesting depth exceeded", text)
	}

	// next token: xref or tag
	start = i
	for i < len(text) && !isSep(text[i]) {
		i++
	}
	tok := text[start:i]

	var xref string
	if len(tok) > 2 && tok[0] == '@' && tok[len(tok)-1] == '@' {
		xref = tok
		for i < len(text) && isSep(text[i]) {
			i++
		}
		if i >= len(text) {
			return rawLine{}, warn, newParseError(CodeXrefWithoutTag, lineNum, "line with xref must have a tag", text)
		}
		start = i
		for i < len(text) && !isSep(text[i]) {
			i++
		}
		tok = text[start:i]
	}
	tag := tok

	var value string
	if i < len(text) {
		value = text[i+1:] // skip the single separator; rest is verbatim
	}

	return rawLine{
		level:      level,
		xref:       xref,
		tag:        tag,
		value:      value,
		lineNumber: lineNum,
	}, warn, nil
}

// buildDocument converts flat raw lines into a hierarchical GedcomDocument.
// It returns the structural findings (UNKNOWN_RECORD).
func buildDocument(lines []rawLine) (*gedcom.GedcomDocument, []*ParseError) {
	doc := &gedcom.GedcomDocument{}
	var findings []*ParseError

	if len(lines) == 0 {
		return doc, nil
	}

	// Phase 1: group lines into level-0 blocks
	type block struct {
		lines []rawLine
	}
	var blocks []block
	var cur *block

	for _, rl := range lines {
		if rl.level == 0 {
			if cur != nil {
				blocks = append(blocks, *cur)
			}
			cur = &block{lines: []rawLine{rl}}
		} else if cur != nil {
			cur.lines = append(cur.lines, rl)
		}
	}
	if cur != nil {
		blocks = append(blocks, *cur)
	}

	// Phase 2: convert each block into a GedcomRecord and classify
	for _, b := range blocks {
		record := buildRecord(b.lines, 0)
		if !classifyRecord(doc, record) {
			findings = append(findings, withDetails(newParseError(CodeUnknownRecord, b.lines[0].lineNumber,
				"unknown top-level record type", ""), "tag", record.Tag))
		}
	}

	return doc, findings
}

// buildRecord builds a hierarchical GedcomRecord from a slice of raw lines.
// startIdx points to the root line of this record.
func buildRecord(lines []rawLine, startIdx int) gedcom.GedcomRecord {
	root := lines[startIdx]
	rec := gedcom.GedcomRecord{
		Level: root.level,
		Tag:   root.tag,
		Xref:  root.xref,
		Value: root.value,
		Line:  root.lineNumber,
	}

	// Collect children using a stack-based approach
	// We need to build the hierarchy from the flat list
	type stackEntry struct {
		record *gedcom.GedcomRecord
		level  int
	}

	stack := []stackEntry{{record: &rec, level: root.level}}

	for i := startIdx + 1; i < len(lines); i++ {
		rl := lines[i]
		child := gedcom.GedcomRecord{
			Level: rl.level,
			Tag:   rl.tag,
			Xref:  rl.xref,
			Value: rl.value,
			Line:  rl.lineNumber,
		}

		// Pop stack until we find the parent for this level
		for len(stack) > 1 && stack[len(stack)-1].level >= rl.level {
			stack = stack[:len(stack)-1]
		}

		parent := stack[len(stack)-1].record
		parent.Children = append(parent.Children, child)
		// Push a pointer to the just-added child
		stack = append(stack, stackEntry{
			record: &parent.Children[len(parent.Children)-1],
			level:  rl.level,
		})
	}

	return rec
}

// classifyRecord routes a level-0 record into the appropriate document slice.
// It reports false for an unknown record type, which is dropped.
func classifyRecord(doc *gedcom.GedcomDocument, rec gedcom.GedcomRecord) bool {
	switch rec.Tag {
	case "HEAD":
		doc.Header = rec
	case "TRLR":
		doc.Trailer = rec
	case "INDI":
		doc.Individuals = append(doc.Individuals, rec)
	case "FAM":
		doc.Families = append(doc.Families, rec)
	case "SOUR":
		doc.Sources = append(doc.Sources, rec)
	case "NOTE":
		doc.Notes = append(doc.Notes, rec)
	case "REPO":
		doc.Repositories = append(doc.Repositories, rec)
	case "OBJE":
		doc.Media = append(doc.Media, rec)
	case "SUBM":
		doc.Submitters = append(doc.Submitters, rec)
	default:
		return false
	}
	return true
}
