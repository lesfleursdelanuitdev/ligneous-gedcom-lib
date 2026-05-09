package exporter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/gedcom"
	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
)

// Example: DB / JSON may store a note xref as "N1" instead of "@N1@". GEDCOM requires
// `0 @N1@ NOTE …`, not `0 N1 NOTE …` (which parsers treat as xref "N1" missing @ delimiters).
func TestToGEDCOM_NormalizesLevel0XrefDelimiters(t *testing.T) {
	doc := &gedcom.GedcomDocument{
		Header: gedcom.GedcomRecord{Level: 0, Tag: "HEAD"},
		Individuals: []gedcom.GedcomRecord{
			{Level: 0, Tag: "INDI", Xref: "I1"},
		},
		Notes: []gedcom.GedcomRecord{
			{Level: 0, Tag: "NOTE", Xref: "N1", Value: "Norman Peter Gonsalves passed away in 2022."},
		},
		Media: []gedcom.GedcomRecord{
			{Level: 0, Tag: "OBJE", Xref: "M15"},
		},
		Trailer: gedcom.GedcomRecord{Level: 0, Tag: "TRLR"},
	}
	out := ToGEDCOM(doc)
	if strings.Contains(out, "0 N1 NOTE ") {
		t.Errorf("bare N1 must not appear as level-0 xref; got:\n%s", out)
	}
	if !strings.Contains(out, "0 @N1@ NOTE Norman Peter Gonsalves passed away in 2022.") {
		t.Errorf("expected 0 @N1@ NOTE …; got:\n%s", out)
	}
	if !strings.Contains(out, "0 @I1@ INDI") {
		t.Errorf("expected 0 @I1@ INDI; got:\n%s", out)
	}
	if !strings.Contains(out, "0 @M15@ OBJE") {
		t.Errorf("expected 0 @M15@ OBJE; got:\n%s", out)
	}
}

func TestToGEDCOM_Minimal(t *testing.T) {
	input := `0 HEAD
1 SOUR TestSystem
1 GEDC
2 VERS 5.5
0 @I1@ INDI
1 NAME John /Doe/
1 SEX M
0 TRLR
`
	doc, _, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	output := ToGEDCOM(doc)

	// Verify key content is present
	if !strings.Contains(output, "0 HEAD") {
		t.Error("output missing HEAD")
	}
	if !strings.Contains(output, "0 @I1@ INDI") {
		t.Error("output missing INDI record")
	}
	if !strings.Contains(output, "1 NAME John /Doe/") {
		t.Error("output missing NAME")
	}
	if !strings.Contains(output, "1 SEX M") {
		t.Error("output missing SEX")
	}
	if !strings.Contains(output, "0 TRLR") {
		t.Error("output missing TRLR")
	}
}

func TestToGEDCOM_RoundTrip(t *testing.T) {
	input := `0 HEAD
1 SOUR TestSystem
1 GEDC
2 VERS 5.5
1 CHAR UTF-8
0 @I1@ INDI
1 NAME John /Doe/
2 GIVN John
2 SURN Doe
1 SEX M
1 BIRT
2 DATE 1 JAN 1950
2 PLAC New York, USA
0 @F1@ FAM
1 HUSB @I1@
1 MARR
2 DATE 15 JUN 1975
0 TRLR
`
	doc, _, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	output := ToGEDCOM(doc)

	// Parse again
	doc2, _, err := parser.Parse(strings.NewReader(output))
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}

	if doc2.IndividualCount() != doc.IndividualCount() {
		t.Errorf("individual count mismatch: %d vs %d", doc.IndividualCount(), doc2.IndividualCount())
	}
	if doc2.FamilyCount() != doc.FamilyCount() {
		t.Errorf("family count mismatch: %d vs %d", doc.FamilyCount(), doc2.FamilyCount())
	}

	// Verify data preserved
	indi := doc2.Individuals[0]
	if indi.ChildValue("NAME") != "John /Doe/" {
		t.Errorf("name not preserved: %q", indi.ChildValue("NAME"))
	}
}

func TestToGEDCOM_RealFiles(t *testing.T) {
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

			output := ToGEDCOM(doc)

			// Re-parse
			doc2, _, err := parser.Parse(strings.NewReader(output))
			if err != nil {
				t.Fatalf("re-parse: %v", err)
			}

			if doc2.IndividualCount() != doc.IndividualCount() {
				t.Errorf("individual count mismatch: %d vs %d", doc.IndividualCount(), doc2.IndividualCount())
			}
			if doc2.FamilyCount() != doc.FamilyCount() {
				t.Errorf("family count mismatch: %d vs %d", doc.FamilyCount(), doc2.FamilyCount())
			}

			t.Logf("%s round-trip: %d individuals, %d families",
				name, doc2.IndividualCount(), doc2.FamilyCount())
		})
	}
}
