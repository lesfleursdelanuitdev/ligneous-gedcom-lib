// Command gedcomstdin reads a JSON EnrichedDocument from stdin (same shape as
// POST /api/v1/export) and writes GEDCOM text to stdout.
//
// Example:
//
//	node ../ligneous-frontend/scripts/test-gedcom-export.mjs | go run .
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/enricher"
	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/exporter"
)

func main() {
	var payload struct {
		Enriched *enricher.EnrichedDocument `json:"enriched"`
		Format   string                     `json:"format"`
	}
	if err := json.NewDecoder(os.Stdin).Decode(&payload); err != nil {
		fmt.Fprintf(os.Stderr, "decode stdin: %v\n", err)
		os.Exit(1)
	}
	if payload.Enriched == nil {
		fmt.Fprintln(os.Stderr, "missing enriched document (expected {\"enriched\":{...}})")
		os.Exit(1)
	}
	format := payload.Format
	if format == "" {
		format = "gedcom"
	}
	switch format {
	case "gedcom":
		if _, err := os.Stdout.WriteString(exporter.EnrichedToGEDCOMWithOriginal(payload.Enriched)); err != nil {
			fmt.Fprintf(os.Stderr, "write: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unsupported format %q (only gedcom)\n", format)
		os.Exit(1)
	}
}
