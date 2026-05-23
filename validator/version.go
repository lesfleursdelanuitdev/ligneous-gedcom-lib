package validator

import (
	"strings"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/gedcom"
)

// gedcomVersionFromHeader reads GEDC.VERS under HEAD (e.g. "5.5.1", "7.0").
// Returns empty string if not found.
func gedcomVersionFromHeader(head gedcom.GedcomRecord) string {
	for _, c := range head.Children {
		if c.Tag != "GEDC" {
			continue
		}
		for _, gc := range c.Children {
			if gc.Tag == "VERS" {
				return strings.TrimSpace(gc.Value)
			}
		}
	}
	return ""
}

func isGedcom7(ver string) bool {
	v := strings.TrimSpace(ver)
	return strings.HasPrefix(v, "7")
}

func isGedcom55Or551(ver string) bool {
	v := strings.TrimSpace(ver)
	return v == "5.5" || v == "5.5.1"
}

// headerHasSubm returns true if HEAD has a SUBM child with a non-empty xref-like value.
func headerHasSubm(head gedcom.GedcomRecord) bool {
	for _, c := range head.Children {
		if c.Tag == "SUBM" && strings.TrimSpace(c.Value) != "" {
			return true
		}
	}
	return false
}
