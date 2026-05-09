// Package pedigreedoc documents mixed biological/adoptive parentage in GEDCOM
// using a single FAMC.PEDI of "birth" plus an inline NOTE with explicit xrefs.
package pedigreedoc

import (
	"regexp"
	"strings"
)

// MixedParentageNotePrefix begins the machine-readable inline NOTE under INDI.FAMC.
const MixedParentageNotePrefix = "Ligneous mixed parentage:"

// ParentEdge is the minimal parent–child fields used for mixed-parent detection.
type ParentEdge struct {
	ParentXref       string
	Pedigree         string
	RelationshipType string
}

// NormalizeXrefPointer returns xref in "@...@ " form.
func NormalizeXrefPointer(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "@") && strings.HasSuffix(s, "@") && len(s) >= 2 {
		return s
	}
	inner := strings.Trim(s, "@")
	if inner == "" {
		return ""
	}
	return "@" + inner + "@"
}

// XrefEqual compares two xrefs after normalization.
func XrefEqual(a, b string) bool {
	return NormalizeXrefPointer(a) == NormalizeXrefPointer(b)
}

// IsBirthLike returns true when this edge is treated as a birth/biological link
// for pedigree purposes (empty pedigree/relationship defaults to biological).
func IsBirthLike(e ParentEdge) bool {
	if nonBirthPedigree(e.Pedigree) {
		return false
	}
	rt := strings.ToLower(strings.TrimSpace(e.RelationshipType))
	switch rt {
	case "adopted", "foster", "sealing", "step":
		return false
	default:
		return true
	}
}

func nonBirthPedigree(p string) bool {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "adopted", "adoptive", "foster", "sealing", "step":
		return true
	default:
		return false
	}
}

// IsMixedBiologicalAdoptivePair reports two distinct parents where exactly one
// is birth-like and the other is not (e.g. biological + adoptive).
func IsMixedBiologicalAdoptivePair(a, b ParentEdge) bool {
	if XrefEqual(a.ParentXref, b.ParentXref) {
		return false
	}
	aBio, bBio := IsBirthLike(a), IsBirthLike(b)
	return aBio != bBio
}

// BiologicalAndAdoptiveXrefs returns (biological, adoptive) parent xrefs; callers
// must ensure IsMixedBiologicalAdoptivePair is true.
func BiologicalAndAdoptiveXrefs(a, b ParentEdge) (bioXref, adoptXref string) {
	if IsBirthLike(a) && !IsBirthLike(b) {
		return NormalizeXrefPointer(a.ParentXref), NormalizeXrefPointer(b.ParentXref)
	}
	if IsBirthLike(b) && !IsBirthLike(a) {
		return NormalizeXrefPointer(b.ParentXref), NormalizeXrefPointer(a.ParentXref)
	}
	return "", ""
}

// FormatMixedParentageNote returns a single-line inline NOTE body (no CONT).
func FormatMixedParentageNote(biologicalParentXref, adoptiveParentXref string) string {
	return MixedParentageNotePrefix +
		" biological=" + NormalizeXrefPointer(biologicalParentXref) +
		"; adoptive=" + NormalizeXrefPointer(adoptiveParentXref)
}

var mixedNoteLineRE = regexp.MustCompile(
	`(?i)\A\s*` + regexp.QuoteMeta(MixedParentageNotePrefix) +
		`\s*biological=(@[^@]+@)\s*;\s*adoptive=(@[^@]+@)\s*\z`,
)

// ParseMixedParentageNote extracts biological and adoptive parent xrefs from an
// inline NOTE line. Returns ok false if the text does not match the format.
func ParseMixedParentageNote(note string) (bioXref, adoptXref string, ok bool) {
	note = strings.TrimSpace(note)
	first := note
	if idx := strings.IndexByte(note, '\n'); idx >= 0 {
		first = strings.TrimSpace(note[:idx])
	}
	m := mixedNoteLineRE.FindStringSubmatch(first)
	if m == nil {
		return "", "", false
	}
	return NormalizeXrefPointer(m[1]), NormalizeXrefPointer(m[2]), true
}
