package repair

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
)

// Long source and citation text lines are split with CONC, now that the
// enricher joins them back (v0.3.0). TITL under OBJE is still read without
// its CONC lines, so it stays.
func TestRepairLongTextTags(t *testing.T) {
	r := strings.Repeat
	in := "0 HEAD\n1 GEDC\n2 VERS 5.5.1\n1 CHAR UTF-8\n" +
		"0 @S1@ SOUR\n" +
		"1 TITL " + r("t", 300) + "\n" + // 6
		"1 AUTH " + r("ab ", 100) + "z\n" + // 7
		"1 PUBL " + r("p", 260) + "\n2 CONT " + r("q", 270) + "\n" + // 8, 9
		"1 ABBR " + r("é", 140) + "\n" + // 10
		"1 TEXT " + r("x", 248) + "\n2 CONC " + r("y", 300) + "\n" + // 11 (255 bytes), 12
		"0 @I1@ INDI\n1 NAME A /B/\n" +
		"1 SOUR @S1@\n2 PAGE " + r("1", 280) + "\n2 DATA\n3 TEXT " + r("d", 290) + "\n" + // 16, 18
		"1 BIRT\n2 SOUR @S1@\n3 PAGE " + r("2", 270) + "\n" + // 21
		"1 OBJE\n2 FILE x.jpg\n2 TITL " + r("m", 270) + "\n" + // 24
		"0 TRLR\n"
	res, err := Repair([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	split := map[int]bool{}
	for _, c := range res.Changes {
		if c.Fix == FixLongLines {
			split[c.Line] = true
		}
	}
	for _, n := range []int{6, 7, 8, 9, 10, 12, 16, 18, 21} {
		if !split[n] {
			t.Errorf("line %d not split (changes %v)", n, res.Changes)
		}
	}
	if split[24] || split[11] {
		t.Errorf("OBJE TITL or a short line split: %v", res.Changes)
	}
	if a, b := enrichedJSON(t, []byte(in)), enrichedJSON(t, res.Data); a != b {
		t.Errorf("enriched content changed\n%s", firstDiff(a, b))
	}
	long := 0
	for _, l := range lines(res.Data) {
		if len(l.text) > 255 {
			long++
			if !strings.HasPrefix(l.text, "2 TITL m") {
				t.Errorf("long line left: %.40q", l.text)
			}
		}
	}
	if long != 1 {
		t.Errorf("%d long lines left, want the OBJE TITL", long)
	}
	again, _ := Repair(res.Data)
	if len(again.Changes) != 0 {
		t.Errorf("second repair %v", again.Changes)
	}
}

// pres2020's long PAGE lines are split, and no LINE_TOO_LONG is left for them.
func TestRepairPres2020Pages(t *testing.T) {
	res, _ := repairFixture(t, "pres2020.ged")
	pages := 0
	for _, c := range res.Changes {
		if c.Fix == FixLongLines {
			pages++
		}
	}
	if pages != 23 {
		t.Errorf("%d lines split, want the 23 PAGE lines", pages)
	}
	opts := parser.DefaultOptions()
	opts.Lenient = true
	_, findings, _ := parser.ParseWithOptions(bytes.NewReader(res.Data), opts)
	for _, f := range findings {
		if f.Code == "LINE_TOO_LONG" {
			t.Errorf("LINE_TOO_LONG left on line %d", f.Line)
		}
	}
	assertLeftHas(t, res, "LINE_TOO_LONG", false)
}
