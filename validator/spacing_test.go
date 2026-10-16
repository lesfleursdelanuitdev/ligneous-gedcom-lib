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

// A date finding points at the DATE line of the event whose year was used.
// Here the first BIRT and the first DEAT have no DATE; the second ones do.
func TestDateFindingLineFollowsTheEventUsed(t *testing.T) {
	in := "0 HEAD\n1 CHAR UTF-8\n" + // 1-2
		"0 @I1@ INDI\n1 NAME A /B/\n" + // 3-4
		"1 BIRT\n2 PLAC Paris\n" + // 5-6: first BIRT, no DATE
		"1 BIRT\n2 DATE 1900\n" + // 7-8
		"1 DEAT\n" + // 9: first DEAT, no DATE
		"1 DEAT\n2 DATE 1890\n" + // 10-11
		"1 FAMS @F1@\n" + // 12
		"0 @I2@ INDI\n1 NAME C /B/\n1 BIRT\n1 BIRT\n2 DATE 1905\n1 FAMC @F1@\n" + // 13-18
		"0 @F1@ FAM\n1 HUSB @I1@\n1 CHIL @I2@\n" + // 19-21
		"1 MARR\n1 MARR\n2 DATE 1880\n" + // 22-24
		"0 TRLR\n"
	doc, _, err := parser.Parse(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	opts := DefaultOptions()
	opts.DateConsistency = true
	want := map[string]int{
		CodeDeathBeforeBirth:          11, // DEAT DATE 1890 vs BIRT DATE 1900
		CodeFatherTooYoungAtBirth:     17, // child's second BIRT DATE
		CodeMarriageBeforeSpouseBirth: 24, // second MARR DATE
	}
	got := map[string]int{}
	for _, e := range ValidateWithOptions(doc, opts) {
		if _, ok := want[e.Code]; ok {
			got[e.Code] = e.Line
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lines = %v, want %v", got, want)
	}
}
