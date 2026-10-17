package enricher

import (
	"strings"
	"testing"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
)

// Source and citation text tags are read with their CONC (joined verbatim)
// and CONT (joined with "\n") lines, in file order. Before v0.3.0 only the
// first line was read: Title "Census of", Page "p. 1", and so on.
const textContGED = "0 HEAD\n1 GEDC\n2 VERS 5.5.1\n1 CHAR UTF-8\n" +
	"0 @S1@ SOUR\n" +
	"1 TITL Census of\n2 CONC  1900, Oslo\n" +
	"1 AUTH Statistisk\n2 CONC  sentralbyrå\n" +
	"1 ABBR Cen\n2 CONC sus\n" +
	"1 PUBL Kristiania\n2 CONT 1901\n2 CONC , reprinted\n" +
	"1 TEXT Line one\n2 CONT Line two\n2 CONT \n2 CONT Line four\n" +
	"0 @I1@ INDI\n1 NAME A /B/\n" +
	"1 SOUR @S1@\n2 PAGE p. 1\n3 CONC 2, entry\n3 CONC  7\n" +
	"2 DATA\n3 TEXT quoted\n4 CONT text\n4 CONC , ok\n" +
	"1 BIRT\n2 SOUR @S1@\n3 PAGE event\n4 CONT page\n" +
	"0 @F1@ FAM\n1 HUSB @I1@\n1 SOUR @S1@\n2 PAGE fam\n3 CONC ily\n" +
	"0 TRLR\n"

func TestEnrichSourceTextContinuations(t *testing.T) {
	doc, _, err := parser.Parse(strings.NewReader(textContGED))
	if err != nil {
		t.Fatal(err)
	}
	ed := Enrich(doc)
	if len(ed.Sources) != 1 {
		t.Fatalf("%d sources", len(ed.Sources))
	}
	s := ed.Sources[0]
	for _, c := range []struct{ field, got, want string }{
		{"Title", s.Title, "Census of 1900, Oslo"},
		{"Author", s.Author, "Statistisk sentralbyrå"},
		{"Abbreviation", s.Abbreviation, "Census"},
		{"Publication", s.Publication, "Kristiania\n1901, reprinted"},
		{"Text", s.Text, "Line one\nLine two\n\nLine four"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.field, c.got, c.want)
		}
	}
	if len(ed.IndividualSources) != 1 {
		t.Fatalf("%d individual citations", len(ed.IndividualSources))
	}
	if c := ed.IndividualSources[0]; c.Page != "p. 12, entry 7" || c.CitationText != "quoted\ntext, ok" {
		t.Errorf("individual citation Page %q, CitationText %q", c.Page, c.CitationText)
	}
	if len(ed.EventSources) != 1 || ed.EventSources[0].Page != "event\npage" {
		t.Errorf("event citations %+v", ed.EventSources)
	}
	if len(ed.FamilySources) != 1 || ed.FamilySources[0].Page != "family" {
		t.Errorf("family citations %+v", ed.FamilySources)
	}
}

// Without continuation lines the values are read exactly as before.
func TestEnrichSourceTextWithoutContinuations(t *testing.T) {
	const ged = "0 HEAD\n0 @S1@ SOUR\n1 TITL  Title with spaces  \n1 TEXT t\n0 @I1@ INDI\n1 SOUR @S1@\n2 PAGE  p. 3 \n0 TRLR\n"
	doc, _, err := parser.Parse(strings.NewReader(ged))
	if err != nil {
		t.Fatal(err)
	}
	ed := Enrich(doc)
	if want := doc.Sources[0].ChildValue("TITL"); ed.Sources[0].Title != want {
		t.Errorf("Title %q, want %q", ed.Sources[0].Title, want)
	}
	if want := doc.Individuals[0].Children[0].ChildValue("PAGE"); ed.IndividualSources[0].Page != want {
		t.Errorf("Page %q, want %q", ed.IndividualSources[0].Page, want)
	}
}
