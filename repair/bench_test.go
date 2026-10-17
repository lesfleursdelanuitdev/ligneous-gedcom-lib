package repair

import (
	"os"
	"path/filepath"
	"testing"
)

// benchFile is REPAIR_BENCH_FILE, or royal92.ged.
func benchFile(b *testing.B) []byte {
	path := os.Getenv("REPAIR_BENCH_FILE")
	if path == "" {
		path = filepath.Join("..", "testdata", "royal92.ged")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}
	return raw
}

func BenchmarkRepairAll(b *testing.B) {
	raw := benchFile(b)
	b.SetBytes(int64(len(raw)))
	for i := 0; i < b.N; i++ {
		if _, err := Repair(raw); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRepairSubset(b *testing.B) {
	raw := benchFile(b)
	b.SetBytes(int64(len(raw)))
	opt := Options{Fixes: []Fix{FixEncoding, FixBlankLines, FixLeadingSpace, FixTrailer}}
	for i := 0; i < b.N; i++ {
		if _, err := RepairWithOptions(raw, opt); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRepairSubsetRaw(b *testing.B) {
	raw := benchFile(b)
	b.SetBytes(int64(len(raw)))
	opt := Options{Fixes: []Fix{FixBlankLines, FixLeadingSpace, FixTrailer}}
	for i := 0; i < b.N; i++ {
		if _, err := RepairWithOptions(raw, opt); err != nil {
			b.Fatal(err)
		}
	}
}
