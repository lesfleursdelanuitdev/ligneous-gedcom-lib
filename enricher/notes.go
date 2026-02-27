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

	// Phase 2: Notes on individuals (skip event-tag children; those are
	// handled by Phase 4).
	for _, indi := range e.doc.Individuals {
		if indi.Xref == "" {
			continue
		}
		e.extractRecordNotes(ed, indi, func(noteIdx int) {
			ed.IndividualNotes = append(ed.IndividualNotes, IndividualNoteLink{
				IndividualXref: indi.Xref,
				NoteIndex:      noteIdx,
			})
		}, individualEventTags)
	}

	// Phase 3: Notes on families (skip event-tag children).
	for _, fam := range e.doc.Families {
		if fam.Xref == "" {
			continue
		}
		e.extractRecordNotes(ed, fam, func(noteIdx int) {
			ed.FamilyNotes = append(ed.FamilyNotes, FamilyNoteLink{
				FamilyXref: fam.Xref,
				NoteIndex:  noteIdx,
			})
		}, familyEventTags)
	}

	// Phase 4: Notes on events (recurse fully).
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
		}, nil)
	}

	// Phase 5: Notes on sources (recurse fully).
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
		}, nil)
	}
}

// extractRecordNotes finds all NOTE tags anywhere in a record's sub-tree and
// calls linkFn with the note index for each unique note found. skipTags
// specifies child tags to NOT recurse into (e.g. event tags that are handled
// by a separate phase). Pass nil to recurse into everything.
func (e *enricherState) extractRecordNotes(ed *EnrichedDocument, rec gedcom.GedcomRecord, linkFn func(noteIdx int), skipTags map[string]bool) {
	seen := make(map[int]bool)
	e.extractRecordNotesRecursive(ed, rec, linkFn, seen, skipTags)
}

func (e *enricherState) extractRecordNotesRecursive(ed *EnrichedDocument, rec gedcom.GedcomRecord, linkFn func(noteIdx int), seen map[int]bool, skipTags map[string]bool) {
	for _, noteChild := range rec.ChildrenByTag("NOTE") {
		noteIdx := e.resolveOrCreateNote(ed, noteChild)
		if noteIdx >= 0 && !seen[noteIdx] {
			seen[noteIdx] = true
			linkFn(noteIdx)
		}
	}
	for _, child := range rec.Children {
		if child.Tag != "NOTE" && !skipTags[child.Tag] {
			e.extractRecordNotesRecursive(ed, child, linkFn, seen, skipTags)
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
