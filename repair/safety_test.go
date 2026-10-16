package repair

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

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

func TestRepairKeepsUnchangedLines(t *testing.T) {
	for _, path := range sampleFiles(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, res := readAndRepair(t, path)
			decoded, _, _ := parser.Decode(raw)
			touched := map[int]bool{}
			for _, c := range res.Changes {
				touched[c.Line] = true
			}
			out := texts(lines(res.Data))
			j := 0
			for i, l := range lines(decoded) {
				if touched[i+1] {
					continue
				}
				for j < len(out) && out[j] != l.text {
					j++
				}
				if j == len(out) {
					t.Fatalf("original line %d %q is missing or out of order", i+1, l.text)
				}
				j++
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
