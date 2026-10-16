package enricher

import (
	"fmt"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/gedcom"
)

// genLargeDoc builds an in-memory document with n people. Each person has a
// BIRT and a DEAT event, a top-level NOTE reference, a SOUR citation on the
// person and on each event, and a BIRT sub-note. Two people in three share a
// family, with a child from the following person.
func genLargeDoc(n int) *gedcom.GedcomDocument {
	doc := &gedcom.GedcomDocument{}
	doc.Header = gedcom.GedcomRecord{Tag: "HEAD"}
	nSrc := n/10 + 1
	for s := 1; s <= nSrc; s++ {
		doc.Sources = append(doc.Sources, gedcom.GedcomRecord{
			Tag: "SOUR", Xref: fmt.Sprintf("@S%d@", s),
			Children: []gedcom.GedcomRecord{{Level: 1, Tag: "TITL", Value: fmt.Sprintf("Source %d", s)}},
		})
	}
	for i := 1; i <= n; i++ {
		doc.Notes = append(doc.Notes, gedcom.GedcomRecord{
			Tag: "NOTE", Xref: fmt.Sprintf("@N%d@", i), Value: fmt.Sprintf("Note text %d", i),
		})
		src := fmt.Sprintf("@S%d@", i%nSrc+1)
		ind := gedcom.GedcomRecord{
			Tag: "INDI", Xref: fmt.Sprintf("@I%d@", i),
			Children: []gedcom.GedcomRecord{
				{Level: 1, Tag: "NAME", Value: fmt.Sprintf("Given%d /Fam%d/", i, i%500)},
				{Level: 1, Tag: "SEX", Value: "M"},
				{Level: 1, Tag: "BIRT", Children: []gedcom.GedcomRecord{
					{Level: 2, Tag: "DATE", Value: fmt.Sprintf("1 JAN %d", 1800+i%100)},
					{Level: 2, Tag: "PLAC", Value: fmt.Sprintf("Town%d, County, Country", i%300)},
					{Level: 2, Tag: "NOTE", Value: "inline birth note"},
					{Level: 2, Tag: "SOUR", Value: src, Children: []gedcom.GedcomRecord{{Level: 3, Tag: "PAGE", Value: "p1"}}},
				}},
				{Level: 1, Tag: "DEAT", Children: []gedcom.GedcomRecord{
					{Level: 2, Tag: "DATE", Value: fmt.Sprintf("1 JAN %d", 1880+i%100)},
					{Level: 2, Tag: "SOUR", Value: src},
				}},
				{Level: 1, Tag: "NOTE", Value: fmt.Sprintf("@N%d@", i)},
				{Level: 1, Tag: "SOUR", Value: src},
			},
		}
		if i%3 == 1 && i+1 <= n {
			ind.Children = append(ind.Children, gedcom.GedcomRecord{Level: 1, Tag: "FAMS", Value: fmt.Sprintf("@F%d@", i)})
		}
		if i%3 == 2 {
			ind.Children = append(ind.Children, gedcom.GedcomRecord{Level: 1, Tag: "FAMS", Value: fmt.Sprintf("@F%d@", i-1)})
		}
		if i%3 == 0 && i-2 >= 1 {
			ind.Children = append(ind.Children, gedcom.GedcomRecord{Level: 1, Tag: "FAMC", Value: fmt.Sprintf("@F%d@", i-2)})
		}
		doc.Individuals = append(doc.Individuals, ind)
	}
	for i := 1; i+2 <= n; i += 3 {
		doc.Families = append(doc.Families, gedcom.GedcomRecord{
			Tag: "FAM", Xref: fmt.Sprintf("@F%d@", i),
			Children: []gedcom.GedcomRecord{
				{Level: 1, Tag: "HUSB", Value: fmt.Sprintf("@I%d@", i)},
				{Level: 1, Tag: "WIFE", Value: fmt.Sprintf("@I%d@", i+1)},
				{Level: 1, Tag: "CHIL", Value: fmt.Sprintf("@I%d@", i+2)},
				{Level: 1, Tag: "MARR", Children: []gedcom.GedcomRecord{
					{Level: 2, Tag: "DATE", Value: "1 JAN 1850"},
					{Level: 2, Tag: "SOUR", Value: fmt.Sprintf("@S%d@", i%nSrc+1)},
				}},
			},
		})
	}
	doc.Trailer = gedcom.GedcomRecord{Tag: "TRLR"}
	return doc
}
