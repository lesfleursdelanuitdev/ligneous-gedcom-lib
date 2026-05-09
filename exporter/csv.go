package exporter

import (
	"encoding/csv"
	"io"
	"strings"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/gedcom"
)

// CSVHeader is the column header for CSV export.
// Rows with Type=INDI are individuals; Type=OBJE are multimedia records (file + inline note as description).
var CSVHeader = []string{
	"XREF", "Type", "Name", "Sex", "Birth Date", "Birth Place",
	"Death Date", "Death Place", "Father XREF", "Mother XREF",
	"Spouse XREFs", "Children XREFs", "Notes",
	"Media_File", "Media_Form", "Media_Title", "Media_Description",
}

// ToCSV writes individuals from a GedcomDocument to a CSV writer.
func ToCSV(w io.Writer, doc *gedcom.GedcomDocument) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	if err := cw.Write(CSVHeader); err != nil {
		return err
	}

	idx := doc.XRefIndex()

	for _, indi := range doc.Individuals {
		row := individualToCSVRow(indi, doc, idx)
		if err := cw.Write(row); err != nil {
			return err
		}
	}

	for _, obje := range doc.Media {
		row := mediaObjeToCSVRow(obje)
		if err := cw.Write(row); err != nil {
			return err
		}
	}

	return cw.Error()
}

// ToCSVString returns the CSV export as a string.
func ToCSVString(doc *gedcom.GedcomDocument) (string, error) {
	var sb strings.Builder
	if err := ToCSV(&sb, doc); err != nil {
		return "", err
	}
	return sb.String(), nil
}

func individualToCSVRow(indi gedcom.GedcomRecord, doc *gedcom.GedcomDocument, idx map[string]*gedcom.GedcomRecord) []string {
	name := indi.ChildValue("NAME")
	sex := indi.ChildValue("SEX")

	var birthDate, birthPlace, deathDate, deathPlace string

	birthRecs := indi.ChildrenByTag("BIRT")
	if len(birthRecs) > 0 {
		birthDate = birthRecs[0].ChildValue("DATE")
		birthPlace = birthRecs[0].ChildValue("PLAC")
	}

	deathRecs := indi.ChildrenByTag("DEAT")
	if len(deathRecs) > 0 {
		deathDate = deathRecs[0].ChildValue("DATE")
		deathPlace = deathRecs[0].ChildValue("PLAC")
	}

	var fatherXref, motherXref string
	var spouseXrefs, childrenXrefs []string

	for _, fam := range doc.Families {
		// Check if this individual is a child in this family
		isChild := false
		for _, chil := range fam.ChildrenByTag("CHIL") {
			if chil.Value == indi.Xref {
				isChild = true
				break
			}
		}
		if isChild {
			husbRecs := fam.ChildrenByTag("HUSB")
			if len(husbRecs) > 0 {
				fatherXref = husbRecs[0].Value
			}
			wifeRecs := fam.ChildrenByTag("WIFE")
			if len(wifeRecs) > 0 {
				motherXref = wifeRecs[0].Value
			}
		}

		// Check if this individual is a spouse in this family
		isSpouse := false
		husbRecs := fam.ChildrenByTag("HUSB")
		wifeRecs := fam.ChildrenByTag("WIFE")
		if len(husbRecs) > 0 && husbRecs[0].Value == indi.Xref {
			isSpouse = true
			if len(wifeRecs) > 0 {
				spouseXrefs = append(spouseXrefs, wifeRecs[0].Value)
			}
		}
		if len(wifeRecs) > 0 && wifeRecs[0].Value == indi.Xref {
			isSpouse = true
			if len(husbRecs) > 0 {
				spouseXrefs = append(spouseXrefs, husbRecs[0].Value)
			}
		}
		if isSpouse {
			for _, chil := range fam.ChildrenByTag("CHIL") {
				if chil.Value != "" {
					childrenXrefs = append(childrenXrefs, chil.Value)
				}
			}
		}
	}

	notes := collectNoteXrefs(indi, nil)

	return []string{
		indi.Xref,
		"INDI",
		name,
		sex,
		birthDate,
		birthPlace,
		deathDate,
		deathPlace,
		fatherXref,
		motherXref,
		strings.Join(spouseXrefs, ";"),
		strings.Join(childrenXrefs, ";"),
		strings.Join(notes, " | "),
		"", "", "", "",
	}
}

func mediaObjeToCSVRow(obje gedcom.GedcomRecord) []string {
	var file, form, title string
	if fr := obje.FirstChildByTag("FILE"); fr != nil {
		file = fr.Value
		form = fr.ChildValue("FORM")
	}
	title = obje.ChildValue("TITL")
	name := title
	if name == "" {
		name = file
	}
	desc := collectOBJEInlineNoteText(obje)
	return []string{
		obje.Xref,
		"OBJE",
		name,
		"", "", "", "", "", "", "", "", "", "",
		file,
		form,
		title,
		desc,
	}
}
