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
