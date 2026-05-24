package exporter

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/gedcom"
)

// MaxPhysicalGEDCOMLineLen is the GEDCOM 5.5 / 5.5.1 limit for a physical record line
// (excluding the terminating newline/CRLF).
const MaxPhysicalGEDCOMLineLen = 255

func gedcomLevelStringLen(level int) int {
	return len(strconv.Itoa(level))
}

// gedcomPhysicalLineByteLen matches exporter.writeLine output length in bytes (UTF-8)
// for a record with optional level-0 xref.
func gedcomPhysicalLineByteLen(level int, xref, tag, value string) int {
	n := gedcomLevelStringLen(level) + 1 // level + space
	if level == 0 && strings.TrimSpace(xref) != "" {
		n += len(ensureXrefPointer(xref)) + 1
	}
	n += len(tag)
	if value != "" {
		n += 1 + len(value)
	}
	return n
}

// gedcomValuePrefixByteLen is the byte length of the prefix up to and including the delimiter
// space before the payload (e.g. "0 @N1@ NOTE " or "1 CONT ").
func gedcomValuePrefixByteLen(level int, xref, tag string) int {
	n := gedcomLevelStringLen(level) + 1
	if level == 0 && strings.TrimSpace(xref) != "" {
		n += len(ensureXrefPointer(xref)) + 1
	}
	n += len(tag) + 1
	return n
}

// maxTagLinePayload returns max bytes for the value on a TAG line (NOTE, TEXT, CONT, …).
func maxTagLinePayload(level int, xref string, tag string) int {
	prefixLen := gedcomValuePrefixByteLen(level, xref, tag)
	if prefixLen >= MaxPhysicalGEDCOMLineLen {
		return 0
	}
	return MaxPhysicalGEDCOMLineLen - prefixLen
}

// maxTagOnlyPayload is max bytes for "level TAG value" when xref is never on this line.
func maxTagPayload(level int, tag string) int {
	return maxTagLinePayload(level, "", tag)
}

// splitUTF8IntoMaxByteChunks splits s into substrings each at most maxBytes when encoded as UTF-8,
// without breaking code points. A single rune longer than maxBytes is still emitted alone.
func splitUTF8IntoMaxByteChunks(s string, maxBytes int) []string {
	if maxBytes <= 0 {
		if s == "" {
			return nil
		}
		return []string{s}
	}
	var chunks []string
	remaining := s
	for len(remaining) > 0 {
		size := 0
		acc := 0
		for len(remaining) > size {
			_, rw := utf8.DecodeRuneInString(remaining[size:])
			if rw == 0 {
				break
			}
			if acc+rw > maxBytes {
				if acc > 0 {
					break
				}
				size += rw
				acc += rw
				break
			}
			acc += rw
			size += rw
		}
		if size == 0 {
			break
		}
		chunks = append(chunks, remaining[:size])
		remaining = remaining[size:]
	}
	return chunks
}

// appendWrappedTaggedOntoRecord splits free-text into primary TAG / CONT / CONC lines so no
// physical line exceeds MaxPhysicalGEDCOMLineLen. Used for NOTE, SOURCE TEXT, etc.
func appendWrappedTaggedOntoRecord(rec *gedcom.GedcomRecord, lineLevel int, xref string, tag string, content string) {
	paragraphs := strings.Split(content, "\n")
	for i, para := range paragraphs {
		if i == 0 {
			appendFirstParagraphTagged(rec, lineLevel, xref, tag, para)
			continue
		}
		appendContinuedParagraphNote(rec, lineLevel, para)
	}
}

// appendWrappedNoteOntoRecord wraps NOTE payloads (optional level-0 xref).
func appendWrappedNoteOntoRecord(rec *gedcom.GedcomRecord, noteLevel int, xref string, content string) {
	appendWrappedTaggedOntoRecord(rec, noteLevel, xref, "NOTE", content)
}

func appendFirstParagraphTagged(rec *gedcom.GedcomRecord, lineLevel int, xref string, tag string, para string) {
	maxVal := maxTagLinePayload(lineLevel, xref, tag)
	if maxVal < 1 {
		maxVal = 1
	}
	chunks := splitUTF8IntoMaxByteChunks(para, maxVal)
	if len(chunks) == 0 {
		rec.Value = ""
		return
	}
	rec.Tag = tag
	rec.Value = chunks[0]
	childLevel := lineLevel + 1
	for _, ch := range chunks[1:] {
		rec.AddChild(gedcom.GedcomRecord{Level: childLevel, Tag: "CONC", Value: ch})
	}
}

// buildWrappedSubtag builds a GedcomRecord for a scalar tag value, splitting
// any overflow into CONC children so no physical line exceeds MaxPhysicalGEDCOMLineLen.
func buildWrappedSubtag(level int, tag, value string) gedcom.GedcomRecord {
	rec := gedcom.GedcomRecord{Level: level, Tag: tag}
	maxVal := maxTagPayload(level, tag)
	chunks := splitUTF8IntoMaxByteChunks(value, maxVal)
	if len(chunks) == 0 {
		return rec
	}
	rec.Value = chunks[0]
	for _, ch := range chunks[1:] {
		rec.AddChild(gedcom.GedcomRecord{Level: level + 1, Tag: "CONC", Value: ch})
	}
	return rec
}

func appendContinuedParagraphNote(rec *gedcom.GedcomRecord, noteLevel int, para string) {
	contLevel := noteLevel + 1
	maxCont := maxTagPayload(contLevel, "CONT")
	if maxCont < 1 {
		maxCont = 1
	}
	chunks := splitUTF8IntoMaxByteChunks(para, maxCont)
	if len(chunks) == 0 {
		return
	}
	rec.AddChild(gedcom.GedcomRecord{Level: contLevel, Tag: "CONT", Value: chunks[0]})
	for _, ch := range chunks[1:] {
		rec.AddChild(gedcom.GedcomRecord{Level: contLevel, Tag: "CONC", Value: ch})
	}
}
