// Command gedvalidate parses a GEDCOM file and prints validator findings (for local testing).
// Usage: go run ./cmd/gedvalidate /path/to/file.ged
package main

import (
	"fmt"
	"os"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/validator"
)

func main() {
	path := "/apps/tmp/gedcom-export-6791c94e-a2a7-43c1-b73f-32cc0cb164e9.ged"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open: %v\n", err)
		os.Exit(2)
	}
	defer f.Close()

	doc, warns, err := parser.Parse(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse: %v\n", err)
		os.Exit(2)
	}
	fmt.Printf("file: %s\n", path)
	fmt.Printf("parser warnings: %d\n", len(warns))

	errs := validator.Validate(doc)
	var nErr, nWarn, nHint int
	for _, e := range errs {
		switch e.Severity {
		case validator.SeverityError:
			nErr++
			fmt.Printf("[error] %s: %s (xref=%q)\n", e.Code, e.Message, e.Xref)
		case validator.SeverityWarning:
			nWarn++
			if nWarn <= 40 {
				fmt.Printf("[warn] %s: %s (xref=%q)\n", e.Code, e.Message, e.Xref)
			}
		case validator.SeverityHint:
			nHint++
		}
	}
	if nWarn > 40 {
		fmt.Printf("... (%d more warnings omitted)\n", nWarn-40)
	}
	fmt.Printf("summary: errors=%d warnings=%d hints=%d\n", nErr, nWarn, nHint)
	if nErr > 0 {
		os.Exit(1)
	}
}
