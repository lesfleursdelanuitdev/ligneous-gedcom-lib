package validator

import (
	"fmt"
	"strings"
)

// DefaultGEDCOMPhysicalLineMax is the GEDCOM 5.5 recommendation for maximum characters per line
// (excluding the record terminator).
const DefaultGEDCOMPhysicalLineMax = 255

// PhysicalLineLengthWarnings returns one warning per physical line in gedcomUTF8 that exceeds maxRunes.
// Pass the serialized GEDCOM text (e.g. from exporter.ToGEDCOM). Lines are split on \n with \r stripped;
// length is measured in bytes (UTF-8), matching len() in Go and typical GEDCOM tooling.
func PhysicalLineLengthWarnings(gedcomUTF8 string, maxBytes int) []*ValidationError {
	if maxBytes <= 0 {
		maxBytes = DefaultGEDCOMPhysicalLineMax
	}
	normalized := strings.ReplaceAll(gedcomUTF8, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")
	var errs []*ValidationError
	for i, line := range lines {
		if len(line) > maxBytes {
			ve := &ValidationError{
				Severity: SeverityWarning,
				Code:     "LINE_EXCEEDS_MAX_PHYSICAL_LENGTH",
				Message:  fmt.Sprintf("line %d has length %d bytes (limit %d)", i+1, len(line), maxBytes),
			}
			setAssociatedXrefs(ve)
			errs = append(errs, ve)
		}
	}
	return errs
}
