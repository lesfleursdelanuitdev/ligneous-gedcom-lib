package validator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/gedcom"
	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
)

func TestValidateMinimalDoc(t *testing.T) {
	doc := &gedcom.GedcomDocument{
		Header:  gedcom.NewRecord(0, "HEAD"),
		Trailer: gedcom.NewRecord(0, "TRLR"),
		Individuals: []gedcom.GedcomRecord{
			{
				Level: 0, Tag: "INDI", Xref: "@I1@",
				Children: []gedcom.GedcomRecord{
					{Level: 1, Tag: "NAME", Value: "John /Doe/"},
					{Level: 1, Tag: "SEX", Value: "M"},
				},
			},
		},
	}

	errs := Validate(doc)
	if len(errs) != 0 {
		for _, e := range errs {
			t.Errorf("unexpected: %v", e)
		}
	}
}

func TestValidateMissingHeader(t *testing.T) {
	doc := &gedcom.GedcomDocument{
		Trailer: gedcom.NewRecord(0, "TRLR"),
	}

	errs := Validate(doc)
	found := false
	for _, e := range errs {
		if e.Code == "MISSING_HEADER" {
			found = true
		}
	}
	if !found {
		t.Error("expected MISSING_HEADER error")
	}
}

func TestValidateMissingTrailer(t *testing.T) {
	doc := &gedcom.GedcomDocument{
		Header: gedcom.NewRecord(0, "HEAD"),
	}

	errs := Validate(doc)
	found := false
	for _, e := range errs {
		if e.Code == "MISSING_TRAILER" {
			found = true
		}
	}
	if !found {
		t.Error("expected MISSING_TRAILER error")
	}
}

func TestValidateMissingName(t *testing.T) {
	doc := &gedcom.GedcomDocument{
		Header:  gedcom.NewRecord(0, "HEAD"),
		Trailer: gedcom.NewRecord(0, "TRLR"),
		Individuals: []gedcom.GedcomRecord{
			{Level: 0, Tag: "INDI", Xref: "@I1@"},
		},
	}

	errs := Validate(doc)
	found := false
	for _, e := range errs {
		if e.Code == "MISSING_NAME" {
			found = true
		}
	}
	if !found {
		t.Error("expected MISSING_NAME warning")
	}
}

func TestValidateInvalidSex(t *testing.T) {
	doc := &gedcom.GedcomDocument{
		Header:  gedcom.NewRecord(0, "HEAD"),
		Trailer: gedcom.NewRecord(0, "TRLR"),
		Individuals: []gedcom.GedcomRecord{
			{
				Level: 0, Tag: "INDI", Xref: "@I1@",
				Children: []gedcom.GedcomRecord{
					{Level: 1, Tag: "NAME", Value: "John /Doe/"},
					{Level: 1, Tag: "SEX", Value: "Z"},
				},
			},
		},
	}

	errs := Validate(doc)
	found := false
	for _, e := range errs {
		if e.Code == "INVALID_SEX" {
			found = true
		}
	}
	if !found {
		t.Error("expected INVALID_SEX warning")
	}
}

func TestValidateEmptyFamily(t *testing.T) {
	doc := &gedcom.GedcomDocument{
		Header:  gedcom.NewRecord(0, "HEAD"),
		Trailer: gedcom.NewRecord(0, "TRLR"),
		Families: []gedcom.GedcomRecord{
			{Level: 0, Tag: "FAM", Xref: "@F1@"},
		},
	}

	errs := Validate(doc)
	found := false
	for _, e := range errs {
		if e.Code == "EMPTY_FAMILY" {
			found = true
		}
	}
	if !found {
		t.Error("expected EMPTY_FAMILY warning")
	}
}

func TestValidateBrokenXRef(t *testing.T) {
	doc := &gedcom.GedcomDocument{
		Header:  gedcom.NewRecord(0, "HEAD"),
		Trailer: gedcom.NewRecord(0, "TRLR"),
		Families: []gedcom.GedcomRecord{
			{
				Level: 0, Tag: "FAM", Xref: "@F1@",
				Children: []gedcom.GedcomRecord{
					{Level: 1, Tag: "HUSB", Value: "@I999@"},
				},
			},
		},
	}

	errs := Validate(doc)
	found := false
	for _, e := range errs {
		if e.Code == CodeOrphanedHusb {
			found = true
		}
	}
	if !found {
		t.Error("expected ORPHANED_HUSB error")
	}
}

func TestValidateAssociations(t *testing.T) {
	doc := &gedcom.GedcomDocument{
		Header:  gedcom.NewRecord(0, "HEAD"),
		Trailer: gedcom.NewRecord(0, "TRLR"),
		Individuals: []gedcom.GedcomRecord{
			{
				Level: 0, Tag: "INDI", Xref: "@I1@",
				Children: []gedcom.GedcomRecord{
					{Level: 1, Tag: "NAME", Value: "John /Doe/"},
					{
						Level: 1, Tag: "ASSO", Value: "@I2@",
						Children: []gedcom.GedcomRecord{
							{Level: 2, Tag: "RELA", Value: "Witness"},
						},
					},
				},
			},
			{
				Level: 0, Tag: "INDI", Xref: "@I2@",
				Children: []gedcom.GedcomRecord{
					{Level: 1, Tag: "NAME", Value: "Jane /Doe/"},
				},
			},
			{
				Level: 0, Tag: "INDI", Xref: "@I3@",
				Children: []gedcom.GedcomRecord{
					{Level: 1, Tag: "NAME", Value: "Bad /Assoc/"},
					{Level: 1, Tag: "ASSO", Value: "@I999@"},
				},
			},
		},
	}

	errs := Validate(doc)
	var hasBrokenAssociate, hasMissingRela bool
	for _, e := range errs {
		if e.Code == CodeOrphanedAssociateXref {
			hasBrokenAssociate = true
		}
		if e.Code == CodeMissingAssociateRela {
			hasMissingRela = true
		}
	}
	if !hasBrokenAssociate {
		t.Error("expected ORPHANED_ASSOCIATE_XREF")
	}
	if !hasMissingRela {
		t.Error("expected MISSING_ASSOCIATE_RELA")
	}
}

func TestValidateDateConsistency(t *testing.T) {
	doc := &gedcom.GedcomDocument{
		Header:  gedcom.NewRecord(0, "HEAD"),
		Trailer: gedcom.NewRecord(0, "TRLR"),
		Individuals: []gedcom.GedcomRecord{
			{
				Level: 0, Tag: "INDI", Xref: "@I1@",
				Children: []gedcom.GedcomRecord{
					{Level: 1, Tag: "NAME", Value: "John /Doe/"},
					{Level: 1, Tag: "BIRT", Children: []gedcom.GedcomRecord{
						{Level: 2, Tag: "DATE", Value: "1 JAN 2000"},
					}},
					{Level: 1, Tag: "DEAT", Children: []gedcom.GedcomRecord{
						{Level: 2, Tag: "DATE", Value: "1 JAN 1990"},
					}},
				},
			},
		},
	}

	opts := DefaultOptions()
	opts.DateConsistency = true
	errs := ValidateWithOptions(doc, opts)

	found := false
	for _, e := range errs {
		if e.Code == "DEATH_BEFORE_BIRTH" {
			found = true
		}
	}
	if !found {
		t.Error("expected DEATH_BEFORE_BIRTH error")
	}
}

func TestValidateSwappedOppositeSexSpouses(t *testing.T) {
	doc := &gedcom.GedcomDocument{
		Header:  gedcom.NewRecord(0, "HEAD"),
		Trailer: gedcom.NewRecord(0, "TRLR"),
		Individuals: []gedcom.GedcomRecord{
			{
				Level: 0, Tag: "INDI", Xref: "@I1@",
				Children: []gedcom.GedcomRecord{
					{Level: 1, Tag: "NAME", Value: "Jane /Roe/"},
					{Level: 1, Tag: "SEX", Value: "F"},
				},
			},
			{
				Level: 0, Tag: "INDI", Xref: "@I2@",
				Children: []gedcom.GedcomRecord{
					{Level: 1, Tag: "NAME", Value: "John /Roe/"},
					{Level: 1, Tag: "SEX", Value: "M"},
				},
			},
		},
		Families: []gedcom.GedcomRecord{
			{
				Level: 0, Tag: "FAM", Xref: "@F1@",
				Children: []gedcom.GedcomRecord{
					{Level: 1, Tag: "HUSB", Value: "@I1@"},
					{Level: 1, Tag: "WIFE", Value: "@I2@"},
				},
			},
		},
	}
	var found bool
	for _, e := range Validate(doc) {
		if e.Code == CodeSwappedOppositeSexSpouses {
			found = true
			if e.Xref != "@F1@" {
				t.Errorf("expected family xref @F1@, got %q", e.Xref)
			}
			want := []string{"@F1@", "@I1@", "@I2@"}
			if len(e.AssociatedXrefs) != len(want) {
				t.Fatalf("AssociatedXrefs len got %d want %d: %#v", len(e.AssociatedXrefs), len(want), e.AssociatedXrefs)
			}
			for i := range want {
				if e.AssociatedXrefs[i] != want[i] {
					t.Fatalf("AssociatedXrefs[%d] got %q want %q (full %#v)", i, e.AssociatedXrefs[i], want[i], e.AssociatedXrefs)
				}
			}
		}
	}
	if !found {
		t.Fatal("expected SWAPPED_OPPOSITE_SEX_SPOUSE_TAGS")
	}
}

func TestValidateNoSwappedWarningForSameSex(t *testing.T) {
	doc := &gedcom.GedcomDocument{
		Header:  gedcom.NewRecord(0, "HEAD"),
		Trailer: gedcom.NewRecord(0, "TRLR"),
		Individuals: []gedcom.GedcomRecord{
			{
				Level: 0, Tag: "INDI", Xref: "@I1@",
				Children: []gedcom.GedcomRecord{
					{Level: 1, Tag: "NAME", Value: "A /One/"},
					{Level: 1, Tag: "SEX", Value: "F"},
				},
			},
			{
				Level: 0, Tag: "INDI", Xref: "@I2@",
				Children: []gedcom.GedcomRecord{
					{Level: 1, Tag: "NAME", Value: "B /Two/"},
					{Level: 1, Tag: "SEX", Value: "F"},
				},
			},
		},
		Families: []gedcom.GedcomRecord{
			{
				Level: 0, Tag: "FAM", Xref: "@F1@",
				Children: []gedcom.GedcomRecord{
					{Level: 1, Tag: "HUSB", Value: "@I1@"},
					{Level: 1, Tag: "WIFE", Value: "@I2@"},
				},
			},
		},
	}
	for _, e := range Validate(doc) {
		if e.Code == CodeSwappedOppositeSexSpouses {
			t.Fatalf("unexpected swap warning for same-sex F/F couple")
		}
	}
}

func TestValidateChildBornBeforeFather(t *testing.T) {
	doc := &gedcom.GedcomDocument{
		Header:  gedcom.NewRecord(0, "HEAD"),
		Trailer: gedcom.NewRecord(0, "TRLR"),
		Individuals: []gedcom.GedcomRecord{
			{
				Level: 0, Tag: "INDI", Xref: "@I1@",
				Children: []gedcom.GedcomRecord{
					{Level: 1, Tag: "NAME", Value: "Child /X/"},
					{Level: 1, Tag: "BIRT", Children: []gedcom.GedcomRecord{{Level: 2, Tag: "DATE", Value: "1900"}}},
				},
			},
			{
				Level: 0, Tag: "INDI", Xref: "@I2@",
				Children: []gedcom.GedcomRecord{
					{Level: 1, Tag: "NAME", Value: "Father /X/"},
					{Level: 1, Tag: "BIRT", Children: []gedcom.GedcomRecord{{Level: 2, Tag: "DATE", Value: "1910"}}},
				},
			},
		},
		Families: []gedcom.GedcomRecord{
			{
				Level: 0, Tag: "FAM", Xref: "@F1@",
				Children: []gedcom.GedcomRecord{
					{Level: 1, Tag: "HUSB", Value: "@I2@"},
					{Level: 1, Tag: "CHIL", Value: "@I1@"},
				},
			},
		},
	}
	var found bool
	for _, e := range Validate(doc) {
		if e.Code == CodeChildBornBeforeFather {
			found = true
			want := []string{"@I1@", "@I2@", "@F1@"}
			if len(e.AssociatedXrefs) != len(want) {
				t.Fatalf("AssociatedXrefs len got %d want %d: %#v", len(e.AssociatedXrefs), len(want), e.AssociatedXrefs)
			}
			for i := range want {
				if e.AssociatedXrefs[i] != want[i] {
					t.Fatalf("AssociatedXrefs[%d] got %q want %q", i, e.AssociatedXrefs[i], want[i])
				}
			}
		}
	}
	if !found {
		t.Fatal("expected CHILD_BORN_BEFORE_FATHER warning")
	}
}

func TestMotherTooYoungAssociatedXrefsOrder(t *testing.T) {
	doc := &gedcom.GedcomDocument{
		Header:  gedcom.NewRecord(0, "HEAD"),
		Trailer: gedcom.NewRecord(0, "TRLR"),
		Individuals: []gedcom.GedcomRecord{
			{
				Level: 0, Tag: "INDI", Xref: "@I1@",
				Children: []gedcom.GedcomRecord{
					{Level: 1, Tag: "NAME", Value: "Child /X/"},
					{Level: 1, Tag: "BIRT", Children: []gedcom.GedcomRecord{{Level: 2, Tag: "DATE", Value: "2000"}}},
				},
			},
			{
				Level: 0, Tag: "INDI", Xref: "@I2@",
				Children: []gedcom.GedcomRecord{
					{Level: 1, Tag: "NAME", Value: "Father /X/"},
					{Level: 1, Tag: "BIRT", Children: []gedcom.GedcomRecord{{Level: 2, Tag: "DATE", Value: "1970"}}},
				},
			},
			{
				Level: 0, Tag: "INDI", Xref: "@I3@",
				Children: []gedcom.GedcomRecord{
					{Level: 1, Tag: "NAME", Value: "Mother /X/"},
					{Level: 1, Tag: "BIRT", Children: []gedcom.GedcomRecord{{Level: 2, Tag: "DATE", Value: "1995"}}},
				},
			},
		},
		Families: []gedcom.GedcomRecord{
			{
				Level: 0, Tag: "FAM", Xref: "@F1@",
				Children: []gedcom.GedcomRecord{
					{Level: 1, Tag: "HUSB", Value: "@I2@"},
					{Level: 1, Tag: "WIFE", Value: "@I3@"},
					{Level: 1, Tag: "CHIL", Value: "@I1@"},
				},
			},
		},
	}
	var target *ValidationError
	for _, e := range Validate(doc) {
		if e.Code == CodeMotherTooYoungAtBirth {
			target = e
			break
		}
	}
	if target == nil {
		t.Fatal("expected MOTHER_TOO_YOUNG_AT_CHILD_BIRTH warning")
	}
	want := []string{"@I1@", "@I3@", "@F1@"}
	if len(target.AssociatedXrefs) != len(want) {
		t.Fatalf("AssociatedXrefs len got %d want %d: %#v", len(target.AssociatedXrefs), len(want), target.AssociatedXrefs)
	}
	for i := range want {
		if target.AssociatedXrefs[i] != want[i] {
			t.Fatalf("AssociatedXrefs[%d] got %q want %q (full %#v)", i, target.AssociatedXrefs[i], want[i], target.AssociatedXrefs)
		}
	}
}

func TestValidateOrphanedFamcCode(t *testing.T) {
	doc := &gedcom.GedcomDocument{
		Header:  gedcom.NewRecord(0, "HEAD"),
		Trailer: gedcom.NewRecord(0, "TRLR"),
		Individuals: []gedcom.GedcomRecord{
			{
				Level: 0, Tag: "INDI", Xref: "@I1@",
				Children: []gedcom.GedcomRecord{
					{Level: 1, Tag: "NAME", Value: "John /Doe/"},
					{Level: 1, Tag: "FAMC", Value: "@F999@"},
				},
			},
		},
	}
	var found bool
	for _, e := range Validate(doc) {
		if e.Code == CodeOrphanedFamc {
			found = true
			if e.RelatedXref != "@F999@" {
				t.Errorf("RelatedXref = %q", e.RelatedXref)
			}
			got := uniqueXrefsInOrder(e.AssociatedXrefs...)
			want := uniqueXrefsInOrder("@I1@", "@F999@")
			if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
				t.Fatalf("AssociatedXrefs got %#v want %#v", e.AssociatedXrefs, want)
			}
		}
	}
	if !found {
		t.Fatal("expected ORPHANED_FAMC")
	}
}

func TestValidateAgeAtDeathExceeds120Code(t *testing.T) {
	doc := &gedcom.GedcomDocument{
		Header:  gedcom.NewRecord(0, "HEAD"),
		Trailer: gedcom.NewRecord(0, "TRLR"),
		Individuals: []gedcom.GedcomRecord{
			{
				Level: 0, Tag: "INDI", Xref: "@I1@",
				Children: []gedcom.GedcomRecord{
					{Level: 1, Tag: "NAME", Value: "Old /Person/"},
					{Level: 1, Tag: "BIRT", Children: []gedcom.GedcomRecord{{Level: 2, Tag: "DATE", Value: "1800"}}},
					{Level: 1, Tag: "DEAT", Children: []gedcom.GedcomRecord{{Level: 2, Tag: "DATE", Value: "1925"}}},
				},
			},
		},
	}
	o := DefaultOptions()
	o.DateConsistency = true
	var found bool
	for _, e := range ValidateWithOptions(doc, o) {
		if e.Code == CodeAgeAtDeathExceeds120 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected AGE_AT_DEATH_EXCEEDS_120")
	}
}

func TestValidateRealFiles(t *testing.T) {
	files, err := filepath.Glob("../testdata/*.ged")
	if err != nil {
		t.Fatalf("glob error: %v", err)
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

			opts := DefaultOptions()
			opts.DateConsistency = true
			errs := ValidateWithOptions(doc, opts)

			t.Logf("%s: %d validation findings", name, len(errs))
			for _, e := range errs {
				if e.Severity == SeverityError {
					t.Logf("  [error] %s", e)
				}
			}
		})
	}
}

const lineFixture = `0 HEAD
1 GEDC
2 VERS 5.5.1
1 CHAR UTF-8
0 @I1@ INDI
1 NAME Mom /Old/
1 SEX F
1 BIRT
2 DATE 1900
1 FAMS @F1@
0 @I2@ INDI
1 NAME Kid /Old/
1 BIRT
2 DATE 1905
1 FAMC @F1@
0 @I3@ INDI
1 NAME Gone /Early/
1 BIRT
2 DATE 1900
1 DEAT
2 DATE 1850
0 @I4@ INDI
1 NAME Lost /Child/
1 FAMC @F9@
0 @I5@ INDI
1 NAME Bob
0 @I6@ INDI
1 NAME Q /Sex/
1 SEX Q
0 @I7@ INDI
1 NAME Friend /One/
1 ASSO @Z9@
2 RELA friend
0 @F1@ FAM
1 WIFE @I1@
1 CHIL @I2@
1 CHIL @I8@
0 @F2@ FAM
0 TRLR
`

func parseLenient(t *testing.T, s string) *gedcom.GedcomDocument {
	t.Helper()
	doc, _, err := parser.ParseWithOptions(strings.NewReader(s), &parser.Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestFindingLinePointsAtTag(t *testing.T) {
	lines := strings.Split(lineFixture, "\n")
	doc := parseLenient(t, lineFixture)
	errs := ValidateWithOptions(doc, &Options{DateConsistency: true})
	want := map[string]string{
		CodeMotherTooYoungAtBirth:   "DATE",
		CodeDeathBeforeBirth:        "DATE",
		CodeOrphanedFamc:            "FAMC",
		CodeNameMissingSurnameSlash: "NAME",
		CodeInvalidSex:              "SEX",
		CodeOrphanedAssociateXref:   "ASSO",
		CodeOrphanedChil:            "CHIL",
		CodeEmptyFamily:             "FAM",
	}
	seen := map[string]bool{}
	for _, e := range errs {
		tag, ok := want[e.Code]
		if !ok {
			continue
		}
		seen[e.Code] = true
		if e.Line < 1 || e.Line > len(lines) {
			t.Errorf("%s: line %d out of range", e.Code, e.Line)
			continue
		}
		if !strings.Contains(lines[e.Line-1], tag) {
			t.Errorf("%s: line %d is %q, want it to contain %s", e.Code, e.Line, lines[e.Line-1], tag)
		}
	}
	for code := range want {
		if !seen[code] {
			t.Errorf("fixture did not raise %s", code)
		}
	}
	for _, e := range errs {
		if e.Code == CodeMotherTooYoungAtBirth && e.Line != 14 {
			t.Errorf("MOTHER_TOO_YOUNG line = %d, want 14 (child's birth DATE)", e.Line)
		}
	}
}

func TestEveryFindingHasLine(t *testing.T) {
	var files []string
	for _, g := range []string{"../testdata/*.ged", "../testdata/malformed/*.ged"} {
		m, err := filepath.Glob(g)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	for _, path := range files {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			doc := parseLenient(t, string(data))
			for _, e := range ValidateWithOptions(doc, &Options{DateConsistency: true}) {
				if e.Line <= 0 {
					t.Errorf("%s (xref %q) has Line %d", e.Code, e.Xref, e.Line)
				}
			}
		})
	}
	// Findings with no record at all.
	doc := &gedcom.GedcomDocument{LineCount: 9}
	for _, e := range Validate(doc) {
		if e.Code == CodeMissingHeader && e.Line != 1 {
			t.Errorf("MISSING_HEADER line = %d, want 1", e.Line)
		}
		if e.Code == CodeMissingTrailer && e.Line != 9 {
			t.Errorf("MISSING_TRAILER line = %d, want 9", e.Line)
		}
	}
}

func TestNewDetails(t *testing.T) {
	doc := parseLenient(t, lineFixture)
	errs := ValidateWithOptions(doc, &Options{DateConsistency: true})
	find := func(code string) *ValidationError {
		for _, e := range errs {
			if e.Code == code {
				return e
			}
		}
		t.Fatalf("no %s in fixture", code)
		return nil
	}
	check := func(code, key, want string) {
		t.Helper()
		e := find(code)
		got, ok := e.Details[key]
		if !ok || (want != "" && got != want) {
			t.Errorf("%s Details[%s] = %q (present %v), want %q", code, key, got, ok, want)
		}
	}
	check(CodeDeathBeforeBirth, "death_year", "1850")
	check(CodeDeathBeforeBirth, "birth_year", "1900")
	check(CodeInvalidSex, "sex", "Q")
	check(CodeOrphanedChil, "family", "@F1@")
	check(CodeEmptyFamily, "family", "@F2@")
	check(CodeMotherTooYoungAtBirth, "family", "@F1@")
	check(CodeMotherTooYoungAtBirth, "age", "5")
}

func TestNewDetailsInvalidTargetAndAges(t *testing.T) {
	src := `0 HEAD
1 CHAR UTF-8
0 @I1@ INDI
1 NAME A /B/
1 ASSO @F1@
2 RELA x
1 BIRT
2 DATE 1700
1 DEAT
2 DATE 1850
0 @I2@ INDI
1 NAME Dad /B/
1 BIRT
2 DATE 1900
0 @I3@ INDI
1 NAME Kid /B/
1 BIRT
2 DATE 1905
0 @I4@ INDI
1 NAME Old /B/
1 BIRT
2 DATE 1800
0 @I5@ INDI
1 NAME Late /B/
1 BIRT
2 DATE 1900
0 @F1@ FAM
1 HUSB @I2@
1 CHIL @I3@
0 @F3@ FAM
1 HUSB @I4@
1 CHIL @I5@
0 TRLR
`
	errs := ValidateWithOptions(parseLenient(t, src), &Options{DateConsistency: true})
	want := map[string]string{
		CodeInvalidAssociateTarget: "",
		CodeFatherTooYoungAtBirth:  "5",
		CodeFatherTooOldAtBirth:    "100",
		CodeAgeAtDeathExceeds120:   "150",
	}
	for code, age := range want {
		var e *ValidationError
		for _, x := range errs {
			if x.Code == code {
				e = x
			}
		}
		if e == nil {
			t.Errorf("no %s", code)
			continue
		}
		if code == CodeInvalidAssociateTarget {
			if e.Details["target_tag"] != "FAM" {
				t.Errorf("target_tag = %q", e.Details["target_tag"])
			}
			continue
		}
		if e.Details["age"] != age {
			t.Errorf("%s age = %q, want %s", code, e.Details["age"], age)
		}
		if code == CodeAgeAtDeathExceeds120 && e.Details["age_years"] != age {
			t.Errorf("age_years = %q", e.Details["age_years"])
		}
	}
}
