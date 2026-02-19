package enricher

import (
	"strings"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/gedcom"
)

// extractNotes processes top-level notes and inline notes from individuals,
// families, events, and sources.
func (e *enricherState) extractNotes(ed *EnrichedDocument) {
	// Phase 1: Top-level notes from doc.Notes
	for _, noteRec := range e.doc.Notes {
		content := noteRec.Value
		for _, cont := range noteRec.ChildrenByTag("CONT") {
			content += "\n" + cont.Value
		}
		for _, conc := range noteRec.ChildrenByTag("CONC") {
			content += conc.Value
		}

		idx := len(ed.Notes)
		ed.Notes = append(ed.Notes, EnrichedNote{
			Xref:       noteRec.Xref,
			Content:    content,
			IsTopLevel: true,
		})
		if noteRec.Xref != "" {
			e.noteXrefIndex[noteRec.Xref] = idx
		}
	}

	// Phase 2: Inline notes on individuals
	for _, indi := range e.doc.Individuals {
		if indi.Xref == "" {
			continue
		}
		e.extractRecordNotes(ed, indi, func(noteIdx int) {
			ed.IndividualNotes = append(ed.IndividualNotes, IndividualNoteLink{
				IndividualXref: indi.Xref,
				NoteIndex:      noteIdx,
			})
		})
	}

	// Phase 3: Inline notes on families
	for _, fam := range e.doc.Families {
		if fam.Xref == "" {
			continue
		}
		e.extractRecordNotes(ed, fam, func(noteIdx int) {
			ed.FamilyNotes = append(ed.FamilyNotes, FamilyNoteLink{
				FamilyXref: fam.Xref,
				NoteIndex:  noteIdx,
			})
		})
	}

	// Phase 4: Notes on events (processed after events are extracted)
	for evtIdx, evt := range ed.Events {
		rec := e.findEventRecord(evt)
		if rec == nil {
			continue
		}
		e.extractRecordNotes(ed, *rec, func(noteIdx int) {
			ed.EventNotes = append(ed.EventNotes, EventNoteLink{
				EventIndex: evtIdx,
				NoteIndex:  noteIdx,
			})
		})
	}

	// Phase 5: Notes on sources
	for srcIdx, src := range ed.Sources {
		srcRec := e.doc.FindByXref(src.Xref)
		if srcRec == nil {
			continue
		}
		e.extractRecordNotes(ed, *srcRec, func(noteIdx int) {
			ed.SourceNotes = append(ed.SourceNotes, SourceNoteLink{
				SourceIndex: srcIdx,
				NoteIndex:   noteIdx,
			})
		})
	}
}

// extractRecordNotes finds all NOTE sub-tags on a record and calls linkFn with
// the note index for each.
func (e *enricherState) extractRecordNotes(ed *EnrichedDocument, rec gedcom.GedcomRecord, linkFn func(noteIdx int)) {
	for _, noteChild := range rec.ChildrenByTag("NOTE") {
		noteIdx := e.resolveOrCreateNote(ed, noteChild)
		if noteIdx >= 0 {
			linkFn(noteIdx)
		}
	}
}

// resolveOrCreateNote handles both reference notes (@N1@) and inline notes.
func (e *enricherState) resolveOrCreateNote(ed *EnrichedDocument, noteRec gedcom.GedcomRecord) int {
	val := strings.TrimSpace(noteRec.Value)

	// Reference to a top-level note: value starts with @
	if strings.HasPrefix(val, "@") && strings.HasSuffix(val, "@") {
		if idx, ok := e.noteXrefIndex[val]; ok {
			return idx
		}
		return -1
	}

	// Inline note
	if val == "" {
		return -1
	}

	content := val
	for _, cont := range noteRec.ChildrenByTag("CONT") {
		content += "\n" + cont.Value
	}
	for _, conc := range noteRec.ChildrenByTag("CONC") {
		content += conc.Value
	}

	idx := len(ed.Notes)
	ed.Notes = append(ed.Notes, EnrichedNote{
		Content:    content,
		IsTopLevel: false,
	})
	return idx
}

// findEventRecord locates the raw GedcomRecord for an event by walking the
// owner record's children.
func (e *enricherState) findEventRecord(evt Event) *gedcom.GedcomRecord {
	owner := e.doc.FindByXref(evt.OwnerXref)
	if owner == nil {
		return nil
	}
	sortOrder := 0
	for i := range owner.Children {
		child := &owner.Children[i]
		isEvent := false
		if evt.OwnerType == "INDI" {
			isEvent = individualEventTags[child.Tag]
		} else {
			isEvent = familyEventTags[child.Tag]
		}
		if isEvent {
			if sortOrder == evt.SortOrder {
				return child
			}
			sortOrder++
		}
	}
	return nil
}
