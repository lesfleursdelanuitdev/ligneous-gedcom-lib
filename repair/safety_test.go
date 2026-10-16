package repair

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/enricher"
	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
)

// sampleFiles is every .ged file the safety tests run on.
func sampleFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, pattern := range []string{"*.ged", "encodings/*.ged", "malformed/*.ged", "repair/*.ged"} {
		m, err := filepath.Glob(filepath.Join("..", "testdata", pattern))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	sort.Strings(files)
	if len(files) < 20 {
		t.Fatalf("only %d sample files found", len(files))
	}
	return files
}

func readAndRepair(t *testing.T, path string) ([]byte, *Result) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Repair(raw)
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}
	return raw, res
}

// enrichedJSON parses data leniently, enriches it and returns the JSON with
// the Document and every "line" key removed.
func enrichedJSON(t *testing.T, data []byte) string {
	t.Helper()
	opts := parser.DefaultOptions()
	opts.Lenient = true
	doc, _, err := parser.ParseWithOptions(bytes.NewReader(data), opts)
	if err != nil {
		t.Fatalf("lenient parse: %v", err)
	}
	ed := enricher.Enrich(doc)
	ed.Document = nil
	b, err := json.Marshal(ed)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	stripLine(v)
	out, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func stripLine(v any) {
	switch x := v.(type) {
	case map[string]any:
		delete(x, "line")
		for _, c := range x {
			stripLine(c)
		}
	case []any:
		for _, c := range x {
			stripLine(c)
		}
	}
}

// firstDiff returns a short excerpt around the first difference of a and b.
func firstDiff(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	from := max(0, i-200)
	return "original: …" + a[from:min(len(a), i+200)] + "\nrepaired: …" + b[from:min(len(b), i+200)]
}

func TestRepairPreservesContent(t *testing.T) {
	for _, path := range sampleFiles(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, res := readAndRepair(t, path)
			before, after := enrichedJSON(t, raw), enrichedJSON(t, res.Data)
			if before != after {
				t.Errorf("enriched content changed (changes %v)\n%s", res.Changes, firstDiff(before, after))
			}
		})
	}
}

func TestRepairIsIdempotent(t *testing.T) {
	for _, path := range sampleFiles(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			_, res := readAndRepair(t, path)
			again, err := Repair(res.Data)
			if err != nil {
				t.Fatal(err)
			}
			if len(again.Changes) != 0 {
				t.Errorf("second repair changed %v", again.Changes)
			}
			if !bytes.Equal(again.Data, res.Data) {
				t.Error("second repair changed the data")
			}
			if strings.Join(again.Left, ",") != strings.Join(res.Left, ",") {
				t.Errorf("Left %v, then %v", res.Left, again.Left)
			}
		})
	}
}

// TestRepairKeepsUnchangedLines walks the original and the repaired lines in
// lockstep. Each original line maps to its output: identical when untouched,
// without leading whitespace, absent when blank, its pieces when split, or
// "1 CHAR UTF-8" when rewritten. The only inserted lines are CHAR and TRLR.
func TestRepairKeepsUnchangedLines(t *testing.T) {
	for _, path := range sampleFiles(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, res := readAndRepair(t, path)
			decoded, _, _ := parser.Decode(raw)
			orig := lines(decoded)
			out := texts(lines(res.Data))
			whats := map[int][]string{}
			for _, c := range res.Changes {
				whats[c.Line] = append(whats[c.Line], c.What)
			}
			has := func(line int, prefix string) (string, bool) {
				for _, w := range whats[line] {
					if strings.HasPrefix(w, prefix) {
						return w, true
					}
				}
				return "", false
			}
			want := 0
			charInsert := false
			j := 0
			next := func() string {
				if j >= len(out) {
					t.Fatalf("output ended early")
				}
				j++
				return out[j-1]
			}
			for i, l := range orig {
				n := i + 1
				if _, ok := has(n, "removed a blank line"); ok {
					continue
				}
				text := l.text
				if _, ok := has(n, "removed spaces"); ok {
					text = strings.TrimLeft(text, " \t")
				}
				if charInsert && j < len(out) && out[j] == "1 CHAR UTF-8" && text != out[j] {
					j++ // the inserted CHAR line
					want++
					charInsert = false
				}
				switch w, split := has(n, "split a "); {
				case split:
					var chars, k int
					if _, err := fmt.Sscanf(w, "split a %d-character line into %d lines", &chars, &k); err != nil {
						t.Fatal(err)
					}
					if chars != utf8.RuneCountInString(l.text) {
						t.Errorf("line %d: %q, but the line has %d characters", n, w, utf8.RuneCountInString(l.text))
					}
					joined := next()
					for p := 1; p < k; p++ {
						piece := next()
						at := strings.Index(piece, " CONC ")
						if at < 0 {
							t.Fatalf("line %d piece %d is not CONC: %q", n, p+1, piece)
						}
						joined += piece[at+len(" CONC "):]
					}
					if joined != text {
						t.Errorf("line %d: pieces do not join back", n)
					}
					want += k
				default:
					_, setChar := has(n, "set the character set")
					if setChar && strings.Contains(text, "CHAR") && !strings.Contains(text, "HEAD") {
						text = "1 CHAR UTF-8"
					} else if setChar {
						charInsert = true
					}
					if got := next(); got != text {
						t.Fatalf("original line %d %q became %q", n, l.text, got)
					}
					want++
				}
			}
			if charInsert && j < len(out) && out[j] == "1 CHAR UTF-8" {
				j++
				want++
				charInsert = false
			}
			if charInsert {
				t.Error("CHAR insertion reported but not found")
			}
			if _, ok := has(len(orig)+1, "added the end-of-file marker"); ok {
				if next() != "0 TRLR" {
					t.Error("TRLR not added at the end")
				}
				want++
			}
			if j != len(out) || len(out) != want {
				t.Errorf("output has %d lines, want %d (matched %d)", len(out), want, j)
			}
		})
	}
}

func TestRepairKeepsCustomRecords(t *testing.T) {
	_, res := readAndRepair(t, filepath.Join("..", "testdata", "repair", "custom-tags.ged"))
	want := "0 @X1@ _EVDEF\n1 NAME custom\n2 _ROLE witness\n"
	if !strings.Contains(string(res.Data), want) {
		t.Errorf("custom record lost:\n%s", res.Data)
	}
	for _, s := range []string{"1 _UID 0123456789ABCDEF\n", "2 _MILT Army\n"} {
		if !strings.Contains(string(res.Data), s) {
			t.Errorf("%q lost", s)
		}
	}
}
