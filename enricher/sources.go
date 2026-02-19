package enricher

import (
	"strconv"
	"strings"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/gedcom"
)

// extractSources processes top-level sources and source citations from
// individuals, families, and events.
func (e *enricherState) extractSources(ed *EnrichedDocument) {
	// Phase 1: Top-level sources
	for _, srcRec := range e.doc.Sources {
		if srcRec.Xref == "" {
			continue
		}

		repoXref := ""
		callNumber := ""
		if repoRef := srcRec.FirstChildByTag("REPO"); repoRef != nil {
			repoXref = repoRef.Value
			callNumber = repoRef.ChildValue("CALN")
		}

		idx := len(ed.Sources)
		ed.Sources = append(ed.Sources, EnrichedSource{
			Xref:           srcRec.Xref,
			Title:          srcRec.ChildValue("TITL"),
			Author:         srcRec.ChildValue("AUTH"),
			Abbreviation:   srcRec.ChildValue("ABBR"),
			Publication:    srcRec.ChildValue("PUBL"),
			Text:           srcRec.ChildValue("TEXT"),
			RepositoryXref: repoXref,
			CallNumber:     callNumber,
		})
		e.sourceXrefIndex[srcRec.Xref] = idx
	}

	// Phase 2: Source citations on individuals
	for _, indi := range e.doc.Individuals {
		if indi.Xref == "" {
			continue
		}
		e.extractRecordSources(ed, indi, func(srcIdx int, page string, quality int, citation string) {
			ed.IndividualSources = append(ed.IndividualSources, IndividualSourceLink{
				IndividualXref: indi.Xref,
				SourceIndex:    srcIdx,
				Page:           page,
				Quality:        quality,
				CitationText:   citation,
			})
		})
	}

	// Phase 3: Source citations on families
	for _, fam := range e.doc.Families {
		if fam.Xref == "" {
			continue
		}
		e.extractRecordSources(ed, fam, func(srcIdx int, page string, quality int, citation string) {
			ed.FamilySources = append(ed.FamilySources, FamilySourceLink{
				FamilyXref:   fam.Xref,
				SourceIndex:  srcIdx,
				Page:         page,
				Quality:      quality,
				CitationText: citation,
			})
		})
	}

	// Phase 4: Source citations on events
	for evtIdx, evt := range ed.Events {
		rec := e.findEventRecord(evt)
		if rec == nil {
			continue
		}
		e.extractRecordSources(ed, *rec, func(srcIdx int, page string, quality int, citation string) {
			ed.EventSources = append(ed.EventSources, EventSourceLink{
				EventIndex:   evtIdx,
				SourceIndex:  srcIdx,
				Page:         page,
				Quality:      quality,
				CitationText: citation,
			})
		})
	}
}

type sourceLinkFn func(srcIdx int, page string, quality int, citation string)

// extractRecordSources finds SOUR sub-tags on a record and calls linkFn for each.
func (e *enricherState) extractRecordSources(ed *EnrichedDocument, rec gedcom.GedcomRecord, linkFn sourceLinkFn) {
	for _, sourChild := range rec.ChildrenByTag("SOUR") {
		xref := strings.TrimSpace(sourChild.Value)
		if xref == "" || !strings.HasPrefix(xref, "@") {
			continue
		}

		srcIdx, ok := e.sourceXrefIndex[xref]
		if !ok {
			continue
		}

		page := sourChild.ChildValue("PAGE")
		qualityStr := sourChild.ChildValue("QUAY")
		quality := 0
		if q, err := strconv.Atoi(qualityStr); err == nil {
			quality = q
		}

		citation := ""
		if dataRec := sourChild.FirstChildByTag("DATA"); dataRec != nil {
			citation = dataRec.ChildValue("TEXT")
		}

		linkFn(srcIdx, page, quality, citation)
	}
}
