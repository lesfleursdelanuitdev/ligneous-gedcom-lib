package enricher

import "testing"

func TestEventCatalogTag_EvenCustom(t *testing.T) {
	tag := EventCatalogTag("EVEN", "Military Service")
	if tag == "EVEN" || tag == "" {
		t.Fatalf("unexpected tag %q", tag)
	}
	if len(tag) < 10 || tag[:6] != "EVEN__" {
		t.Fatalf("expected EVEN__ prefix, got %q", tag)
	}
}

func TestEventLabelFor(t *testing.T) {
	if g, w := EventLabelFor("BIRT", ""), "Birth"; g != w {
		t.Fatalf("got %q want %q", g, w)
	}
	if g, w := EventLabelFor("EVEN", "Military Service"), "Military Service"; g != w {
		t.Fatalf("got %q want %q", g, w)
	}
	if g := EventLabelFor("MARR", ""); g != "Marriage" {
		t.Fatalf("got %q want Marriage", g)
	}
}
