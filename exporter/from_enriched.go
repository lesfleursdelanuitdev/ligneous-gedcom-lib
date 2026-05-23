package exporter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/enricher"
	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/gedcom"
	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/pedigreedoc"
)

// ensureXrefPointer returns a GEDCOM cross-reference in "@...@ " form.
// Database rows sometimes store "N1" instead of "@N1@"; subordinate NOTE/SOUR
// lines must use the pointer form per the GEDCOM spec.
func ensureXrefPointer(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "@") && strings.HasSuffix(s, "@") && len(s) >= 2 {
		return s
	}
	inner := strings.Trim(s, "@")
	if inner == "" {
		return ""
	}
	return "@" + inner + "@"
}

func parentEdgeFromPC(pc enricher.ParentChildEdge) pedigreedoc.ParentEdge {
	return pedigreedoc.ParentEdge{
		ParentXref:       pc.ParentXref,
		Pedigree:         pc.Pedigree,
		RelationshipType: pc.RelationshipType,
	}
}

// FromEnriched reconstructs a GedcomDocument from an EnrichedDocument.
// The resulting document can be passed to ToGEDCOM for GEDCOM text output.
func FromEnriched(ed *enricher.EnrichedDocument) *gedcom.GedcomDocument {
	doc := &gedcom.GedcomDocument{
		Header:  buildHeader(),
		Trailer: gedcom.GedcomRecord{Level: 0, Tag: "TRLR"},
	}

	indiEvents := groupEventsByOwner(ed, "INDI")
	famEvents := groupEventsByOwner(ed, "FAM")
	indiAttrs := groupAttributesByOwner(ed, "INDI")
	famAttrs := groupAttributesByOwner(ed, "FAM")
	indiResidences := groupResidencesByOwner(ed, "INDI")
	famResidences := groupResidencesByOwner(ed, "FAM")
	indiNotes := groupIndividualNotes(ed)
	famNotes := groupFamilyNotes(ed)
	eventNotes := groupEventNotes(ed)
	indiSources := groupIndividualSources(ed)
	famSources := groupFamilySources(ed)
	famChildren := groupFamilyChildren(ed)
	parentChild := groupParentChildByChild(ed)
	spousesByIndi := groupSpousesByIndividual(ed)
	indiMedia := groupIndividualMedia(ed)
	famMedia := groupFamilyMedia(ed)
	famSurnames := groupFamilySurnames(ed)
	sourceMediaByIdx := groupSourceMediaBySourceIndex(ed)
	eventSources := groupEventSourcesByEventIndex(ed)
	eventMedia := groupEventMediaByEventIndex(ed)
	ownerAssociates := groupAssociatesByOwner(ed)
	eventAssociates := groupAssociatesByOwnerEvent(ed)

	for _, indi := range ed.Individuals {
		rec := buildIndividualRecord(ed, indi, indiEvents[indi.Xref],
			indiAttrs[indi.Xref], indiResidences[indi.Xref],
			indiNotes[indi.Xref], indiSources[indi.Xref],
			parentChild[indi.Xref], spousesByIndi[indi.Xref], eventNotes,
			indiMedia[indi.Xref], eventSources, eventMedia,
			ownerAssociates["INDI|"+indi.Xref], eventAssociates)
		doc.Individuals = append(doc.Individuals, rec)
	}

	for _, fam := range ed.Families {
		rec := buildFamilyRecord(ed, fam, famEvents[fam.Xref],
			famAttrs[fam.Xref], famResidences[fam.Xref],
			famNotes[fam.Xref], famSources[fam.Xref],
			famChildren[fam.Xref], eventNotes,
			famMedia[fam.Xref], famSurnames[fam.Xref], eventSources, eventMedia,
			ownerAssociates["FAM|"+fam.Xref], eventAssociates)
		doc.Families = append(doc.Families, rec)
	}

	for i := range ed.Sources {
		doc.Sources = append(doc.Sources, buildSourceRecord(ed, ed.Sources[i], sourceMediaByIdx[i]))
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
	attrs []enricher.Attribute,
	residences []enricher.Residence,
	noteLinks []enricher.IndividualNoteLink,
	sourceLinks []enricher.IndividualSourceLink,
	parentLinks []enricher.ParentChildEdge,
	spouseEdges []enricher.SpouseEdge,
	eventNotes map[int][]enricher.EventNoteLink,
	mediaLinks []enricher.IndividualMediaLink,
	eventSources map[int][]enricher.EventSourceLink,
	eventMedia map[int][]int,
	associates []enricher.AssociateEdge,
	eventAssociates map[string][]enricher.AssociateEdge,
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
		evtRec := buildEventRecord(ed, evt, 1, eventNotes, eventSources[evt.Index], eventMedia[evt.Index], eventAssociates)
		rec.AddChild(evtRec)
	}
	for _, attr := range attrs {
		rec.AddChild(buildAttributeRecord(ed, attr, 1))
	}
	for _, res := range residences {
		rec.AddChild(buildResidenceRecord(ed, res, 1))
	}

	byFam := make(map[string][]enricher.ParentChildEdge)
	for _, pc := range parentLinks {
		byFam[pc.FamilyXref] = append(byFam[pc.FamilyXref], pc)
	}
	famKeys := make([]string, 0, len(byFam))
	for k := range byFam {
		famKeys = append(famKeys, k)
	}
	sort.Strings(famKeys)
	for _, famXref := range famKeys {
		edges := byFam[famXref]
		child := gedcom.GedcomRecord{Level: 1, Tag: "FAMC", Value: ensureXrefPointer(famXref)}
		if len(edges) == 2 &&
			pedigreedoc.IsMixedBiologicalAdoptivePair(parentEdgeFromPC(edges[0]), parentEdgeFromPC(edges[1])) {
			child.AddChild(gedcom.GedcomRecord{Level: 2, Tag: "PEDI", Value: "birth"})
			bio, adopt := pedigreedoc.BiologicalAndAdoptiveXrefs(parentEdgeFromPC(edges[0]), parentEdgeFromPC(edges[1]))
			child.AddChild(buildInlineNote(2, pedigreedoc.FormatMixedParentageNote(bio, adopt)))
		} else {
			pedi := ""
			for _, e := range edges {
				if strings.TrimSpace(e.Pedigree) != "" {
					pedi = e.Pedigree
					break
				}
			}
			if pedi != "" {
				child.AddChild(gedcom.GedcomRecord{Level: 2, Tag: "PEDI", Value: pedi})
			}
		}
		rec.AddChild(child)
	}

	famsXrefs := make(map[string]bool)
	for _, se := range spouseEdges {
		if se.FamilyXref == "" {
			continue
		}
		if !famsXrefs[se.FamilyXref] {
			famsXrefs[se.FamilyXref] = true
			rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "FAMS", Value: ensureXrefPointer(se.FamilyXref)})
		}
	}

	// Single-parent families: there are no `gedcom_spouses_v2` rows (and thus no SpouseEdge
	// entries) when only one HUSB/WIFE slot is filled, but the parent still needs a FAMS
	// pointer to that FAM (matches common GEDCOM practice and our DB model).
	for _, fam := range ed.Families {
		if fam.Xref == "" {
			continue
		}
		soloHusband := fam.HusbandXref == indi.Xref && fam.WifeXref == ""
		soloWife := fam.WifeXref == indi.Xref && fam.HusbandXref == ""
		if (soloHusband || soloWife) && !famsXrefs[fam.Xref] {
			famsXrefs[fam.Xref] = true
			rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "FAMS", Value: ensureXrefPointer(fam.Xref)})
		}
	}

	for _, nl := range noteLinks {
		if nl.NoteIndex >= 0 && nl.NoteIndex < len(ed.Notes) {
			note := ed.Notes[nl.NoteIndex]
			if note.IsTopLevel && note.Xref != "" {
				rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "NOTE", Value: ensureXrefPointer(note.Xref)})
			} else if note.Content != "" {
				rec.AddChild(buildInlineNote(1, note.Content))
			}
		}
	}

	for _, sl := range sourceLinks {
		if sl.SourceIndex >= 0 && sl.SourceIndex < len(ed.Sources) {
			src := ed.Sources[sl.SourceIndex]
			sourRec := gedcom.GedcomRecord{Level: 1, Tag: "SOUR", Value: ensureXrefPointer(src.Xref)}
			if sl.Page != "" {
				sourRec.AddChild(gedcom.GedcomRecord{Level: 2, Tag: "PAGE", Value: sl.Page})
			}
			rec.AddChild(sourRec)
		}
	}

	appendOBJELinksFromIndividualMediaLinks(&rec, ed, mediaLinks)
	appendAssociationRecords(&rec, associates, 1)

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

		var gnLinks []enricher.NameFormGivenNameLink
		for _, gnLink := range ed.NameFormGivenNames {
			if gnLink.NameFormIndex == nfIdx && gnLink.GivenNameIndex >= 0 && gnLink.GivenNameIndex < len(ed.GivenNames) {
				gnLinks = append(gnLinks, gnLink)
			}
		}
		sort.Slice(gnLinks, func(i, j int) bool {
			if gnLinks[i].Position != gnLinks[j].Position {
				return gnLinks[i].Position < gnLinks[j].Position
			}
			return gnLinks[i].GivenNameIndex < gnLinks[j].GivenNameIndex
		})
		for _, gnLink := range gnLinks {
			givnParts = append(givnParts, ed.GivenNames[gnLink.GivenNameIndex].Value)
		}

		var snLinks []enricher.NameFormSurnameLink
		for _, snLink := range ed.NameFormSurnames {
			if snLink.NameFormIndex == nfIdx && snLink.SurnameIndex >= 0 && snLink.SurnameIndex < len(ed.Surnames) {
				snLinks = append(snLinks, snLink)
			}
		}
		sort.Slice(snLinks, func(i, j int) bool {
			if snLinks[i].Position != snLinks[j].Position {
				return snLinks[i].Position < snLinks[j].Position
			}
			return snLinks[i].SurnameIndex < snLinks[j].SurnameIndex
		})
		for _, snLink := range snLinks {
			surnParts = append(surnParts, ed.Surnames[snLink.SurnameIndex].Value)
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
	eventSources []enricher.EventSourceLink,
	eventMediaIndices []int,
	eventAssociates map[string][]enricher.AssociateEdge,
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
		if ds := enricher.FormatGEDCOMDate(ed.Dates[evt.DateIndex]); ds != "" {
			evtRec.AddChild(gedcom.GedcomRecord{Level: level + 1, Tag: "DATE", Value: ds})
		}
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
				evtRec.AddChild(gedcom.GedcomRecord{Level: level + 1, Tag: "NOTE", Value: ensureXrefPointer(note.Xref)})
			} else if note.Content != "" {
				evtRec.AddChild(buildInlineNote(level+1, note.Content))
			}
		}
	}

	for _, sl := range eventSources {
		if sl.SourceIndex < 0 || sl.SourceIndex >= len(ed.Sources) {
			continue
		}
		src := ed.Sources[sl.SourceIndex]
		sourRec := gedcom.GedcomRecord{Level: level + 1, Tag: "SOUR", Value: ensureXrefPointer(src.Xref)}
		if sl.Page != "" {
			sourRec.AddChild(gedcom.GedcomRecord{Level: level + 2, Tag: "PAGE", Value: sl.Page})
		}
		evtRec.AddChild(sourRec)
	}

	appendOBJELinksFromMediaIndices(&evtRec, ed, eventMediaIndices, level+1)
	appendAssociationRecords(&evtRec, eventAssociates[eventAssociationKey(evt.OwnerType, evt.OwnerXref, evt.EventType)], level+1)

	return evtRec
}

func buildFamilyRecord(
	ed *enricher.EnrichedDocument,
	fam enricher.EnrichedFamily,
	events []enricher.Event,
	attrs []enricher.Attribute,
	residences []enricher.Residence,
	noteLinks []enricher.FamilyNoteLink,
	sourceLinks []enricher.FamilySourceLink,
	children []enricher.FamilyChildEdge,
	eventNotes map[int][]enricher.EventNoteLink,
	mediaLinks []enricher.FamilyMediaLink,
	surnameLinks []enricher.FamilySurnameLink,
	eventSources map[int][]enricher.EventSourceLink,
	eventMedia map[int][]int,
	associates []enricher.AssociateEdge,
	eventAssociates map[string][]enricher.AssociateEdge,
) gedcom.GedcomRecord {
	rec := gedcom.GedcomRecord{Level: 0, Tag: "FAM", Xref: fam.Xref}

	if fam.HusbandXref != "" {
		rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "HUSB", Value: ensureXrefPointer(fam.HusbandXref)})
	}
	if fam.WifeXref != "" {
		rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "WIFE", Value: ensureXrefPointer(fam.WifeXref)})
	}

	for _, child := range children {
		rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "CHIL", Value: ensureXrefPointer(child.ChildXref)})
	}

	// Emit NCHI from the computed children count.
	if fam.ChildrenCount > 0 {
		rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "NCHI", Value: fmt.Sprintf("%d", fam.ChildrenCount)})
	}

	appendFamilySurnameNote(&rec, ed, surnameLinks)

	for _, evt := range events {
		evtRec := buildEventRecord(ed, evt, 1, eventNotes, eventSources[evt.Index], eventMedia[evt.Index], eventAssociates)
		rec.AddChild(evtRec)
	}
	for _, attr := range attrs {
		rec.AddChild(buildAttributeRecord(ed, attr, 1))
	}
	for _, res := range residences {
		rec.AddChild(buildResidenceRecord(ed, res, 1))
	}

	for _, nl := range noteLinks {
		if nl.NoteIndex >= 0 && nl.NoteIndex < len(ed.Notes) {
			note := ed.Notes[nl.NoteIndex]
			if note.IsTopLevel && note.Xref != "" {
				rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "NOTE", Value: ensureXrefPointer(note.Xref)})
			} else if note.Content != "" {
				rec.AddChild(buildInlineNote(1, note.Content))
			}
		}
	}

	for _, sl := range sourceLinks {
		if sl.SourceIndex >= 0 && sl.SourceIndex < len(ed.Sources) {
			src := ed.Sources[sl.SourceIndex]
			sourRec := gedcom.GedcomRecord{Level: 1, Tag: "SOUR", Value: ensureXrefPointer(src.Xref)}
			if sl.Page != "" {
				sourRec.AddChild(gedcom.GedcomRecord{Level: 2, Tag: "PAGE", Value: sl.Page})
			}
			rec.AddChild(sourRec)
		}
	}

	appendOBJELinksFromFamilyMediaLinks(&rec, ed, mediaLinks)
	appendAssociationRecords(&rec, associates, 1)

	return rec
}

func buildSourceRecord(ed *enricher.EnrichedDocument, src enricher.EnrichedSource, mediaIndices []int) gedcom.GedcomRecord {
	rec := gedcom.GedcomRecord{Level: 0, Tag: "SOUR", Xref: ensureXrefPointer(src.Xref)}
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
	if strings.TrimSpace(src.Text) != "" {
		txt := gedcom.GedcomRecord{Level: 1, Tag: "TEXT"}
		appendWrappedTaggedOntoRecord(&txt, 1, "", "TEXT", src.Text)
		rec.AddChild(txt)
	}
	if src.RepositoryXref != "" {
		repoRef := gedcom.GedcomRecord{Level: 1, Tag: "REPO", Value: ensureXrefPointer(src.RepositoryXref)}
		if src.CallNumber != "" {
			repoRef.AddChild(gedcom.GedcomRecord{Level: 2, Tag: "CALN", Value: src.CallNumber})
		}
		rec.AddChild(repoRef)
	}

	appendOBJELinksFromMediaIndices(&rec, ed, mediaIndices, 1)

	return rec
}

func buildNoteRecord(note enricher.EnrichedNote) gedcom.GedcomRecord {
	rec := gedcom.GedcomRecord{Level: 0, Tag: "NOTE", Xref: ensureXrefPointer(note.Xref)}
	appendWrappedNoteOntoRecord(&rec, 0, note.Xref, note.Content)
	return rec
}

func buildRepositoryRecord(repo enricher.EnrichedRepository) gedcom.GedcomRecord {
	rec := gedcom.GedcomRecord{Level: 0, Tag: "REPO", Xref: ensureXrefPointer(repo.Xref)}
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
	rec := gedcom.GedcomRecord{Level: 0, Tag: "OBJE", Xref: ensureXrefPointer(media.Xref)}
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
	if strings.TrimSpace(media.Description) != "" {
		rec.AddChild(buildInlineNote(1, strings.TrimSpace(media.Description)))
	}
	return rec
}

func buildInlineNote(level int, content string) gedcom.GedcomRecord {
	rec := gedcom.GedcomRecord{Level: level, Tag: "NOTE"}
	appendWrappedNoteOntoRecord(&rec, level, "", content)
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
	for k := range m {
		sort.Slice(m[k], func(i, j int) bool {
			if m[k][i].SortOrder != m[k][j].SortOrder {
				return m[k][i].SortOrder < m[k][j].SortOrder
			}
			return m[k][i].Index < m[k][j].Index
		})
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

func groupIndividualMedia(ed *enricher.EnrichedDocument) map[string][]enricher.IndividualMediaLink {
	m := make(map[string][]enricher.IndividualMediaLink)
	for _, lm := range ed.IndividualMedia {
		m[lm.IndividualXref] = append(m[lm.IndividualXref], lm)
	}
	return m
}

func groupFamilyMedia(ed *enricher.EnrichedDocument) map[string][]enricher.FamilyMediaLink {
	m := make(map[string][]enricher.FamilyMediaLink)
	for _, lm := range ed.FamilyMedia {
		m[lm.FamilyXref] = append(m[lm.FamilyXref], lm)
	}
	return m
}

func groupFamilySurnames(ed *enricher.EnrichedDocument) map[string][]enricher.FamilySurnameLink {
	m := make(map[string][]enricher.FamilySurnameLink)
	for _, fs := range ed.FamilySurnames {
		m[fs.FamilyXref] = append(m[fs.FamilyXref], fs)
	}
	return m
}

func groupSourceMediaBySourceIndex(ed *enricher.EnrichedDocument) map[int][]int {
	m := make(map[int][]int)
	for _, sm := range ed.SourceMedia {
		if sm.SourceIndex < 0 {
			continue
		}
		m[sm.SourceIndex] = append(m[sm.SourceIndex], sm.MediaIndex)
	}
	return m
}

func groupEventSourcesByEventIndex(ed *enricher.EnrichedDocument) map[int][]enricher.EventSourceLink {
	m := make(map[int][]enricher.EventSourceLink)
	for _, es := range ed.EventSources {
		m[es.EventIndex] = append(m[es.EventIndex], es)
	}
	return m
}

func groupEventMediaByEventIndex(ed *enricher.EnrichedDocument) map[int][]int {
	m := make(map[int][]int)
	for _, em := range ed.EventMedia {
		m[em.EventIndex] = append(m[em.EventIndex], em.MediaIndex)
	}
	return m
}

func groupAssociatesByOwner(ed *enricher.EnrichedDocument) map[string][]enricher.AssociateEdge {
	m := make(map[string][]enricher.AssociateEdge)
	for _, a := range ed.Associates {
		if a.OwnerEventType != "" {
			continue
		}
		m[a.OwnerType+"|"+a.OwnerXref] = append(m[a.OwnerType+"|"+a.OwnerXref], a)
	}
	return m
}

func groupAssociatesByOwnerEvent(ed *enricher.EnrichedDocument) map[string][]enricher.AssociateEdge {
	m := make(map[string][]enricher.AssociateEdge)
	for _, a := range ed.Associates {
		if a.OwnerEventType == "" {
			continue
		}
		m[eventAssociationKey(a.OwnerType, a.OwnerXref, a.OwnerEventType)] = append(m[eventAssociationKey(a.OwnerType, a.OwnerXref, a.OwnerEventType)], a)
	}
	return m
}

func eventAssociationKey(ownerType, ownerXref, eventType string) string {
	return ownerType + "|" + ownerXref + "|" + eventType
}

func appendAssociationRecords(rec *gedcom.GedcomRecord, associates []enricher.AssociateEdge, level int) {
	for _, a := range associates {
		target := ensureXrefPointer(a.AssociateXref)
		if target == "" {
			continue
		}
		assoRec := gedcom.GedcomRecord{Level: level, Tag: "ASSO", Value: target}
		if r := strings.TrimSpace(a.Relationship); r != "" {
			assoRec.AddChild(gedcom.GedcomRecord{Level: level + 1, Tag: "RELA", Value: r})
		}
		rec.AddChild(assoRec)
	}
}

func appendOBJELinksFromMediaIndices(rec *gedcom.GedcomRecord, ed *enricher.EnrichedDocument, mediaIndices []int, childLevel int) {
	seen := make(map[string]bool)
	for _, idx := range mediaIndices {
		if idx < 0 || idx >= len(ed.Media) {
			continue
		}
		xref := ensureXrefPointer(ed.Media[idx].Xref)
		if xref == "" {
			continue
		}
		if seen[xref] {
			continue
		}
		seen[xref] = true
		rec.AddChild(gedcom.GedcomRecord{Level: childLevel, Tag: "OBJE", Value: xref})
	}
}

func appendOBJELinksFromIndividualMediaLinks(rec *gedcom.GedcomRecord, ed *enricher.EnrichedDocument, links []enricher.IndividualMediaLink) {
	indices := make([]int, 0, len(links))
	for _, l := range links {
		indices = append(indices, l.MediaIndex)
	}
	appendOBJELinksFromMediaIndices(rec, ed, indices, 1)
}

func appendOBJELinksFromFamilyMediaLinks(rec *gedcom.GedcomRecord, ed *enricher.EnrichedDocument, links []enricher.FamilyMediaLink) {
	indices := make([]int, 0, len(links))
	for _, l := range links {
		indices = append(indices, l.MediaIndex)
	}
	appendOBJELinksFromMediaIndices(rec, ed, indices, 1)
}

func appendFamilySurnameNote(rec *gedcom.GedcomRecord, ed *enricher.EnrichedDocument, links []enricher.FamilySurnameLink) {
	if len(links) == 0 {
		return
	}
	var parts []string
	for _, l := range links {
		if l.SurnameIndex >= 0 && l.SurnameIndex < len(ed.Surnames) {
			parts = append(parts, ed.Surnames[l.SurnameIndex].Value)
		}
	}
	if len(parts) == 0 {
		return
	}
	rec.AddChild(gedcom.GedcomRecord{Level: 1, Tag: "NOTE", Value: "Family surname(s): " + strings.Join(parts, ", ")})
}

func buildAttributeRecord(ed *enricher.EnrichedDocument, attr enricher.Attribute, level int) gedcom.GedcomRecord {
	rec := gedcom.GedcomRecord{Level: level, Tag: attr.AttributeType, Value: attr.Value}
	if attr.CustomType != "" {
		rec.AddChild(gedcom.GedcomRecord{Level: level + 1, Tag: "TYPE", Value: attr.CustomType})
	}
	if attr.DateIndex >= 0 && attr.DateIndex < len(ed.Dates) {
		if ds := enricher.FormatGEDCOMDate(ed.Dates[attr.DateIndex]); ds != "" {
			rec.AddChild(gedcom.GedcomRecord{Level: level + 1, Tag: "DATE", Value: ds})
		}
	}
	if attr.PlaceIndex >= 0 && attr.PlaceIndex < len(ed.Places) {
		if p := strings.TrimSpace(ed.Places[attr.PlaceIndex].Original); p != "" {
			rec.AddChild(gedcom.GedcomRecord{Level: level + 1, Tag: "PLAC", Value: p})
		}
	}
	if attr.Agency != "" {
		rec.AddChild(gedcom.GedcomRecord{Level: level + 1, Tag: "AGNC", Value: attr.Agency})
	}
	return rec
}

func buildResidenceRecord(ed *enricher.EnrichedDocument, res enricher.Residence, level int) gedcom.GedcomRecord {
	rec := gedcom.GedcomRecord{Level: level, Tag: "RESI"}
	if res.DateIndex >= 0 && res.DateIndex < len(ed.Dates) {
		if ds := enricher.FormatGEDCOMDate(ed.Dates[res.DateIndex]); ds != "" {
			rec.AddChild(gedcom.GedcomRecord{Level: level + 1, Tag: "DATE", Value: ds})
		}
	}
	if res.PlaceIndex >= 0 && res.PlaceIndex < len(ed.Places) {
		if p := strings.TrimSpace(ed.Places[res.PlaceIndex].Original); p != "" {
			rec.AddChild(gedcom.GedcomRecord{Level: level + 1, Tag: "PLAC", Value: p})
		}
	}
	if res.Address != "" {
		addrRec := gedcom.GedcomRecord{Level: level + 1, Tag: "ADDR"}
		addrRec.AddChild(gedcom.GedcomRecord{Level: level + 2, Tag: "ADR1", Value: res.Address})
		rec.AddChild(addrRec)
	}
	return rec
}

func groupAttributesByOwner(ed *enricher.EnrichedDocument, ownerType string) map[string][]enricher.Attribute {
	m := make(map[string][]enricher.Attribute)
	for _, a := range ed.Attributes {
		if a.OwnerType == ownerType {
			m[a.OwnerXref] = append(m[a.OwnerXref], a)
		}
	}
	for k := range m {
		sort.Slice(m[k], func(i, j int) bool {
			if m[k][i].SortOrder != m[k][j].SortOrder {
				return m[k][i].SortOrder < m[k][j].SortOrder
			}
			return m[k][i].Index < m[k][j].Index
		})
	}
	return m
}

func groupResidencesByOwner(ed *enricher.EnrichedDocument, ownerType string) map[string][]enricher.Residence {
	m := make(map[string][]enricher.Residence)
	for _, r := range ed.Residences {
		if r.OwnerType == ownerType {
			m[r.OwnerXref] = append(m[r.OwnerXref], r)
		}
	}
	for k := range m {
		sort.Slice(m[k], func(i, j int) bool {
			if m[k][i].SortOrder != m[k][j].SortOrder {
				return m[k][i].SortOrder < m[k][j].SortOrder
			}
			return m[k][i].Index < m[k][j].Index
		})
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
