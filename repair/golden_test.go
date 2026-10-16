package repair

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// The goldens pin what Repair did in v0.2.1 (master ee009e5): the repaired
// bytes (by hash and length), the Changes and Left, for every sample file
// and every in-memory unreadable input. They were recorded on ee009e5 with
//
//	go test ./repair -run TestRepairGolden -update-golden
//
// and must never be re-recorded to make a failing test pass.
var updateGolden = flag.Bool("update-golden", false, "rewrite testdata/golden-v0.2.1.json")

const goldenPath = "testdata/golden-v0.2.1.json"

type goldenChange struct {
	Line int    `json:"line"`
	What string `json:"what"`
}

type goldenEntry struct {
	SHA256  string         `json:"sha256,omitempty"`
	Len     int            `json:"len"`
	Changes []goldenChange `json:"changes"`
	Left    []string       `json:"left"`
	Err     string         `json:"err,omitempty"`
}

// goldenInputs is every input the goldens cover, by name.
func goldenInputs(t *testing.T) map[string][]byte {
	t.Helper()
	in := map[string][]byte{}
	for _, path := range sampleFiles(t) {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		rel, _ := filepath.Rel(filepath.Join("..", "testdata"), path)
		in[filepath.ToSlash(rel)] = raw
	}
	for name, raw := range unreadableInputs() {
		in["unreadable/"+name] = raw
	}
	return in
}

func goldenOf(res *Result, err error) goldenEntry {
	if err != nil {
		return goldenEntry{Err: err.Error()}
	}
	sum := sha256.Sum256(res.Data)
	e := goldenEntry{SHA256: hex.EncodeToString(sum[:]), Len: len(res.Data), Left: res.Left}
	e.Changes = []goldenChange{}
	for _, c := range res.Changes {
		e.Changes = append(e.Changes, goldenChange{c.Line, c.What})
	}
	return e
}

func TestRepairGolden(t *testing.T) {
	inputs := goldenInputs(t)
	if *updateGolden {
		out := map[string]goldenEntry{}
		for name, raw := range inputs {
			out[name] = goldenOf(Repair(raw))
		}
		b, err := json.MarshalIndent(out, "", " ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	b, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]goldenEntry
	if err := json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(inputs))
	for name := range inputs {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(want) != len(names) {
		t.Errorf("goldens cover %d inputs, the test has %d", len(want), len(names))
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			w, ok := want[name]
			if !ok {
				t.Fatalf("no golden for %s", name)
			}
			for _, run := range goldenRuns() {
				got := goldenOf(run.f(inputs[name]))
				if !reflect.DeepEqual(got, w) {
					t.Errorf("%s:\n got  %+v\n want %+v", run.name, got, w)
				}
			}
		})
	}
}
