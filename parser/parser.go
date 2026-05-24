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

	reader := stripBOM(r)
	lines, warnings, err := parseLines(reader, opts)
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

	doc := buildDocument(lines, warnings)
	return doc, warnings, nil
}

// stripBOM wraps a reader to remove a UTF-8 BOM if present.
func stripBOM(r io.Reader) io.Reader {
	buf := make([]byte, 3)
	n, err := io.ReadFull(r, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return r
	}
	if n == 3 && bytes.Equal(buf, []byte{0xEF, 0xBB, 0xBF}) {
		return r
	}
	return io.MultiReader(bytes.NewReader(buf[:n]), r)
}

// parseLines tokenizes the input into raw lines.
func parseLines(r io.Reader, opts *Options) ([]rawLine, []*ParseError, error) {
	scanner := bufio.NewScanner(r)
	var lines []rawLine
	var warnings []*ParseError
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		text := strings.TrimRight(scanner.Text(), "\r\n")

		if strings.TrimSpace(text) == "" {
			warnings = append(warnings, newParseError(lineNum, "empty line skipped", text))
			continue
		}

		if len(text) > 255 {
			warnings = append(warnings, newParseError(lineNum, "line exceeds 255-character GEDCOM limit", text[:80]+"..."))
		}

		rl, err := parseSingleLine(text, lineNum, opts)
		if err != nil {
			return nil, warnings, err
		}
		lines = append(lines, rl)
	}

	if err := scanner.Err(); err != nil {
		return nil, warnings, wrapParseError(lineNum, "error reading input", "", err)
	}

	return lines, warnings, nil
}

func parseSingleLine(text string, lineNum int, opts *Options) (rawLine, error) {
	parts := strings.Fields(text)
	if len(parts) < 2 {
		return rawLine{}, newParseError(lineNum, "line must have at least level and tag", text)
	}

	level, err := strconv.Atoi(parts[0])
	if err != nil {
		return rawLine{}, wrapParseError(lineNum, "invalid level number", text, err)
	}
	if level < 0 {
		return rawLine{}, newParseError(lineNum, "level cannot be negative", text)
	}
	if opts.MaxNestingDepth > 0 && level > opts.MaxNestingDepth {
		return rawLine{}, newParseError(lineNum, "maximum nesting depth exceeded", text)
	}

	var xref, tag string
	var valueStart int

	if len(parts[1]) > 2 && parts[1][0] == '@' && parts[1][len(parts[1])-1] == '@' {
		xref = parts[1]
		if len(parts) < 3 {
			return rawLine{}, newParseError(lineNum, "line with xref must have a tag", text)
		}
		tag = parts[2]
		valueStart = 3
	} else {
		tag = parts[1]
		valueStart = 2
	}

	var value string
	if valueStart < len(parts) {
		tagPos := strings.Index(text, tag)
		if tagPos >= 0 {
			afterTag := tagPos + len(tag)
			if afterTag < len(text) {
				value = strings.TrimLeft(text[afterTag:], " ")
			}
		}
	}

	return rawLine{
		level:      level,
		xref:       xref,
		tag:        tag,
		value:      value,
		lineNumber: lineNum,
	}, nil
}

// buildDocument converts flat raw lines into a hierarchical GedcomDocument.
func buildDocument(lines []rawLine, warnings []*ParseError) *gedcom.GedcomDocument {
	doc := &gedcom.GedcomDocument{}

	if len(lines) == 0 {
		return doc
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
		classifyRecord(doc, record)
	}

	return doc
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
func classifyRecord(doc *gedcom.GedcomDocument, rec gedcom.GedcomRecord) {
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
		// Unknown top-level record types are silently dropped.
		// In the future, could store in an "Other" slice.
	}
}
