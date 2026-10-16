package explain

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/gedcom"
	gparser "github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/validator"
)

var validatorCodes = []string{
	"MISSING_HEADER", "MISSING_TRAILER", "MISSING_XREF_INDIVIDUAL", "MISSING_XREF_FAMILY",
	"MISSING_NAME", "NAME_TAG_EMPTY", "NAME_MISSING_SURNAME_SLASH", "INVALID_SEX", "EMPTY_FAMILY",
	"ORPHANED_FAMC", "ORPHANED_FAMS", "ORPHANED_HUSB", "ORPHANED_WIFE", "ORPHANED_CHIL",
	"ORPHANED_SOUR", "ORPHANED_NOTE", "ORPHANED_XREF_OTHER", "MISSING_ASSOCIATE_XREF",
	"ORPHANED_ASSOCIATE_XREF", "INVALID_ASSOCIATE_TARGET", "MISSING_ASSOCIATE_RELA",
	"DEATH_BEFORE_BIRTH", "AGE_AT_DEATH_EXCEEDS_120", "CHILD_BORN_BEFORE_FATHER",
	"CHILD_BORN_BEFORE_MOTHER", "FATHER_TOO_YOUNG_AT_CHILD_BIRTH", "MOTHER_TOO_YOUNG_AT_CHILD_BIRTH",
	"FATHER_TOO_OLD_AT_CHILD_BIRTH", "MOTHER_TOO_OLD_AT_CHILD_BIRTH", "MARRIAGE_BEFORE_SPOUSE_BIRTH",
	"MARRIED_BELOW_MINIMUM_AGE", "EVENT_DATE_UNPARSEABLE", "BAPTISM_BEFORE_BIRTH",
	"BURIAL_BEFORE_DEATH", "SWAPPED_OPPOSITE_SEX_SPOUSE_TAGS", "HEADER_MISSING_SUBM",
	"XREF_EXCEEDS_VERSION_LIMIT",
}

var parserCodes = []string{
	"EMPTY_LINE", "LINE_TOO_LONG", "LEADING_WHITESPACE", "BAD_LINE", "BAD_LEVEL", "XREF_WITHOUT_TAG",
	"TOO_DEEP", "READ_ERROR", "LINES_BEFORE_HEAD", "LEVEL_JUMP", "UNKNOWN_RECORD",
	"ENCODING_NOT_UTF8", "ENCODING_MISLABELED", "ENCODING_UNDECLARED", "ENCODING_BAD_BYTES",
}

// codeConstants returns the string values of the Code* constants in a Go file.
func codeConstants(t *testing.T, path string) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, s := range gd.Specs {
			vs := s.(*ast.ValueSpec)
			for i, n := range vs.Names {
				if !strings.HasPrefix(n.Name, "Code") || i >= len(vs.Values) {
					continue
				}
				if lit, ok := vs.Values[i].(*ast.BasicLit); ok {
					v, _ := strconv.Unquote(lit.Value)
					out = append(out, v)
				}
			}
		}
	}
	return out
}

func sameSet(t *testing.T, what string, got, want []string) {
	t.Helper()
	m := map[string]bool{}
	for _, w := range want {
		m[w] = true
	}
	for _, g := range got {
		if !m[g] {
			t.Errorf("%s: %s is in the source but not in this test's list", what, g)
		}
		delete(m, g)
	}
	for w := range m {
		t.Errorf("%s: %s is in this test's list but not in the source", what, w)
	}
}

func TestCatalogCoversEveryCode(t *testing.T) {
	sameSet(t, "validator/codes.go", codeConstants(t, "../validator/codes.go"), validatorCodes)
	sameSet(t, "parser/errors.go", codeConstants(t, "../parser/errors.go"), parserCodes)
	all := append(append([]string{"LINE_EXCEEDS_MAX_PHYSICAL_LENGTH"}, validatorCodes...), parserCodes...)
	for _, c := range all {
		if _, ok := Lookup(c); !ok {
			t.Errorf("no catalog entry for %s", c)
		}
	}
	if len(All()) != len(all) {
		t.Errorf("catalog has %d entries, want %d (no extras, no duplicates)", len(All()), len(all))
	}
	seen := map[string]bool{}
	for _, e := range All() {
		if seen[e.Code] {
			t.Errorf("duplicate entry %s", e.Code)
		}
		seen[e.Code] = true
	}
	a, _ := Lookup("LINE_TOO_LONG")
	b, _ := Lookup("LINE_EXCEEDS_MAX_PHYSICAL_LENGTH")
	if a.Title != b.Title || a.Meaning != b.Meaning || a.HowToFix != b.HowToFix {
		t.Error("LINE_EXCEEDS_MAX_PHYSICAL_LENGTH should copy LINE_TOO_LONG's text")
	}
}

func TestNoEmptyFields(t *testing.T) {
	groups := map[string]bool{"format": true, "header": true, "links": true, "dates": true, "names": true}
	for _, e := range All() {
		if e.Code == "" || e.Title == "" || e.Meaning == "" || e.HowToFix == "" {
			t.Errorf("%s has an empty field: %+v", e.Code, e)
		}
		if !groups[e.Group] {
			t.Errorf("%s has group %q", e.Code, e.Group)
		}
		if strings.Contains(e.Meaning+e.Title+e.HowToFix, "(same") {
			t.Errorf("%s still says \"(same …)\"", e.Code)
		}
	}
}

func TestPlaceholders(t *testing.T) {
	e := Entry{Title: "{a} and {b}", Meaning: "{a} again, `{c}`", HowToFix: "none { x }"}
	got := strings.Join(Placeholders(e), ",")
	if got != "a,b,c" {
		t.Errorf("Placeholders = %q, want a,b,c", got)
	}
	if _, ok := Lookup("NOPE"); ok {
		t.Error("Lookup of unknown code succeeded")
	}
}

var roles = map[string]bool{"person": true, "child": true, "father": true, "mother": true, "target": true, "family": true}

// Details keys behind the non-role placeholders. The page must cope with a
// missing key: `declared` is absent when the file has no CHAR line.
var placeholderKeys = map[string][]string{
	"ENCODING_NOT_UTF8":               []string{"encoding"},
	"ENCODING_MISLABELED":             {"declared", "encoding"},
	"ENCODING_BAD_BYTES":              {"count"},
	"LINES_BEFORE_HEAD":               {"count"},
	"UNKNOWN_RECORD":                  {"tag"},
	"HEADER_MISSING_SUBM":             {"gedcom_version"},
	"ORPHANED_XREF_OTHER":             {"pointer_tag"},
	"INVALID_ASSOCIATE_TARGET":        {"target_tag"},
	"INVALID_SEX":                     {"sex"},
	"DEATH_BEFORE_BIRTH":              {"death_year", "birth_year"},
	"AGE_AT_DEATH_EXCEEDS_120":        {"age"},
	"BAPTISM_BEFORE_BIRTH":            {"bapm", "birt"},
	"BURIAL_BEFORE_DEATH":             {"buri", "deat"},
	"FATHER_TOO_YOUNG_AT_CHILD_BIRTH": {"age"},
	"MOTHER_TOO_YOUNG_AT_CHILD_BIRTH": {"age"},
	"FATHER_TOO_OLD_AT_CHILD_BIRTH":   {"age"},
	"MOTHER_TOO_OLD_AT_CHILD_BIRTH":   {"age"},
	"MARRIAGE_BEFORE_SPOUSE_BIRTH":    {"marriage_year"},
	"MARRIED_BELOW_MINIMUM_AGE":       {"age"},
	"EVENT_DATE_UNPARSEABLE":          {"event", "date"},
}

const validatorFixture = `0 HEAD
1 GEDC
2 VERS 5.5.1
1 CHAR UTF-8
0 @I1@ INDI
1 NAME Mom /Old/
1 SEX F
1 BIRT
2 DATE 1900
1 FAMS @F1@
0 @I2@ INDI
1 NAME Kid /Old/
1 BIRT
2 DATE 1905
1 FAMC @F1@
0 @I3@ INDI
1 NAME Gone /Early/
1 BIRT
2 DATE 1900
1 DEAT
2 DATE 1850
0 @I6@ INDI
1 NAME Q /Sex/
1 SEX Q
0 @I11@ INDI
1 NAME Pal /Three/
1 ASSO @F2@
2 RELA friend
1 BIRT
2 DATE 1900
1 BAPM
2 DATE 1890
1 DEAT
2 DATE 1950
1 BURI
2 DATE 1940
1 RESI
2 DATE whenever
0 @I12@ INDI
1 NAME Bride /Early/
1 BIRT
2 DATE 1900
1 FAMS @F2@
0 @I13@ INDI
1 NAME Groom /Early/
1 BIRT
2 DATE 1890
1 FAMS @F2@
0 @F1@ FAM
1 WIFE @I1@
1 CHIL @I2@
1 CHIL @I8@
0 @F2@ FAM
1 HUSB @I13@
1 WIFE @I12@
1 MARR
2 DATE 1895
0 TRLR
`

// The same fixture shapes as validator's own tests: parents too young and
// too old, and a note pointer that goes nowhere.
const ageFixture = `0 HEAD
1 CHAR UTF-8
0 @I1@ INDI
1 NAME A /B/
1 ASSO @F1@
2 RELA x
1 BIRT
2 DATE 1700
1 DEAT
2 DATE 1850
0 @I2@ INDI
1 NAME Dad /B/
1 BIRT
2 DATE 1900
0 @I3@ INDI
1 NAME Kid /B/
1 BIRT
2 DATE 1905
0 @I4@ INDI
1 NAME Old /B/
1 BIRT
2 DATE 1800
1 NOTE @N9@
0 @I5@ INDI
1 NAME Late /B/
1 BIRT
2 DATE 1900
0 @F1@ FAM
1 HUSB @I2@
1 CHIL @I3@
0 @F3@ FAM
1 HUSB @I4@
1 CHIL @I5@
0 @F4@ FAM
1 WIFE @I4@
1 CHIL @I5@
0 TRLR
`

func collect(t *testing.T) map[string]map[string]bool {
	t.Helper()
	seen := map[string]map[string]bool{}
	add := func(code string, d map[string]string) {
		if seen[code] == nil {
			seen[code] = map[string]bool{}
		}
		for k := range d {
			seen[code][k] = true
		}
	}
	run := func(data []byte) {
		doc, pe, _ := gparser.ParseWithOptions(strings.NewReader(string(data)), &gparser.Options{Lenient: true})
		for _, e := range pe {
			add(e.Code, e.Details)
		}
		if doc != nil {
			addValidator(doc, add)
		}
	}
	var files []string
	for _, g := range []string{"../testdata/*.ged", "../testdata/malformed/*.ged", "../testdata/encodings/*.ged"} {
		m, _ := filepath.Glob(g)
		files = append(files, m...)
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		run(data)
	}
	run([]byte(validatorFixture))
	run([]byte(ageFixture))
	run([]byte("junk\n0 HEAD\n1 CHAR UTF-8\n0 @X1@ _CUSTOM\n0 TRLR\n"))
	run([]byte("0 HEAD\n1 CHAR ANSEL\n0 @I1@ INDI\n1 NAME A\xffB\n0 TRLR\n"))
	return seen
}

func addValidator(doc *gedcom.GedcomDocument, add func(string, map[string]string)) {
	for _, e := range validator.ValidateWithOptions(doc, &validator.Options{DateConsistency: true}) {
		add(e.Code, e.Details)
	}
}

func TestPlaceholdersAreFillable(t *testing.T) {
	for _, e := range All() {
		for _, p := range Placeholders(e) {
			if roles[p] {
				continue
			}
			ok := false
			for _, k := range placeholderKeys[e.Code] {
				ok = ok || k == p
			}
			if !ok {
				t.Errorf("%s: placeholder {%s} is neither a role nor a listed Details key", e.Code, p)
			}
		}
	}
	seen := collect(t)
	// ORPHANED_XREF_OTHER cannot fire today: validateXRefs only checks tags
	// that have their own ORPHANED_* code. checkRef puts pointer_tag on every
	// orphan finding, so ORPHANED_FAMC stands in for it.
	seen["ORPHANED_XREF_OTHER"] = seen["ORPHANED_FAMC"]
	for code, keys := range placeholderKeys {
		for _, k := range keys {
			if !seen[code][k] {
				t.Errorf("%s: no fixture finding carries Details[%q]", code, k)
			}
		}
	}
}
