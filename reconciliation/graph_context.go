package reconciliation

import (
	"fmt"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/enricher"
)

func findIndividualByXref(ed *enricher.EnrichedDocument, xref string) *enricher.EnrichedIndividual {
	k := normXref(xref)
	if k == "" {
		return nil
	}
	for i := range ed.Individuals {
		ind := &ed.Individuals[i]
		if normXref(ind.Xref) == k {
			return ind
		}
	}
	return nil
}

// individualNeighborFingerprints builds order-insensitive multiset keys for graph context:
// "P:surname|birthYear", "S:spouseSurname|birthYear", "C:childSurname|birthYear".
func individualNeighborFingerprints(ed *enricher.EnrichedDocument, indi *enricher.EnrichedIndividual) []string {
	ix := normXref(indi.Xref)
	if ix == "" {
		return nil
	}
	var keys []string

	for _, e := range ed.ParentChild {
		if normXref(e.ChildXref) != ix {
			continue
		}
		p := findIndividualByXref(ed, e.ParentXref)
		if p == nil {
			keys = append(keys, "P:?|0")
			continue
		}
		by, _ := yearAt(ed, p.BirthDateIndex)
		keys = append(keys, fmt.Sprintf("P:%s|%d", p.PrimarySurnameLower, by))
	}

	for _, e := range ed.Spouses {
		if normXref(e.IndividualXref) != ix {
			continue
		}
		s := findIndividualByXref(ed, e.SpouseXref)
		if s == nil {
			keys = append(keys, "S:?|0")
			continue
		}
		by, _ := yearAt(ed, s.BirthDateIndex)
		keys = append(keys, fmt.Sprintf("S:%s|%d", s.PrimarySurnameLower, by))
	}

	for _, e := range ed.ParentChild {
		if normXref(e.ParentXref) != ix {
			continue
		}
		c := findIndividualByXref(ed, e.ChildXref)
		if c == nil {
			keys = append(keys, "C:?|0")
			continue
		}
		by, _ := yearAt(ed, c.BirthDateIndex)
		keys = append(keys, fmt.Sprintf("C:%s|%d", c.PrimarySurnameLower, by))
	}

	return keys
}

func jaccardMultisets(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	la := countMap(a)
	lb := countMap(b)
	keys := make(map[string]struct{})
	for k := range la {
		keys[k] = struct{}{}
	}
	for k := range lb {
		keys[k] = struct{}{}
	}
	inter, union := 0, 0
	for k := range keys {
		na, nb := la[k], lb[k]
		inter += minInt(na, nb)
		union += maxInt(na, nb)
	}
	if union == 0 {
		return 1.0
	}
	return float64(inter) / float64(union)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
