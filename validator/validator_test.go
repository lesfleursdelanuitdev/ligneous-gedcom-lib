package validator

import (
	"os"
	"path/filepath"
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
