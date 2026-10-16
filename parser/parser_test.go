package parser

import (
	"errors"
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

// withoutEncoding drops the encoding findings. The inputs of the tests that use
// it are header-only snippets with no CHAR line, which now earns an
// ENCODING_UNDECLARED hint that these tests are not about.
func withoutEncoding(ws []*ParseError) []*ParseError {
	var out []*ParseError
	for _, w := range ws {
		if !strings.HasPrefix(w.Code, "ENCODING_") {
			out = append(out, w)
		}
	}
	return out
}

func TestParseEmptyLine(t *testing.T) {
	input := "0 HEAD\n\n0 TRLR\n"
	doc, warnings, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	warnings = withoutEncoding(warnings)
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning for empty line, got %d", len(warnings))
	} else if warnings[0].Code != CodeEmptyLine || warnings[0].Severity != "warning" {
		t.Errorf("got code %q severity %q", warnings[0].Code, warnings[0].Severity)
	}
	if doc.Header.Tag != "HEAD" {
		t.Errorf("expected HEAD tag, got %q", doc.Header.Tag)
	}
}

func TestParseInvalidLevel(t *testing.T) {
	input := "abc HEAD\n0 TRLR\n"
	_, _, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid level")
	}
	var pe *ParseError
	if !errors.As(err, &pe) || pe.Code != CodeBadLevel || pe.Severity != "error" {
		t.Errorf("got %#v", err)
	}
}

func TestParseMissingTag(t *testing.T) {
	input := "0\n"
	_, _, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for line with only level")
	}
	var pe *ParseError
	if !errors.As(err, &pe) || pe.Code != CodeBadLine || pe.Severity != "error" {
		t.Errorf("got %#v", err)
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

func TestParseCROnlyFile(t *testing.T) {
	doc, _, err := Parse(strings.NewReader("0 HEAD\r1 GEDC\r2 VERS 5.5.1\r0 @I1@ INDI\r1 NAME A /B/\r0 TRLR\r"))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Individuals) != 1 || doc.Trailer.Tag != "TRLR" || doc.Header.Value != "" {
		t.Errorf("indi=%d trailer=%q header value=%q", len(doc.Individuals), doc.Trailer.Tag, doc.Header.Value)
	}
}

func TestTokeniseXrefContainingTag(t *testing.T) {
	doc, _, err := Parse(strings.NewReader("0 HEAD\n0 @NOTE1@ NOTE Hello there\n0 TRLR\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Notes) != 1 || doc.Notes[0].Value != "Hello there" || doc.Notes[0].Xref != "@NOTE1@" {
		t.Errorf("notes = %+v", doc.Notes)
	}
}

func childValues(t *testing.T, line string) string {
	t.Helper()
	doc, _, err := Parse(strings.NewReader("0 HEAD\n0 @N1@ NOTE x\n" + line + "\n0 TRLR\n"))
	if err != nil {
		t.Fatal(err)
	}
	return doc.Notes[0].Children[0].Value
}

func TestTokeniseConcKeepsLeadingSpace(t *testing.T) {
	if got := childValues(t, "1 CONC  world"); got != " world" {
		t.Errorf("got %q", got)
	}
}

func TestTokeniseEmptyValues(t *testing.T) {
	for line, want := range map[string]string{"1 CONC": "", "1 NAME ": "", "1 CONC  ": " ", "2 DATE": ""} {
		if got := childValues(t, line); got != want {
			t.Errorf("%q: got %q want %q", line, got, want)
		}
	}
}

func TestLeadingWhitespaceWarning(t *testing.T) {
	doc, warnings, err := Parse(strings.NewReader("0 HEAD\n0 @I1@ INDI\n  1 NAME A /B/\n0 TRLR\n"))
	if err != nil {
		t.Fatal(err)
	}
	warnings = withoutEncoding(warnings)
	if len(warnings) != 1 || warnings[0].Code != CodeLeadingWhitespace || warnings[0].Line != 3 || warnings[0].Severity != "warning" {
		t.Fatalf("warnings = %+v", warnings)
	}
	if doc.Individuals[0].Children[0].Value != "A /B/" {
		t.Errorf("value = %q", doc.Individuals[0].Children[0].Value)
	}
}

func TestLongLineOver64KB(t *testing.T) {
	doc, warnings, err := Parse(strings.NewReader("0 HEAD\n0 @N1@ NOTE " + strings.Repeat("x", 70000) + "\n0 TRLR\n"))
	if err != nil {
		t.Fatal(err)
	}
	warnings = withoutEncoding(warnings)
	if len(doc.Notes[0].Value) != 70000 {
		t.Errorf("value length %d", len(doc.Notes[0].Value))
	}
	if len(warnings) != 1 || warnings[0].Code != CodeLineTooLong || warnings[0].Severity != "warning" {
		t.Errorf("warnings = %+v", warnings)
	}
}

func findCode(ws []*ParseError, code string) []*ParseError {
	var out []*ParseError
	for _, w := range ws {
		if w.Code == code {
			out = append(out, w)
		}
	}
	return out
}

func TestLinesBeforeHead(t *testing.T) {
	input := "junk one\n\x00junk two\n0 HEAD\n1 CHAR UTF-8\n0 @I1@ INDI\n0 TRLR\n"
	doc, ws, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	f := findCode(ws, CodeLinesBeforeHead)
	if len(f) != 1 || f[0].Line != 1 || f[0].Details["count"] != "2" || f[0].Severity != "warning" {
		t.Fatalf("bad finding: %+v", f)
	}
	if len(findCode(ws, CodeBadLine)) != 0 {
		t.Error("junk was tokenised")
	}
	if doc.Header.Tag != "HEAD" || len(doc.Individuals) != 1 {
		t.Error("tree changed")
	}
}

func TestNoHeadSkipsNothing(t *testing.T) {
	_, ws, err := Parse(strings.NewReader("0 @I1@ INDI\n0 TRLR\n"))
	if err != nil || len(findCode(ws, CodeLinesBeforeHead)) != 0 {
		t.Errorf("err=%v ws=%v", err, ws)
	}
}

func TestLevelJump(t *testing.T) {
	input := "0 HEAD\n0 @I1@ INDI\n1 BIRT\n3 DATE 1900\n0 TRLR\n"
	doc, ws, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	f := findCode(ws, CodeLevelJump)
	if len(f) != 1 || f[0].Line != 4 || f[0].Details["from"] != "1" || f[0].Details["to"] != "3" || f[0].Severity != "warning" {
		t.Fatalf("bad finding: %+v", f)
	}
	if doc.Individuals[0].Children[0].ChildValue("DATE") != "1900" {
		t.Error("DATE not under BIRT")
	}
}

func TestUnknownTopLevelRecord(t *testing.T) {
	input := "0 HEAD\n0 @X1@ _EVDEF\n1 NAME a\n1 TYPE b\n0 TRLR\n"
	doc, ws, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	f := findCode(ws, CodeUnknownRecord)
	if len(f) != 1 || f[0].Line != 2 || f[0].Details["tag"] != "_EVDEF" || f[0].Severity != "hint" {
		t.Fatalf("bad finding: %+v", f)
	}
	if len(doc.Individuals)+len(doc.Families)+len(doc.Sources)+len(doc.Notes)+len(doc.Submitters) != 0 {
		t.Error("record landed in a slice")
	}
}

func TestLenientSkipsBadLines(t *testing.T) {
	input := "0 HEAD\n1 CHAR UTF-8\n0 @I1@ INDI\n1 NAME A\nABC NAME x\n0 @I2@ INDI\n1 NAME B\n1 SEX M\n1\n0 @I3@ INDI\n0 TRLR\n"
	doc, ws, err := ParseWithOptions(strings.NewReader(input), &Options{Lenient: true, MaxNestingDepth: 100})
	if err != nil {
		t.Fatal(err)
	}
	bl := findCode(ws, CodeBadLevel)
	bd := findCode(ws, CodeBadLine)
	if len(bl) != 1 || bl[0].Line != 5 || bl[0].Severity != "error" || len(bd) != 1 || bd[0].Line != 9 || bd[0].Severity != "error" {
		t.Fatalf("bad findings: %+v %+v", bl, bd)
	}
	if len(doc.Individuals) != 3 || doc.Trailer.Tag != "TRLR" {
		t.Errorf("individuals=%d", len(doc.Individuals))
	}
	_, _, err = Parse(strings.NewReader(input))
	var pe *ParseError
	if !errors.As(err, &pe) || pe.Line != 5 {
		t.Errorf("strict err = %v", err)
	}
}

func TestRecordLineNumbers(t *testing.T) {
	input := "0 HEAD\n1 CHAR UTF-8\n1 GEDC\n0 @I1@ INDI\n1 NAME A /B/\n1 BIRT\n2 DATE 1900\n0 TRLR\n"
	doc, _, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Header.Line != 1 {
		t.Errorf("header line = %d, want 1", doc.Header.Line)
	}
	indi := doc.Individuals[0]
	if indi.Line != 4 {
		t.Errorf("INDI line = %d, want 4", indi.Line)
	}
	if got := indi.Children[0].Line; got != 5 {
		t.Errorf("NAME line = %d, want 5", got)
	}
	if got := indi.Children[1].Children[0].Line; got != 7 {
		t.Errorf("DATE line = %d, want 7", got)
	}
	if doc.Trailer.Line != 8 {
		t.Errorf("TRLR line = %d, want 8", doc.Trailer.Line)
	}
}

func TestRecordLineNumbersAfterBlankAndJunk(t *testing.T) {
	input := "junk\n0 HEAD\n\n1 CHAR UTF-8\n0 @I1@ INDI\n1 NAME A /B/\n0 TRLR\n"
	doc, _, err := ParseWithOptions(strings.NewReader(input), &Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Header.Line != 2 || doc.Individuals[0].Line != 5 || doc.Individuals[0].Children[0].Line != 6 {
		t.Errorf("lines = %d %d %d, want 2 5 6", doc.Header.Line, doc.Individuals[0].Line, doc.Individuals[0].Children[0].Line)
	}
}
