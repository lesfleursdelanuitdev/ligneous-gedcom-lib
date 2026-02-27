package exporter

import (
	"strings"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/enricher"
	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/gedcom"
)

// FromEnriched reconstructs a GedcomDocument from an EnrichedDocument.
// The resulting document can be passed to ToGEDCOM for GEDCOM text output.
func FromEnriched(ed *enricher.EnrichedDocument) *gedcom.GedcomDocument {
	doc := &gedcom.GedcomDocument{
		Header:  buildHeader(),
		Trailer: gedcom.GedcomRecord{Level: 0, Tag: "TRLR"},
	}

	indiEvents := groupEventsByOwner(ed, "INDI")
	famEvents := groupEventsByOwner(ed, "FAM")
	indiNotes := groupIndividualNotes(ed)
	famNotes := groupFamilyNotes(ed)
	eventNotes := groupEventNotes(ed)
	indiSources := groupIndividualSources(ed)
	famSources := groupFamilySources(ed)
	famChildren := groupFamilyChildren(ed)
	parentChild := groupParentChildByChild(ed)
	spousesByIndi := groupSpousesByIndividual(ed)

	for _, indi := range ed.Individuals {
		rec := buildIndividualRecord(ed, indi, indiEvents[indi.Xref],
			indiNotes[indi.Xref], indiSources[indi.Xref],
			parentChild[indi.Xref], spousesByIndi[indi.Xref], eventNotes)
		doc.Individuals = append(doc.Individuals, rec)
	}

	for _, fam := range ed.Families {
		rec := buildFamilyRecord(ed, fam, famEvents[fam.Xref],
			famNotes[fam.Xref], famSources[fam.Xref],
			famChildren[fam.Xref], eventNotes)
		doc.Families = append(doc.Families, rec)
	}

	for _, src := range ed.Sources {
		doc.Sources = append(doc.Sources, buildSourceRecord(src))
	}

	for _, note := range ed.Notes {
		if note.IsTopLevel && note.Xref != "" {
			doc.Notes = append(doc.Notes, buildNoteRecord(note))
		}
	}

	for _, repo := range ed.Repositories {
		doc.Repositories = append(doc.Repositories, buildRepositoryRecord(repo))
	}

	for _, media := range ed.Media {
		doc.Media = append(doc.Media, buildMediaRecord(media))
	}

	return doc
}

func buildHeader() gedcom.GedcomRecord {
	header := gedcom.GedcomRecord{Level: 0, Tag: "HEAD"}
	sour := gedcom.GedcomRecord{Level: 1, Tag: "SOUR", Value: "Ligneous"}
	sour.AddChild(gedcom.GedcomRecord{Level: 2, Tag: "NAME", Value: "Ligneous Genealogy"})
	sour.AddChild(gedcom.GedcomRecord{Level: 2, Tag: "VERS", Value: "1.0"})
	header.AddChild(sour)

	gedc := gedcom.GedcomRecord{Level: 1, Tag: "GEDC"}
	gedc.AddChild(gedcom.GedcomRecord{Level: 2, Tag: "VERS", Value: "5.5.1"})
	gedc.AddChild(gedcom.GedcomRecord{Level: 2, Tag: "FORM", Value: "LINEAGE-LINKED"})
	header.AddChild(gedc)

	header.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "CHAR", Value: "UTF-8"})
	return header
}

func buildIndividualRecord(
	ed *enricher.EnrichedDocument,
	indi enricher.EnrichedIndividual,
	events []enricher.Event,
	noteLinks []enricher.IndividualNoteLink,
	sourceLinks []enricher.IndividualSourceLink,
	parentLinks []enricher.ParentChildEdge,
	spouseEdges []enricher.SpouseEdge,
	eventNotes map[int][]enricher.EventNoteLink,
) gedcom.GedcomRecord {
	rec := gedcom.GedcomRecord{Level: 0, Tag: "INDI", Xref: indi.Xref}

	for _, nameRec := range buildNameRecords(ed, indi) {
		rec.AddChild(nameRec)
	}

	if indi.Sex != "" {
		rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "SEX", Value: indi.Sex})
	}
	if indi.Gender != "" {
		rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "_GENDER", Value: indi.Gender})
	}

	for _, evt := range events {
		evtRec := buildEventRecord(ed, evt, 1, eventNotes)
		rec.AddChild(evtRec)
	}

	famcXrefs := make(map[string]bool)
	for _, pc := range parentLinks {
		if !famcXrefs[pc.FamilyXref] {
			famcXrefs[pc.FamilyXref] = true
			child := gedcom.GedcomRecord{Level: 1, Tag: "FAMC", Value: pc.FamilyXref}
			if pc.Pedigree != "" {
				child.AddChild(gedcom.GedcomRecord{Level: 2, Tag: "PEDI", Value: pc.Pedigree})
			}
			rec.AddChild(child)
		}
	}

	famsXrefs := make(map[string]bool)
	for _, se := range spouseEdges {
		if !famsXrefs[se.FamilyXref] {
			famsXrefs[se.FamilyXref] = true
			rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "FAMS", Value: se.FamilyXref})
		}
	}

	for _, nl := range noteLinks {
		if nl.NoteIndex >= 0 && nl.NoteIndex < len(ed.Notes) {
			note := ed.Notes[nl.NoteIndex]
			if note.IsTopLevel && note.Xref != "" {
				rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "NOTE", Value: note.Xref})
			} else if note.Content != "" {
				rec.AddChild(buildInlineNote(1, note.Content))
			}
		}
	}

	for _, sl := range sourceLinks {
		if sl.SourceIndex >= 0 && sl.SourceIndex < len(ed.Sources) {
			src := ed.Sources[sl.SourceIndex]
			sourRec := gedcom.GedcomRecord{Level: 1, Tag: "SOUR", Value: src.Xref}
			if sl.Page != "" {
				sourRec.AddChild(gedcom.GedcomRecord{Level: 2, Tag: "PAGE", Value: sl.Page})
			}
			rec.AddChild(sourRec)
		}
	}

	return rec
}

func buildNameRecords(ed *enricher.EnrichedDocument, indi enricher.EnrichedIndividual) []gedcom.GedcomRecord {
	var recs []gedcom.GedcomRecord

	for nfIdx, nf := range ed.NameForms {
		if nf.IndividualXref != indi.Xref {
			continue
		}

		var givnParts []string
		var surnParts []string

		for _, gnLink := range ed.NameFormGivenNames {
			if gnLink.NameFormIndex == nfIdx && gnLink.GivenNameIndex >= 0 && gnLink.GivenNameIndex < len(ed.GivenNames) {
				givnParts = append(givnParts, ed.GivenNames[gnLink.GivenNameIndex].Value)
			}
		}
		for _, snLink := range ed.NameFormSurnames {
			if snLink.NameFormIndex == nfIdx && snLink.SurnameIndex >= 0 && snLink.SurnameIndex < len(ed.Surnames) {
				surnParts = append(surnParts, ed.Surnames[snLink.SurnameIndex].Value)
			}
		}

		var fullVal string
		if len(surnParts) > 0 {
			fullVal = strings.TrimSpace(strings.Join(givnParts, " ")) + " /" + strings.Join(surnParts, "/") + "/"
		} else {
			fullVal = strings.TrimSpace(strings.Join(givnParts, " "))
		}

		nameRec := gedcom.GedcomRecord{Level: 1, Tag: "NAME", Value: fullVal}
		if len(givnParts) > 0 {
			nameRec.AddChild(gedcom.GedcomRecord{Level: 2, Tag: "GIVN", Value: strings.Join(givnParts, " ")})
		}
		if len(surnParts) > 0 {
			nameRec.AddChild(gedcom.GedcomRecord{Level: 2, Tag: "SURN", Value: strings.Join(surnParts, " ")})
		}
		if nf.NameType != "" && nf.NameType != "birth" {
			nameRec.AddChild(gedcom.GedcomRecord{Level: 2, Tag: "TYPE", Value: nf.NameType})
		}
		recs = append(recs, nameRec)
	}

	if len(recs) == 0 {
		recs = append(recs, gedcom.GedcomRecord{Level: 1, Tag: "NAME", Value: indi.FullName})
	}

	return recs
}

func buildEventRecord(
	ed *enricher.EnrichedDocument,
	evt enricher.Event,
	level int,
	eventNotes map[int][]enricher.EventNoteLink,
) gedcom.GedcomRecord {
	tag := evt.EventType
	if evt.CustomType != "" && tag == "EVEN" {
		tag = "EVEN"
	}
	evtRec := gedcom.GedcomRecord{Level: level, Tag: tag}
	if evt.Value != "" {
		evtRec.Value = evt.Value
	}

	if evt.DateIndex >= 0 && evt.DateIndex < len(ed.Dates) {
		evtRec.AddChild(gedcom.GedcomRecord{Level: level + 1, Tag: "DATE", Value: ed.Dates[evt.DateIndex].Original})
	}
	if evt.PlaceIndex >= 0 && evt.PlaceIndex < len(ed.Places) {
		evtRec.AddChild(gedcom.GedcomRecord{Level: level + 1, Tag: "PLAC", Value: ed.Places[evt.PlaceIndex].Original})
	}
	if evt.Cause != "" {
		evtRec.AddChild(gedcom.GedcomRecord{Level: level + 1, Tag: "CAUS", Value: evt.Cause})
	}
	if evt.Agency != "" {
		evtRec.AddChild(gedcom.GedcomRecord{Level: level + 1, Tag: "AGNC", Value: evt.Agency})
	}
	if evt.CustomType != "" && tag == "EVEN" {
		evtRec.AddChild(gedcom.GedcomRecord{Level: level + 1, Tag: "TYPE", Value: evt.CustomType})
	}

	for _, enl := range eventNotes[evt.Index] {
		if enl.NoteIndex >= 0 && enl.NoteIndex < len(ed.Notes) {
			note := ed.Notes[enl.NoteIndex]
			if note.IsTopLevel && note.Xref != "" {
				evtRec.AddChild(gedcom.GedcomRecord{Level: level + 1, Tag: "NOTE", Value: note.Xref})
			} else if note.Content != "" {
				evtRec.AddChild(buildInlineNote(level+1, note.Content))
			}
		}
	}

	return evtRec
}

func buildFamilyRecord(
	ed *enricher.EnrichedDocument,
	fam enricher.EnrichedFamily,
	events []enricher.Event,
	noteLinks []enricher.FamilyNoteLink,
	sourceLinks []enricher.FamilySourceLink,
	children []enricher.FamilyChildEdge,
	eventNotes map[int][]enricher.EventNoteLink,
) gedcom.GedcomRecord {
	rec := gedcom.GedcomRecord{Level: 0, Tag: "FAM", Xref: fam.Xref}

	if fam.HusbandXref != "" {
		rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "HUSB", Value: fam.HusbandXref})
	}
	if fam.WifeXref != "" {
		rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "WIFE", Value: fam.WifeXref})
	}

	for _, child := range children {
		rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "CHIL", Value: child.ChildXref})
	}

	for _, evt := range events {
		evtRec := buildEventRecord(ed, evt, 1, eventNotes)
		rec.AddChild(evtRec)
	}

	for _, nl := range noteLinks {
		if nl.NoteIndex >= 0 && nl.NoteIndex < len(ed.Notes) {
			note := ed.Notes[nl.NoteIndex]
			if note.IsTopLevel && note.Xref != "" {
				rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "NOTE", Value: note.Xref})
			} else if note.Content != "" {
				rec.AddChild(buildInlineNote(1, note.Content))
			}
		}
	}

	for _, sl := range sourceLinks {
		if sl.SourceIndex >= 0 && sl.SourceIndex < len(ed.Sources) {
			src := ed.Sources[sl.SourceIndex]
			sourRec := gedcom.GedcomRecord{Level: 1, Tag: "SOUR", Value: src.Xref}
			if sl.Page != "" {
				sourRec.AddChild(gedcom.GedcomRecord{Level: 2, Tag: "PAGE", Value: sl.Page})
			}
			rec.AddChild(sourRec)
		}
	}

	return rec
}

func buildSourceRecord(src enricher.EnrichedSource) gedcom.GedcomRecord {
	rec := gedcom.GedcomRecord{Level: 0, Tag: "SOUR", Xref: src.Xref}
	if src.Title != "" {
		rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "TITL", Value: src.Title})
	}
	if src.Author != "" {
		rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "AUTH", Value: src.Author})
	}
	if src.Publication != "" {
		rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "PUBL", Value: src.Publication})
	}
	if src.Abbreviation != "" {
		rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "ABBR", Value: src.Abbreviation})
	}
	if src.Text != "" {
		rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "TEXT", Value: src.Text})
	}
	if src.RepositoryXref != "" {
		repoRef := gedcom.GedcomRecord{Level: 1, Tag: "REPO", Value: src.RepositoryXref}
		if src.CallNumber != "" {
			repoRef.AddChild(gedcom.GedcomRecord{Level: 2, Tag: "CALN", Value: src.CallNumber})
		}
		rec.AddChild(repoRef)
	}
	return rec
}

func buildNoteRecord(note enricher.EnrichedNote) gedcom.GedcomRecord {
	lines := strings.Split(note.Content, "\n")
	rec := gedcom.GedcomRecord{Level: 0, Tag: "NOTE", Xref: note.Xref, Value: lines[0]}
	for _, line := range lines[1:] {
		rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "CONT", Value: line})
	}
	return rec
}

func buildRepositoryRecord(repo enricher.EnrichedRepository) gedcom.GedcomRecord {
	rec := gedcom.GedcomRecord{Level: 0, Tag: "REPO", Xref: repo.Xref}
	if repo.Name != "" {
		rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "NAME", Value: repo.Name})
	}
	if repo.Address != "" || repo.City != "" || repo.State != "" || repo.Country != "" {
		addr := gedcom.GedcomRecord{Level: 1, Tag: "ADDR", Value: repo.Address}
		if repo.City != "" {
			addr.AddChild(gedcom.GedcomRecord{Level: 2, Tag: "CITY", Value: repo.City})
		}
		if repo.State != "" {
			addr.AddChild(gedcom.GedcomRecord{Level: 2, Tag: "STAE", Value: repo.State})
		}
		if repo.Country != "" {
			addr.AddChild(gedcom.GedcomRecord{Level: 2, Tag: "CTRY", Value: repo.Country})
		}
		rec.AddChild(addr)
	}
	if repo.Phone != "" {
		rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "PHON", Value: repo.Phone})
	}
	if repo.Email != "" {
		rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "EMAIL", Value: repo.Email})
	}
	if repo.Website != "" {
		rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "WWW", Value: repo.Website})
	}
	return rec
}

func buildMediaRecord(media enricher.EnrichedMedia) gedcom.GedcomRecord {
	rec := gedcom.GedcomRecord{Level: 0, Tag: "OBJE", Xref: media.Xref}
	if media.File != "" {
		fileRec := gedcom.GedcomRecord{Level: 1, Tag: "FILE", Value: media.File}
		if media.Form != "" {
			fileRec.AddChild(gedcom.GedcomRecord{Level: 2, Tag: "FORM", Value: media.Form})
		}
		rec.AddChild(fileRec)
	}
	if media.Title != "" {
		rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "TITL", Value: media.Title})
	}
	return rec
}

func buildInlineNote(level int, content string) gedcom.GedcomRecord {
	lines := strings.Split(content, "\n")
	rec := gedcom.GedcomRecord{Level: level, Tag: "NOTE", Value: lines[0]}
	for _, line := range lines[1:] {
		rec.AddChild(gedcom.GedcomRecord{Level: level + 1, Tag: "CONT", Value: line})
	}
	return rec
}

// ---------------------------------------------------------------------------
// Index-building helpers
// ---------------------------------------------------------------------------

func groupEventsByOwner(ed *enricher.EnrichedDocument, ownerType string) map[string][]enricher.Event {
	m := make(map[string][]enricher.Event)
	for _, evt := range ed.Events {
		if evt.OwnerType == ownerType {
			m[evt.OwnerXref] = append(m[evt.OwnerXref], evt)
		}
	}
	return m
}

func groupIndividualNotes(ed *enricher.EnrichedDocument) map[string][]enricher.IndividualNoteLink {
	m := make(map[string][]enricher.IndividualNoteLink)
	for _, nl := range ed.IndividualNotes {
		m[nl.IndividualXref] = append(m[nl.IndividualXref], nl)
	}
	return m
}

func groupFamilyNotes(ed *enricher.EnrichedDocument) map[string][]enricher.FamilyNoteLink {
	m := make(map[string][]enricher.FamilyNoteLink)
	for _, nl := range ed.FamilyNotes {
		m[nl.FamilyXref] = append(m[nl.FamilyXref], nl)
	}
	return m
}

func groupEventNotes(ed *enricher.EnrichedDocument) map[int][]enricher.EventNoteLink {
	m := make(map[int][]enricher.EventNoteLink)
	for _, enl := range ed.EventNotes {
		m[enl.EventIndex] = append(m[enl.EventIndex], enl)
	}
	return m
}

func groupIndividualSources(ed *enricher.EnrichedDocument) map[string][]enricher.IndividualSourceLink {
	m := make(map[string][]enricher.IndividualSourceLink)
	for _, sl := range ed.IndividualSources {
		m[sl.IndividualXref] = append(m[sl.IndividualXref], sl)
	}
	return m
}

func groupFamilySources(ed *enricher.EnrichedDocument) map[string][]enricher.FamilySourceLink {
	m := make(map[string][]enricher.FamilySourceLink)
	for _, sl := range ed.FamilySources {
		m[sl.FamilyXref] = append(m[sl.FamilyXref], sl)
	}
	return m
}

func groupFamilyChildren(ed *enricher.EnrichedDocument) map[string][]enricher.FamilyChildEdge {
	m := make(map[string][]enricher.FamilyChildEdge)
	for _, fc := range ed.FamilyChildren {
		m[fc.FamilyXref] = append(m[fc.FamilyXref], fc)
	}
	return m
}

func groupParentChildByChild(ed *enricher.EnrichedDocument) map[string][]enricher.ParentChildEdge {
	m := make(map[string][]enricher.ParentChildEdge)
	for _, pc := range ed.ParentChild {
		m[pc.ChildXref] = append(m[pc.ChildXref], pc)
	}
	return m
}

func groupSpousesByIndividual(ed *enricher.EnrichedDocument) map[string][]enricher.SpouseEdge {
	m := make(map[string][]enricher.SpouseEdge)
	for _, se := range ed.Spouses {
		m[se.IndividualXref] = append(m[se.IndividualXref], se)
	}
	return m
}

// EnrichedToGEDCOM is a convenience function that converts an EnrichedDocument
// directly to GEDCOM text format.
func EnrichedToGEDCOM(ed *enricher.EnrichedDocument) string {
	doc := FromEnriched(ed)
	return ToGEDCOM(doc)
}

// EnrichedToGEDCOMWithOriginal converts an EnrichedDocument to GEDCOM text,
// preferring the original GedcomDocument if available (lossless round-trip).
// Falls back to reconstructing from enriched data if Document is nil.
func EnrichedToGEDCOMWithOriginal(ed *enricher.EnrichedDocument) string {
	if ed.Document != nil {
		return ToGEDCOM(ed.Document)
	}
	return EnrichedToGEDCOM(ed)
}

// FromEnrichedToJSON converts an EnrichedDocument to denormalized JSON.
func FromEnrichedToJSON(ed *enricher.EnrichedDocument) *DenormalizedJSON {
	doc := FromEnriched(ed)
	return ToJSON(doc)
}

// FromEnrichedToCSV converts an EnrichedDocument to CSV string.
func FromEnrichedToCSV(ed *enricher.EnrichedDocument) (string, error) {
	doc := FromEnriched(ed)
	return ToCSVString(doc)
}

