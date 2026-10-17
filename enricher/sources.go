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
			repoXref = repoRef.TrimmedValue()
			callNumber = repoRef.ChildValue("CALN")
		}

		idx := len(ed.Sources)
		ed.Sources = append(ed.Sources, EnrichedSource{
			Xref:           srcRec.Xref,
			Title:          childText(&srcRec, "TITL"),
			Author:         childText(&srcRec, "AUTH"),
			Abbreviation:   childText(&srcRec, "ABBR"),
			Publication:    childText(&srcRec, "PUBL"),
			Text:           childText(&srcRec, "TEXT"),
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

		page := childText(&sourChild, "PAGE")
		qualityStr := childTrimmed(&sourChild, "QUAY")
		quality := 0
		if q, err := strconv.Atoi(qualityStr); err == nil {
			quality = q
		}

		citation := ""
		if dataRec := sourChild.FirstChildByTag("DATA"); dataRec != nil {
			citation = childText(dataRec, "TEXT")
		}

		linkFn(srcIdx, page, quality, citation)
	}
}

// childText returns the value of rec's first child with tag, with that
// child's CONC lines joined verbatim and its CONT lines joined with "\n", in
// file order. It is for the text tags of sources and citations (TITL, AUTH,
// ABBR, PUBL, TEXT, PAGE), so a value split across lines reads whole.
func childText(rec *gedcom.GedcomRecord, tag string) string {
	child := rec.FirstChildByTag(tag)
	if child == nil {
		return ""
	}
	v := child.Value
	for _, c := range child.Children {
		switch c.Tag {
		case "CONC":
			v += c.Value
		case "CONT":
			v += "\n" + c.Value
		}
	}
	return v
}
