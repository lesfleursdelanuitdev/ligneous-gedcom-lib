package gedcom

import "testing"

func TestRecordsByXrefAgreesWithFindByXref(t *testing.T) {
	doc := &GedcomDocument{
		Individuals:  []GedcomRecord{{Tag: "INDI", Xref: "@A@", Value: "indi"}, {Tag: "INDI", Xref: "@A@", Value: "indi-dup"}, {Tag: "INDI"}},
		Families:     []GedcomRecord{{Tag: "FAM", Xref: "@A@", Value: "fam"}, {Tag: "FAM", Xref: "@F@"}},
		Sources:      []GedcomRecord{{Tag: "SOUR", Xref: "@S@"}, {Tag: "SOUR", Xref: "@F@", Value: "src"}},
		Notes:        []GedcomRecord{{Tag: "NOTE", Xref: "@N@"}},
		Repositories: []GedcomRecord{{Tag: "REPO", Xref: "@R@"}, {Tag: "REPO", Xref: "@N@", Value: "repo"}},
		Media:        []GedcomRecord{{Tag: "OBJE", Xref: "@M@"}},
		Submitters:   []GedcomRecord{{Tag: "SUBM", Xref: "@U@"}, {Tag: "SUBM", Xref: "@M@", Value: "subm"}},
	}
	idx := doc.RecordsByXref()
	for _, x := range []string{"@A@", "@F@", "@S@", "@N@", "@R@", "@M@", "@U@", "", "@missing@"} {
		if got, want := idx[x], doc.FindByXref(x); got != want {
			t.Errorf("xref %q: index %p, FindByXref %p", x, got, want)
		}
	}
	if idx["@A@"].Value != "indi" {
		t.Errorf("first record must win, got %q", idx["@A@"].Value)
	}
}
