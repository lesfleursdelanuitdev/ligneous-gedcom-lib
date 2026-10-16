package enricher

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
)

const goldenPath = "testdata/enrich_golden.txt"

// enrichDigests returns "name length sha256" for the JSON of Enrich(doc) with
// Document nil, for every testdata/*.ged and testdata/malformed/*.ged file.
func enrichDigests(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, pat := range []string{"../testdata/*.ged", "../testdata/malformed/*.ged"} {
		m, err := filepath.Glob(pat)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	sort.Strings(files)
	if len(files) == 0 {
		t.Fatal("no testdata files")
	}
	var out []string
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		doc, _, err := parser.Parse(bytes.NewReader(data))
		if err != nil || doc == nil {
			out = append(out, fmt.Sprintf("%s parse-error", strings.TrimPrefix(f, "../testdata/")))
			continue
		}
		ed := Enrich(doc)
		ed.Document = nil
		js, err := json.Marshal(ed)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(js)
		out = append(out, fmt.Sprintf("%s %d %s", strings.TrimPrefix(f, "../testdata/"), len(js), hex.EncodeToString(sum[:])))
	}
	return out
}

// TestEnrichMatchesGolden compares Enrich output against digests recorded at
// master 29d699c (before the xref index). Regenerate with UPDATE_GOLDEN=1 only
// when output is meant to change.
func TestEnrichMatchesGolden(t *testing.T) {
	got := strings.Join(enrichDigests(t), "\n") + "\n"
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Skip("golden written")
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("missing golden (run UPDATE_GOLDEN=1 at the baseline): %v", err)
	}
	if string(want) != got {
		t.Fatalf("Enrich output changed\nwant:\n%s\ngot:\n%s", want, got)
	}
}

// TestEnrichGeneratedMatchesGolden covers events, notes and sources on a
// generated document, whose digest was also recorded at the baseline.
func TestEnrichGeneratedMatchesGolden(t *testing.T) {
	ed := Enrich(genLargeDoc(300))
	ed.Document = nil
	js, _ := json.Marshal(ed)
	sum := sha256.Sum256(js)
	got := fmt.Sprintf("gen300 %d %s\n", len(js), hex.EncodeToString(sum[:]))
	path := "testdata/enrich_gen_golden.txt"
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Skip("golden written")
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != got {
		t.Fatalf("want %s got %s", want, got)
	}
}
