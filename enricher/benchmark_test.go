package enricher

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
)

type testFile struct {
	name string
	data []byte
}

func loadTestFiles(b *testing.B) []testFile {
	b.Helper()
	files, err := filepath.Glob("../testdata/*.ged")
	if err != nil {
		b.Fatal(err)
	}
	if len(files) == 0 {
		b.Skip("no testdata found")
	}
	var result []testFile
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			b.Fatal(err)
		}
		result = append(result, testFile{name: filepath.Base(path), data: data})
	}
	return result
}

func BenchmarkParseOnly(b *testing.B) {
	files := loadTestFiles(b)
	for _, tf := range files {
		b.Run(tf.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tf.data)))
			for i := 0; i < b.N; i++ {
				_, _, err := parser.Parse(bytes.NewReader(tf.data))
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkEnrichOnly(b *testing.B) {
	files := loadTestFiles(b)
	for _, tf := range files {
		b.Run(tf.name, func(b *testing.B) {
			doc, _, err := parser.Parse(bytes.NewReader(tf.data))
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				Enrich(doc)
			}
		})
	}
}

func BenchmarkGenerateIDs(b *testing.B) {
	files := loadTestFiles(b)
	for _, tf := range files {
		b.Run(tf.name, func(b *testing.B) {
			doc, _, err := parser.Parse(bytes.NewReader(tf.data))
			if err != nil {
				b.Fatal(err)
			}
			ed := Enrich(doc)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				resetIDs(ed)
				GenerateIDs(ed)
			}
		})
	}
}

func resetIDs(ed *EnrichedDocument) {
	for i := range ed.Individuals {
		ed.Individuals[i].ID = ""
	}
	for i := range ed.Families {
		ed.Families[i].ID = ""
	}
	for i := range ed.Dates {
		ed.Dates[i].ID = ""
	}
	for i := range ed.Places {
		ed.Places[i].ID = ""
	}
	for i := range ed.Surnames {
		ed.Surnames[i].ID = ""
	}
	for i := range ed.GivenNames {
		ed.GivenNames[i].ID = ""
	}
	for i := range ed.Events {
		ed.Events[i].ID = ""
	}
}
