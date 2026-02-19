package exporter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
)

func TestToCSV_Minimal(t *testing.T) {
	input := `0 HEAD
1 GEDC
2 VERS 5.5
0 @I1@ INDI
1 NAME John /Doe/
1 SEX M
1 BIRT
2 DATE 1 JAN 1950
2 PLAC New York, USA
1 FAMS @F1@
0 @I2@ INDI
1 NAME Jane /Smith/
1 SEX F
1 FAMS @F1@
0 @I3@ INDI
1 NAME Baby /Doe/
1 SEX M
1 FAMC @F1@
0 @F1@ FAM
1 HUSB @I1@
1 WIFE @I2@
1 CHIL @I3@
0 TRLR
`
	doc, _, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	csvStr, err := ToCSVString(doc)
	if err != nil {
		t.Fatalf("ToCSVString: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(csvStr), "\n")
	if len(lines) < 4 { // header + 3 individuals
		t.Fatalf("expected at least 4 lines, got %d", len(lines))
	}

	// Check header
	if !strings.Contains(lines[0], "XREF") {
		t.Error("missing XREF in header")
	}
	if !strings.Contains(lines[0], "Birth Date") {
		t.Error("missing Birth Date in header")
	}

	// Check that individual data appears
	if !strings.Contains(csvStr, "@I1@") {
		t.Error("missing @I1@ in CSV")
	}
	if !strings.Contains(csvStr, "John /Doe/") {
		t.Error("missing John /Doe/ in CSV")
	}
	if !strings.Contains(csvStr, "1 JAN 1950") {
		t.Error("missing birth date in CSV")
	}
}

func TestToCSV_FamilyRelationships(t *testing.T) {
	input := `0 HEAD
1 GEDC
2 VERS 5.5
0 @I1@ INDI
1 NAME Father /Test/
1 SEX M
1 FAMS @F1@
0 @I2@ INDI
1 NAME Mother /Test/
1 SEX F
1 FAMS @F1@
0 @I3@ INDI
1 NAME Child /Test/
1 SEX M
1 FAMC @F1@
0 @F1@ FAM
1 HUSB @I1@
1 WIFE @I2@
1 CHIL @I3@
0 TRLR
`
	doc, _, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	csvStr, err := ToCSVString(doc)
	if err != nil {
		t.Fatalf("ToCSVString: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(csvStr), "\n")

	// Find the child row
	for _, line := range lines[1:] {
		if strings.Contains(line, "Child /Test/") {
			// Father and mother xrefs should be in this row
			if !strings.Contains(line, "@I1@") {
				t.Error("child row missing father xref @I1@")
			}
			if !strings.Contains(line, "@I2@") {
				t.Error("child row missing mother xref @I2@")
			}
			return
		}
	}
	t.Error("child row not found in CSV")
}

func TestToCSV_RealFiles(t *testing.T) {
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

			csvStr, err := ToCSVString(doc)
			if err != nil {
				t.Fatalf("ToCSVString: %v", err)
			}

			lines := strings.Split(strings.TrimSpace(csvStr), "\n")
			expectedRows := doc.IndividualCount() + 1 // +1 for header
			if len(lines) != expectedRows {
				t.Errorf("expected %d CSV rows, got %d", expectedRows, len(lines))
			}

			t.Logf("%s: %d CSV rows (header + %d individuals)",
				name, len(lines), doc.IndividualCount())
		})
	}
}
