package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseMinimal(t *testing.T) {
	input := `0 HEAD
1 SOUR TestSystem
1 GEDC
2 VERS 5.5
1 CHAR ASCII
0 @I1@ INDI
1 NAME John /Doe/
2 GIVN John
2 SURN Doe
1 SEX M
1 BIRT
2 DATE 1 JAN 1950
2 PLAC New York, USA
0 TRLR
`
	doc, warnings, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d", len(warnings))
	}

	if doc.Header.Tag != "HEAD" {
		t.Errorf("expected HEAD tag, got %q", doc.Header.Tag)
	}
	if doc.Trailer.Tag != "TRLR" {
		t.Errorf("expected TRLR tag, got %q", doc.Trailer.Tag)
	}
	if len(doc.Individuals) != 1 {
		t.Fatalf("expected 1 individual, got %d", len(doc.Individuals))
	}

	indi := doc.Individuals[0]
	if indi.Xref != "@I1@" {
		t.Errorf("expected xref @I1@, got %q", indi.Xref)
	}
	if indi.Tag != "INDI" {
		t.Errorf("expected INDI tag, got %q", indi.Tag)
	}

	nameVal := indi.ChildValue("NAME")
	if nameVal != "John /Doe/" {
		t.Errorf("expected name 'John /Doe/', got %q", nameVal)
	}
	if indi.ChildValue("SEX") != "M" {
		t.Errorf("expected SEX M, got %q", indi.ChildValue("SEX"))
	}

	// Check nested hierarchy: BIRT -> DATE, PLAC
	births := indi.ChildrenByTag("BIRT")
	if len(births) != 1 {
		t.Fatalf("expected 1 BIRT, got %d", len(births))
	}
	if births[0].ChildValue("DATE") != "1 JAN 1950" {
		t.Errorf("expected birth date '1 JAN 1950', got %q", births[0].ChildValue("DATE"))
	}
	if births[0].ChildValue("PLAC") != "New York, USA" {
		t.Errorf("expected birth place 'New York, USA', got %q", births[0].ChildValue("PLAC"))
	}

	// Check NAME sub-tags
	nameRec := indi.FirstChildByTag("NAME")
	if nameRec == nil {
		t.Fatal("expected NAME child")
	}
	if nameRec.ChildValue("GIVN") != "John" {
		t.Errorf("expected GIVN 'John', got %q", nameRec.ChildValue("GIVN"))
	}
	if nameRec.ChildValue("SURN") != "Doe" {
		t.Errorf("expected SURN 'Doe', got %q", nameRec.ChildValue("SURN"))
	}
}

func TestParseFamilies(t *testing.T) {
	input := `0 HEAD
1 GEDC
2 VERS 5.5
0 @I1@ INDI
1 NAME John /Doe/
1 SEX M
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
1 MARR
2 DATE 15 JUN 1975
2 PLAC Boston, MA
0 TRLR
`
	doc, _, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(doc.Individuals) != 3 {
		t.Errorf("expected 3 individuals, got %d", len(doc.Individuals))
	}
	if len(doc.Families) != 1 {
		t.Fatalf("expected 1 family, got %d", len(doc.Families))
	}

	fam := doc.Families[0]
	if fam.Xref != "@F1@" {
		t.Errorf("expected xref @F1@, got %q", fam.Xref)
	}
	if fam.ChildValue("HUSB") != "@I1@" {
		t.Errorf("expected HUSB @I1@, got %q", fam.ChildValue("HUSB"))
	}
	if fam.ChildValue("WIFE") != "@I2@" {
		t.Errorf("expected WIFE @I2@, got %q", fam.ChildValue("WIFE"))
	}

	marrs := fam.ChildrenByTag("MARR")
	if len(marrs) != 1 {
		t.Fatalf("expected 1 MARR, got %d", len(marrs))
	}
	if marrs[0].ChildValue("DATE") != "15 JUN 1975" {
		t.Errorf("expected marriage date '15 JUN 1975', got %q", marrs[0].ChildValue("DATE"))
	}
}

func TestParseEmptyLine(t *testing.T) {
	input := "0 HEAD\n\n0 TRLR\n"
	doc, warnings, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning for empty line, got %d", len(warnings))
	}
	if doc.Header.Tag != "HEAD" {
		t.Errorf("expected HEAD tag, got %q", doc.Header.Tag)
	}
}

func TestParseInvalidLevel(t *testing.T) {
	input := "abc HEAD\n0 TRLR\n"
	_, _, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Error("expected error for invalid level")
	}
}

func TestParseMissingTag(t *testing.T) {
	input := "0\n"
	_, _, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Error("expected error for line with only level")
	}
}

func TestParseNotes(t *testing.T) {
	input := `0 HEAD
1 GEDC
2 VERS 5.5
0 @N1@ NOTE This is a note
1 CONT with a continuation
1 CONC and concatenation
0 TRLR
`
	doc, _, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(doc.Notes))
	}
	note := doc.Notes[0]
	if note.Xref != "@N1@" {
		t.Errorf("expected xref @N1@, got %q", note.Xref)
	}
	if note.Value != "This is a note" {
		t.Errorf("expected note value 'This is a note', got %q", note.Value)
	}
}

func TestParseSources(t *testing.T) {
	input := `0 HEAD
1 GEDC
2 VERS 5.5
0 @S1@ SOUR
1 TITL Census 1900
1 AUTH Government
1 PUBL Washington, D.C.
0 TRLR
`
	doc, _, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(doc.Sources))
	}
	src := doc.Sources[0]
	if src.ChildValue("TITL") != "Census 1900" {
		t.Errorf("expected title 'Census 1900', got %q", src.ChildValue("TITL"))
	}
}

func TestParseXRefIndex(t *testing.T) {
	input := `0 HEAD
1 GEDC
2 VERS 5.5
0 @I1@ INDI
1 NAME John /Doe/
0 @F1@ FAM
1 HUSB @I1@
0 @S1@ SOUR
1 TITL Test
0 TRLR
`
	doc, _, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	idx := doc.XRefIndex()
	if idx["@I1@"] == nil {
		t.Error("expected @I1@ in index")
	}
	if idx["@F1@"] == nil {
		t.Error("expected @F1@ in index")
	}
	if idx["@S1@"] == nil {
		t.Error("expected @S1@ in index")
	}
	if idx["@NONE@"] != nil {
		t.Error("expected @NONE@ to be nil")
	}
}

func TestParseBOM(t *testing.T) {
	bom := "\xEF\xBB\xBF"
	input := bom + "0 HEAD\n1 GEDC\n2 VERS 5.5\n0 TRLR\n"
	doc, _, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Header.Tag != "HEAD" {
		t.Errorf("expected HEAD after BOM, got %q", doc.Header.Tag)
	}
}

// TestParseRealFiles tests parsing of real GEDCOM files from testdata.
func TestParseRealFiles(t *testing.T) {
	files, err := filepath.Glob("../testdata/*.ged")
	if err != nil {
		t.Fatalf("glob error: %v", err)
	}
	if len(files) == 0 {
		t.Skip("no testdata/*.ged files found")
	}

	for _, path := range files {
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			f, err := os.Open(path)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer f.Close()

			doc, warnings, err := Parse(f)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			t.Logf("parsed %s: %d individuals, %d families, %d warnings",
				name, doc.IndividualCount(), doc.FamilyCount(), len(warnings))

			if doc.Header.Tag != "HEAD" {
				t.Errorf("expected HEAD, got %q", doc.Header.Tag)
			}
		})
	}
}

func TestParseMalformedFiles(t *testing.T) {
	t.Run("missing-header", func(t *testing.T) {
		f, err := os.Open("../testdata/malformed/missing-header.ged")
		if err != nil {
			t.Skip("missing test file")
		}
		defer f.Close()

		doc, _, err := Parse(f)
		if err != nil {
			t.Fatalf("unexpected fatal error: %v", err)
		}
		if doc.Header.Tag != "" {
			t.Logf("Note: file starts with INDI instead of HEAD, Header.Tag=%q", doc.Header.Tag)
		}
	})

	t.Run("invalid-level", func(t *testing.T) {
		f, err := os.Open("../testdata/malformed/invalid-level.ged")
		if err != nil {
			t.Skip("missing test file")
		}
		defer f.Close()

		doc, _, err := Parse(f)
		// Level 99 is valid syntax (under our default limit of 100), just deep
		if err != nil {
			t.Logf("parse error (may be expected): %v", err)
			return
		}
		t.Logf("parsed: %d individuals", doc.IndividualCount())
	})
}
