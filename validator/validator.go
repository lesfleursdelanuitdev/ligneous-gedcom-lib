// Package validator validates GedcomDocument instances against GEDCOM
// specification rules: structural correctness, required fields, cross-reference
// integrity, optional strict year-based date checks, and always-on genealogical
// warnings (parsed dates, marriage timing, spouse tag sanity, etc.).
//
// Usage:
//
//	errs := validator.Validate(doc)
//	errs := validator.ValidateWithOptions(doc, &validator.Options{DateConsistency: true})
//
// Each finding has a unique machine-readable Code. Use RelatedXref and Details
// for additional context. AssociatedXrefs lists every GEDCOM xref involved in the
// finding (deduplicated, stable order); see setAssociatedXrefs in associated_xrefs.go.
package validator

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/enricher"
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
	Severity    Severity `json:"Severity"`
	Code        string   `json:"Code"`
	Message     string   `json:"Message"`
	Xref        string   `json:"Xref"`
	RelatedXref string   `json:"RelatedXref,omitempty"`
	// AssociatedXrefs lists every record xref implicated by the rule (primary, related,
	// and any other individuals/families/sources), deduplicated in a stable order per Code.
	AssociatedXrefs []string          `json:"AssociatedXrefs,omitempty"`
	Details           map[string]string `json:"Details,omitempty"`
}

func (e *ValidationError) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s] %s: %s", e.Severity, e.Code, e.Message))
	if e.Xref != "" {
		sb.WriteString(fmt.Sprintf(" (xref: %s)", e.Xref))
	}
	if e.RelatedXref != "" {
		sb.WriteString(fmt.Sprintf(" (related: %s)", e.RelatedXref))
	}
	return sb.String()
}

// Options configures which validation rules to run.
type Options struct {
	DateConsistency bool // Run strict year-based death-before-birth and lifespan checks.
	MinParentAge    int  // Minimum age to be a parent (default: 12)
	MaxParentAge    int  // Maximum age to be a parent (default: 80)
	MinMarriageAge  int  // Minimum age at marriage for warnings (default: 14)
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

func normalizeOpts(o *Options) *Options {
	if o == nil {
		return DefaultOptions()
	}
	out := *o
	if out.MinParentAge <= 0 {
		out.MinParentAge = 12
	}
	if out.MaxParentAge <= 0 {
		out.MaxParentAge = 80
	}
	if out.MinMarriageAge <= 0 {
		out.MinMarriageAge = 14
	}
	return &out
}

// Validate runs all validation rules with default options.
func Validate(doc *gedcom.GedcomDocument) []*ValidationError {
	return ValidateWithOptions(doc, DefaultOptions())
}

// ValidateWithOptions runs validation with custom options.
func ValidateWithOptions(doc *gedcom.GedcomDocument, opts *Options) []*ValidationError {
	opts = normalizeOpts(opts)

	var errs []*ValidationError

	errs = append(errs, validateStructure(doc)...)
	errs = append(errs, validateIndividuals(doc)...)
	errs = append(errs, validateFamilies(doc)...)
	errs = append(errs, validateHeaderAndXrefLength(doc)...)
	errs = append(errs, validateGenealogyWarnings(doc, opts)...)
	errs = append(errs, validateXRefs(doc)...)
	errs = append(errs, validateAssociations(doc)...)

	if opts.DateConsistency {
		errs = append(errs, validateDateConsistencyStrict(doc, opts)...)
	}

	for _, e := range errs {
		setAssociatedXrefs(e)
	}
	return errs
}

func validateGenealogyWarnings(doc *gedcom.GedcomDocument, opts *Options) []*ValidationError {
	var errs []*ValidationError
	for i := range doc.Individuals {
		indi := &doc.Individuals[i]
		walkEventDatesForUnparseable(*indi, indi.Xref, &errs)
		validateIndividualEventOrder(*indi, &errs)
	}
	for i := range doc.Families {
		fam := &doc.Families[i]
		walkEventDatesForUnparseable(*fam, fam.Xref, &errs)
	}
	validateMarriageAndBirthWarnings(doc, opts, &errs)
	validateChildVersusParentBirthWarnings(doc, opts, &errs)
	validateSwappedOppositeSexSpouses(doc, &errs)
	return errs
}

func validateHeaderAndXrefLength(doc *gedcom.GedcomDocument) []*ValidationError {
	var errs []*ValidationError
	if doc.Header.Tag == "" {
		return errs
	}
	ver := gedcomVersionFromHeader(doc.Header)
	if isGedcom55Or551(ver) && !headerHasSubm(doc.Header) {
		errs = append(errs, &ValidationError{
			Severity: SeverityWarning,
			Code:     CodeHeaderMissingSubm,
			Message:  fmt.Sprintf("GEDCOM %s header should include a SUBM pointer to a submitter record", ver),
			Xref:     "",
			Details:  map[string]string{"gedcom_version": ver},
		})
	}

	if !isGedcom7(ver) {
		checkXrefLen := func(xref string) {
			if !isXref(xref) {
				return
			}
			inner := strings.Trim(xref, "@")
			if len(inner) > 20 {
				errs = append(errs, &ValidationError{
					Severity: SeverityWarning,
					Code:     CodeXrefExceedsVersionLimit,
					Message:  fmt.Sprintf("XRef %s exceeds 20-character identifier limit for GEDCOM versions before 7.0", xref),
					Xref:     xref,
					Details:  map[string]string{"length": strconv.Itoa(len(inner)), "gedcom_version": ver},
				})
			}
		}
		for i := range doc.Individuals {
			checkXrefLen(doc.Individuals[i].Xref)
		}
		for i := range doc.Families {
			checkXrefLen(doc.Families[i].Xref)
		}
		for i := range doc.Sources {
			checkXrefLen(doc.Sources[i].Xref)
		}
		for i := range doc.Notes {
			checkXrefLen(doc.Notes[i].Xref)
		}
		for i := range doc.Repositories {
			checkXrefLen(doc.Repositories[i].Xref)
		}
		for i := range doc.Media {
			checkXrefLen(doc.Media[i].Xref)
		}
		for i := range doc.Submitters {
			checkXrefLen(doc.Submitters[i].Xref)
		}
	}

	return errs
}

// validateStructure checks top-level structural requirements.
func validateStructure(doc *gedcom.GedcomDocument) []*ValidationError {
	var errs []*ValidationError

	if doc.Header.Tag == "" {
		errs = append(errs, &ValidationError{
			Severity: SeverityError,
			Code:     CodeMissingHeader,
			Message:  "GEDCOM file must start with a HEAD record",
		})
	}

	if doc.Trailer.Tag == "" {
		errs = append(errs, &ValidationError{
			Severity: SeverityWarning,
			Code:     CodeMissingTrailer,
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
				Code:     CodeMissingXrefIndividual,
				Message:  "Individual record must have an xref",
			})
			continue
		}

		names := indi.ChildrenByTag("NAME")
		if len(names) == 0 {
			errs = append(errs, &ValidationError{
				Severity: SeverityWarning,
				Code:     CodeMissingName,
				Message:  "Individual record missing NAME tag",
				Xref:     indi.Xref,
			})
		} else {
			for _, nm := range names {
				v := strings.TrimSpace(nm.Value)
				if v == "" {
					errs = append(errs, &ValidationError{
						Severity: SeverityWarning,
						Code:     CodeNameTagEmpty,
						Message:  "NAME tag is present but its value is empty",
						Xref:     indi.Xref,
					})
					continue
				}
				if !strings.Contains(v, "/") {
					errs = append(errs, &ValidationError{
						Severity: SeverityWarning,
						Code:     CodeNameMissingSurnameSlash,
						Message:  "NAME value has no '/' surname delimiters (unusual for a GEDCOM NAME line)",
						Xref:     indi.Xref,
						Details:  map[string]string{"name": truncateRunes(v, 120)},
					})
				}
			}
		}

		sexChildren := indi.ChildrenByTag("SEX")
		if len(sexChildren) > 0 {
			sex := sexChildren[0].Value
			validSex := map[string]bool{"M": true, "F": true, "U": true, "X": true, "N": true}
			if sex != "" && !validSex[sex] {
				errs = append(errs, &ValidationError{
					Severity: SeverityWarning,
					Code:     CodeInvalidSex,
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
				Code:     CodeMissingXrefFamily,
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
				Code:     CodeEmptyFamily,
				Message:  "Family record has no members (no HUSB, WIFE, or CHIL tags)",
				Xref:     fam.Xref,
			})
		}
	}

	return errs
}

func orphanCodeForPointerTag(refTag string) string {
	switch refTag {
	case "FAMC":
		return CodeOrphanedFamc
	case "FAMS":
		return CodeOrphanedFams
	case "HUSB":
		return CodeOrphanedHusb
	case "WIFE":
		return CodeOrphanedWife
	case "CHIL":
		return CodeOrphanedChil
	case "SOUR":
		return CodeOrphanedSour
	case "NOTE":
		return CodeOrphanedNote
	default:
		return CodeOrphanedXrefOther
	}
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
			code := orphanCodeForPointerTag(refTag)
			errs = append(errs, &ValidationError{
				Severity:    SeverityError,
				Code:        code,
				Message:     fmt.Sprintf("%s reference to non-existent record %s", refTag, refValue),
				Xref:        sourceXref,
				RelatedXref: refValue,
				Details:     map[string]string{"pointer_tag": refTag, "pointer": refValue},
			})
		}
	}

	for _, indi := range doc.Individuals {
		for _, famc := range indi.ChildrenByTag("FAMC") {
			checkRef(famc.Value, indi.Xref, "FAMC")
		}
		for _, fams := range indi.ChildrenByTag("FAMS") {
			checkRef(fams.Value, indi.Xref, "FAMS")
		}
	}

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
							Code:     CodeMissingAssociateXref,
							Message:  "ASSO tag is missing an associated xref pointer",
							Xref:     sourceXref,
						})
					} else if targetRec, ok := idx[target]; !ok {
						errs = append(errs, &ValidationError{
							Severity:    SeverityError,
							Code:        CodeOrphanedAssociateXref,
							Message:     fmt.Sprintf("ASSO reference to non-existent record %s", target),
							Xref:        sourceXref,
							RelatedXref: target,
							Details:     map[string]string{"pointer": target},
						})
					} else if targetRec.Tag != "INDI" {
						errs = append(errs, &ValidationError{
							Severity: SeverityWarning,
							Code:     CodeInvalidAssociateTarget,
							Message:  fmt.Sprintf("ASSO reference %s points to %s, expected INDI", target, targetRec.Tag),
							Xref:     sourceXref,
							Details:  map[string]string{"target": target},
						})
					}

					if strings.TrimSpace(child.ChildValue("RELA")) == "" {
						errs = append(errs, &ValidationError{
							Severity: SeverityWarning,
							Code:     CodeMissingAssociateRela,
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

// validateDateConsistencyStrict runs year-based checks when DateConsistency is enabled.
func validateDateConsistencyStrict(doc *gedcom.GedcomDocument, opts *Options) []*ValidationError {
	var errs []*ValidationError

	for _, indi := range doc.Individuals {
		birthYear := extractEventYear(indi, "BIRT")
		deathYear := extractEventYear(indi, "DEAT")

		if birthYear > 0 && deathYear > 0 && deathYear < birthYear {
			errs = append(errs, &ValidationError{
				Severity: SeverityError,
				Code:     CodeDeathBeforeBirth,
				Message:  fmt.Sprintf("Death year %d is before birth year %d", deathYear, birthYear),
				Xref:     indi.Xref,
			})
		}

		if birthYear > 0 && deathYear > 0 {
			age := deathYear - birthYear
			if age > 120 {
				errs = append(errs, &ValidationError{
					Severity: SeverityWarning,
					Code:     CodeAgeAtDeathExceeds120,
					Message:  fmt.Sprintf("Age at death (%d years) exceeds 120", age),
					Xref:     indi.Xref,
					Details:  map[string]string{"age_years": strconv.Itoa(age)},
				})
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
	pd := enricher.ParseDateString(dateVal)
	if pd.Year > 0 {
		return pd.Year
	}
	return parseYear(dateVal)
}

// parseYear extracts a 4-digit year from a GEDCOM date string (fallback).
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
