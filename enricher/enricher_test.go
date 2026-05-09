package enricher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
)

// --- Date parser tests ---

func TestParseDateExact(t *testing.T) {
	tests := []struct {
		input string
		typ   DateType
		year  int
		month int
		day   int
	}{
		{"1 JAN 1950", DateExact, 1950, 1, 1},
		{"15 MAR 1800", DateExact, 1800, 3, 15},
		{"25 DEC 2000", DateExact, 2000, 12, 25},
		{"JAN 1950", DateExact, 1950, 1, 0},
		{"1950", DateExact, 1950, 0, 0},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			pd := ParseDateString(tc.input)
			if pd.Type != tc.typ {
				t.Errorf("expected type %s, got %s", tc.typ, pd.Type)
			}
			if pd.Year != tc.year {
				t.Errorf("expected year %d, got %d", tc.year, pd.Year)
			}
			if pd.Month != tc.month {
				t.Errorf("expected month %d, got %d", tc.month, pd.Month)
			}
			if pd.Day != tc.day {
				t.Errorf("expected day %d, got %d", tc.day, pd.Day)
			}
			if pd.Hash == "" {
				t.Error("hash should not be empty")
			}
		})
	}
}

func TestParseDateModifiers(t *testing.T) {
	tests := []struct {
		input string
		typ   DateType
		year  int
	}{
		{"ABT 1850", DateAbout, 1850},
		{"abt. 1900", DateAbout, 1900},
		{"ABOUT 1750", DateAbout, 1750},
		{"BEF 1800", DateBefore, 1800},
		{"bef. JAN 1900", DateBefore, 1900},
		{"AFT 1900", DateAfter, 1900},
		{"CAL 1850", DateCalculated, 1850},
		{"EST 1900", DateEstimated, 1900},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			pd := ParseDateString(tc.input)
			if pd.Type != tc.typ {
				t.Errorf("expected type %s, got %s", tc.typ, pd.Type)
			}
			if pd.Year != tc.year {
				t.Errorf("expected year %d, got %d", tc.year, pd.Year)
			}
		})
	}
}

func TestParseDateRange(t *testing.T) {
	tests := []struct {
		input   string
		typ     DateType
		year    int
		endYear int
	}{
		{"BET 1800 AND 1850", DateBetween, 1800, 1850},
		{"bet. JAN 1900 and DEC 1910", DateBetween, 1900, 1910},
		{"FROM 1800 TO 1850", DateFromTo, 1800, 1850},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			pd := ParseDateString(tc.input)
			if pd.Type != tc.typ {
				t.Errorf("expected type %s, got %s", tc.typ, pd.Type)
			}
			if pd.Year != tc.year {
				t.Errorf("expected start year %d, got %d", tc.year, pd.Year)
			}
			if pd.EndYear != tc.endYear {
				t.Errorf("expected end year %d, got %d", tc.endYear, pd.EndYear)
			}
		})
	}
}

func TestParseDateDedup(t *testing.T) {
	a := ParseDateString("1 JAN 1950")
	b := ParseDateString("1 JAN 1950")
	if a.Hash != b.Hash {
		t.Error("identical dates should produce same hash")
	}
	c := ParseDateString("2 JAN 1950")
	if a.Hash == c.Hash {
		t.Error("different dates should produce different hashes")
	}
}

func TestParseDateEmpty(t *testing.T) {
	pd := ParseDateString("")
	if pd.Type != DateUnknown {
		t.Errorf("expected UNKNOWN for empty, got %s", pd.Type)
	}
}

func TestParseDateCalendar(t *testing.T) {
	pd := ParseDateString("@#DJULIAN@ 5 OCT 1582")
	if pd.Calendar != "JULIAN" {
		t.Errorf("expected JULIAN calendar, got %s", pd.Calendar)
	}
	if pd.Year != 1582 {
		t.Errorf("expected year 1582, got %d", pd.Year)
	}
}

// --- Place parser tests ---

func TestParsePlaceFourParts(t *testing.T) {
	pp := ParsePlaceString("Springfield, Sangamon, Illinois, USA")
	if pp.Name != "Springfield" {
		t.Errorf("expected name Springfield, got %q", pp.Name)
	}
	if pp.County != "Sangamon" {
		t.Errorf("expected county Sangamon, got %q", pp.County)
	}
	if pp.State != "Illinois" {
		t.Errorf("expected state Illinois, got %q", pp.State)
	}
	if pp.Country != "USA" {
		t.Errorf("expected country USA, got %q", pp.Country)
	}
}

func TestParsePlaceThreeParts(t *testing.T) {
	pp := ParsePlaceString("Paris, Ile-de-France, France")
	if pp.Name != "Paris" {
		t.Errorf("expected name Paris, got %q", pp.Name)
	}
}

func TestParsePlaceTwoParts(t *testing.T) {
	pp := ParsePlaceString("London, England")
	if pp.Name != "London" {
		t.Errorf("expected name London, got %q", pp.Name)
	}
	if pp.Country != "England" {
		t.Errorf("expected country England, got %q", pp.Country)
	}
}

func TestParsePlaceOnePart(t *testing.T) {
	pp := ParsePlaceString("Germany")
	if pp.Name != "Germany" {
		t.Errorf("expected name Germany, got %q", pp.Name)
	}
}

func TestParsePlaceDedup(t *testing.T) {
	a := ParsePlaceString("London, England")
	b := ParsePlaceString("London, England")
	if a.Hash != b.Hash {
		t.Error("identical places should produce same hash")
	}
	c := ParsePlaceString("Paris, France")
	if a.Hash == c.Hash {
		t.Error("different places should produce different hashes")
	}
}

// --- Name extraction tests ---

func TestExtractSurnameFromFullName(t *testing.T) {
	tests := []struct{ input, want string }{
		{"John /Doe/", "Doe"},
		{"Jane /Smith/ Jr.", "Smith"},
		{"NoSurname", ""},
		{"/OnlySurname/", "OnlySurname"},
	}
	for _, tc := range tests {
		got := extractSurnameFromFullName(tc.input)
		if got != tc.want {
			t.Errorf("extractSurname(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestExtractGivenFromFullName(t *testing.T) {
	tests := []struct{ input, want string }{
		{"John /Doe/", "John"},
		{"Jane Marie /Smith/", "Jane Marie"},
		{"NoSurname", "NoSurname"},
	}
	for _, tc := range tests {
		got := extractGivenFromFullName(tc.input)
		if got != tc.want {
			t.Errorf("extractGiven(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// --- Enricher integration test with notes, sources, repos, media ---

func TestEnrichFull(t *testing.T) {
	input := `0 HEAD
1 GEDC
2 VERS 5.5
0 @N1@ NOTE This is a top-level note.
0 @S1@ SOUR
1 TITL Census Records
1 AUTH National Archives
1 ABBR CENSUS
1 REPO @R1@
2 CALN 123-A
0 @R1@ REPO
1 NAME National Archives
1 ADDR 700 Pennsylvania Ave
2 CITY Washington
2 STAE DC
2 CTRY USA
1 PHON 555-1234
1 EMAIL archives@example.com
1 WWW https://archives.gov
0 @M1@ OBJE
1 FILE photo.jpg
2 FORM JPEG
1 TITL Family Photo
0 @I1@ INDI
1 NAME John /Doe/
2 GIVN John
2 SURN Doe
1 SEX M
1 BIRT
2 DATE 1 JAN 1950
2 PLAC New York, USA
2 OBJE @M1@
2 SOUR @S1@
3 PAGE p. 42
3 QUAY 3
1 DEAT
2 DATE 15 MAR 2020
2 PLAC Los Angeles, California, USA
2 NOTE Died peacefully.
1 NOTE @N1@
1 SOUR @S1@
2 PAGE p. 100
1 OBJE @M1@
1 FAMS @F1@
0 @I2@ INDI
1 NAME Jane /Smith/
2 GIVN Jane
2 SURN Smith
1 SEX F
1 BIRT
2 DATE 5 FEB 1955
1 FAMS @F1@
0 @I3@ INDI
1 NAME Baby /Doe/
2 GIVN Baby
2 SURN Doe
1 SEX M
1 BIRT
2 DATE 10 JUN 1980
2 PLAC New York, USA
1 FAMC @F1@
0 @F1@ FAM
1 HUSB @I1@
1 WIFE @I2@
1 CHIL @I3@
1 MARR
2 DATE 20 JUL 1975
2 PLAC Boston, Massachusetts, USA
1 NOTE @N1@
1 SOUR @S1@
2 PAGE p. 200
0 TRLR
`
	doc, _, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	ed := Enrich(doc)

	// --- Basic stats ---
	if ed.Stats.Individuals != 3 {
		t.Errorf("expected 3 individuals, got %d", ed.Stats.Individuals)
	}
	if ed.Stats.Families != 1 {
		t.Errorf("expected 1 family, got %d", ed.Stats.Families)
	}
	if ed.Stats.Dates != 5 {
		t.Errorf("expected 5 unique dates, got %d", ed.Stats.Dates)
	}
	if ed.Stats.Places != 3 {
		t.Errorf("expected 3 unique places, got %d", ed.Stats.Places)
	}
	if ed.Stats.Surnames != 2 {
		t.Errorf("expected 2 unique surnames, got %d", ed.Stats.Surnames)
	}
	if ed.Stats.GivenNames != 3 {
		t.Errorf("expected 3 unique given names, got %d", ed.Stats.GivenNames)
	}
	if len(ed.Events) != 5 {
		t.Errorf("expected 5 events, got %d", len(ed.Events))
	}

	// --- EnrichedIndividual ---
	if len(ed.Individuals) != 3 {
		t.Fatalf("expected 3 enriched individuals, got %d", len(ed.Individuals))
	}
	john := ed.Individuals[0]
	if john.Xref != "@I1@" {
		t.Errorf("expected @I1@, got %s", john.Xref)
	}
	if john.Sex != "M" {
		t.Errorf("expected sex M, got %s", john.Sex)
	}
	if john.BirthDateIndex < 0 {
		t.Error("John should have a birth date index")
	}
	if john.DeathDateIndex < 0 {
		t.Error("John should have a death date index")
	}

	// --- EnrichedFamily ---
	if len(ed.Families) != 1 {
		t.Fatalf("expected 1 enriched family, got %d", len(ed.Families))
	}
	fam := ed.Families[0]
	if fam.HusbandXref != "@I1@" {
		t.Errorf("expected husband @I1@, got %s", fam.HusbandXref)
	}
	if fam.WifeXref != "@I2@" {
		t.Errorf("expected wife @I2@, got %s", fam.WifeXref)
	}
	if fam.MarriageDateIndex < 0 {
		t.Error("family should have a marriage date index")
	}
	if fam.ChildrenCount != 1 {
		t.Errorf("expected 1 child, got %d", fam.ChildrenCount)
	}

	// --- Notes ---
	if ed.Stats.Notes < 2 {
		t.Errorf("expected at least 2 notes (1 top-level + inline), got %d", ed.Stats.Notes)
	}
	foundTopLevel := false
	for _, n := range ed.Notes {
		if n.IsTopLevel && n.Xref == "@N1@" {
			foundTopLevel = true
		}
	}
	if !foundTopLevel {
		t.Error("should find top-level note @N1@")
	}
	if len(ed.IndividualNotes) < 1 {
		t.Errorf("expected at least 1 individual-note link, got %d", len(ed.IndividualNotes))
	}
	if len(ed.FamilyNotes) < 1 {
		t.Errorf("expected at least 1 family-note link, got %d", len(ed.FamilyNotes))
	}

	// --- Sources ---
	if ed.Stats.Sources != 1 {
		t.Errorf("expected 1 source, got %d", ed.Stats.Sources)
	}
	if ed.Sources[0].Title != "Census Records" {
		t.Errorf("expected title 'Census Records', got %q", ed.Sources[0].Title)
	}
	if ed.Sources[0].RepositoryXref != "@R1@" {
		t.Errorf("expected repo xref @R1@, got %s", ed.Sources[0].RepositoryXref)
	}
	if len(ed.IndividualSources) < 1 {
		t.Error("expected individual-source links")
	}
	if len(ed.FamilySources) < 1 {
		t.Error("expected family-source links")
	}
	if len(ed.EventSources) < 1 {
		t.Error("expected event-source links")
	}
	// Check source link metadata
	for _, isl := range ed.IndividualSources {
		if isl.Page == "p. 100" && isl.Quality != 0 {
			t.Errorf("individual source link quality: expected 0, got %d", isl.Quality)
		}
	}

	// --- Repositories ---
	if ed.Stats.Repositories != 1 {
		t.Errorf("expected 1 repository, got %d", ed.Stats.Repositories)
	}
	repo := ed.Repositories[0]
	if repo.Name != "National Archives" {
		t.Errorf("expected repo name 'National Archives', got %q", repo.Name)
	}
	if repo.City != "Washington" {
		t.Errorf("expected city Washington, got %q", repo.City)
	}
	if repo.Email != "archives@example.com" {
		t.Errorf("expected email, got %q", repo.Email)
	}
	if len(ed.SourceRepositories) != 1 {
		t.Errorf("expected 1 source-repo link, got %d", len(ed.SourceRepositories))
	}
	if ed.SourceRepositories[0].CallNumber != "123-A" {
		t.Errorf("expected call number '123-A', got %q", ed.SourceRepositories[0].CallNumber)
	}

	// --- Media ---
	if ed.Stats.Media < 1 {
		t.Errorf("expected at least 1 media object, got %d", ed.Stats.Media)
	}
	foundMedia := false
	for _, m := range ed.Media {
		if m.Title == "Family Photo" {
			foundMedia = true
			if m.File != "photo.jpg" {
				t.Errorf("expected file 'photo.jpg', got %q", m.File)
			}
		}
	}
	if !foundMedia {
		t.Error("should find media 'Family Photo'")
	}
	if len(ed.IndividualMedia) < 1 {
		t.Error("expected individual-media links")
	}
	if len(ed.EventMedia) < 1 {
		t.Errorf("expected event-level media (OBJE under BIRT), got %d", len(ed.EventMedia))
	}
	if len(ed.EventSources) < 1 {
		t.Errorf("expected event-level sources (SOUR under BIRT), got %d", len(ed.EventSources))
	}

	// --- Relationship edges ---
	if len(ed.Spouses) != 2 {
		t.Errorf("expected 2 spouse edges, got %d", len(ed.Spouses))
	}
	if len(ed.ParentChild) != 2 {
		t.Errorf("expected 2 parent-child edges, got %d", len(ed.ParentChild))
	}
	if len(ed.FamilyChildren) != 1 {
		t.Errorf("expected 1 family-child edge, got %d", len(ed.FamilyChildren))
	}
}

// --- Nested note extraction test ---

func TestEnrichNestedNotes(t *testing.T) {
	input := `0 HEAD
1 GEDC
2 VERS 5.5
0 @N1@ NOTE Top-level note one.
0 @N2@ NOTE Top-level note two.
0 @I1@ INDI
1 NAME John /Doe/
2 NOTE @N1@
1 RESI
2 ADDR 123 Main St
3 NOTE Address remark.
1 NOTE @N2@
0 @F1@ FAM
1 HUSB @I1@
1 MARR
2 PLAC Boston
2 NOTE @N1@
0 @S1@ SOUR
1 TITL Census
1 DATA
2 NOTE Source data note.
0 TRLR
`
	doc, _, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	ed := Enrich(doc)

	// @I1@ links to: @N1@ (nested under NAME) and @N2@ (direct).
	// The inline "Address remark." under RESI > ADDR is NOT here because
	// RESI is an event tag (skipped by Phase 2, handled by Phase 4).
	if len(ed.IndividualNotes) != 2 {
		t.Errorf("expected 2 individual-note links, got %d", len(ed.IndividualNotes))
		for i, link := range ed.IndividualNotes {
			t.Logf("  [%d] xref=%s noteIdx=%d content=%q",
				i, link.IndividualXref, link.NoteIndex, ed.Notes[link.NoteIndex].Content)
		}
	}

	// Family-level scan skips event tags, so FamilyNotes should be 0.
	if len(ed.FamilyNotes) != 0 {
		t.Errorf("expected 0 family-note links (event notes go to EventNotes), got %d", len(ed.FamilyNotes))
	}

	// Event notes: MARR has NOTE @N1@, RESI has nested "Address remark."
	if len(ed.EventNotes) != 2 {
		t.Errorf("expected 2 event-note links, got %d", len(ed.EventNotes))
	}

	// Source @S1@ has an inline note nested under DATA.
	if len(ed.SourceNotes) != 1 {
		t.Errorf("expected 1 source-note link, got %d", len(ed.SourceNotes))
	}
}

func TestEnrichMixedParentageFamcNote(t *testing.T) {
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
	ed := Enrich(doc)
	if len(ed.ParentChild) != 2 {
		t.Fatalf("parent_child=%d want 2", len(ed.ParentChild))
	}
	var father, mother *ParentChildEdge
	for i := range ed.ParentChild {
		pc := &ed.ParentChild[i]
		switch pc.ParentType {
		case "father":
			father = pc
		case "mother":
			mother = pc
		}
	}
	if father == nil || mother == nil {
		t.Fatal("missing father or mother edge")
	}
	if father.RelationshipType != "biological" || father.Pedigree != "birth" {
		t.Errorf("father: rel=%q ped=%q", father.RelationshipType, father.Pedigree)
	}
	if mother.RelationshipType != "adopted" || mother.Pedigree != "adopted" {
		t.Errorf("mother: rel=%q ped=%q", mother.RelationshipType, mother.Pedigree)
	}
}

func TestEnrichGEDCOM55EventWhitelist(t *testing.T) {
	// GEDCOM 5.5: sample INDIVIDUAL_ATTRIBUTE_STRUCTURE (FACT), LDS (BAPL),
	// FAMILY_EVENT_STRUCTURE (CENS, RESI), and LDS_SPOUSE_SEALING (SLGS).
	input := `0 HEAD
1 GEDC
2 VERS 5.5
0 @I1@ INDI
1 NAME John /Doe/
1 BIRT
2 DATE 1 JAN 1950
1 FACT
2 TYPE Military service
2 DATE 1 JUN 1941
1 BAPL
2 DATE 2 JUN 1955
2 TEMP LOGAN
1 FAMS @F1@
0 @I2@ INDI
1 NAME Jane /Doe/
1 FAMS @F1@
0 @F1@ FAM
1 HUSB @I1@
1 WIFE @I2@
1 CENS
2 DATE 1880
1 RESI
2 PLAC Salt Lake City, Utah, USA
1 SLGS
2 DATE 3 JUL 1960
0 TRLR
`
	doc, _, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	ed := Enrich(doc)

	if len(ed.Events) != 6 {
		t.Fatalf("expected 6 events (BIRT+FACT+BAPL + CENS+RESI+SLGS), got %d", len(ed.Events))
	}
	if len(ed.IndividualEvents) != 3 {
		t.Errorf("expected 3 individual-event links, got %d", len(ed.IndividualEvents))
	}
	if len(ed.FamilyEvents) != 3 {
		t.Errorf("expected 3 family-event links, got %d", len(ed.FamilyEvents))
	}

	wantTypes := map[string]bool{
		"BIRT": true, "FACT": true, "BAPL": true,
		"CENS": true, "RESI": true, "SLGS": true,
	}
	for _, evt := range ed.Events {
		if !wantTypes[evt.EventType] {
			t.Errorf("unexpected event type %q", evt.EventType)
		}
		delete(wantTypes, evt.EventType)
	}
	if len(wantTypes) != 0 {
		t.Errorf("missing event types: %v", wantTypes)
	}
}

func TestEnrichAssociations(t *testing.T) {
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

	ed := Enrich(doc)
	if len(ed.Associates) != 3 {
		t.Fatalf("expected 3 associates, got %d", len(ed.Associates))
	}

	var foundIndividual, foundEvent, foundFamily bool
	for _, asso := range ed.Associates {
		if asso.OwnerXref == "@I1@" && asso.OwnerType == "INDI" && asso.OwnerEventType == "" && asso.AssociateXref == "@I2@" && asso.Relationship == "Godfather" {
			foundIndividual = true
		}
		if asso.OwnerXref == "@I1@" && asso.OwnerType == "INDI" && asso.OwnerEventType == "BIRT" && asso.AssociateXref == "@I3@" && asso.Relationship == "Witness" {
			foundEvent = true
		}
		if asso.OwnerXref == "@F1@" && asso.OwnerType == "FAM" && asso.OwnerEventType == "" && asso.AssociateXref == "@I2@" && asso.Relationship == "Neighbor" {
			foundFamily = true
		}
	}
	if !foundIndividual || !foundEvent || !foundFamily {
		t.Fatalf("missing expected association edges: individual=%v event=%v family=%v", foundIndividual, foundEvent, foundFamily)
	}
}

func TestEnrichNestedNoteDedup(t *testing.T) {
	input := `0 HEAD
1 GEDC
2 VERS 5.5
0 @N1@ NOTE Shared note.
0 @I1@ INDI
1 NAME Jane /Doe/
2 NOTE @N1@
1 NOTE @N1@
0 TRLR
`
	doc, _, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	ed := Enrich(doc)

	// Same note referenced twice on the same individual should be deduped.
	if len(ed.IndividualNotes) != 1 {
		t.Errorf("expected 1 individual-note link (deduped), got %d", len(ed.IndividualNotes))
	}
}

// --- UUID generation test ---

func TestGenerateIDs(t *testing.T) {
	input := `0 HEAD
1 GEDC
2 VERS 5.5
0 @I1@ INDI
1 NAME John /Doe/
1 SEX M
1 BIRT
2 DATE 1 JAN 1950
0 @F1@ FAM
1 HUSB @I1@
0 TRLR
`
	doc, _, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	ed := Enrich(doc)

	// Before GenerateIDs, all IDs should be empty
	if ed.Individuals[0].ID != "" {
		t.Error("ID should be empty before GenerateIDs")
	}
	if len(ed.Dates) > 0 && ed.Dates[0].ID != "" {
		t.Error("date ID should be empty before GenerateIDs")
	}

	GenerateIDs(ed)

	// After GenerateIDs, all IDs should be populated
	if ed.Individuals[0].ID == "" {
		t.Error("individual ID should be populated after GenerateIDs")
	}
	if len(ed.Dates) > 0 && ed.Dates[0].ID == "" {
		t.Error("date ID should be populated after GenerateIDs")
	}
	if len(ed.Events) > 0 && ed.Events[0].ID == "" {
		t.Error("event ID should be populated after GenerateIDs")
	}

	// IDs should be unique
	ids := make(map[string]bool)
	for _, indi := range ed.Individuals {
		if ids[indi.ID] {
			t.Errorf("duplicate ID: %s", indi.ID)
		}
		ids[indi.ID] = true
	}
	for _, d := range ed.Dates {
		if ids[d.ID] {
			t.Errorf("duplicate ID: %s", d.ID)
		}
		ids[d.ID] = true
	}
}

// --- Multiple families test ---

func TestEnrichMultipleFamilies(t *testing.T) {
	input := `0 HEAD
1 GEDC
2 VERS 5.5
0 @I1@ INDI
1 NAME John /Doe/
1 SEX M
1 FAMS @F1@
1 FAMS @F2@
0 @I2@ INDI
1 NAME Jane /Smith/
1 SEX F
1 FAMS @F1@
0 @I3@ INDI
1 NAME Mary /Jones/
1 SEX F
1 FAMS @F2@
0 @I4@ INDI
1 NAME Child1 /Doe/
1 FAMC @F1@
0 @I5@ INDI
1 NAME Child2 /Doe/
1 FAMC @F2@
0 @F1@ FAM
1 HUSB @I1@
1 WIFE @I2@
1 CHIL @I4@
0 @F2@ FAM
1 HUSB @I1@
1 WIFE @I3@
1 CHIL @I5@
0 TRLR
`
	doc, _, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	ed := Enrich(doc)

	if len(ed.Spouses) != 4 {
		t.Errorf("expected 4 spouse edges, got %d", len(ed.Spouses))
	}
	if len(ed.ParentChild) != 4 {
		t.Errorf("expected 4 parent-child edges, got %d", len(ed.ParentChild))
	}
	if len(ed.FamilyChildren) != 2 {
		t.Errorf("expected 2 family-child edges, got %d", len(ed.FamilyChildren))
	}
	if len(ed.Individuals) != 5 {
		t.Errorf("expected 5 enriched individuals, got %d", len(ed.Individuals))
	}
	if len(ed.Families) != 2 {
		t.Errorf("expected 2 enriched families, got %d", len(ed.Families))
	}
}

// --- Real file tests ---

func TestEnrichRealFiles(t *testing.T) {
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

			ed := Enrich(doc)

			t.Logf("%s enriched:", name)
			t.Logf("  individuals=%d families=%d", ed.Stats.Individuals, ed.Stats.Families)
			t.Logf("  dates=%d places=%d surnames=%d given_names=%d",
				ed.Stats.Dates, ed.Stats.Places, ed.Stats.Surnames, ed.Stats.GivenNames)
			t.Logf("  events=%d notes=%d sources=%d repos=%d media=%d",
				ed.Stats.Events, ed.Stats.Notes, ed.Stats.Sources, ed.Stats.Repositories, ed.Stats.Media)
			t.Logf("  spouse_edges=%d parent_child=%d family_child=%d",
				len(ed.Spouses), len(ed.ParentChild), len(ed.FamilyChildren))
			t.Logf("  note_links: indi=%d fam=%d event=%d source=%d",
				len(ed.IndividualNotes), len(ed.FamilyNotes), len(ed.EventNotes), len(ed.SourceNotes))
			t.Logf("  source_links: indi=%d fam=%d event=%d",
				len(ed.IndividualSources), len(ed.FamilySources), len(ed.EventSources))
			t.Logf("  repo_links=%d media_links: indi=%d fam=%d source=%d",
				len(ed.SourceRepositories), len(ed.IndividualMedia), len(ed.FamilyMedia), len(ed.SourceMedia))

			// Invariants
			if ed.Stats.Individuals != doc.IndividualCount() {
				t.Errorf("individual count mismatch")
			}
			if ed.Stats.Families != doc.FamilyCount() {
				t.Errorf("family count mismatch")
			}
			if len(ed.Individuals) != doc.IndividualCount() {
				t.Errorf("enriched individual count mismatch: %d vs %d",
					len(ed.Individuals), doc.IndividualCount())
			}
			if len(ed.Families) != doc.FamilyCount() {
				t.Errorf("enriched family count mismatch: %d vs %d",
					len(ed.Families), doc.FamilyCount())
			}

			for _, evt := range ed.Events {
				if evt.DateIndex >= len(ed.Dates) {
					t.Errorf("event %d has date index %d but only %d dates",
						evt.Index, evt.DateIndex, len(ed.Dates))
				}
				if evt.PlaceIndex >= len(ed.Places) {
					t.Errorf("event %d has place index %d but only %d places",
						evt.Index, evt.PlaceIndex, len(ed.Places))
				}
			}

			if len(ed.Spouses)%2 != 0 {
				t.Error("spouse edges should be even (bidirectional)")
			}

			for _, pc := range ed.ParentChild {
				if pc.ParentType != "father" && pc.ParentType != "mother" {
					t.Errorf("invalid parent type %q", pc.ParentType)
				}
			}

			// UUID generation should work without panics
			GenerateIDs(ed)
			for _, indi := range ed.Individuals {
				if indi.ID == "" {
					t.Error("individual ID empty after GenerateIDs")
					break
				}
			}
		})
	}
}
