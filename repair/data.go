package repair

import (
	"fmt"
	"strings"
)

// The data kinds. Unlike the format kinds they change what the file says,
// so they run only when named in Options.Fixes. Every decision is taken on
// the decoded tree; the lines are removed (or added) in the output, before
// the format kinds run.

// unit is a line and every line nested under it: lines start to end.
type unit struct{ start, end int }

func (u unit) size() int { return u.end - u.start + 1 }

// brokenLink is a pointer line whose target is in no level-0 record.
type brokenLink struct {
	unit
	kind, xref string
}

// emptyFamily is a FAM record with no members and nothing but CHAN, RIN,
// _UID and REFN, and the FAMS/FAMC lines that point to it.
type emptyFamily struct {
	unit
	xref  string
	links []unit
}

// dataFacts is what the data kinds need to know, the same whatever is chosen.
type dataFacts struct {
	broken      []brokenLink
	families    []emptyFamily
	famWithData bool // a family with no members has data of its own: kept
	missingSubm bool // HEADER_MISSING_SUBM
	subAfter    int  // the HEAD SUBM line goes after this line
	trlr        int  // the first 0 TRLR line, or -1
	tokens      map[string]bool
}

// isPointer reports whether v (trimmed) is an xref pointer: "@X@", with no
// "@" inside and not an escape such as "@#DJULIAN@".
func isPointer(v string) bool {
	return len(v) > 2 && v[0] == '@' && v[len(v)-1] == '@' && v[1] != '#' &&
		!strings.Contains(v[1:len(v)-1], "@")
}

// pointerKind names the record a pointer tag points to.
func pointerKind(tag string) string {
	switch tag {
	case "FAMC", "FAMS":
		return "family"
	case "HUSB", "WIFE", "CHIL", "ASSO", "ALIA", "ANCI", "DESI":
		return "person"
	case "SOUR":
		return "source"
	case "NOTE":
		return "note"
	case "OBJE":
		return "media"
	case "REPO":
		return "repository"
	case "SUBM", "SUBN":
		return "submitter"
	}
	return "record"
}

var emptyFamilyTags = map[string]bool{"CHAN": true, "RIN": true, "_UID": true, "REFN": true}

func (a *analysis) analyseData() {
	d := &a.facts
	d.trlr = -1
	d.tokens = map[string]bool{}
	defined := map[string]int{}
	for i, nd := range a.nodes {
		if !nd.ok {
			continue
		}
		if nd.level == 0 && nd.xref != "" {
			defined[nd.xref]++
			d.tokens[nd.xref] = true
		}
		if nd.level == 0 && nd.tag == "TRLR" && d.trlr < 0 {
			d.trlr = i
		}
		if v := strings.TrimSpace(value(a.clean[i], nd)); nd.level > 0 && isPointer(v) {
			d.tokens[v] = true
		}
	}
	top := func(i int) int { // the level-0 line above i
		for a.nodes[i].parent >= 0 {
			i = a.nodes[i].parent
		}
		return i
	}

	// Broken links: the outermost pointer lines to a missing record.
	end := -1
	for i, nd := range a.nodes {
		if !nd.ok || nd.level == 0 || i <= end {
			continue
		}
		v := strings.TrimSpace(value(a.clean[i], nd))
		if !isPointer(v) || defined[v] > 0 || a.nodes[top(i)].level != 0 {
			continue
		}
		u := unit{i, lastDescendant(a.nodes, i)}
		d.broken = append(d.broken, brokenLink{u, pointerKind(nd.tag), v})
		end = u.end
	}

}

// dataLines is the Available line count of a data kind: the lines it
// removes, or the 3 lines the submitter adds.
func (a *analysis) dataLines(f Fix) int {
	n := 0
	switch f {
	case FixBrokenLinks:
		for _, b := range a.facts.broken {
			n += b.size()
		}
	}
	return n
}

// planData applies the chosen data kinds to p: removed lines in p.removed,
// added lines in p.insert*. Without the conversion a unit is left alone
// where removing it could change how the file is read.
func (a *analysis) planData(sel fixSet, name string, o *outcome, p *plan, guard bool, add func(int, string, Fix)) {
	canDrop := func(u unit) bool {
		if o.conv {
			return true
		}
		if guard && (a.prevHigh(u.start) || highEnd(a.raw[u.end].text)) {
			return false
		}
		return u.start >= a.charScanEnd // never inside the CHAR window
	}
	drop := func(u unit, what string, f Fix, counted bool) {
		var nested []string
		for j := u.start; j <= u.end; j++ {
			if p.removed[j] {
				continue
			}
			p.removed[j] = true
			o.touched.mark(f, j+1)
			if j > u.start && strings.TrimSpace(a.src[j].text) != "" {
				nested = append(nested, a.src[j].text)
			}
		}
		if counted && len(nested) > 0 {
			what += fmt.Sprintf(" (and %d lines under it)", len(nested))
		}
		o.changes = append(o.changes, Change{Line: u.start + 1, What: what, Fix: f, Removed: nested})
	}

	if sel[FixBrokenLinks] {
		for _, b := range a.facts.broken {
			if p.removed[b.start] || !canDrop(b.unit) {
				continue
			}
			drop(b.unit, fmt.Sprintf("removed a link to %s %s, which doesn't exist", b.kind, b.xref), FixBrokenLinks, true)
		}
	}
}
