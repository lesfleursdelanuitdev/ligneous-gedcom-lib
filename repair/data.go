package repair

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
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
	broken   []brokenLink
	families []emptyFamily // empty as the file stands
	// familiesWithBroken: empty once broken links are gone (broken-links
	// chosen too). Available counts these.
	familiesWithBroken []emptyFamily
	famWithData        bool           // no family can go, and a memberless one has data of its own
	emptySubm          int            // an empty "1 SUBM" line in HEAD, or -1
	brokenLine         map[int]bool   // every line of a broken-link unit
	missingSubm        bool           // HEADER_MISSING_SUBM
	subAfter           int            // the HEAD SUBM line goes after this line
	trlr               int            // the first 0 TRLR line, or -1
	defined            map[string]int // level-0 xrefs, with how often each is defined
}

// isPointer reports whether v (trimmed) is an xref pointer: "@X@", with no
// "@" inside and not an escape such as "@#DJULIAN@".
func isPointer(v string) bool {
	return len(v) > 2 && v[0] == '@' && v[len(v)-1] == '@' && v[1] != '#' &&
		!strings.Contains(v[1:len(v)-1], "@")
}

// pointerTags are the tags whose value is a link when it is an xref. CONT,
// CONC and every "_" tag are text or custom data: never a link here.
var pointerTags = map[string]bool{
	"FAMC": true, "FAMS": true, "HUSB": true, "WIFE": true, "CHIL": true,
	"ASSO": true, "ALIA": true, "ANCI": true, "DESI": true,
	"SOUR": true, "NOTE": true, "OBJE": true, "REPO": true, "SUBM": true, "SUBN": true,
}

// voidPointer is GEDCOM 7's deliberate null pointer: never broken.
const voidPointer = "@VOID@"

// linkOf returns the xref line i links to, when its tag carries links.
func (a *analysis) linkOf(i int) (string, bool) {
	nd := a.nodes[i]
	if !nd.ok || nd.level == 0 || !pointerTags[nd.tag] {
		return "", false
	}
	v := strings.TrimSpace(value(a.clean[i], nd))
	return v, isPointer(v) && v != voidPointer
}

// xrefLike returns the xref-like value of line i, on any tag but CONT/CONC
// (whose values are text): for the xrefs a new record must not reuse, and
// the lines that point to a family.
func (a *analysis) xrefLike(i int) (string, bool) {
	nd := a.nodes[i]
	if !nd.ok || nd.level == 0 || nd.tag == "CONT" || nd.tag == "CONC" {
		return "", false
	}
	v := strings.TrimSpace(value(a.clean[i], nd))
	return v, isPointer(v)
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
	d.emptySubm = -1
	d.brokenLine = map[int]bool{}
	defined := map[string]int{}
	d.defined = defined
	for i, nd := range a.nodes {
		if !nd.ok || nd.level != 0 {
			continue
		}
		if nd.xref != "" {
			defined[nd.xref]++
		}
		if nd.tag == "TRLR" && d.trlr < 0 {
			d.trlr = i
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
	brokenAt := map[int]bool{}
	for i, nd := range a.nodes {
		if i <= end {
			continue
		}
		v, ok := a.linkOf(i)
		if !ok || defined[v] > 0 || a.nodes[top(i)].level != 0 {
			continue
		}
		if t := top(i); a.nodes[t].tag == "HEAD" && (nd.tag == "SUBM" || nd.tag == "SUBN") {
			// Removing it would leave the header without a submitter
			// (HEADER_MISSING_SUBM), which the validator does not report
			// for a broken one; it stays.
			continue
		}
		u := unit{i, lastDescendant(a.nodes, i)}
		d.broken = append(d.broken, brokenLink{u, pointerKind(nd.tag), v})
		brokenAt[i] = true
		for j := u.start; j <= u.end; j++ {
			d.brokenLine[j] = true
		}
		end = u.end
	}

	// Empty families: no members and no data of their own; every pointer
	// to them is a FAMS or FAMC line of a person (removed with them).
	// The lines pointing to a memberless family (any tag but CONT/CONC):
	// only those families can go, so only their pointers are collected.
	pointers := map[string][]int{}
	for _, nd := range a.nodes {
		if !nd.ok || nd.level != 0 || nd.tag != "FAM" || nd.xref == "" {
			continue
		}
		members := false
		for _, c := range nd.children {
			t := a.nodes[c].tag
			members = members || (t == "HUSB" || t == "WIFE" || t == "CHIL") && !brokenAt[c]
		}
		if !members {
			pointers[nd.xref] = nil
		}
	}
	if len(pointers) > 0 {
		for i := range a.nodes {
			if v, ok := a.xrefLike(i); ok {
				if list, want := pointers[v]; want {
					pointers[v] = append(list, i)
				}
			}
		}
	}
	// classify finds the empty families. With ignoreBroken, a broken link
	// in a family counts as gone (broken-links removes it in the same run):
	// a family whose only member or only content is a broken link is empty.
	classify := func(ignoreBroken bool) (fams []emptyFamily, withData bool) {
		for i, nd := range a.nodes {
			if !nd.ok || nd.level != 0 || nd.tag != "FAM" || nd.xref == "" || defined[nd.xref] != 1 {
				continue
			}
			members, own := false, false
			for _, c := range nd.children {
				if ignoreBroken && brokenAt[c] {
					continue
				}
				switch t := a.nodes[c].tag; {
				case t == "HUSB" || t == "WIFE" || t == "CHIL":
					members = true
				case !emptyFamilyTags[t]:
					own = true
				}
			}
			if members {
				continue
			}
			if own {
				withData = true
				continue
			}
			f := emptyFamily{unit: unit{i, lastDescendant(a.nodes, i)}, xref: nd.xref}
			ok := true
			for _, j := range pointers[nd.xref] {
				t := a.nodes[j].tag
				if (t != "FAMS" && t != "FAMC") || a.nodes[top(j)].tag != "INDI" {
					ok = false // another kind of link to it: removing it would break that
					break
				}
				f.links = append(f.links, unit{j, lastDescendant(a.nodes, j)})
			}
			if ok {
				fams = append(fams, f)
			}
		}
		return fams, withData
	}
	d.families, _ = classify(false)
	var withData bool
	d.familiesWithBroken, withData = classify(true)
	// The reason is shown only when no family can go at all.
	d.famWithData = withData && len(d.familiesWithBroken) == 0

	// Submitter: offered when the validator reports HEADER_MISSING_SUBM.
	if a.hasHead {
		ver, subm := "", false
		for _, c := range a.nodes[a.head].children {
			switch a.nodes[c].tag {
			case "GEDC":
				for _, g := range a.nodes[c].children {
					if a.nodes[g].tag == "VERS" && ver == "" {
						ver = strings.TrimSpace(value(a.clean[g], a.nodes[g]))
					}
				}
			case "SUBM":
				if strings.TrimSpace(value(a.clean[c], a.nodes[c])) != "" {
					subm = true
				} else if d.emptySubm < 0 {
					d.emptySubm = c
				}
			}
		}
		d.missingSubm = (ver == "5.5" || ver == "5.5.1") && !subm
		d.subAfter = lastDescendant(a.nodes, a.head)
		if a.charLine >= 0 {
			d.subAfter = lastDescendant(a.nodes, a.charLine)
		}
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
	case FixEmptyFamilies:
		for _, fam := range a.facts.familiesWithBroken {
			n += fam.size()
			for _, l := range fam.links {
				n += l.size()
			}
		}
	case FixSubmitter:
		if a.facts.missingSubm {
			n = 3
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
			o.dataRemoved[j+1] = true
			if f == FixEmptyFamilies && sel[FixBrokenLinks] && a.facts.brokenLine[j] {
				// A broken link inside a removed family is removed too:
				// broken-links counts it.
				o.touched.mark(FixBrokenLinks, j+1)
			}
			if j > u.start { // every removed line is recorded; a blank one as ""
				text := a.src[j].text
				if strings.TrimSpace(text) == "" {
					text = ""
				}
				nested = append(nested, text)
			}
		}
		if counted && len(nested) > 0 {
			if len(nested) == 1 {
				what += " (and 1 line under it)"
			} else {
				what += fmt.Sprintf(" (and %d lines under it)", len(nested))
			}
		}
		o.changes = append(o.changes, Change{Line: u.start + 1, What: what, Fix: f, Removed: nested})
	}

	if sel[FixEmptyFamilies] {
		fams := a.facts.families
		if sel[FixBrokenLinks] {
			fams = a.facts.familiesWithBroken
		} else if len(a.facts.familiesWithBroken) > len(fams) {
			o.skipReason[FixEmptyFamilies] = reasonNeedsBroken // none can go alone
			o.partial[FixEmptyFamilies] = reasonSomeFamilies   // some can
		}
		for _, fam := range fams {
			ok := canDrop(fam.unit)
			for _, l := range fam.links {
				ok = ok && canDrop(l)
			}
			if !ok {
				continue // all or nothing: a half-removed family is a broken link
			}
			drop(fam.unit, "removed an empty family "+fam.xref, FixEmptyFamilies, false)
			for _, l := range fam.links {
				drop(l, "removed a link to the removed family "+fam.xref, FixEmptyFamilies, false)
			}
		}
	}
	if sel[FixBrokenLinks] {
		for _, b := range a.facts.broken {
			if p.removed[b.start] || !canDrop(b.unit) {
				continue
			}
			drop(b.unit, fmt.Sprintf("removed a link to %s %s, which doesn't exist", b.kind, b.xref), FixBrokenLinks, true)
		}
	}
	if sel[FixSubmitter] && a.facts.missingSubm {
		a.planSubmitter(name, o, p)
	}
}

// cleanName is the name with zero-width characters (U+200B-U+200D, U+FEFF)
// and other control characters removed, every run of white space (tabs,
// line breaks, U+00A0, ...) made one space, and no space at either end.
func cleanName(s string) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		switch {
		case r >= '\u200b' && r <= '\u200d', r == '\ufeff':
			continue
		case unicode.IsSpace(r):
			space = true
			continue
		case unicode.IsControl(r):
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	return b.String()
}

// encodeName writes name in the output encoding, or reports false.
func (a *analysis) encodeName(name string, conv bool) (string, bool) {
	if conv || a.detected == "UTF-8" {
		return name, true
	}
	ascii := true
	for i := 0; i < len(name); i++ {
		ascii = ascii && name[i] < utf8.RuneSelf
	}
	if ascii {
		return name, true
	}
	var cm *charmap.Charmap
	switch a.detected {
	case "Windows-1252":
		cm = charmap.Windows1252
	case "CP437":
		cm = charmap.CodePage437
	case "MacRoman":
		cm = charmap.Macintosh
	default: // ASCII, ANSEL: ASCII names only
		return "", false
	}
	out, err := cm.NewEncoder().String(name)
	if err != nil {
		return "", false
	}
	return out, true
}

// planSubmitter adds "1 SUBM @SUBMn@" to HEAD (after CHAR, or at the end
// of HEAD) and "0 @SUBMn@ SUBM" / "1 NAME name" before the trailer.
func (a *analysis) planSubmitter(name string, o *outcome, p *plan) {
	name = cleanName(name)
	if name == "" {
		o.skipReason[FixSubmitter] = reasonNeedsName
		return
	}
	if utf8.RuneCountInString(name) > 120 {
		o.skipReason[FixSubmitter] = reasonNameLong
		return
	}
	raw, ok := a.encodeName(name, o.conv)
	if !ok {
		o.skipReason[FixSubmitter] = reasonNeedsUTF8
		return
	}
	if len("1 NAME ")+len(raw) > maxLine || len("1 NAME ")+len(name) > maxLine {
		o.skipReason[FixSubmitter] = reasonNameLong
		return
	}
	xref := ""
	for k := 1; xref == ""; k++ {
		if x := "@SUBM" + strconv.Itoa(k) + "@"; !a.xrefInUse(x) {
			xref = x
		}
	}
	d := a.facts
	pointerLine := a.head + 1
	if d.emptySubm >= 0 { // fill in the empty "1 SUBM" line
		p.replace[d.emptySubm] = "1 SUBM " + xref
		pointerLine = d.emptySubm + 1
		o.touched.mark(FixSubmitter, pointerLine)
	} else {
		p.insertAfter[d.subAfter] = append(p.insertAfter[d.subAfter], insLine{"1 SUBM " + xref, "1 SUBM " + xref})
		o.touched.mark(FixSubmitter, -1)
	}
	rec := []insLine{{"0 " + xref + " SUBM", "0 " + xref + " SUBM"}, {"1 NAME " + raw, "1 NAME " + name}}
	recLine := len(a.src) + 1
	if d.trlr >= 0 {
		p.insertBefore[d.trlr] = append(p.insertBefore[d.trlr], rec...)
		recLine = d.trlr + 1
	} else {
		p.appendEnd = append(p.appendEnd, rec...)
	}
	o.subPointer, o.subRecord = "1 SUBM "+xref, []string{rec[0].dec, rec[1].dec}
	o.changes = append(o.changes,
		Change{Line: pointerLine, What: fmt.Sprintf("added you as the person who made this file (%s)", name), Fix: FixSubmitter},
		Change{Line: recLine, What: "added a submitter record for " + name, Fix: FixSubmitter})
	for k := -2; k >= -3; k-- { // the record's two added lines, with no line number of their own
		o.touched.mark(FixSubmitter, k)
	}
}

// xrefInUse reports whether x is a level-0 xref or the value of a line (any
// tag but CONT/CONC) of the file: a new record must not take it.
func (a *analysis) xrefInUse(x string) bool {
	if a.facts.defined[x] > 0 {
		return true
	}
	for i := range a.nodes {
		if v, ok := a.xrefLike(i); ok && v == x {
			return true
		}
	}
	return false
}
