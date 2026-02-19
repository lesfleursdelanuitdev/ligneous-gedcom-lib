package enricher

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// ParsePlaceString splits a GEDCOM place string (comma-separated hierarchy)
// into structured components. The convention is most-specific to least-specific:
// City, County, State, Country.
func ParsePlaceString(original string) ParsedPlace {
	pp := ParsedPlace{
		Original: original,
	}

	trimmed := strings.TrimSpace(original)
	if trimmed == "" {
		pp.Hash = hashPlaceFields(pp)
		return pp
	}

	parts := strings.Split(trimmed, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}

	// Remove empty trailing parts
	for len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}

	// Assign from most-specific (index 0) to least-specific based on count.
	// Common patterns:
	//   1 part:  "Country" or "City"
	//   2 parts: "City, Country"
	//   3 parts: "City, State, Country"
	//   4 parts: "City, County, State, Country"
	//   5+ parts: extra specificity at the front, last 4 are city/county/state/country
	switch {
	case len(parts) >= 4:
		pp.Name = parts[0]
		pp.County = parts[len(parts)-3]
		pp.State = parts[len(parts)-2]
		pp.Country = parts[len(parts)-1]
	case len(parts) == 3:
		pp.Name = parts[0]
		pp.State = parts[1]
		pp.Country = parts[2]
	case len(parts) == 2:
		pp.Name = parts[0]
		pp.Country = parts[1]
	case len(parts) == 1:
		pp.Name = parts[0]
	}

	pp.Hash = hashPlaceFields(pp)
	return pp
}

func hashPlaceFields(pp ParsedPlace) string {
	raw := fmt.Sprintf("%s|%s|%s|%s|%s",
		pp.Original, pp.Name, pp.County, pp.State, pp.Country)
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum[:16])
}
