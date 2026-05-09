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
		if e.Code == "BROKEN_XREF" {
			found = true
		}
	}
	if !found {
		t.Error("expected BROKEN_XREF error")
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
		if e.Code == "BROKEN_ASSOCIATE_XREF" {
			hasBrokenAssociate = true
		}
		if e.Code == "MISSING_ASSOCIATE_RELA" {
			hasMissingRela = true
		}
	}
	if !hasBrokenAssociate {
		t.Error("expected BROKEN_ASSOCIATE_XREF")
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
