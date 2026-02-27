package exporter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/enricher"
	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
)

func TestFromEnriched_Minimal(t *testing.T) {
	input := `0 HEAD
1 GEDC
2 VERS 5.5
0 @N1@ NOTE This is a top-level note
0 @N2@ NOTE Another note
1 CONT with continuation
0 @S1@ SOUR
1 TITL Census 1900
1 AUTH Government
0 @I1@ INDI
1 NAME John /Doe/
2 GIVN John
2 SURN Doe
1 SEX M
1 BIRT
2 DATE 1 JAN 1950
2 PLAC New York, USA
1 NOTE @N1@
1 SOUR @S1@
0 @I2@ INDI
1 NAME Jane /Smith/
2 GIVN Jane
2 SURN Smith
1 SEX F
0 @F1@ FAM
1 HUSB @I1@
1 WIFE @I2@
1 CHIL @I3@
1 MARR
2 DATE 15 JUN 1975
2 NOTE @N2@
0 @I3@ INDI
1 NAME Junior /Doe/
2 GIVN Junior
2 SURN Doe
1 SEX M
0 TRLR
`
	doc, _, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	ed := enricher.Enrich(doc)

	reconstructed := FromEnriched(ed)

	if len(reconstructed.Individuals) != 3 {
		t.Errorf("expected 3 individuals, got %d", len(reconstructed.Individuals))
	}
	if len(reconstructed.Families) != 1 {
		t.Errorf("expected 1 family, got %d", len(reconstructed.Families))
	}
	if len(reconstructed.Sources) != 1 {
		t.Errorf("expected 1 source, got %d", len(reconstructed.Sources))
	}
	if len(reconstructed.Notes) != 2 {
		t.Errorf("expected 2 top-level notes, got %d", len(reconstructed.Notes))
	}

	gedcomText := ToGEDCOM(reconstructed)

	if !strings.Contains(gedcomText, "0 @I1@ INDI") {
		t.Error("missing @I1@ INDI")
	}
	if !strings.Contains(gedcomText, "1 NAME John /Doe/") {
		t.Error("missing NAME John /Doe/")
	}
	if !strings.Contains(gedcomText, "1 NOTE @N1@") {
		t.Error("missing NOTE reference @N1@ on individual")
	}
	if !strings.Contains(gedcomText, "1 SOUR @S1@") {
		t.Error("missing SOUR reference @S1@ on individual")
	}
	if !strings.Contains(gedcomText, "0 @F1@ FAM") {
		t.Error("missing @F1@ FAM")
	}
	if !strings.Contains(gedcomText, "1 HUSB @I1@") {
		t.Error("missing HUSB @I1@")
	}
	if !strings.Contains(gedcomText, "1 WIFE @I2@") {
		t.Error("missing WIFE @I2@")
	}
	if !strings.Contains(gedcomText, "0 @N1@ NOTE This is a top-level note") {
		t.Error("missing top-level note @N1@")
	}
	if !strings.Contains(gedcomText, "0 @N2@ NOTE Another note") {
		t.Error("missing top-level note @N2@")
	}
	if !strings.Contains(gedcomText, "1 CONT with continuation") {
		t.Error("missing CONT line in note")
	}

	// Verify the output can be re-parsed
	doc2, _, err := parser.Parse(strings.NewReader(gedcomText))
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if doc2.IndividualCount() != 3 {
		t.Errorf("re-parsed: expected 3 individuals, got %d", doc2.IndividualCount())
	}
	if doc2.FamilyCount() != 1 {
		t.Errorf("re-parsed: expected 1 family, got %d", doc2.FamilyCount())
	}
}

func TestFromEnriched_NoteReferences(t *testing.T) {
	input := `0 HEAD
1 GEDC
2 VERS 5.5
0 @N1@ NOTE Individual note
0 @N2@ NOTE Event note
0 @I1@ INDI
1 NAME Test /Person/
1 SEX M
1 NOTE @N1@
1 BIRT
2 DATE 1 JAN 1900
2 NOTE @N2@
0 TRLR
`
	doc, _, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	ed := enricher.Enrich(doc)

	gedcomText := EnrichedToGEDCOM(ed)

	// Individual should have NOTE @N1@ (individual-level note)
	if !strings.Contains(gedcomText, "1 NOTE @N1@") {
		t.Error("missing individual-level NOTE @N1@")
	}

	// Birth event should have NOTE @N2@
	if !strings.Contains(gedcomText, "2 NOTE @N2@") {
		t.Error("missing event-level NOTE @N2@")
	}

	// Verify all notes are preserved as top-level records
	if !strings.Contains(gedcomText, "0 @N1@ NOTE Individual note") {
		t.Error("missing top-level @N1@ record")
	}
	if !strings.Contains(gedcomText, "0 @N2@ NOTE Event note") {
		t.Error("missing top-level @N2@ record")
	}
}

func TestFromEnriched_RealFiles(t *testing.T) {
	files, err := filepath.Glob("../testdata/*.ged")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Skip("no testdata found")
	}

	for _, path := range files {
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			f, err := os.Open(path)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer f.Close()

			doc, _, err := parser.Parse(f)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			ed := enricher.Enrich(doc)

			// Reconstruct from enriched
			gedcomText := EnrichedToGEDCOM(ed)

			// Re-parse the output
			doc2, _, err := parser.Parse(strings.NewReader(gedcomText))
			if err != nil {
				t.Fatalf("re-parse: %v", err)
			}

			// Individual and family counts should match
			if doc2.IndividualCount() != doc.IndividualCount() {
				t.Errorf("individual count: original=%d, reconstructed=%d",
					doc.IndividualCount(), doc2.IndividualCount())
			}
			if doc2.FamilyCount() != doc.FamilyCount() {
				t.Errorf("family count: original=%d, reconstructed=%d",
					doc.FamilyCount(), doc2.FamilyCount())
			}

			// Note count should match (top-level notes only)
			if len(doc2.Notes) != len(doc.Notes) {
				t.Errorf("note count: original=%d, reconstructed=%d",
					len(doc.Notes), len(doc2.Notes))
			}

			t.Logf("%s: %d indi, %d fam, %d notes, %d sources",
				name, doc2.IndividualCount(), doc2.FamilyCount(),
				len(doc2.Notes), len(doc2.Sources))
		})
	}
}
