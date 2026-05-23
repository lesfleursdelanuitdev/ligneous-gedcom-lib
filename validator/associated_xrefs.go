package validator

import (
	"sort"
	"strings"
)

// uniqueXrefsInOrder returns distinct non-empty xref strings in first-seen order.
func uniqueXrefsInOrder(parts ...string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// sortedDetailXrefValues returns Details values that look like GEDCOM xrefs, in sorted key order (deterministic).
func sortedDetailXrefValues(d map[string]string) []string {
	if d == nil {
		return nil
	}
	keys := make([]string, 0, len(d))
	for k := range d {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var xs []string
	for _, k := range keys {
		v := strings.TrimSpace(d[k])
		if isXref(v) {
			xs = append(xs, v)
		}
	}
	return xs
}

// setAssociatedXrefs fills AssociatedXrefs when it is still nil/empty. It is safe to call multiple times.
// Order is chosen per rule so clients can show child → parent → family (or family + spouses) consistently.
func setAssociatedXrefs(e *ValidationError) {
	if e == nil || len(e.AssociatedXrefs) > 0 {
		return
	}
	switch e.Code {
	case CodeFatherTooYoungAtBirth, CodeFatherTooOldAtBirth, CodeChildBornBeforeFather:
		child := strings.TrimSpace(e.Details["child"])
		if child == "" {
			child = strings.TrimSpace(e.RelatedXref)
		}
		parent := strings.TrimSpace(e.Details["parent"])
		fam := strings.TrimSpace(e.Xref)
		e.AssociatedXrefs = uniqueXrefsInOrder(child, parent, fam)
	case CodeMotherTooYoungAtBirth, CodeMotherTooOldAtBirth, CodeChildBornBeforeMother:
		child := strings.TrimSpace(e.Details["child"])
		if child == "" {
			child = strings.TrimSpace(e.RelatedXref)
		}
		parent := strings.TrimSpace(e.Details["parent"])
		fam := strings.TrimSpace(e.Xref)
		e.AssociatedXrefs = uniqueXrefsInOrder(child, parent, fam)
	case CodeSwappedOppositeSexSpouses:
		f := strings.TrimSpace(e.Xref)
		h := strings.TrimSpace(e.Details["husb_xref"])
		w := strings.TrimSpace(e.Details["wife_xref"])
		e.AssociatedXrefs = uniqueXrefsInOrder(f, h, w)
	case CodeMarriageBeforeSpouseBirth, CodeMarriedBelowMinimumAge:
		e.AssociatedXrefs = uniqueXrefsInOrder(e.Xref, e.RelatedXref)
	case CodeEventDateUnparseable, CodeBaptismBeforeBirth, CodeBurialBeforeDeath,
		CodeDeathBeforeBirth, CodeAgeAtDeathExceeds120, CodeInvalidSex,
		CodeMissingName, CodeNameTagEmpty, CodeNameMissingSurnameSlash,
		CodeEmptyFamily, CodeMissingXrefFamily, CodeMissingXrefIndividual, CodeXrefExceedsVersionLimit:
		e.AssociatedXrefs = uniqueXrefsInOrder(e.Xref)
	case CodeOrphanedAssociateXref:
		e.AssociatedXrefs = uniqueXrefsInOrder(e.Xref, e.RelatedXref)
	case CodeInvalidAssociateTarget:
		t := strings.TrimSpace(e.Details["target"])
		e.AssociatedXrefs = uniqueXrefsInOrder(e.Xref, t)
	case CodeHeaderMissingSubm, CodeMissingHeader, CodeMissingTrailer:
		e.AssociatedXrefs = nil
	default:
		all := append([]string{e.Xref, e.RelatedXref}, sortedDetailXrefValues(e.Details)...)
		out := uniqueXrefsInOrder(all...)
		if len(out) == 0 {
			e.AssociatedXrefs = nil
		} else {
			e.AssociatedXrefs = out
		}
	}
}

// ApplyAssociatedXrefs runs setAssociatedXrefs on each element. Use after building ad-hoc error slices
// (for example PhysicalLineLengthWarnings) before JSON serialization.
func ApplyAssociatedXrefs(errs []*ValidationError) {
	for _, e := range errs {
		setAssociatedXrefs(e)
	}
}
