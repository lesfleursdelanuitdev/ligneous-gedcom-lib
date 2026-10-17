package exporter

import (
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/enricher"
	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
)

// textContGED is enricher/textcont_test.go's fixture plus values long
// enough to need wrapping. Since v0.3.0 the enricher joins CONC/CONT on
// these tags, so Title, Author, Publication, Abbreviation, Text and Page can
// hold "\n" and more than 255 bytes.
var textContGED = "0 HEAD\n1 GEDC\n2 VERS 5.5.1\n1 CHAR UTF-8\n" +
	"0 @S1@ SOUR\n" +
	"1 TITL Census of\n2 CONC  1900, Oslo\n2 CONC " + strings.Repeat("t", 240) + "\n2 CONC " + strings.Repeat("u", 200) + "\n" +
	"1 AUTH Statistisk\n2 CONC  sentralbyrå\n" +
	"1 ABBR Cen\n2 CONC sus\n" +
	"1 PUBL Kristiania\n2 CONT 1901\n2 CONC , reprinted\n" +
	"1 TEXT Line one\n2 CONT Line two\n2 CONT \n2 CONT Line four\n" +
	"0 @I1@ INDI\n1 NAME A /B/\n" +
	"1 SOUR @S1@\n2 PAGE p. 1\n3 CONC 2, entry\n3 CONC  7\n3 CONT " + strings.Repeat("p", 250) + "\n3 CONC " + strings.Repeat("q", 200) + "\n" +
	"1 BIRT\n2 SOUR @S1@\n3 PAGE event\n4 CONT page\n" +
	"0 @F1@ FAM\n1 HUSB @I1@\n1 SOUR @S1@\n2 PAGE fam\n3 CONC ily\n" +
	"0 TRLR\n"

func enrichString(t *testing.T, s string) *enricher.EnrichedDocument {
	t.Helper()
	doc, _, err := parser.Parse(strings.NewReader(s))
	if err != nil {
		t.Fatal(err)
	}
	return enricher.Enrich(doc)
}

// Export, parse and enrich again: every line is a valid GEDCOM line of at
// most 255 bytes, and the source and citation text reads the same.
func TestExportSourceTextRoundTrip(t *testing.T) {
	ed := enrichString(t, textContGED)
	out := EnrichedToGEDCOM(ed)
	for i, l := range strings.Split(strings.TrimRight(out, "\r\n"), "\n") {
		l = strings.TrimRight(l, "\r")
		if len(l) > 255 {
			t.Errorf("line %d is %d bytes: %.60q", i+1, len(l), l)
		}
		f := strings.Fields(l)
		if len(f) < 2 {
			t.Errorf("line %d is not a GEDCOM line: %q", i+1, l)
			continue
		}
		if _, err := strconv.Atoi(f[0]); err != nil {
			t.Errorf("line %d has no level: %.60q", i+1, l)
		}
	}
	back := enrichString(t, out)
	if len(back.Sources) != 1 || len(ed.Sources) != 1 {
		t.Fatalf("sources %d, %d", len(ed.Sources), len(back.Sources))
	}
	a, b := ed.Sources[0], back.Sources[0]
	for _, c := range []struct{ field, want, got string }{
		{"Title", a.Title, b.Title}, {"Author", a.Author, b.Author}, {"Publication", a.Publication, b.Publication},
		{"Abbreviation", a.Abbreviation, b.Abbreviation}, {"Text", a.Text, b.Text},
	} {
		if c.got != c.want {
			t.Errorf("%s after the round trip %q, want %q", c.field, c.got, c.want)
		}
	}
	pages := func(e *enricher.EnrichedDocument) []string {
		var p []string
		for _, l := range e.IndividualSources {
			p = append(p, l.Page)
		}
		for _, l := range e.EventSources {
			p = append(p, l.Page)
		}
		for _, l := range e.FamilySources {
			p = append(p, l.Page)
		}
		return p
	}
	if a, b := strings.Join(pages(ed), "|"), strings.Join(pages(back), "|"); a != b {
		t.Errorf("pages after the round trip %q, want %q", b, a)
	}
	if !strings.Contains(a.Title, "1900, Oslo") || !strings.Contains(a.Publication, "\n") {
		t.Fatalf("setup: %q %q", a.Title, a.Publication)
	}
}

// The denormalised JSON reads TITL, AUTH and PUBL whole, like the enricher.
func TestJSONSourceTextJoined(t *testing.T) {
	doc, _, err := parser.Parse(strings.NewReader(textContGED))
	if err != nil {
		t.Fatal(err)
	}
	js := ToJSON(doc)
	ed := enricher.Enrich(doc)
	s := js.Sources["@S1@"]
	e := ed.Sources[0]
	if s.Title != e.Title || s.Author != e.Author || s.Publication != e.Publication {
		t.Errorf("JSON source %q / %q / %q, enricher %q / %q / %q", s.Title, s.Author, s.Publication, e.Title, e.Author, e.Publication)
	}
}

// Wrapping never cuts next to a Unicode space (a reader that trims line
// ends would lose it): spaces at every offset around the cut, including
// U+00A0 and U+3000, read back the same.
func TestExportWrapNotAtSpace(t *testing.T) {
	var b strings.Builder
	b.WriteString("0 HEAD\n1 GEDC\n2 VERS 5.5.1\n1 CHAR UTF-8\n")
	var want []string
	for k := 0; k < 40; k++ {
		v := strings.Repeat("x", k) + strings.Repeat("ab cd  ef gh　ij ", 40) + "end"
		want = append(want, v)
		b.WriteString("0 @S" + strconv.Itoa(k) + "@ SOUR\n1 TITL " + v[:200] + "\n2 CONC " + v[200:] + "\n")
	}
	b.WriteString("0 TRLR\n")
	ed := enrichString(t, b.String())
	out := EnrichedToGEDCOM(ed)
	ls := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
	for i := 0; i+1 < len(ls); i++ {
		f := strings.Fields(ls[i+1])
		if len(f) < 2 || f[1] != "CONC" {
			continue
		}
		cont := strings.SplitN(ls[i+1], " CONC ", 2)[1]
		r, _ := utf8.DecodeLastRuneInString(ls[i])
		s, _ := utf8.DecodeRuneInString(cont)
		if unicode.IsSpace(r) || unicode.IsSpace(s) {
			t.Errorf("cut next to a space between lines %d and %d: …%q | %q…", i+1, i+2, tail(ls[i], 8), head(cont, 8))
		}
	}
	back := enrichString(t, out)
	for k, s := range back.Sources {
		if s.Title != want[k] {
			t.Errorf("source %d: title changed", k)
		}
	}
}

func tail(s string, n int) string { return s[max(0, len(s)-n):] }
func head(s string, n int) string { return s[:min(len(s), n)] }
