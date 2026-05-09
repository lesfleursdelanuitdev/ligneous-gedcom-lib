package reconciliation

import (
	"fmt"
	"strings"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/enricher"
)

// softBlockingKey groups individuals for candidate comparisons (deterministic).
func softBlockingKey(ed *enricher.EnrichedDocument, indi *enricher.EnrichedIndividual) string {
	sur := strings.TrimSpace(strings.ToLower(indi.PrimarySurnameLower))
	if sur == "" {
		sur = "unknown_surname"
	}
	by, ok := yearAt(ed, indi.BirthDateIndex)
	decade := "unknown_decade"
	if ok && by != 0 {
		decade = fmt.Sprintf("d%d", (by/10)*10)
	}
	return sur + "|" + decade
}
