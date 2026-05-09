package pedigreedoc

import "testing"

func TestFormatParseRoundTrip(t *testing.T) {
	bio, adopt := "@I1@", "@I2@"
	line := FormatMixedParentageNote(bio, adopt)
	gotBio, gotAdopt, ok := ParseMixedParentageNote(line)
	if !ok || !XrefEqual(gotBio, bio) || !XrefEqual(gotAdopt, adopt) {
		t.Fatalf("round trip: ok=%v bio=%q adopt=%q from %q", ok, gotBio, gotAdopt, line)
	}
}

func TestParseCaseInsensitivePrefix(t *testing.T) {
	_, _, ok := ParseMixedParentageNote("ligneous mixed parentage: biological=@A@; adoptive=@B@")
	if !ok {
		t.Fatal("expected ok for lowercase prefix")
	}
}

func TestIsMixedBiologicalAdoptivePair(t *testing.T) {
	f := ParentEdge{ParentXref: "@I1@", Pedigree: "birth", RelationshipType: "biological"}
	m := ParentEdge{ParentXref: "@I2@", Pedigree: "adopted", RelationshipType: "adopted"}
	if !IsMixedBiologicalAdoptivePair(f, m) {
		t.Fatal("expected mixed")
	}
	b, a := BiologicalAndAdoptiveXrefs(f, m)
	if !XrefEqual(b, "@I1@") || !XrefEqual(a, "@I2@") {
		t.Fatalf("xrefs b=%q a=%q", b, a)
	}
	if IsMixedBiologicalAdoptivePair(f, f) {
		t.Fatal("same parent should not be mixed")
	}
}
