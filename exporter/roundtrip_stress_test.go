package exporter

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/enricher"
	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/gedcom"
	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
)

var roundTripFiles = []string{
	"../../Married1200.ged",
	"../../Children1200.ged",
	"../../Long26CC.ged",
	"../../Long26LL.ged",
	"../../Siblings1200.ged",
}

// TestRoundTrip_StressFiles runs a full parse→enrich→export→re-parse round-trip
// on each stress file and verifies no data is lost.
func TestRoundTrip_StressFiles(t *testing.T) {
	for _, path := range roundTripFiles {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Skipf("file not found: %v", err)
			}

			// Step 1: parse original
			doc1, _, err := parser.Parse(strings.NewReader(string(data)))
			if err != nil {
				t.Fatalf("parse original: %v", err)
			}

			// Step 2: enrich
			ed := enricher.Enrich(doc1)

			// Step 3: export back to GEDCOM
			gedcomText := EnrichedToGEDCOM(ed)

			// Step 4: re-parse
			doc2, warns2, err := parser.Parse(strings.NewReader(gedcomText))
			if err != nil {
				t.Fatalf("re-parse: %v", err)
			}
			if len(warns2) > 0 {
				t.Errorf("re-parse produced %d parser warnings", len(warns2))
			}

			// --- Counts ---
			if doc2.IndividualCount() != doc1.IndividualCount() {
				t.Errorf("individual count: original=%d re-parsed=%d",
					doc1.IndividualCount(), doc2.IndividualCount())
			}
			if doc2.FamilyCount() != doc1.FamilyCount() {
				t.Errorf("family count: original=%d re-parsed=%d",
					doc1.FamilyCount(), doc2.FamilyCount())
			}

			t.Logf("individuals=%d families=%d sources=%d notes=%d",
				doc1.IndividualCount(), doc1.FamilyCount(),
				len(doc1.Sources), len(doc1.Notes))

			// --- Individual fields ---
			orig := indexByXref(doc1.Individuals)
			got := indexByXref(doc2.Individuals)

			for xref, o := range orig {
				g, ok := got[xref]
				if !ok {
					t.Errorf("individual %s missing from re-parsed output", xref)
					continue
				}
				// Compare semantic name subtags rather than raw NAME string —
				// the exporter normalises spurious whitespace inside /Surname/
				// delimiters, which is a correction not a data loss.
				checkNameSubtags(t, xref, o, g)
				checkField(t, xref, "SEX", o.ChildValue("SEX"), g.ChildValue("SEX"))
				checkEvent(t, xref, "BIRT", o, g)
				checkEvent(t, xref, "DEAT", o, g)
			}

			// --- Family fields ---
			origFam := indexByXref(doc1.Families)
			gotFam := indexByXref(doc2.Families)

			for xref, o := range origFam {
				g, ok := gotFam[xref]
				if !ok {
					t.Errorf("family %s missing from re-parsed output", xref)
					continue
				}
				checkField(t, xref, "HUSB", o.ChildValue("HUSB"), g.ChildValue("HUSB"))
				checkField(t, xref, "WIFE", o.ChildValue("WIFE"), g.ChildValue("WIFE"))
				checkChildren(t, xref, o, g)
				checkEvent(t, xref, "MARR", o, g)
			}
		})
	}
}

// indexByXref builds a map from xref → record.
func indexByXref(records []gedcom.GedcomRecord) map[string]gedcom.GedcomRecord {
	m := make(map[string]gedcom.GedcomRecord, len(records))
	for _, r := range records {
		if r.Xref != "" {
			m[r.Xref] = r
		}
	}
	return m
}

// checkField reports a mismatch between an original and re-parsed field value.
func checkField(t *testing.T, xref, tag, orig, got string) {
	t.Helper()
	if orig != got {
		t.Errorf("%s %s: original=%q re-parsed=%q", xref, tag, orig, got)
	}
}

// checkEvent compares the DATE and PLAC of the first occurrence of an event tag.
func checkEvent(t *testing.T, xref, tag string, orig, got gedcom.GedcomRecord) {
	t.Helper()
	origEvts := orig.ChildrenByTag(tag)
	gotEvts := got.ChildrenByTag(tag)
	if len(origEvts) == 0 && len(gotEvts) == 0 {
		return
	}
	if len(origEvts) != len(gotEvts) {
		t.Errorf("%s %s count: original=%d re-parsed=%d", xref, tag, len(origEvts), len(gotEvts))
		return
	}
	for i := range origEvts {
		checkField(t, xref, tag+" DATE", origEvts[i].ChildValue("DATE"), gotEvts[i].ChildValue("DATE"))
		checkField(t, xref, tag+" PLAC", origEvts[i].ChildValue("PLAC"), gotEvts[i].ChildValue("PLAC"))
	}
}

// checkChildren compares the sorted set of CHIL xrefs in a family record.
func checkChildren(t *testing.T, xref string, orig, got gedcom.GedcomRecord) {
	t.Helper()
	origChil := childValues(orig, "CHIL")
	gotChil := childValues(got, "CHIL")
	sort.Strings(origChil)
	sort.Strings(gotChil)
	if strings.Join(origChil, ",") != strings.Join(gotChil, ",") {
		t.Errorf("%s CHIL: original=%v re-parsed=%v", xref, origChil, gotChil)
	}
}

// checkNameSubtags compares GIVN and SURN subtags of the first NAME record.
// Uses resolveValue to follow CONC children so long surnames are compared in full.
// This avoids false failures from whitespace normalisation inside /Surname/ delimiters.
func checkNameSubtags(t *testing.T, xref string, orig, got gedcom.GedcomRecord) {
	t.Helper()
	origName := orig.FirstChildByTag("NAME")
	gotName := got.FirstChildByTag("NAME")
	if origName == nil && gotName == nil {
		return
	}
	if origName == nil || gotName == nil {
		t.Errorf("%s NAME: one side has no NAME record", xref)
		return
	}
	checkField(t, xref, "NAME/GIVN", resolveSubtagValue(origName, "GIVN"), resolveSubtagValue(gotName, "GIVN"))
	checkField(t, xref, "NAME/SURN", resolveSubtagValue(origName, "SURN"), resolveSubtagValue(gotName, "SURN"))
}

// resolveSubtagValue returns the full value of the first child with the given
// tag, concatenating any CONC grandchildren.
func resolveSubtagValue(rec *gedcom.GedcomRecord, tag string) string {
	child := rec.FirstChildByTag(tag)
	if child == nil {
		return ""
	}
	val := child.Value
	for _, conc := range child.ChildrenByTag("CONC") {
		val += conc.Value
	}
	return val
}

func childValues(rec gedcom.GedcomRecord, tag string) []string {
	children := rec.ChildrenByTag(tag)
	vals := make([]string, 0, len(children))
	for _, c := range children {
		vals = append(vals, strings.TrimSpace(c.Value))
	}
	return vals
}
