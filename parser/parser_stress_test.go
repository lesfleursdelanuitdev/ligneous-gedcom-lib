package parser

import (
	"os"
	"testing"
)

var stressFiles = []string{
	"../../ENFAN22.GED",
	"../../Married1200.ged",
	"../../Children1200.ged",
	"../../Long26CC.ged",
	"../../Long26LL.ged",
	"../../Siblings1200.ged",
}

func TestStressRealFiles(t *testing.T) {
	for _, path := range stressFiles {
		path := path
		t.Run(path, func(t *testing.T) {
			f, err := os.Open(path)
			if err != nil {
				t.Skipf("file not found: %v", err)
			}
			defer f.Close()

			doc, warns, err := Parse(f)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			t.Logf("individuals=%d families=%d sources=%d notes=%d parser_warnings=%d",
				doc.IndividualCount(), doc.FamilyCount(),
				len(doc.Sources), len(doc.Notes), len(warns))
		})
	}
}

func BenchmarkParseENFAN22(b *testing.B)      { benchmarkFile(b, "../../ENFAN22.GED") }
func BenchmarkParseMarried1200(b *testing.B)  { benchmarkFile(b, "../../Married1200.ged") }
func BenchmarkParseChildren1200(b *testing.B) { benchmarkFile(b, "../../Children1200.ged") }
func BenchmarkParseLong26CC(b *testing.B)     { benchmarkFile(b, "../../Long26CC.ged") }
func BenchmarkParseLong26LL(b *testing.B)     { benchmarkFile(b, "../../Long26LL.ged") }
func BenchmarkParseSiblings1200(b *testing.B) { benchmarkFile(b, "../../Siblings1200.ged") }

func benchmarkFile(b *testing.B, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		b.Skipf("file not found: %v", err)
	}
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f, _ := os.Open(path)
		_, _, _ = Parse(f)
		f.Close()
	}
}
