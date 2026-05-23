package validator

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/enricher"
	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
)

type benchGedFile struct {
	name string
	data []byte
}

func loadBenchGedFiles(b *testing.B) []benchGedFile {
	b.Helper()
	paths, err := filepath.Glob("../testdata/*.ged")
	if err != nil {
		b.Fatal(err)
	}
	if len(paths) == 0 {
		b.Skip("no testdata found")
	}
	var out []benchGedFile
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			b.Fatal(err)
		}
		out = append(out, benchGedFile{name: filepath.Base(path), data: data})
	}
	return out
}

func BenchmarkParseAndValidate(b *testing.B) {
	for _, tf := range loadBenchGedFiles(b) {
		b.Run(tf.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tf.data)))
			for i := 0; i < b.N; i++ {
				doc, _, err := parser.Parse(bytes.NewReader(tf.data))
				if err != nil {
					b.Fatal(err)
				}
				Validate(doc)
			}
		})
	}
}

func BenchmarkParseValidateEnrich(b *testing.B) {
	for _, tf := range loadBenchGedFiles(b) {
		b.Run(tf.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tf.data)))
			for i := 0; i < b.N; i++ {
				doc, _, err := parser.Parse(bytes.NewReader(tf.data))
				if err != nil {
					b.Fatal(err)
				}
				Validate(doc)
				enricher.Enrich(doc)
			}
		})
	}
}
