package validator

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/gedcom"
	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
)

func parseSpacing(t *testing.T, name string) *gedcom.GedcomDocument {
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

func findingKeys(errs []*ValidationError) []string {
	var out []string
	for _, e := range errs {
		out = append(out, e.Code+" "+e.Xref+" "+e.RelatedXref)
	}
	sort.Strings(out)
	return out
}

// Two spaces before a pointer, SEX, NAME or DATE value are read like one:
// no false ORPHANED_* or INVALID_SEX, and the same findings as the
// single-space file.
func TestDoubleSpaceValues(t *testing.T) {
	opts := DefaultOptions()
	opts.DateConsistency = true
	double := ValidateWithOptions(parseSpacing(t, "double-space-values.ged"), opts)
	single := ValidateWithOptions(parseSpacing(t, "single-space-values.ged"), opts)
	for _, e := range double {
		if strings.HasPrefix(e.Code, "ORPHANED_") || e.Code == CodeInvalidSex {
			t.Errorf("false finding %s on line %d: %s", e.Code, e.Line, e.Message)
		}
	}
	if a, b := findingKeys(double), findingKeys(single); !reflect.DeepEqual(a, b) {
		t.Errorf("findings differ:\n double %v\n single %v", a, b)
	}
}

func TestTrimmedValue(t *testing.T) {
	r := gedcom.GedcomRecord{Tag: "HUSB", Value: "  @I1@ \t"}
	if got := r.TrimmedValue(); got != "@I1@" {
		t.Errorf("TrimmedValue = %q", got)
	}
	if r.Value != "  @I1@ \t" {
		t.Error("Value changed")
	}
}
