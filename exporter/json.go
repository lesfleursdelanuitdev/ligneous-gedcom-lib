package exporter

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/gedcom"
)

// ToJSON converts a GedcomDocument to an API-friendly DenormalizedJSON.
func ToJSON(doc *gedcom.GedcomDocument) *DenormalizedJSON {
	idx := doc.XRefIndex()

	result := &DenormalizedJSON{
		File: FileMetadata{
			IndividualsCount: len(doc.Individuals),
			FamiliesCount:    len(doc.Families),
		},
		Notes:   make(map[string]DenormalizedNote),
		Sources: make(map[string]DenormalizedSource),
	}

	// Build notes map first for reference lookups
	for _, noteRec := range doc.Notes {
		if noteRec.Xref != "" {
			result.Notes[noteRec.Xref] = DenormalizedNote{
				Xref:    noteRec.Xref,
				Content: extractNoteContent(noteRec),
			}
		}
	}

	// Build sources map
	for _, srcRec := range doc.Sources {
		if srcRec.Xref != "" {
			result.Sources[srcRec.Xref] = DenormalizedSource{
				Xref:        srcRec.Xref,
				Title:       srcRec.ChildValue("TITL"),
				Author:      srcRec.ChildValue("AUTH"),
				Publication: srcRec.ChildValue("PUBL"),
			}
		}
	}

	// Convert individuals
	for _, indiRec := range doc.Individuals {
		result.Individuals = append(result.Individuals, convertIndividual(indiRec, idx))
	}

	// Convert families
	for _, famRec := range doc.Families {
		result.Families = append(result.Families, convertFamily(famRec, idx))
	}

	return result
}

// WriteJSON writes DenormalizedJSON to a writer.
func WriteJSON(w io.Writer, doc *gedcom.GedcomDocument) error {
	data := ToJSON(doc)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func convertIndividual(rec gedcom.GedcomRecord, idx map[string]*gedcom.GedcomRecord) DenormalizedIndividual {
	indi := DenormalizedIndividual{
		Xref:       rec.Xref,
		GivenNames: []string{},
		Surnames:   []string{},
		Parents:    []DenormalizedRelationship{},
		Spouses:    []DenormalizedRelationship{},
		Children:   []DenormalizedRelationship{},
		Events:     []DenormalizedEvent{},
		Notes:      []DenormalizedNoteRef{},
		Sources:    []DenormalizedSourceRef{},
	}

	// Extract name components
	nameRecs := rec.ChildrenByTag("NAME")
	if len(nameRecs) > 0 {
		indi.Name = nameRecs[0].Value
		for _, givn := range nameRecs[0].ChildrenByTag("GIVN") {
			if givn.Value != "" {
				indi.GivenNames = append(indi.GivenNames, givn.Value)
			}
		}
		for _, surn := range nameRecs[0].ChildrenByTag("SURN") {
			if surn.Value != "" {
				indi.Surnames = append(indi.Surnames, surn.Value)
			}
		}
	}

	// Extract SEX
	indi.Sex = rec.ChildValue("SEX")

	// Extract events
	for _, child := range rec.Children {
		if isEventTag(child.Tag) {
			event := extractEvent(child)
			if child.Tag == "BIRT" && indi.Birth == nil {
				indi.Birth = &event
			} else if child.Tag == "DEAT" && indi.Death == nil {
				indi.Death = &event
			}
			indi.Events = append(indi.Events, event)
		}
	}

	// Extract FAMC (parents)
	for _, famc := range rec.ChildrenByTag("FAMC") {
		if famc.Value != "" {
			indi.Parents = append(indi.Parents, DenormalizedRelationship{
				Xref:         famc.Value,
				Relationship: "parent",
			})
		}
	}

	// Extract FAMS (spouses)
	for _, fams := range rec.ChildrenByTag("FAMS") {
		if fams.Value != "" {
			indi.Spouses = append(indi.Spouses, DenormalizedRelationship{
				Xref:         fams.Value,
				Relationship: "spouse",
			})
		}
	}

	// Recursively extract NOTE references, skipping event sub-trees
	indi.Notes = collectNoteRefs(rec, individualEventTags)

	// Extract SOUR references
	for _, sour := range rec.ChildrenByTag("SOUR") {
		if sour.Value != "" {
			indi.Sources = append(indi.Sources, DenormalizedSourceRef{Xref: sour.Value})
		}
	}

	return indi
}

func convertFamily(rec gedcom.GedcomRecord, idx map[string]*gedcom.GedcomRecord) DenormalizedFamily {
	fam := DenormalizedFamily{
		Xref:     rec.Xref,
		Children: []DenormalizedRelationship{},
		Events:   []DenormalizedEvent{},
		Notes:    []DenormalizedNoteRef{},
		Sources:  []DenormalizedSourceRef{},
	}

	// Extract HUSB
	husbRecs := rec.ChildrenByTag("HUSB")
	if len(husbRecs) > 0 && husbRecs[0].Value != "" {
		fam.Husband = &DenormalizedRelationship{
			Xref: husbRecs[0].Value,
			Name: resolveIndiName(husbRecs[0].Value, idx),
		}
	}

	// Extract WIFE
	wifeRecs := rec.ChildrenByTag("WIFE")
	if len(wifeRecs) > 0 && wifeRecs[0].Value != "" {
		fam.Wife = &DenormalizedRelationship{
			Xref: wifeRecs[0].Value,
			Name: resolveIndiName(wifeRecs[0].Value, idx),
		}
	}

	// Extract CHIL
	for _, chil := range rec.ChildrenByTag("CHIL") {
		if chil.Value != "" {
			fam.Children = append(fam.Children, DenormalizedRelationship{
				Xref: chil.Value,
				Name: resolveIndiName(chil.Value, idx),
			})
		}
	}

	// Extract events
	for _, child := range rec.Children {
		if isEventTag(child.Tag) {
			event := extractEvent(child)
			if child.Tag == "MARR" && fam.Marriage == nil {
				fam.Marriage = &event
			} else if child.Tag == "DIV" && fam.Divorce == nil {
				fam.Divorce = &event
			}
			fam.Events = append(fam.Events, event)
		}
	}

	// Recursively extract NOTE references, skipping event sub-trees
	fam.Notes = collectNoteRefs(rec, familyEventTags)

	// Extract SOUR references
	for _, sour := range rec.ChildrenByTag("SOUR") {
		if sour.Value != "" {
			fam.Sources = append(fam.Sources, DenormalizedSourceRef{Xref: sour.Value})
		}
	}

	return fam
}

func extractEvent(rec gedcom.GedcomRecord) DenormalizedEvent {
	eventType := rec.Tag
	if eventType == "EVEN" {
		if t := rec.ChildValue("TYPE"); t != "" {
			eventType = t
		}
	}

	event := DenormalizedEvent{
		Type:  eventType,
		Date:  rec.ChildValue("DATE"),
		Place: rec.ChildValue("PLAC"),
	}

	if event.Date != "" {
		event.DateYear = parseYear(event.Date)
	}

	return event
}

func extractNoteContent(noteRec gedcom.GedcomRecord) string {
	var parts []string
	if noteRec.Value != "" {
		parts = append(parts, noteRec.Value)
	}
	for _, child := range noteRec.Children {
		switch child.Tag {
		case "CONT":
			parts = append(parts, child.Value)
		case "CONC":
			if len(parts) > 0 {
				parts[len(parts)-1] += child.Value
			} else {
				parts = append(parts, child.Value)
			}
		}
	}
	return strings.Join(parts, "\n")
}

func resolveIndiName(xref string, idx map[string]*gedcom.GedcomRecord) string {
	rec := idx[xref]
	if rec == nil {
		return ""
	}
	return rec.ChildValue("NAME")
}

func isEventTag(tag string) bool {
	switch tag {
	case "BIRT", "CHR", "DEAT", "BURI", "CREM", "ADOP", "BAPM",
		"BARM", "BASM", "BLES", "CHRA", "CONF", "FCOM", "ORDN",
		"NATU", "EMIG", "IMMI", "CENS", "PROB", "WILL", "GRAD",
		"RETI", "EVEN", "MARR", "ANUL", "DIV", "DIVF", "ENGA",
		"MARB", "MARC", "MARL", "MARS", "RESI":
		return true
	}
	return false
}

func parseYear(dateStr string) int {
	if dateStr == "" {
		return 0
	}
	for _, part := range strings.Fields(dateStr) {
		if len(part) == 4 {
			if y, err := strconv.Atoi(part); err == nil && y > 1000 && y < 3000 {
				return y
			}
		}
	}
	return 0
}

// Event tag sets mirror the enricher's definitions so that note extraction
// for individuals/families correctly skips event sub-trees (handled separately).
var individualEventTags = map[string]bool{
	"BIRT": true, "CHR": true, "DEAT": true, "BURI": true, "CREM": true,
	"ADOP": true, "BAPM": true, "BARM": true, "BASM": true, "BLES": true,
	"CHRA": true, "CONF": true, "FCOM": true, "ORDN": true, "NATU": true,
	"EMIG": true, "IMMI": true, "CENS": true, "PROB": true, "WILL": true,
	"GRAD": true, "RETI": true, "EVEN": true, "RESI": true,
}

var familyEventTags = map[string]bool{
	"MARR": true, "ANUL": true, "DIV": true, "DIVF": true, "ENGA": true,
	"MARB": true, "MARC": true, "MARL": true, "MARS": true, "EVEN": true,
}

// collectNoteRefs recursively finds all NOTE references in a record's subtree.
// skipTags prevents recursion into certain child tags (e.g. event tags that
// should be attributed to the event, not the parent record). Returns a
// deduplicated slice of note references.
func collectNoteRefs(rec gedcom.GedcomRecord, skipTags map[string]bool) []DenormalizedNoteRef {
	seen := make(map[string]bool)
	var refs []DenormalizedNoteRef
	collectNoteRefsRecursive(rec, skipTags, seen, &refs)
	return refs
}

func collectNoteRefsRecursive(rec gedcom.GedcomRecord, skipTags map[string]bool, seen map[string]bool, refs *[]DenormalizedNoteRef) {
	for _, child := range rec.Children {
		if child.Tag == "NOTE" && child.Value != "" && !seen[child.Value] {
			seen[child.Value] = true
			*refs = append(*refs, DenormalizedNoteRef{Xref: child.Value})
		}
		if child.Tag != "NOTE" && (skipTags == nil || !skipTags[child.Tag]) {
			collectNoteRefsRecursive(child, skipTags, seen, refs)
		}
	}
}

// collectNoteXrefs recursively finds all NOTE xref strings in a record's subtree.
// Same logic as collectNoteRefs but returns raw strings for CSV use.
func collectNoteXrefs(rec gedcom.GedcomRecord, skipTags map[string]bool) []string {
	seen := make(map[string]bool)
	var xrefs []string
	collectNoteXrefsRecursive(rec, skipTags, seen, &xrefs)
	return xrefs
}

func collectNoteXrefsRecursive(rec gedcom.GedcomRecord, skipTags map[string]bool, seen map[string]bool, xrefs *[]string) {
	for _, child := range rec.Children {
		if child.Tag == "NOTE" && child.Value != "" && !seen[child.Value] {
			seen[child.Value] = true
			*xrefs = append(*xrefs, child.Value)
		}
		if child.Tag != "NOTE" && (skipTags == nil || !skipTags[child.Tag]) {
			collectNoteXrefsRecursive(child, skipTags, seen, xrefs)
		}
	}
}

// ToJSONString is a convenience function that returns JSON as a formatted string.
func ToJSONString(doc *gedcom.GedcomDocument) (string, error) {
	data := ToJSON(doc)
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal JSON: %w", err)
	}
	return string(b), nil
}
