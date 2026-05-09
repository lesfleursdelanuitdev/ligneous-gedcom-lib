package exporter

import (
	"strings"
	"testing"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/enricher"
)

func TestMaxPhysicalLineEnrichedNoteRoundTrip(t *testing.T) {
	const long = 400
	body := strings.Repeat("a", long)
	ed := &enricher.EnrichedDocument{
		Notes: []enricher.EnrichedNote{{
			Xref:       "@N1@",
			Content:    body,
			IsTopLevel: true,
		}},
	}
	out := EnrichedToGEDCOM(ed)
	for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		if len(line) > MaxPhysicalGEDCOMLineLen {
			t.Fatalf("line longer than %d: %d chars\n%s", MaxPhysicalGEDCOMLineLen, len(line), line[:min(120, len(line))])
		}
	}
	if !strings.Contains(out, "CONC") {
		t.Fatal("expected CONC continuation lines for long NOTE")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
