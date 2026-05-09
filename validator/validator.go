// Package validator validates GedcomDocument instances against GEDCOM
// specification rules: structural correctness, required fields, cross-reference
// integrity, and optionally date consistency.
//
// Usage:
//
//	errs := validator.Validate(doc)
//	errs := validator.ValidateWithOptions(doc, &validator.Options{DateConsistency: true})
package validator

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/gedcom"
)

// Severity indicates the importance of a validation finding.
type Severity int

const (
	SeverityHint    Severity = iota // Informational, non-blocking
	SeverityWarning                 // Potentially problematic
	SeverityError                   // Structural problem
)

func (s Severity) String() string {
	switch s {
	case SeverityHint:
		return "hint"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	default:
		return "unknown"
	}
}

// ValidationError describes a single validation finding.
type ValidationError struct {
	Severity Severity
	Code     string
	Message  string
	Xref     string
}

func (e *ValidationError) Error() string {
	if e.Xref != "" {
		return fmt.Sprintf("[%s] %s: %s (xref: %s)", e.Severity, e.Code, e.Message, e.Xref)
	}
	return fmt.Sprintf("[%s] %s: %s", e.Severity, e.Code, e.Message)
}

// Options configures which validation rules to run.
type Options struct {
	DateConsistency bool // Run date consistency checks (birth < death, parent ages, etc.)
	MinParentAge    int  // Minimum age to be a parent (default: 12)
	MaxParentAge    int  // Maximum age to be a parent (default: 80)
	MinMarriageAge  int  // Minimum age for marriage (default: 14)
}

// DefaultOptions returns sensible defaults.
func DefaultOptions() *Options {
	return &Options{
		DateConsistency: false,
		MinParentAge:    12,
		MaxParentAge:    80,
		MinMarriageAge:  14,
	}
}

// Validate runs all validation rules with default options.
func Validate(doc *gedcom.GedcomDocument) []*ValidationError {
	return ValidateWithOptions(doc, DefaultOptions())
}

// ValidateWithOptions runs validation with custom options.
func ValidateWithOptions(doc *gedcom.GedcomDocument, opts *Options) []*ValidationError {
	if opts == nil {
		opts = DefaultOptions()
	}

	var errs []*ValidationError

	errs = append(errs, validateStructure(doc)...)
	errs = append(errs, validateIndividuals(doc)...)
	errs = append(errs, validateFamilies(doc)...)
	errs = append(errs, validateXRefs(doc)...)
	errs = append(errs, validateAssociations(doc)...)

	if opts.DateConsistency {
		errs = append(errs, validateDateConsistency(doc, opts)...)
	}

	return errs
}

// validateStructure checks top-level structural requirements.
func validateStructure(doc *gedcom.GedcomDocument) []*ValidationError {
	var errs []*ValidationError

	if doc.Header.Tag == "" {
		errs = append(errs, &ValidationError{
			Severity: SeverityError,
			Code:     "MISSING_HEADER",
			Message:  "GEDCOM file must start with a HEAD record",
		})
	}

	if doc.Trailer.Tag == "" {
		errs = append(errs, &ValidationError{
			Severity: SeverityWarning,
			Code:     "MISSING_TRAILER",
			Message:  "GEDCOM file should end with a TRLR record",
		})
	}

	return errs
}

// validateIndividuals checks each individual record.
func validateIndividuals(doc *gedcom.GedcomDocument) []*ValidationError {
	var errs []*ValidationError

	for _, indi := range doc.Individuals {
		if indi.Xref == "" {
			errs = append(errs, &ValidationError{
				Severity: SeverityError,
				Code:     "MISSING_XREF",
				Message:  "Individual record must have an xref",
			})
			continue
		}

		names := indi.ChildrenByTag("NAME")
		if len(names) == 0 {
			errs = append(errs, &ValidationError{
				Severity: SeverityWarning,
				Code:     "MISSING_NAME",
				Message:  "Individual record missing NAME tag",
				Xref:     indi.Xref,
			})
		}

		sexChildren := indi.ChildrenByTag("SEX")
		if len(sexChildren) > 0 {
			sex := sexChildren[0].Value
			validSex := map[string]bool{"M": true, "F": true, "U": true, "X": true, "N": true}
			if sex != "" && !validSex[sex] {
				errs = append(errs, &ValidationError{
					Severity: SeverityWarning,
					Code:     "INVALID_SEX",
					Message:  fmt.Sprintf("Invalid SEX value %q, expected M/F/U/X/N", sex),
					Xref:     indi.Xref,
				})
			}
		}
	}

	return errs
}

// validateFamilies checks each family record.
func validateFamilies(doc *gedcom.GedcomDocument) []*ValidationError {
	var errs []*ValidationError

	for _, fam := range doc.Families {
		if fam.Xref == "" {
			errs = append(errs, &ValidationError{
				Severity: SeverityError,
				Code:     "MISSING_XREF",
				Message:  "Family record must have an xref",
			})
			continue
		}

		husb := fam.ChildrenByTag("HUSB")
		wife := fam.ChildrenByTag("WIFE")
		chil := fam.ChildrenByTag("CHIL")

		if len(husb) == 0 && len(wife) == 0 && len(chil) == 0 {
			errs = append(errs, &ValidationError{
				Severity: SeverityWarning,
				Code:     "EMPTY_FAMILY",
				Message:  "Family record has no members (no HUSB, WIFE, or CHIL tags)",
				Xref:     fam.Xref,
			})
		}
	}

	return errs
}

// validateXRefs checks that all cross-references point to existing records.
func validateXRefs(doc *gedcom.GedcomDocument) []*ValidationError {
	var errs []*ValidationError

	idx := doc.XRefIndex()

	checkRef := func(refValue, sourceXref, refTag string) {
		if refValue == "" {
			return
		}
		if _, ok := idx[refValue]; !ok {
			errs = append(errs, &ValidationError{
				Severity: SeverityError,
				Code:     "BROKEN_XREF",
				Message:  fmt.Sprintf("%s reference to non-existent record %s", refTag, refValue),
				Xref:     sourceXref,
			})
		}
	}

	// Check individual references
	for _, indi := range doc.Individuals {
		for _, famc := range indi.ChildrenByTag("FAMC") {
			checkRef(famc.Value, indi.Xref, "FAMC")
		}
		for _, fams := range indi.ChildrenByTag("FAMS") {
			checkRef(fams.Value, indi.Xref, "FAMS")
		}
	}

	// Check family references
	for _, fam := range doc.Families {
		for _, husb := range fam.ChildrenByTag("HUSB") {
			checkRef(husb.Value, fam.Xref, "HUSB")
		}
		for _, wife := range fam.ChildrenByTag("WIFE") {
			checkRef(wife.Value, fam.Xref, "WIFE")
		}
		for _, chil := range fam.ChildrenByTag("CHIL") {
			checkRef(chil.Value, fam.Xref, "CHIL")
		}
	}

	// Check source/note references in all records
	allRecords := make([]gedcom.GedcomRecord, 0,
		len(doc.Individuals)+len(doc.Families)+len(doc.Sources))
	allRecords = append(allRecords, doc.Individuals...)
	allRecords = append(allRecords, doc.Families...)
	for _, rec := range allRecords {
		for _, sour := range rec.ChildrenByTag("SOUR") {
			if isXref(sour.Value) {
				checkRef(sour.Value, rec.Xref, "SOUR")
			}
		}
		for _, note := range rec.ChildrenByTag("NOTE") {
			if isXref(note.Value) {
				checkRef(note.Value, rec.Xref, "NOTE")
			}
		}
	}

	return errs
}

func isXref(s string) bool {
	return len(s) > 2 && s[0] == '@' && s[len(s)-1] == '@'
}

func validateAssociations(doc *gedcom.GedcomDocument) []*ValidationError {
	var errs []*ValidationError
	idx := doc.XRefIndex()

	validateOwner := func(owner gedcom.GedcomRecord, ownerType string) {
		sourceXref := owner.Xref
		if sourceXref == "" {
			sourceXref = ownerType
		}
		var walk func(gedcom.GedcomRecord)
		walk = func(rec gedcom.GedcomRecord) {
			for _, child := range rec.Children {
				if child.Tag == "ASSO" {
					target := strings.TrimSpace(child.Value)
					if target == "" {
						errs = append(errs, &ValidationError{
							Severity: SeverityWarning,
							Code:     "MISSING_ASSOCIATE_XREF",
							Message:  "ASSO tag is missing an associated xref pointer",
							Xref:     sourceXref,
						})
					} else if targetRec, ok := idx[target]; !ok {
						errs = append(errs, &ValidationError{
							Severity: SeverityError,
							Code:     "BROKEN_ASSOCIATE_XREF",
							Message:  fmt.Sprintf("ASSO reference to non-existent record %s", target),
							Xref:     sourceXref,
						})
					} else if targetRec.Tag != "INDI" {
						errs = append(errs, &ValidationError{
							Severity: SeverityWarning,
							Code:     "INVALID_ASSOCIATE_TARGET",
							Message:  fmt.Sprintf("ASSO reference %s points to %s, expected INDI", target, targetRec.Tag),
							Xref:     sourceXref,
						})
					}

					if strings.TrimSpace(child.ChildValue("RELA")) == "" {
						errs = append(errs, &ValidationError{
							Severity: SeverityWarning,
							Code:     "MISSING_ASSOCIATE_RELA",
							Message:  "ASSO tag should include a RELA descriptor",
							Xref:     sourceXref,
						})
					}
				}
				walk(child)
			}
		}
		walk(owner)
	}

	for _, indi := range doc.Individuals {
		validateOwner(indi, "INDI")
	}
	for _, fam := range doc.Families {
		validateOwner(fam, "FAM")
	}

	return errs
}

// validateDateConsistency checks temporal logic of events.
func validateDateConsistency(doc *gedcom.GedcomDocument, opts *Options) []*ValidationError {
	var errs []*ValidationError

	for _, indi := range doc.Individuals {
		birthYear := extractEventYear(indi, "BIRT")
		deathYear := extractEventYear(indi, "DEAT")

		if birthYear > 0 && deathYear > 0 && deathYear < birthYear {
			errs = append(errs, &ValidationError{
				Severity: SeverityError,
				Code:     "DEATH_BEFORE_BIRTH",
				Message:  fmt.Sprintf("Death year %d is before birth year %d", deathYear, birthYear),
				Xref:     indi.Xref,
			})
		}

		if birthYear > 0 && deathYear > 0 {
			age := deathYear - birthYear
			if age > 120 {
				errs = append(errs, &ValidationError{
					Severity: SeverityWarning,
					Code:     "UNLIKELY_AGE",
					Message:  fmt.Sprintf("Age at death (%d years) exceeds 120", age),
					Xref:     indi.Xref,
				})
			}
		}
	}

	// Check parent-child age gaps
	idx := doc.XRefIndex()
	for _, fam := range doc.Families {
		childRecs := fam.ChildrenByTag("CHIL")
		husbRecs := fam.ChildrenByTag("HUSB")
		wifeRecs := fam.ChildrenByTag("WIFE")

		var husbBirth, wifeBirth int
		if len(husbRecs) > 0 {
			if h := idx[husbRecs[0].Value]; h != nil {
				husbBirth = extractEventYear(*h, "BIRT")
			}
		}
		if len(wifeRecs) > 0 {
			if w := idx[wifeRecs[0].Value]; w != nil {
				wifeBirth = extractEventYear(*w, "BIRT")
			}
		}

		for _, chilRef := range childRecs {
			c := idx[chilRef.Value]
			if c == nil {
				continue
			}
			childBirth := extractEventYear(*c, "BIRT")
			if childBirth == 0 {
				continue
			}

			if husbBirth > 0 {
				gap := childBirth - husbBirth
				if gap < opts.MinParentAge {
					errs = append(errs, &ValidationError{
						Severity: SeverityWarning,
						Code:     "PARENT_TOO_YOUNG",
						Message:  fmt.Sprintf("Father was %d at child's birth (min: %d)", gap, opts.MinParentAge),
						Xref:     fam.Xref,
					})
				}
				if gap > opts.MaxParentAge {
					errs = append(errs, &ValidationError{
						Severity: SeverityWarning,
						Code:     "PARENT_TOO_OLD",
						Message:  fmt.Sprintf("Father was %d at child's birth (max: %d)", gap, opts.MaxParentAge),
						Xref:     fam.Xref,
					})
				}
			}

			if wifeBirth > 0 {
				gap := childBirth - wifeBirth
				if gap < opts.MinParentAge {
					errs = append(errs, &ValidationError{
						Severity: SeverityWarning,
						Code:     "PARENT_TOO_YOUNG",
						Message:  fmt.Sprintf("Mother was %d at child's birth (min: %d)", gap, opts.MinParentAge),
						Xref:     fam.Xref,
					})
				}
				if gap > opts.MaxParentAge {
					errs = append(errs, &ValidationError{
						Severity: SeverityWarning,
						Code:     "PARENT_TOO_OLD",
						Message:  fmt.Sprintf("Mother was %d at child's birth (max: %d)", gap, opts.MaxParentAge),
						Xref:     fam.Xref,
					})
				}
			}
		}
	}

	return errs
}

// extractEventYear pulls the year from the first event of the given tag type.
func extractEventYear(rec gedcom.GedcomRecord, eventTag string) int {
	events := rec.ChildrenByTag(eventTag)
	if len(events) == 0 {
		return 0
	}
	dateVal := events[0].ChildValue("DATE")
	return parseYear(dateVal)
}

// parseYear extracts a 4-digit year from a GEDCOM date string.
func parseYear(dateStr string) int {
	if dateStr == "" {
		return 0
	}
	for _, part := range strings.Fields(dateStr) {
		if len(part) == 4 {
			if y, err := strconv.Atoi(part); err == nil && y > 1000 && y < 3000 {
				return y
			}
		}
	}
	return 0
}
