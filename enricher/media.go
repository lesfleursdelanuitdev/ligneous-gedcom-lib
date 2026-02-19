package enricher

import (
	"strings"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/gedcom"
)

// extractMedia processes top-level OBJE records and media references from
// individuals, families, and sources.
func (e *enricherState) extractMedia(ed *EnrichedDocument) {
	// Phase 1: Top-level media objects
	for _, mediaRec := range e.doc.Media {
		file := ""
		form := ""
		if fileRec := mediaRec.FirstChildByTag("FILE"); fileRec != nil {
			file = fileRec.Value
			form = fileRec.ChildValue("FORM")
		}

		idx := len(ed.Media)
		ed.Media = append(ed.Media, EnrichedMedia{
			Xref:  mediaRec.Xref,
			File:  file,
			Form:  form,
			Title: mediaRec.ChildValue("TITL"),
		})
		if mediaRec.Xref != "" {
			e.mediaXrefIndex[mediaRec.Xref] = idx
		}
	}

	// Phase 2: Media references on individuals
	for _, indi := range e.doc.Individuals {
		if indi.Xref == "" {
			continue
		}
		e.extractRecordMedia(ed, indi, func(mediaIdx int) {
			ed.IndividualMedia = append(ed.IndividualMedia, IndividualMediaLink{
				IndividualXref: indi.Xref,
				MediaIndex:     mediaIdx,
			})
		})
	}

	// Phase 3: Media references on families
	for _, fam := range e.doc.Families {
		if fam.Xref == "" {
			continue
		}
		e.extractRecordMedia(ed, fam, func(mediaIdx int) {
			ed.FamilyMedia = append(ed.FamilyMedia, FamilyMediaLink{
				FamilyXref: fam.Xref,
				MediaIndex: mediaIdx,
			})
		})
	}

	// Phase 4: Media references on sources
	for srcIdx, src := range ed.Sources {
		srcRec := e.doc.FindByXref(src.Xref)
		if srcRec == nil {
			continue
		}
		e.extractRecordMedia(ed, *srcRec, func(mediaIdx int) {
			ed.SourceMedia = append(ed.SourceMedia, SourceMediaLink{
				SourceIndex: srcIdx,
				MediaIndex:  mediaIdx,
			})
		})
	}
}

// extractRecordMedia finds OBJE sub-tags on a record.
func (e *enricherState) extractRecordMedia(ed *EnrichedDocument, rec gedcom.GedcomRecord, linkFn func(mediaIdx int)) {
	for _, objeChild := range rec.ChildrenByTag("OBJE") {
		mediaIdx := e.resolveOrCreateMedia(ed, objeChild)
		if mediaIdx >= 0 {
			linkFn(mediaIdx)
		}
	}
}

// resolveOrCreateMedia handles both reference (@M1@) and inline OBJE records.
func (e *enricherState) resolveOrCreateMedia(ed *EnrichedDocument, objeRec gedcom.GedcomRecord) int {
	val := strings.TrimSpace(objeRec.Value)

	// Reference to top-level media
	if strings.HasPrefix(val, "@") && strings.HasSuffix(val, "@") {
		if idx, ok := e.mediaXrefIndex[val]; ok {
			return idx
		}
		return -1
	}

	// Inline media object
	file := ""
	form := ""
	if fileRec := objeRec.FirstChildByTag("FILE"); fileRec != nil {
		file = fileRec.Value
		form = fileRec.ChildValue("FORM")
	}
	if file == "" {
		return -1
	}

	idx := len(ed.Media)
	ed.Media = append(ed.Media, EnrichedMedia{
		File:  file,
		Form:  form,
		Title: objeRec.ChildValue("TITL"),
	})
	return idx
}
