package repair

import "strings"

// PlanResult previews what each fix would change, without writing the file.
type PlanResult struct {
	// Available is RepairWithOptions's Available for the same file.
	Available []FixInfo `json:"available"`
	// Units holds, for each available kind, what it would change if chosen
	// together with Options.Fixes: exactly the Changes RepairWithOptions
	// makes for that kind with opt.Fixes plus the kind.
	Units map[Fix][]Unit `json:"units"`
}

// Unit is one change a fix would make, as its Change would say it.
type Unit struct {
	Line int    `json:"line"` // original line of the unit's first line (0 = whole file)
	What string `json:"what"` // the Change's text
	// Lines are the exact lines the change removes (data kinds: the lines
	// under the removed one, as Change.Removed; blank-lines: "") or adds
	// (submitter, trailer, CHAR, the pieces of a split line).
	Lines []string `json:"lines,omitempty"`
}

// Hooks for tests: called when a file is written or checked again. Plan
// does neither.
var onEmit, onLeftCodes func()

// Plan previews RepairWithOptions: for each kind the file could use, what it
// would change if chosen together with opt.Fixes (nil: every format kind).
// It runs the same planning as the repair but writes nothing and does not
// check the result again, so it assumes the check after a repair without
// the conversion passes (it always has on the test files). It costs a few
// plans: the every-format-fix plan (for Available and for the encoding
// kind), one for opt.Fixes with the format kinds that do not change one
// another, a second when blank-lines or leading-space would change another
// chosen kind's result (line-endings, long-lines), and the data kinds' own
// planning for each data kind not chosen.
func Plan(data []byte, opt Options) (*PlanResult, error) {
	sel, err := selected(opt)
	if err != nil {
		return nil, err
	}
	a, err := analyse(data)
	if err != nil {
		return nil, err
	}
	full := a.plan(formatFixes(), "")
	avail := a.available(full.touched)
	res := &PlanResult{Available: avail, Units: map[Fix][]Unit{}}
	offered := fixSet{}
	for _, av := range avail {
		offered[av.Fix] = true
	}

	// x1: opt.Fixes and every offered format kind that changes no other
	// kind's result there; x2: blank-lines / leading-space when they would.
	x1, x2 := cloneSet(sel), fixSet{}
	for _, f := range formatKinds {
		if !offered[f] || sel[f] || f == FixEncoding || f == FixBlankLines || f == FixLeadingSpace {
			continue
		}
		x1[f] = true
	}
	for _, pair := range [][2]Fix{{FixBlankLines, FixLineEndings}, {FixLeadingSpace, FixLongLines}} {
		e, affected := pair[0], pair[1]
		if !offered[e] || sel[e] {
			continue
		}
		if x1[affected] {
			x2[e] = true
		} else {
			x1[e] = true
		}
	}
	from := map[Fix]*outcome{}
	o1 := a.plan(x1, opt.SubmitterName)
	for f := range x1 {
		from[f] = o1
	}
	if len(x2) > 0 {
		with := cloneSet(sel)
		for f := range x2 {
			with[f] = true
		}
		o2 := a.plan(with, opt.SubmitterName)
		for f := range x2 {
			from[f] = o2
		}
	}
	if offered[FixEncoding] && !sel[FixEncoding] {
		from[FixEncoding] = full // nothing else changes the encoding kind's result
	}
	for _, f := range dataKinds {
		if offered[f] && !sel[f] {
			with := cloneSet(sel)
			with[f] = true
			from[f] = a.planWith(with, opt.SubmitterName, true)
		}
	}
	for _, av := range avail {
		o := from[av.Fix]
		units := []Unit{}
		if o != nil {
			units = o.unitsOf(av.Fix)
		}
		res.Units[av.Fix] = units
	}
	return res, nil
}

func cloneSet(s fixSet) fixSet {
	c := fixSet{}
	for f := range s {
		c[f] = true
	}
	return c
}

// unitsOf is o's Changes of kind f as Units.
func (o *outcome) unitsOf(f Fix) []Unit {
	out := []Unit{}
	for _, c := range o.changes {
		if c.Fix != f {
			continue
		}
		u := Unit{Line: c.Line, What: c.What}
		switch {
		case kindOf(f) == KindData && strings.HasPrefix(c.What, "added you"):
			u.Lines = []string{o.subPointer}
		case kindOf(f) == KindData && strings.HasPrefix(c.What, "added a submitter"):
			u.Lines = o.subRecord
		case kindOf(f) == KindData:
			u.Lines = c.Removed
		case f == FixBlankLines:
			u.Lines = []string{""}
		case f == FixTrailer:
			u.Lines = []string{"0 TRLR"}
		case c.What == whatCharset:
			u.Lines = []string{"1 CHAR UTF-8"}
		case f == FixLongLines && c.Line > 0:
			u.Lines = o.p.splits[c.Line-1]
		}
		out = append(out, u)
	}
	return out
}
