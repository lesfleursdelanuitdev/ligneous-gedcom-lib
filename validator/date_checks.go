package validator

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/enricher"
	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/gedcom"
)

// dateSortKey returns a monotonic-ish integer for comparing events; 0 means unknown.
func dateSortKey(pd enricher.ParsedDate) int {
	if pd.Year <= 0 {
		return 0
	}
	d := pd.Day
	if d <= 0 {
		d = 1
	}
	m := pd.Month
	if m <= 0 {
		m = 1
	}
	return pd.Year*400 + m*40 + d
}

func parsedDateFromEventBlock(ev gedcom.GedcomRecord) enricher.ParsedDate {
	d := ev.ChildValue("DATE")
	return enricher.ParseDateString(d)
}

func eventDateUnparseableWarning(recXref, eventTag, dateStr string) *ValidationError {
	ds := strings.TrimSpace(dateStr)
	if ds == "" {
		return nil
	}
	pd := enricher.ParseDateString(ds)
	if pd.Type != enricher.DateUnknown || pd.Year != 0 || pd.Month != 0 || pd.Day != 0 {
		return nil
	}
	return &ValidationError{
		Severity: SeverityWarning,
		Code:     CodeEventDateUnparseable,
		Message:  fmt.Sprintf("%s event has DATE value that could not be parsed reliably: %q", eventTag, truncateRunes(ds, 80)),
		Xref:     recXref,
		Details:  map[string]string{"event": eventTag, "date": ds},
	}
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// walkEventDates emits EVENT_DATE_UNPARSEABLE for any standard event with a DATE line.
func walkEventDatesForUnparseable(rec gedcom.GedcomRecord, recXref string, errs *[]*ValidationError) {
	eventTags := map[string]bool{
		"BIRT": true, "DEAT": true, "BAPM": true, "BURI": true, "CHR": true,
		"MARR": true, "DIV": true, "DIVF": true, "ENGA": true, "RESI": true,
	}
	var walk func(gedcom.GedcomRecord)
	walk = func(r gedcom.GedcomRecord) {
		if eventTags[r.Tag] {
			if w := eventDateUnparseableWarning(recXref, r.Tag, r.ChildValue("DATE")); w != nil {
				*errs = append(*errs, w)
			}
		}
		for _, ch := range r.Children {
			walk(ch)
		}
	}
	walk(rec)
}

func validateIndividualEventOrder(indi gedcom.GedcomRecord, errs *[]*ValidationError) {
	birt := indi.FirstChildByTag("BIRT")
	bapm := indi.FirstChildByTag("BAPM")
	if birt != nil && bapm != nil {
		db := parsedDateFromEventBlock(*birt)
		dp := parsedDateFromEventBlock(*bapm)
		kb, kp := dateSortKey(db), dateSortKey(dp)
		if kb > 0 && kp > 0 && kp < kb {
			*errs = append(*errs, &ValidationError{
				Severity: SeverityWarning,
				Code:     CodeBaptismBeforeBirth,
				Message:  fmt.Sprintf("BAPM date (%s) is before BIRT date (%s)", dp.Original, db.Original),
				Xref:     indi.Xref,
				Details:  map[string]string{"birt": db.Original, "bapm": dp.Original},
			})
		}
	}
	deat := indi.FirstChildByTag("DEAT")
	buri := indi.FirstChildByTag("BURI")
	if deat != nil && buri != nil {
		dd := parsedDateFromEventBlock(*deat)
		dbu := parsedDateFromEventBlock(*buri)
		kd, ku := dateSortKey(dd), dateSortKey(dbu)
		if kd > 0 && ku > 0 && ku < kd {
			*errs = append(*errs, &ValidationError{
				Severity: SeverityWarning,
				Code:     CodeBurialBeforeDeath,
				Message:  fmt.Sprintf("BURI date (%s) is before DEAT date (%s)", dbu.Original, dd.Original),
				Xref:     indi.Xref,
				Details:  map[string]string{"deat": dd.Original, "buri": dbu.Original},
			})
		}
	}
}

func firstSexValue(indi *gedcom.GedcomRecord) string {
	ss := indi.ChildrenByTag("SEX")
	if len(ss) == 0 {
		return ""
	}
	return strings.TrimSpace(ss[0].Value)
}

func validateSwappedOppositeSexSpouses(doc *gedcom.GedcomDocument, errs *[]*ValidationError) {
	idx := doc.XRefIndex()
	for i := range doc.Families {
		fam := &doc.Families[i]
		hs := fam.ChildrenByTag("HUSB")
		ws := fam.ChildrenByTag("WIFE")
		if len(hs) == 0 || len(ws) == 0 {
			continue
		}
		hi := idx[hs[0].Value]
		wi := idx[ws[0].Value]
		if hi == nil || wi == nil {
			continue
		}
		sH := firstSexValue(hi)
		sW := firstSexValue(wi)
		if sH == "F" && sW == "M" {
			*errs = append(*errs, &ValidationError{
				Severity:    SeverityWarning,
				Code:        CodeSwappedOppositeSexSpouses,
				Message:     "Opposite-sex couple: individual tagged HUSB is female and individual tagged WIFE is male (possible swapped HUSB/WIFE)",
				Xref:        fam.Xref,
				RelatedXref: hi.Xref,
				Details: map[string]string{
					"husb_xref": hi.Xref,
					"wife_xref": wi.Xref,
				},
			})
		}
	}
}

func birthSortKeyFromIndividual(indi *gedcom.GedcomRecord) int {
	b := indi.FirstChildByTag("BIRT")
	if b == nil {
		return 0
	}
	pd := parsedDateFromEventBlock(*b)
	return dateSortKey(pd)
}

func marriageSortKeyFromFamily(fam *gedcom.GedcomRecord) int {
	for _, ch := range fam.Children {
		if ch.Tag != "MARR" {
			continue
		}
		pd := parsedDateFromEventBlock(ch)
		k := dateSortKey(pd)
		if k > 0 {
			return k
		}
	}
	return 0
}

func validateMarriageAndBirthWarnings(doc *gedcom.GedcomDocument, opts *Options, errs *[]*ValidationError) {
	idx := doc.XRefIndex()
	minMA := opts.MinMarriageAge
	if minMA <= 0 {
		minMA = 14
	}
	for _, indi := range doc.Individuals {
		birthKey := birthSortKeyFromIndividual(&indi)
		birthYear := extractEventYear(indi, "BIRT")
		if birthYear == 0 && birthKey > 0 {
			birthYear = birthKey / 400
		}
		for _, fams := range indi.ChildrenByTag("FAMS") {
			fam := idx[fams.Value]
			if fam == nil {
				continue
			}
			mk := marriageSortKeyFromFamily(fam)
			mYear := extractEventYear(*fam, "MARR")
			if mYear == 0 && mk > 0 {
				mYear = mk / 400
			}
			if birthYear > 0 && mYear > 0 && mYear < birthYear {
				*errs = append(*errs, &ValidationError{
					Severity:    SeverityWarning,
					Code:        CodeMarriageBeforeSpouseBirth,
					Message:     fmt.Sprintf("Marriage year %d is before this individual's birth year %d", mYear, birthYear),
					Xref:        indi.Xref,
					RelatedXref: fam.Xref,
					Details:     map[string]string{"family": fam.Xref},
				})
			}
			if birthYear > 0 && mYear > 0 {
				age := mYear - birthYear
				if age >= 0 && age < minMA {
					*errs = append(*errs, &ValidationError{
						Severity:    SeverityWarning,
						Code:        CodeMarriedBelowMinimumAge,
						Message:     fmt.Sprintf("Marriage in year %d implies age %d (minimum configured: %d)", mYear, age, minMA),
						Xref:        indi.Xref,
						RelatedXref: fam.Xref,
						Details: map[string]string{
							"marriage_year": strconv.Itoa(mYear),
							"age":           strconv.Itoa(age),
						},
					})
				}
			}
		}
	}
}

func validateChildVersusParentBirthWarnings(doc *gedcom.GedcomDocument, opts *Options, errs *[]*ValidationError) {
	idx := doc.XRefIndex()
	minP := opts.MinParentAge
	maxP := opts.MaxParentAge
	if minP <= 0 {
		minP = 12
	}
	if maxP <= 0 {
		maxP = 80
	}
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
				if gap < 0 {
					*errs = append(*errs, &ValidationError{
						Severity:    SeverityWarning,
						Code:        CodeChildBornBeforeFather,
						Message:     fmt.Sprintf("Child born in %d, father born in %d", childBirth, husbBirth),
						Xref:        fam.Xref,
						RelatedXref: c.Xref,
						Details:     map[string]string{"parent": husbRecs[0].Value},
					})
				} else if gap < minP {
					*errs = append(*errs, &ValidationError{
						Severity:    SeverityWarning,
						Code:        CodeFatherTooYoungAtBirth,
						Message:     fmt.Sprintf("Father was %d at child's birth (min: %d)", gap, minP),
						Xref:        fam.Xref,
						RelatedXref: c.Xref,
						Details: map[string]string{
							"parent": husbRecs[0].Value,
							"child":  c.Xref,
						},
					})
				} else if gap > maxP {
					*errs = append(*errs, &ValidationError{
						Severity:    SeverityWarning,
						Code:        CodeFatherTooOldAtBirth,
						Message:     fmt.Sprintf("Father was %d at child's birth (max: %d)", gap, maxP),
						Xref:        fam.Xref,
						RelatedXref: c.Xref,
						Details: map[string]string{
							"parent": husbRecs[0].Value,
							"child":  c.Xref,
						},
					})
				}
			}

			if wifeBirth > 0 {
				gap := childBirth - wifeBirth
				if gap < 0 {
					*errs = append(*errs, &ValidationError{
						Severity:    SeverityWarning,
						Code:        CodeChildBornBeforeMother,
						Message:     fmt.Sprintf("Child born in %d, mother born in %d", childBirth, wifeBirth),
						Xref:        fam.Xref,
						RelatedXref: c.Xref,
						Details:     map[string]string{"parent": wifeRecs[0].Value},
					})
				} else if gap < minP {
					*errs = append(*errs, &ValidationError{
						Severity:    SeverityWarning,
						Code:        CodeMotherTooYoungAtBirth,
						Message:     fmt.Sprintf("Mother was %d at child's birth (min: %d)", gap, minP),
						Xref:        fam.Xref,
						RelatedXref: c.Xref,
						Details: map[string]string{
							"parent": wifeRecs[0].Value,
							"child":  c.Xref,
						},
					})
				} else if gap > maxP {
					*errs = append(*errs, &ValidationError{
						Severity:    SeverityWarning,
						Code:        CodeMotherTooOldAtBirth,
						Message:     fmt.Sprintf("Mother was %d at child's birth (max: %d)", gap, maxP),
						Xref:        fam.Xref,
						RelatedXref: c.Xref,
						Details: map[string]string{
							"parent": wifeRecs[0].Value,
							"child":  c.Xref,
						},
					})
				}
			}
		}
	}
}
