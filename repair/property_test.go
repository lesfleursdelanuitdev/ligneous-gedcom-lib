package repair

import (
	"fmt"
	"math/rand"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"
)

// genFile builds a random GEDCOM file from seed: mixed encodings, byte
// order marks, CR/LF mixes, blank and U+00A0 lines, leading spaces, long
// lines, characters split across CONC, CHAR past the 50-line window, a
// missing TRLR, broken pointers of every kind, empty / duplicate /
// data-holding families, _FAM, an empty, missing or broken HEAD SUBM, and
// @SUBM1@ already taken. Text is written in UTF-8 with "é" and encoded at
// the end.
func genFile(seed int64) []byte {
	r := rand.New(rand.NewSource(seed))
	pick := func(p float64) bool { return r.Float64() < p }
	enc := []string{"utf8", "utf8", "ansel", "ansi", "mixed", "cp437", "mac", "none", "utf16", "bomstray"}[r.Intn(10)]
	bom := (enc == "utf8" || enc == "mixed" || enc == "none") && pick(0.3) || enc == "bomstray"
	var ls []string
	add := func(s ...string) { ls = append(ls, s...) }
	maybeBlank := func() {
		switch {
		case pick(0.08):
			add("")
		case pick(0.04):
			add(" ") // a blank line in U+00A0
		}
	}
	ind := func() string {
		if pick(0.1) {
			return []string{" ", "  ", "\t"}[r.Intn(3)]
		}
		return ""
	}
	if pick(0.05) {
		add("junk before the header")
	}
	add("0 HEAD")
	if pick(0.9) {
		add("1 GEDC", "2 VERS "+[]string{"5.5.1", "5.5", "7.0"}[r.Intn(3)])
	}
	char := map[string]string{"utf8": "UTF-8", "ansel": "ANSEL", "ansi": "ANSI", "mixed": "UTF-8", "cp437": "IBMPC", "mac": "MACINTOSH", "utf16": "UNICODE"}[enc]
	if pick(0.1) { // CHAR past the 50-line window
		for i := 0; i < 52; i++ {
			add("1 NOTE head " + strconv.Itoa(i))
		}
	}
	if char != "" && pick(0.85) {
		add(ind() + "1 CHAR " + char)
	}
	maybeBlank()
	switch r.Intn(4) {
	case 0:
		add("1 SUBM @U1@")
	case 1:
		add("1 SUBM")
	case 2:
		add("1 SUBM @U9@") // broken
	}
	add("0 @U1@ SUBM", "1 NAME Me")
	if pick(0.3) {
		add("0 @SUBM1@ NOTE taken")
	}
	nI, nF := 2+r.Intn(4), 2+r.Intn(4)
	fam := func() string { return "@F" + strconv.Itoa(1+r.Intn(nF+1)) + "@" } // F(nF+1) is missing
	per := func() string { return "@I" + strconv.Itoa(1+r.Intn(nI+1)) + "@" }
	for i := 1; i <= nI; i++ {
		add(ind() + "0 @I" + strconv.Itoa(i) + "@ INDI")
		if pick(0.5) {
			add(ind() + "1 NAME Ren" + "é" + " /Dupr" + "é" + "/")
		} else {
			add(ind() + "1 NAME Dupr" + "é") // ends in a non-ASCII byte
		}
		maybeBlank()
		for k := r.Intn(3); k > 0; k-- {
			add(ind() + []string{"1 FAMS ", "1 FAMC "}[r.Intn(2)] + fam())
			if pick(0.3) {
				add("2 " + []string{"PEDI birth", "NOTE @N9@", "NOTE linked"}[r.Intn(3)])
			}
			maybeBlank()
		}
		if pick(0.2) {
			add("1 _FAM " + fam())
		}
		if pick(0.4) {
			add("1 BIRT", "2 DATE 1 JAN 1900", "2 SOUR "+[]string{"@S1@", "@S9@"}[r.Intn(2)], "3 PAGE p. "+strings.Repeat("9", r.Intn(300)))
		}
		if pick(0.3) {
			add("1 NOTE see", "2 CONT @I9@", "2 CONC @X9@")
		}
		for _, tag := range []string{"ASSO", "ALIA", "OBJE", "REPO", "SUBM", "NOTE"} {
			if pick(0.06) {
				add("1 "+tag+" @Z"+strconv.Itoa(r.Intn(9))+"@", "2 RELA x")
			}
		}
		if pick(0.15) {
			add("1 NOTE " + strings.Repeat("ab ", 30+r.Intn(120)) + "end")
		}
		if pick(0.1) && (enc == "utf8" || enc == "mixed") {
			add("1 NOTE abc\x00SPLIT", "2 CONC \x00REST def") // a character split across CONC
		}
		if pick(0.1) && enc == "mixed" || enc == "bomstray" && i == 1 {
			add("1 NOTE caf\x00STRAY")
		}
	}
	for f := 1; f <= nF; f++ {
		x := "@F" + strconv.Itoa(f) + "@"
		if pick(0.1) {
			x = "@F1@" // duplicate
		}
		add("0 " + x + " FAM")
		switch r.Intn(5) {
		case 0: // empty
		case 1:
			add("1 CHAN", "2 DATE 1 JAN 2000")
		case 2:
			add("1 HUSB "+per(), "1 WIFE "+per())
		case 3:
			add("1 MARR", "2 DATE 1910")
		case 4:
			add("1 "+[]string{"HUSB", "WIFE", "CHIL", "NOTE", "OBJE", "SOUR"}[r.Intn(6)]+" @Q9@", "2 PAGE 4")
		}
		maybeBlank()
	}
	add("0 @S1@ SOUR", "1 TITL "+strings.Repeat("Title ", r.Intn(60))+"x")
	if pick(0.85) {
		add("0 TRLR")
	}
	terms := []string{"\n", "\r\n", "\r"}
	mode := r.Intn(4)
	var b strings.Builder
	for i, l := range ls {
		b.WriteString(l)
		if i == len(ls)-1 && pick(0.2) {
			break
		}
		if mode == 3 {
			b.WriteString(terms[r.Intn(3)])
		} else {
			b.WriteString(terms[mode])
		}
	}
	text := b.String()
	split := map[string][2]string{ // the split character, per encoding
		"utf8": {"\xC3", "\xA9"}, "mixed": {"\xC3", "\xA9"},
	}[enc]
	text = strings.ReplaceAll(text, "\x00SPLIT", split[0])
	text = strings.ReplaceAll(text, "\x00REST", split[1])
	text = strings.ReplaceAll(text, "\x00STRAY", "\xE9")
	switch enc {
	case "ansel":
		text = strings.ReplaceAll(text, "é", "\xE2e")
		text = strings.ReplaceAll(text, " ", " ")
	case "bomstray": // a BOM over one stray byte and no UTF-8 at all
		text = strings.ReplaceAll(text, "é", "e")
		text = strings.ReplaceAll(text, "\u00a0", "")
	case "ansi", "none":
		text = strings.ReplaceAll(text, "é", "\xE9")
		text = strings.ReplaceAll(text, " ", "\xA0")
	case "cp437":
		text = strings.ReplaceAll(text, "é", "\x82")
		text = strings.ReplaceAll(text, " ", "\xFF")
	case "mac":
		text = strings.ReplaceAll(text, "é", "\x8E")
		text = strings.ReplaceAll(text, " ", "\xCA")
	case "utf16":
		u := []byte{0xFF, 0xFE}
		for _, c := range utf16.Encode([]rune(text)) {
			u = append(u, byte(c), byte(c>>8))
		}
		return u
	}
	if bom {
		text = "\xEF\xBB\xBF" + text
	}
	return []byte(text)
}

// Plan matches RepairWithOptions on random files, and every Skipped reason
// holds up (see checkSkipReason).
func TestPlanProperty(t *testing.T) {
	const files = 200
	sels := planSelections()
	failures := 0
	for seed := int64(1); seed <= files && failures < 20; seed++ {
		raw := genFile(seed)
		if _, err := Repair(raw); err != nil {
			continue
		}
		r := rand.New(rand.NewSource(seed * 7919))
		for n := 0; n < 5; n++ {
			opt := sels[r.Intn(len(sels))]
			if n == 4 { // the submitter alone, with a non-ASCII name
				opt = Options{Fixes: []Fix{FixSubmitter}}
			}
			if r.Intn(3) == 0 { // a random subset
				opt.Fixes = nil
				for _, f := range Fixes() {
					if r.Intn(3) == 0 {
						opt.Fixes = append(opt.Fixes, f)
					}
				}
				if opt.Fixes == nil {
					opt.Fixes = []Fix{}
				}
			}
			opt.SubmitterName = []string{"Test Person", "René", "李"}[r.Intn(3)]
			if n == 4 {
				opt.SubmitterName = "René"
			}
			what := fmt.Sprintf("seed %d %s %q", seed, optName(opt), opt.SubmitterName)
			ok := t.Run(what, func(t *testing.T) {
				pr, err := Plan(raw, opt)
				if err != nil {
					t.Fatal(err)
				}
				res, _ := RepairWithOptions(raw, opt)
				if !reflect.DeepEqual(pr.Available, res.Available) {
					t.Errorf("Available\n plan   %+v\n repair %+v", pr.Available, res.Available)
				}
				for _, av := range pr.Available {
					want, _ := RepairWithOptions(raw, Options{Fixes: withKind(opt, av.Fix), SubmitterName: opt.SubmitterName})
					checkUnits(t, string(av.Fix), pr.Units[av.Fix], want, av.Fix)
				}
				applied := map[Fix]int{}
				for _, a := range res.Applied {
					applied[a.Fix] = a.Lines
				}
				fixes := opt.Fixes
				if fixes == nil {
					fixes = slices.Clone(allFixes)
				}
				for _, s := range res.Skipped {
					checkSkipReason(t, "", raw, Options{Fixes: fixes, SubmitterName: opt.SubmitterName}, s, applied[s.Fix])
				}
				if t.Failed() {
					t.Logf("input %q", raw)
				}
			})
			if !ok {
				failures++
			}
		}
	}
}
