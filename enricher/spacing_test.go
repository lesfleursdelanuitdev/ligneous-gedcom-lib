package enricher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
)

func enrichedNoLines(t *testing.T, name string) (*EnrichedDocument, string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", "repair", name))
	if err != nil {
		t.Fatal(err)
	}
	doc, _, err := parser.Parse(strings.NewReader(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	ed := Enrich(doc)
	ed.Document = nil
	j, err := json.Marshal(ed)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(j, &v); err != nil {
		t.Fatal(err)
	}
	var strip func(any)
	strip = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			delete(x, "line")
			for _, c := range x {
				strip(c)
			}
		case []any:
			for _, c := range x {
				strip(c)
			}
		}
	}
	strip(v)
	out, _ := json.MarshalIndent(v, "", " ")
	return ed, string(out)
}

// Two spaces before pointer, SEX, NAME, DATE and PLAC values give the same
// enriched links, names, dates and places as one space.
func TestEnrichDoubleSpaceValues(t *testing.T) {
	ed, double := enrichedNoLines(t, "double-space-values.ged")
	_, single := enrichedNoLines(t, "single-space-values.ged")
	if double != single {
		i := 0
		for i < len(double) && i < len(single) && double[i] == single[i] {
			i++
		}
		t.Errorf("enriched output differs at byte %d:\n double …%s\n single …%s", i,
			double[max(0, i-150):min(len(double), i+150)], single[max(0, i-150):min(len(single), i+150)])
	}
	if len(ed.Families) != 1 || ed.Families[0].HusbandXref != "@I1@" || ed.Families[0].WifeXref != "@I2@" {
		t.Errorf("family links = %+v", ed.Families)
	}
	if len(ed.Individuals) != 3 || ed.Individuals[0].Sex != "M" {
		t.Errorf("individuals = %+v", ed.Individuals)
	}
}
