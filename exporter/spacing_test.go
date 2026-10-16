package exporter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/gedcom"
	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
)

func parseSpacingFixture(t *testing.T, name string) *gedcom.GedcomDocument {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", "repair", name))
	if err != nil {
		t.Fatal(err)
	}
	doc, _, err := parser.Parse(strings.NewReader(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

// Two spaces before NAME, SEX, DATE, PLAC and pointer values: the CSV and JSON
// exports match the single-space file, and no name starts with a space.
func TestExportDoubleSpaceValues(t *testing.T) {
	double := parseSpacingFixture(t, "double-space-values.ged")
	single := parseSpacingFixture(t, "single-space-values.ged")

	csvD, err := ToCSVString(double)
	if err != nil {
		t.Fatal(err)
	}
	csvS, _ := ToCSVString(single)
	if csvD != csvS {
		t.Errorf("CSV differs:\n double %s\n single %s", csvD, csvS)
	}
	if !strings.Contains(csvD, ",Jean /Dupont/,") && !strings.Contains(csvD, "\"Jean /Dupont/\"") {
		t.Errorf("CSV name not trimmed: %s", csvD)
	}

	jsD, err := ToJSONString(double)
	if err != nil {
		t.Fatal(err)
	}
	jsS, _ := ToJSONString(single)
	if jsD != jsS {
		t.Errorf("JSON differs:\n double %s\n single %s", jsD, jsS)
	}
	j := ToJSON(double)
	for _, indi := range j.Individuals {
		if strings.HasPrefix(indi.Name, " ") {
			t.Errorf("JSON name %q has a leading space", indi.Name)
		}
	}
	if len(j.Families) != 1 || j.Families[0].Husband == nil || j.Families[0].Husband.Name != "Jean /Dupont/" {
		t.Errorf("family husband = %+v", j.Families)
	}
}

// Two spaces before NOTE pointers: the JSON note refs and the CSV notes
// column read "@N1@", as with one space.
func TestExportDoubleSpaceNoteRefs(t *testing.T) {
	double := parseSpacingFixture(t, "double-space-refs.ged")
	single := parseSpacingFixture(t, "single-space-refs.ged")
	csvD, err := ToCSVString(double)
	if err != nil {
		t.Fatal(err)
	}
	csvS, _ := ToCSVString(single)
	if csvD != csvS {
		t.Errorf("CSV differs:\n double %s\n single %s", csvD, csvS)
	}
	if strings.Contains(csvD, " @N1@") {
		t.Errorf("CSV note ref not trimmed: %s", csvD)
	}
	jsD, _ := ToJSONString(double)
	jsS, _ := ToJSONString(single)
	if jsD != jsS {
		t.Errorf("JSON differs:\n double %s\n single %s", jsD, jsS)
	}
	j := ToJSON(double)
	if len(j.Individuals) == 0 || len(j.Individuals[0].Notes) != 1 || j.Individuals[0].Notes[0].Xref != "@N1@" {
		t.Errorf("individual notes = %+v", j.Individuals)
	}
	if len(j.Families) != 1 || len(j.Families[0].Notes) != 1 || j.Families[0].Notes[0].Xref != "@N1@" {
		t.Errorf("family notes = %+v", j.Families)
	}
}
