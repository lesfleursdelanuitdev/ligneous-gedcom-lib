package exporter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
)

func TestToJSON_Minimal(t *testing.T) {
	input := `0 HEAD
1 GEDC
2 VERS 5.5
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
0 @S1@ SOUR
1 TITL Census 1900
1 AUTH Government
0 @N1@ NOTE This is a test note
0 TRLR
`
	doc, _, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result := ToJSON(doc)

	if result.File.IndividualsCount != 1 {
		t.Errorf("expected 1 individual, got %d", result.File.IndividualsCount)
	}
	if result.File.FamiliesCount != 1 {
		t.Errorf("expected 1 family, got %d", result.File.FamiliesCount)
	}

	if len(result.Individuals) != 1 {
		t.Fatalf("expected 1 individual in result, got %d", len(result.Individuals))
	}

	indi := result.Individuals[0]
	if indi.Xref != "@I1@" {
		t.Errorf("expected xref @I1@, got %q", indi.Xref)
	}
	if indi.Name != "John /Doe/" {
		t.Errorf("expected name 'John /Doe/', got %q", indi.Name)
	}
	if indi.Sex != "M" {
		t.Errorf("expected sex M, got %q", indi.Sex)
	}
	if indi.Birth == nil {
		t.Fatal("expected birth event")
	}
	if indi.Birth.Date != "1 JAN 1950" {
		t.Errorf("expected birth date '1 JAN 1950', got %q", indi.Birth.Date)
	}
	if indi.Birth.DateYear != 1950 {
		t.Errorf("expected birth year 1950, got %d", indi.Birth.DateYear)
	}

	if len(result.Families) != 1 {
		t.Fatalf("expected 1 family, got %d", len(result.Families))
	}

	fam := result.Families[0]
	if fam.Husband == nil || fam.Husband.Xref != "@I1@" {
		t.Errorf("expected husband @I1@")
	}
	if fam.Marriage == nil {
		t.Fatal("expected marriage event")
	}
	if fam.Marriage.Date != "15 JUN 1975" {
		t.Errorf("expected marriage date '15 JUN 1975', got %q", fam.Marriage.Date)
	}

	// Check sources
	src, ok := result.Sources["@S1@"]
	if !ok {
		t.Fatal("expected source @S1@")
	}
	if src.Title != "Census 1900" {
		t.Errorf("expected source title 'Census 1900', got %q", src.Title)
	}

	// Check notes
	note, ok := result.Notes["@N1@"]
	if !ok {
		t.Fatal("expected note @N1@")
	}
	if note.Content != "This is a test note" {
		t.Errorf("expected note content 'This is a test note', got %q", note.Content)
	}
}

func TestToJSONString(t *testing.T) {
	input := `0 HEAD
1 GEDC
2 VERS 5.5
0 @I1@ INDI
1 NAME John /Doe/
0 TRLR
`
	doc, _, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	jsonStr, err := ToJSONString(doc)
	if err != nil {
		t.Fatalf("ToJSONString: %v", err)
	}

	if !strings.Contains(jsonStr, `"xref": "@I1@"`) {
		t.Error("JSON output missing xref")
	}
	if !strings.Contains(jsonStr, `"John /Doe/"`) {
		t.Error("JSON output missing name")
	}
}

func TestToJSON_RealFiles(t *testing.T) {
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

			result := ToJSON(doc)

			if result.File.IndividualsCount != len(result.Individuals) {
				t.Errorf("count mismatch: metadata says %d, actual %d",
					result.File.IndividualsCount, len(result.Individuals))
			}

			t.Logf("%s: %d individuals, %d families, %d sources, %d notes",
				name, len(result.Individuals), len(result.Families),
				len(result.Sources), len(result.Notes))
		})
	}
}
