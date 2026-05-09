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

func TestFromEnriched_NoteXrefPointerFormatting(t *testing.T) {
	// DB / JSON payloads may store xrefs without "@" (e.g. "N1"); GEDCOM requires
	// subordinate NOTE lines like "1 NOTE @N1@" for pointers.
	ed := &enricher.EnrichedDocument{
		Individuals: []enricher.EnrichedIndividual{
			{Xref: "@I1@", FullName: "X /Y/", Sex: "M"},
		},
		Notes: []enricher.EnrichedNote{
			{Xref: "N1", Content: "Shared body", IsTopLevel: true},
		},
		IndividualNotes: []enricher.IndividualNoteLink{
			{IndividualXref: "@I1@", NoteIndex: 0},
		},
	}
	out := EnrichedToGEDCOM(ed)
	if strings.Contains(out, "1 NOTE N1\n") || strings.Contains(out, "1 NOTE N1\r") {
		t.Errorf("bare N1 is not a valid GEDCOM pointer; got:\n%s", out)
	}
	if !strings.Contains(out, "1 NOTE @N1@") {
		t.Errorf("expected NOTE pointer @N1@; got:\n%s", out)
	}
	if !strings.Contains(out, "0 @N1@ NOTE") {
		t.Errorf("expected top-level note with xref @N1@; got:\n%s", out)
	}
}

func TestFromEnriched_TopLevelNoteXrefWithoutDelimiters(t *testing.T) {
	ed := enricher.EnrichedDocument{
		Notes: []enricher.EnrichedNote{{
			Xref:       "N4",
			Content:    "Born a month premature.",
			IsTopLevel: true,
		}},
	}
	out := EnrichedToGEDCOM(&ed)
	if strings.Contains(out, "0 N4 NOTE ") {
		t.Fatalf("must not emit bare N4 as xref; got:\n%s", out)
	}
	if !strings.Contains(out, "0 @N4@ NOTE Born a month premature.") {
		t.Fatalf("expected 0 @N4@ NOTE …; got:\n%s", out)
	}
}

func TestFromEnriched_DateUsesGedcomMonthToken(t *testing.T) {
	ed := enricher.EnrichedDocument{
		Dates: []enricher.ParsedDate{enricher.ParseDateString("June 2016")},
		Places: []enricher.ParsedPlace{
			{Original: "NYC", Name: "NYC", Hash: "placehash"},
		},
		Events: []enricher.Event{{
			Index: 0, EventType: "BIRT", DateIndex: 0, PlaceIndex: 0,
			OwnerXref: "@I1@", OwnerType: "INDI", SortOrder: 0,
		}},
		Individuals: []enricher.EnrichedIndividual{{
			Xref: "@I1@", FullName: "Test /Person/", FullNameLower: "test person",
		}},
		IndividualEvents: []enricher.IndividualEventLink{{
			IndividualXref: "@I1@", EventIndex: 0, Role: "principal",
		}},
	}
	out := EnrichedToGEDCOM(&ed)
	if strings.Contains(out, "June 2016") {
		t.Fatalf("expected canonical month token, got:\n%s", out)
	}
	if !strings.Contains(out, "2 DATE JUN 2016") {
		t.Fatalf("expected 2 DATE JUN 2016; got:\n%s", out)
	}
}

func TestFromEnriched_MediaOBJELinks(t *testing.T) {
	input := `0 HEAD
1 GEDC
2 VERS 5.5
0 @M1@ OBJE
1 FILE photo.jpg
2 FORM JPEG
1 TITL Family Photo
0 @I1@ INDI
1 NAME John /Doe/
1 SEX M
1 OBJE @M1@
0 TRLR
`
	doc, _, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ed := enricher.Enrich(doc)
	out := EnrichedToGEDCOM(ed)
	if !strings.Contains(out, "1 OBJE @M1@") {
		t.Errorf("expected individual OBJE pointer in export; got:\n%s", out)
	}
	if !strings.Contains(out, "0 @M1@ OBJE") {
		t.Error("expected top-level multimedia record")
	}
}

func TestFromEnriched_MediaDescriptionAsInlineNote(t *testing.T) {
	ed := enricher.EnrichedDocument{
		Media: []enricher.EnrichedMedia{{
			Xref:        "@M1@",
			File:        "a.jpg",
			Form:        "JPEG",
			Title:       "Portrait",
			Description: "Line one\nLine two",
		}},
	}
	out := EnrichedToGEDCOM(&ed)
	if !strings.Contains(out, "1 NOTE Line one") {
		t.Errorf("expected NOTE with first line; got:\n%s", out)
	}
	if !strings.Contains(out, "2 CONT Line two") {
		t.Errorf("expected CONT for second line; got:\n%s", out)
	}
}

func TestFromEnriched_DeathCause(t *testing.T) {
	input := `0 HEAD
1 GEDC
2 VERS 5.5
0 @I1@ INDI
1 NAME R /I/
1 SEX M
1 DEAT
2 DATE 1 JAN 2000
2 CAUS heart failure
0 TRLR
`
	doc, _, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ed := enricher.Enrich(doc)
	out := EnrichedToGEDCOM(ed)
	if !strings.Contains(out, "2 CAUS heart failure") {
		t.Errorf("expected CAUS under DEAT; got:\n%s", out)
	}
}

func TestFromEnriched_FamilySurnameNote(t *testing.T) {
	doc, _, err := parser.Parse(strings.NewReader(`0 HEAD
1 GEDC
2 VERS 5.5
0 TRLR
`))
	if err != nil {
		t.Fatalf("parse header: %v", err)
	}
	ed := enricher.Enrich(doc)
	ed.Surnames = []enricher.Surname{{Value: "Acosta"}, {Value: "Lopez"}}
	ed.Families = []enricher.EnrichedFamily{{Xref: "@F1@"}}
	ed.FamilySurnames = []enricher.FamilySurnameLink{
		{FamilyXref: "@F1@", SurnameIndex: 0},
		{FamilyXref: "@F1@", SurnameIndex: 1},
	}
	out := EnrichedToGEDCOM(ed)
	if !strings.Contains(out, "Family surname(s): Acosta, Lopez") {
		t.Errorf("expected family surname NOTE; got:\n%s", out)
	}
}

func TestFromEnriched_EventSourceAndMediaUnderEvent(t *testing.T) {
	input := `0 HEAD
1 GEDC
2 VERS 5.5
0 @M1@ OBJE
1 FILE x.jpg
2 FORM JPEG
0 @S1@ SOUR
1 TITL Census
0 @I1@ INDI
1 NAME A /B/
1 BIRT
2 DATE 1 JAN 1900
2 SOUR @S1@
3 PAGE 5
2 OBJE @M1@
0 TRLR
`
	doc, _, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ed := enricher.Enrich(doc)
	out := EnrichedToGEDCOM(ed)
	if !strings.Contains(out, "2 SOUR @S1@") {
		t.Errorf("expected SOUR citation under BIRT; got:\n%s", out)
	}
	if !strings.Contains(out, "3 PAGE 5") {
		t.Errorf("expected PAGE under event SOUR; got:\n%s", out)
	}
	if !strings.Contains(out, "2 OBJE @M1@") {
		t.Errorf("expected OBJE under BIRT; got:\n%s", out)
	}
}

func TestFromEnriched_ScalarOccupationWithoutEvent(t *testing.T) {
	doc, _, err := parser.Parse(strings.NewReader(`0 HEAD
1 GEDC
2 VERS 5.5
0 @I1@ INDI
1 NAME X /Y/
1 SEX M
0 TRLR
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ed := enricher.Enrich(doc)
	ed.Individuals[0].OccupationValues = []string{"Blacksmith"}
	out := EnrichedToGEDCOM(ed)
	if !strings.Contains(out, "1 OCCU Blacksmith") {
		t.Errorf("expected OCCU from OccupationValues; got:\n%s", out)
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

func TestFromEnriched_SingleParentEmitsFAMS(t *testing.T) {
	input := `0 HEAD
1 GEDC
2 VERS 5.5
0 @I1@ INDI
1 NAME Jamie /Test/
1 SEX F
0 @I2@ INDI
1 NAME Child /Test/
1 SEX F
0 @F1@ FAM
1 WIFE @I1@
1 CHIL @I2@
0 TRLR
`
	doc, _, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ed := enricher.Enrich(doc)
	out := ToGEDCOM(FromEnriched(ed))
	if !strings.Contains(out, "0 @I1@ INDI") {
		t.Fatal("missing mother INDI")
	}
	if !strings.Contains(out, "1 FAMS @F1@") {
		t.Fatalf("expected sole WIFE on FAM to get FAMS on INDI; got:\n%s", out)
	}
}

func TestFromEnriched_MixedParentageFamcExport(t *testing.T) {
	input := `0 HEAD
1 GEDC
2 VERS 5.5
0 @I1@ INDI
1 NAME Father /Bio/
0 @I2@ INDI
1 NAME Mother /Adopt/
0 @I3@ INDI
1 NAME Child /Kid/
1 FAMC @F1@
2 PEDI birth
2 NOTE Ligneous mixed parentage: biological=@I1@; adoptive=@I2@
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
	ed := enricher.Enrich(doc)
	out := ToGEDCOM(FromEnriched(ed))
	if !strings.Contains(out, "2 PEDI birth") {
		t.Fatalf("expected FAMC PEDI birth in output:\n%s", out)
	}
	if !strings.Contains(out, "Ligneous mixed parentage:") || !strings.Contains(out, "biological=@I1@") {
		t.Fatalf("expected inline mixed-parentage NOTE under FAMC:\n%s", out)
	}
}

func TestFromEnriched_Associations(t *testing.T) {
	input := `0 HEAD
1 GEDC
2 VERS 5.5
0 @I1@ INDI
1 NAME John /Doe/
1 ASSO @I2@
2 RELA Godfather
1 BIRT
2 DATE 1 JAN 1900
2 ASSO @I3@
3 RELA Witness
0 @I2@ INDI
1 NAME Jane /Smith/
0 @I3@ INDI
1 NAME Witness /Person/
0 @F1@ FAM
1 HUSB @I1@
1 ASSO @I2@
2 RELA Neighbor
0 TRLR
`
	doc, _, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ed := enricher.Enrich(doc)
	out := EnrichedToGEDCOM(ed)

	if !strings.Contains(out, "1 ASSO @I2@") || !strings.Contains(out, "2 RELA Godfather") {
		t.Fatalf("expected individual ASSO/RELA in output:\n%s", out)
	}
	if !strings.Contains(out, "2 ASSO @I3@") || !strings.Contains(out, "3 RELA Witness") {
		t.Fatalf("expected event-level ASSO/RELA in output:\n%s", out)
	}
	if !strings.Contains(out, "0 @F1@ FAM") || !strings.Contains(out, "2 RELA Neighbor") {
		t.Fatalf("expected family ASSO/RELA in output:\n%s", out)
	}
}
