package reconciliation

import (
	"strings"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/enricher"
)

func normXref(x string) string {
	x = strings.TrimSpace(x)
	x = strings.TrimPrefix(x, "@")
	x = strings.TrimSuffix(x, "@")
	return strings.TrimSpace(x)
}

func individualByID(ed *enricher.EnrichedDocument) map[string]*enricher.EnrichedIndividual {
	out := make(map[string]*enricher.EnrichedIndividual)
	for i := range ed.Individuals {
		ind := &ed.Individuals[i]
		if ind.ID != "" {
			out[ind.ID] = ind
		}
	}
	return out
}

func individualByXref(ed *enricher.EnrichedDocument) map[string]*enricher.EnrichedIndividual {
	out := make(map[string]*enricher.EnrichedIndividual)
	for i := range ed.Individuals {
		ind := &ed.Individuals[i]
		k := normXref(ind.Xref)
		if k != "" {
			// last wins if duplicate xrefs (dirty data)
			out[k] = ind
		}
	}
	return out
}

func familyByID(ed *enricher.EnrichedDocument) map[string]*enricher.EnrichedFamily {
	out := make(map[string]*enricher.EnrichedFamily)
	for i := range ed.Families {
		f := &ed.Families[i]
		if f.ID != "" {
			out[f.ID] = f
		}
	}
	return out
}

func familyByXref(ed *enricher.EnrichedDocument) map[string]*enricher.EnrichedFamily {
	out := make(map[string]*enricher.EnrichedFamily)
	for i := range ed.Families {
		f := &ed.Families[i]
		k := normXref(f.Xref)
		if k != "" {
			out[k] = f
		}
	}
	return out
}

func yearAt(ed *enricher.EnrichedDocument, dateIndex int) (int, bool) {
	if dateIndex < 0 || dateIndex >= len(ed.Dates) {
		return 0, false
	}
	y := ed.Dates[dateIndex].Year
	return y, y != 0 || ed.Dates[dateIndex].Original != ""
}

func placeFingerprint(ed *enricher.EnrichedDocument, placeIndex int) string {
	if placeIndex < 0 || placeIndex >= len(ed.Places) {
		return ""
	}
	p := ed.Places[placeIndex]
	return p.Hash
}

// individualEventFingerprints returns order-insensitive multiset keys for INDI-linked events.
func individualEventFingerprints(ed *enricher.EnrichedDocument, indiXref string) []string {
	x := normXref(indiXref)
	var keys []string
	for _, link := range ed.IndividualEvents {
		if normXref(link.IndividualXref) != x {
			continue
		}
		if link.EventIndex < 0 || link.EventIndex >= len(ed.Events) {
			continue
		}
		ev := ed.Events[link.EventIndex]
		dOrig := ""
		if ev.DateIndex >= 0 && ev.DateIndex < len(ed.Dates) {
			dOrig = ed.Dates[ev.DateIndex].Original
		}
		ph := placeFingerprint(ed, ev.PlaceIndex)
		key := ev.EventType + "|" + ev.CustomType + "|" + dOrig + "|" + ph + "|" + ev.Value
		keys = append(keys, key)
	}
	return keys
}

func childXrefsForFamily(ed *enricher.EnrichedDocument, famXref string) []string {
	fx := normXref(famXref)
	var xs []string
	for _, fc := range ed.FamilyChildren {
		if normXref(fc.FamilyXref) == fx {
			xs = append(xs, normXref(fc.ChildXref))
		}
	}
	return xs
}

func multisetDiff(left, right []string) (onlyLeft, onlyRight []string) {
	lc := countMap(left)
	rc := countMap(right)
	for k, n := range lc {
		m := rc[k]
		for i := 0; i < n-m; i++ {
			onlyLeft = append(onlyLeft, k)
		}
	}
	for k, n := range rc {
		m := lc[k]
		for i := 0; i < n-m; i++ {
			onlyRight = append(onlyRight, k)
		}
	}
	return onlyLeft, onlyRight
}

func countMap(keys []string) map[string]int {
	m := make(map[string]int)
	for _, k := range keys {
		m[k]++
	}
	return m
}

func individualNoteFingerprints(ed *enricher.EnrichedDocument, indiXref string) []string {
	x := normXref(indiXref)
	var keys []string
	for _, link := range ed.IndividualNotes {
		if normXref(link.IndividualXref) != x {
			continue
		}
		if link.NoteIndex < 0 || link.NoteIndex >= len(ed.Notes) {
			continue
		}
		n := ed.Notes[link.NoteIndex]
		keys = append(keys, "note|"+n.Content)
	}
	return keys
}

func individualMediaFingerprints(ed *enricher.EnrichedDocument, indiXref string) []string {
	x := normXref(indiXref)
	var keys []string
	for _, link := range ed.IndividualMedia {
		if normXref(link.IndividualXref) != x {
			continue
		}
		if link.MediaIndex < 0 || link.MediaIndex >= len(ed.Media) {
			continue
		}
		m := ed.Media[link.MediaIndex]
		keys = append(keys, "media|"+m.File+"|"+m.Title+"|"+m.Form)
	}
	return keys
}

func individualSourceFingerprints(ed *enricher.EnrichedDocument, indiXref string) []string {
	x := normXref(indiXref)
	var keys []string
	for _, link := range ed.IndividualSources {
		if normXref(link.IndividualXref) != x {
			continue
		}
		if link.SourceIndex < 0 || link.SourceIndex >= len(ed.Sources) {
			continue
		}
		s := ed.Sources[link.SourceIndex]
		keys = append(keys, "src|"+s.Title+"|"+s.Author+"|"+link.Page)
	}
	return keys
}
