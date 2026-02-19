// Package exporter converts GedcomDocument to various output formats:
// GEDCOM text, API-friendly JSON, and CSV.
package exporter

import (
	"fmt"
	"io"
	"strings"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/gedcom"
)

// ToGEDCOM converts a GedcomDocument back to GEDCOM text format.
// This should be a lossless round-trip from Parse.
func ToGEDCOM(doc *gedcom.GedcomDocument) string {
	var b strings.Builder
	writeGEDCOM(&b, doc)
	return b.String()
}

// WriteGEDCOM writes a GedcomDocument to a writer in GEDCOM text format.
func WriteGEDCOM(w io.Writer, doc *gedcom.GedcomDocument) error {
	_, err := io.WriteString(w, ToGEDCOM(doc))
	return err
}

func writeGEDCOM(b *strings.Builder, doc *gedcom.GedcomDocument) {
	writeRecord(b, doc.Header)

	for _, subm := range doc.Submitters {
		writeRecord(b, subm)
	}
	for _, indi := range doc.Individuals {
		writeRecord(b, indi)
	}
	for _, fam := range doc.Families {
		writeRecord(b, fam)
	}
	for _, src := range doc.Sources {
		writeRecord(b, src)
	}
	for _, repo := range doc.Repositories {
		writeRecord(b, repo)
	}
	for _, note := range doc.Notes {
		writeRecord(b, note)
	}
	for _, media := range doc.Media {
		writeRecord(b, media)
	}

	writeRecord(b, doc.Trailer)
}

func writeRecord(b *strings.Builder, rec gedcom.GedcomRecord) {
	if rec.Tag == "" {
		return
	}
	writeLine(b, rec)
	for _, child := range rec.Children {
		writeRecordRecursive(b, child)
	}
}

func writeRecordRecursive(b *strings.Builder, rec gedcom.GedcomRecord) {
	writeLine(b, rec)
	for _, child := range rec.Children {
		writeRecordRecursive(b, child)
	}
}

func writeLine(b *strings.Builder, rec gedcom.GedcomRecord) {
	fmt.Fprintf(b, "%d", rec.Level)
	if rec.Xref != "" {
		b.WriteByte(' ')
		b.WriteString(rec.Xref)
	}
	b.WriteByte(' ')
	b.WriteString(rec.Tag)
	if rec.Value != "" {
		b.WriteByte(' ')
		b.WriteString(rec.Value)
	}
	b.WriteByte('\n')
}
