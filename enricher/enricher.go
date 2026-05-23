package enricher

import (
	"strings"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/gedcom"
	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/pedigreedoc"
)

// individualEventTags lists GEDCOM 5.5.5 INDIVIDUAL_EVENT_STRUCTURE tags plus LDS ordinances.
// NCHI is excluded — it is computed from family children and never stored.
// Attributes (INDIVIDUAL_ATTRIBUTE_STRUCTURE) and RESI are handled separately.
var individualEventTags = map[string]bool{
	"BIRT": true, "CHR": true, "DEAT": true, "BURI": true, "CREM": true,
	"ADOP": true, "BAPM": true, "BARM": true, "BASM": true, "BLES": true,
	"CHRA": true, "CONF": true, "FCOM": true, "ORDN": true, "NATU": true,
	"EMIG": true, "IMMI": true, "CENS": true, "PROB": true, "WILL": true,
	"GRAD": true, "RETI": true, "EVEN": true,
	// LDS_INDIVIDUAL_ORDINANCE
	"BAPL": true, "CONL": true, "ENDL": true, "SLGC": true,
}

// individualAttributeTags lists GEDCOM 5.5.5 INDIVIDUAL_ATTRIBUTE_STRUCTURE tags.
// NCHI is excluded — computed from family children. RESI is handled separately.
var individualAttributeTags = map[string]bool{
	"CAST": true, "DSCR": true, "EDUC": true, "IDNO": true, "NATI": true,
	"NMR": true, "OCCU": true, "PROP": true, "RELI": true,
	"SSN": true, "TITL": true, "FACT": true,
}

// familyEventTags lists GEDCOM 5.5.5 FAMILY_EVENT_STRUCTURE tags plus LDS spouse sealing.
// RESI and FACT are handled separately as family attributes.
var familyEventTags = map[string]bool{
	"MARR": true, "ANUL": true, "DIV": true, "DIVF": true, "ENGA": true,
	"MARB": true, "MARC": true, "MARL": true, "MARS": true, "CENS": true,
	"EVEN": true,
	"SLGS": true,
}

// familyAttributeTags lists GEDCOM 5.5.5 FAMILY_ATTRIBUTE_STRUCTURE tags.
// NCHI is excluded — computed from family children. RESI is handled separately.
var familyAttributeTags = map[string]bool{
	"FACT": true,
}

// Enrich takes a raw GedcomDocument and produces an EnrichedDocument with all
// normalized lookup data, relationship edges, notes, sources, repositories,
// and media extracted.
func Enrich(doc *gedcom.GedcomDocument) *EnrichedDocument {
	e := &enricherState{
		doc:             doc,
		dateIndex:       make(map[string]int),
		placeIndex:      make(map[string]int),
		surnIndex:       make(map[string]int),
		givenIndex:      make(map[string]int),
		noteXrefIndex:   make(map[string]int),
		sourceXrefIndex: make(map[string]int),
		repoXrefIndex:   make(map[string]int),
		mediaXrefIndex:  make(map[string]int),
	}

	ed := &EnrichedDocument{
		Document: doc,
	}

	// Phase 1-5: Extract events (with dates and places), names, and build
	// structured individual/family summaries
	e.extractIndividualData(ed)
	e.extractFamilyData(ed)
	e.extractAssociations(ed)

	// Phase 6-8: Build relationship edges
	e.buildRelationshipEdges(ed)

	// Phase 9: Extract sources (must come before notes so source-note links work)
	e.extractSources(ed)

	// Phase 10: Extract repositories and source-repo junctions
	e.extractRepositories(ed)

	// Phase 11: Extract media
	e.extractMedia(ed)

	// Phase 12: Extract notes (last, so event/source notes can be linked)
	e.extractNotes(ed)

	// Populate lookup tables
	ed.Dates = e.dates
	ed.Places = e.places
	ed.Surnames = e.surnames
	ed.GivenNames = e.givenNames

	ed.Stats = Stats{
		Individuals:  len(doc.Individuals),
		Families:     len(doc.Families),
		Dates:        len(e.dates),
		Places:       len(e.places),
		Surnames:     len(e.surnames),
		GivenNames:   len(e.givenNames),
		Events:       len(ed.Events),
		Attributes:   len(ed.Attributes),
		Residences:   len(ed.Residences),
		Notes:        len(ed.Notes),
		Sources:      len(ed.Sources),
		Repositories: len(ed.Repositories),
		Media:        len(ed.Media),
		EventMedia:   len(ed.EventMedia),
	}

	return ed
}

// enricherState holds dedup caches during the enrichment process.
type enricherState struct {
	doc *gedcom.GedcomDocument

	dates      []ParsedDate
	dateIndex  map[string]int

	places     []ParsedPlace
	placeIndex map[string]int

	surnames   []Surname
	surnIndex  map[string]int

	givenNames []GivenName
	givenIndex map[string]int

	noteXrefIndex   map[string]int
	sourceXrefIndex map[string]int
	repoXrefIndex   map[string]int
	mediaXrefIndex  map[string]int
}

func (e *enricherState) getOrCreateDate(dateStr string) int {
	if strings.TrimSpace(dateStr) == "" {
		return -1
	}
	pd := ParseDateString(dateStr)
	if idx, ok := e.dateIndex[pd.Hash]; ok {
		return idx
	}
	idx := len(e.dates)
	e.dates = append(e.dates, pd)
	e.dateIndex[pd.Hash] = idx
	return idx
}

func (e *enricherState) getOrCreatePlace(placeStr string) int {
	if strings.TrimSpace(placeStr) == "" {
		return -1
	}
	pp := ParsePlaceString(placeStr)
	if idx, ok := e.placeIndex[pp.Hash]; ok {
		return idx
	}
	idx := len(e.places)
	e.places = append(e.places, pp)
	e.placeIndex[pp.Hash] = idx
	return idx
}

func (e *enricherState) getOrCreateSurname(surname string, incrementFreq bool) int {
	trimmed := strings.TrimSpace(surname)
	if trimmed == "" {
		return -1
	}
	lower := strings.ToLower(trimmed)
	if idx, ok := e.surnIndex[lower]; ok {
		if incrementFreq {
			e.surnames[idx].Frequency++
		}
		return idx
	}
	idx := len(e.surnames)
	freq := 0
	if incrementFreq {
		freq = 1
	}
	e.surnames = append(e.surnames, Surname{
		Value:     trimmed,
		Lower:     lower,
		Frequency: freq,
	})
	e.surnIndex[lower] = idx
	return idx
}

func (e *enricherState) getOrCreateGivenName(name string, incrementFreq bool) int {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return -1
	}
	lower := strings.ToLower(trimmed)
	if idx, ok := e.givenIndex[lower]; ok {
		if incrementFreq {
			e.givenNames[idx].Frequency++
		}
		return idx
	}
	idx := len(e.givenNames)
	freq := 0
	if incrementFreq {
		freq = 1
	}
	e.givenNames = append(e.givenNames, GivenName{
		Value:     trimmed,
		Lower:     lower,
		Frequency: freq,
	})
	e.givenIndex[lower] = idx
	return idx
}

// extractIndividualData processes all individuals: builds EnrichedIndividual,
// extracts names, and extracts events.
func (e *enricherState) extractIndividualData(ed *EnrichedDocument) {
	for _, indi := range e.doc.Individuals {
		if indi.Xref == "" {
			continue
		}

		// Extract names (first form = primary; fills NameForms + links)
		primarySurnameLower := e.extractIndividualNames(ed, indi)

		// Build structured individual summary
		fullName := ""
		nameRecs := indi.ChildrenByTag("NAME")
		if len(nameRecs) > 0 {
			fullName = nameRecs[0].Value
		}

		ei := EnrichedIndividual{
			Xref:                  indi.Xref,
			FullName:              fullName,
			FullNameLower:         strings.ToLower(fullName),
			PrimarySurnameLower:   primarySurnameLower,
			Sex:                   indi.ChildValue("SEX"),
			BirthDateIndex:        -1,
			BirthPlaceIndex:       -1,
			DeathDateIndex:        -1,
			DeathPlaceIndex:       -1,
		}

		// Extract scalar denorm fields (OCCU, NATI, RELI, _GENDER) for normalized tables.
		for _, child := range indi.Children {
			val := strings.TrimSpace(child.Value)
			switch child.Tag {
			case "OCCU":
				if val != "" {
					ei.OccupationValues = append(ei.OccupationValues, val)
				}
			case "NATI":
				if val != "" {
					ei.NationalityValues = append(ei.NationalityValues, val)
				}
			case "RELI":
				if val != "" && ei.Religion == "" {
					ei.Religion = val
				}
			case "_GENDER":
				if val != "" {
					ei.Gender = val
				}
			}
		}

		sortOrder := 0
		for _, child := range indi.Children {
			switch {
			case individualEventTags[child.Tag]:
				eventIdx := e.createEvent(ed, child, indi.Xref, "INDI", sortOrder)
				ed.IndividualEvents = append(ed.IndividualEvents, IndividualEventLink{
					IndividualXref: indi.Xref,
					EventIndex:     eventIdx,
					Role:           "principal",
				})
				evt := ed.Events[eventIdx]
				switch child.Tag {
				case "BIRT":
					ei.BirthDateIndex = evt.DateIndex
					ei.BirthPlaceIndex = evt.PlaceIndex
				case "DEAT":
					ei.DeathDateIndex = evt.DateIndex
					ei.DeathPlaceIndex = evt.PlaceIndex
				}
				sortOrder++

			case individualAttributeTags[child.Tag]:
				attrIdx := e.createAttribute(ed, child, indi.Xref, "INDI", sortOrder)
				ed.IndividualAttributes = append(ed.IndividualAttributes, IndividualAttributeLink{
					IndividualXref: indi.Xref,
					AttributeIndex: attrIdx,
				})
				sortOrder++

			case child.Tag == "RESI":
				residIdx := e.createResidence(ed, child, indi.Xref, "INDI", sortOrder)
				ed.IndividualResidences = append(ed.IndividualResidences, IndividualResidenceLink{
					IndividualXref: indi.Xref,
					ResidenceIndex: residIdx,
				})
				sortOrder++
			}
		}

		if ei.BirthPlaceIndex >= 0 && ei.BirthPlaceIndex < len(e.places) {
			c := strings.TrimSpace(e.places[ei.BirthPlaceIndex].Country)
			if c != "" {
				ei.BirthCountry = c
				ei.BirthCountryLower = strings.ToLower(c)
			}
		}
		if ei.DeathPlaceIndex >= 0 && ei.DeathPlaceIndex < len(e.places) {
			c := strings.TrimSpace(e.places[ei.DeathPlaceIndex].Country)
			if c != "" {
				ei.DeathCountry = c
				ei.DeathCountryLower = strings.ToLower(c)
			}
		}
		var birthY, deathY int
		hasBirthY, hasDeathY := false, false
		if ei.BirthDateIndex >= 0 && ei.BirthDateIndex < len(e.dates) {
			if y := e.dates[ei.BirthDateIndex].Year; y != 0 {
				birthY, hasBirthY = y, true
			}
		}
		if ei.DeathDateIndex >= 0 && ei.DeathDateIndex < len(e.dates) {
			if y := e.dates[ei.DeathDateIndex].Year; y != 0 {
				deathY, hasDeathY = y, true
			}
		}
		if hasBirthY && hasDeathY && deathY > birthY {
			ad := deathY - birthY
			ei.AgeAtDeath = &ad
		}

		ed.Individuals = append(ed.Individuals, ei)
	}
}

func (e *enricherState) extractIndividualNames(ed *EnrichedDocument, indi gedcom.GedcomRecord) string {
	nameRecs := indi.ChildrenByTag("NAME")
	if len(nameRecs) == 0 {
		return ""
	}

	var primarySurnameLower string

	for formIdx, nameRec := range nameRecs {
		fullName := nameRec.Value

		nameType := nameRec.ChildValue("TYPE")
		if nameType == "" {
			nameType = "birth"
		}
		nameType = strings.TrimSpace(strings.ToLower(nameType))
		if nameType == "" {
			nameType = "birth"
		}

		nf := NameForm{
			IndividualXref: indi.Xref,
			NameType:       nameType,
			IsPrimary:      formIdx == 0,
			SortOrder:      formIdx,
		}
		ed.NameForms = append(ed.NameForms, nf)
		nfIdx := len(ed.NameForms) - 1

		surname := nameRec.ChildValue("SURN")
		if surname == "" {
			surname = extractSurnameFromFullName(fullName)
		}
		if surname != "" {
			if surnIdx := e.getOrCreateSurname(surname, true); surnIdx >= 0 {
				ed.NameFormSurnames = append(ed.NameFormSurnames, NameFormSurnameLink{
					NameFormIndex: nfIdx,
					SurnameIndex:  surnIdx,
					Position:      1,
				})
				if formIdx == 0 {
					primarySurnameLower = e.surnames[surnIdx].Lower
				}
			}
		}

		givenName := nameRec.ChildValue("GIVN")
		if givenName == "" {
			givenName = extractGivenFromFullName(fullName)
		}
		givenParts := strings.Fields(givenName)
		for i, part := range givenParts {
			if part == "" {
				continue
			}
			if givenIdx := e.getOrCreateGivenName(part, true); givenIdx >= 0 {
				ed.NameFormGivenNames = append(ed.NameFormGivenNames, NameFormGivenNameLink{
					NameFormIndex:  nfIdx,
					GivenNameIndex: givenIdx,
					Position:       i + 1,
				})
			}
		}
	}
	return primarySurnameLower
}

// extractFamilyData processes all families: builds EnrichedFamily, extracts
// events, and links family surnames.
func (e *enricherState) extractFamilyData(ed *EnrichedDocument) {
	idx := e.doc.XRefIndex()

	for _, fam := range e.doc.Families {
		if fam.Xref == "" {
			continue
		}

		ef := EnrichedFamily{
			Xref:               fam.Xref,
			HusbandXref:        fam.ChildValue("HUSB"),
			WifeXref:           fam.ChildValue("WIFE"),
			MarriageDateIndex:  -1,
			MarriagePlaceIndex: -1,
			ChildrenCount:      len(fam.ChildrenByTag("CHIL")),
		}

		sortOrder := 0
		for _, child := range fam.Children {
			switch {
			case familyEventTags[child.Tag]:
				eventIdx := e.createEvent(ed, child, fam.Xref, "FAM", sortOrder)
				ed.FamilyEvents = append(ed.FamilyEvents, FamilyEventLink{
					FamilyXref: fam.Xref,
					EventIndex: eventIdx,
				})
				if child.Tag == "MARR" && ef.MarriageDateIndex == -1 {
					evt := ed.Events[eventIdx]
					ef.MarriageDateIndex = evt.DateIndex
					ef.MarriagePlaceIndex = evt.PlaceIndex
				}
				sortOrder++

			case familyAttributeTags[child.Tag]:
				attrIdx := e.createAttribute(ed, child, fam.Xref, "FAM", sortOrder)
				ed.FamilyAttributes = append(ed.FamilyAttributes, FamilyAttributeLink{
					FamilyXref:     fam.Xref,
					AttributeIndex: attrIdx,
				})
				sortOrder++

			case child.Tag == "RESI":
				residIdx := e.createResidence(ed, child, fam.Xref, "FAM", sortOrder)
				ed.FamilyResidences = append(ed.FamilyResidences, FamilyResidenceLink{
					FamilyXref:     fam.Xref,
					ResidenceIndex: residIdx,
				})
				sortOrder++
			}
		}

		ed.Families = append(ed.Families, ef)

		// Link family to surnames from spouses
		linkedSurnames := make(map[int]bool)
		for _, tag := range []string{"HUSB", "WIFE"} {
			spouseXref := fam.ChildValue(tag)
			if spouseXref == "" {
				continue
			}
			spouseRec := idx[spouseXref]
			if spouseRec == nil {
				continue
			}
			nameRecs := spouseRec.ChildrenByTag("NAME")
			if len(nameRecs) == 0 {
				continue
			}
			surname := nameRecs[0].ChildValue("SURN")
			if surname == "" {
				surname = extractSurnameFromFullName(nameRecs[0].Value)
			}
			if surnIdx := e.getOrCreateSurname(surname, false); surnIdx >= 0 && !linkedSurnames[surnIdx] {
				ed.FamilySurnames = append(ed.FamilySurnames, FamilySurnameLink{
					FamilyXref:   fam.Xref,
					SurnameIndex: surnIdx,
					IsPrimary:    tag == "HUSB",
				})
				linkedSurnames[surnIdx] = true
			}
		}
	}
}

func (e *enricherState) createEvent(ed *EnrichedDocument, rec gedcom.GedcomRecord, ownerXref, ownerType string, sortOrder int) int {
	eventType := rec.Tag
	customType := ""
	if eventType == "EVEN" {
		customType = rec.ChildValue("TYPE")
	}

	dateIdx := e.getOrCreateDate(rec.ChildValue("DATE"))
	placeIdx := e.getOrCreatePlace(rec.ChildValue("PLAC"))

	evt := Event{
		Index:      len(ed.Events),
		EventType:  eventType,
		CustomType: customType,
		EventLabel: EventLabelFor(eventType, customType),
		DateIndex:  dateIdx,
		PlaceIndex: placeIdx,
		Value:      rec.Value,
		Cause:      rec.ChildValue("CAUS"),
		Agency:     rec.ChildValue("AGNC"),
		OwnerXref:  ownerXref,
		OwnerType:  ownerType,
		SortOrder:  sortOrder,
	}

	ed.Events = append(ed.Events, evt)
	return evt.Index
}

func (e *enricherState) createAttribute(ed *EnrichedDocument, rec gedcom.GedcomRecord, ownerXref, ownerType string, sortOrder int) int {
	attributeType := rec.Tag
	customType := ""
	// FACT and IDNO carry a TYPE substructure that names the specific attribute kind.
	if attributeType == "FACT" || attributeType == "IDNO" {
		customType = rec.ChildValue("TYPE")
	}

	dateIdx := e.getOrCreateDate(rec.ChildValue("DATE"))
	placeIdx := e.getOrCreatePlace(rec.ChildValue("PLAC"))

	attr := Attribute{
		Index:         len(ed.Attributes),
		AttributeType: attributeType,
		CustomType:    customType,
		Value:         strings.TrimSpace(rec.Value),
		DateIndex:     dateIdx,
		PlaceIndex:    placeIdx,
		Agency:        rec.ChildValue("AGNC"),
		OwnerXref:     ownerXref,
		OwnerType:     ownerType,
		SortOrder:     sortOrder,
	}

	ed.Attributes = append(ed.Attributes, attr)
	return attr.Index
}

func (e *enricherState) createResidence(ed *EnrichedDocument, rec gedcom.GedcomRecord, ownerXref, ownerType string, sortOrder int) int {
	dateIdx := e.getOrCreateDate(rec.ChildValue("DATE"))
	placeIdx := e.getOrCreatePlace(rec.ChildValue("PLAC"))

	// ADDR substructure: prefer ADR1 child, fall back to the ADDR value itself.
	address := ""
	for _, child := range rec.Children {
		if child.Tag == "ADDR" {
			address = strings.TrimSpace(child.Value)
			if adr1 := child.ChildValue("ADR1"); adr1 != "" {
				address = strings.TrimSpace(adr1)
			}
			break
		}
	}

	res := Residence{
		Index:      len(ed.Residences),
		Address:    address,
		DateIndex:  dateIdx,
		PlaceIndex: placeIdx,
		OwnerXref:  ownerXref,
		OwnerType:  ownerType,
		SortOrder:  sortOrder,
	}

	ed.Residences = append(ed.Residences, res)
	return res.Index
}

func (e *enricherState) extractAssociations(ed *EnrichedDocument) {
	idx := e.doc.XRefIndex()

	for _, indi := range e.doc.Individuals {
		if indi.Xref == "" {
			continue
		}

		e.extractAssociationRecords(ed, idx, indi, "INDI", "", indi.ChildrenByTag("ASSO"))

		for _, child := range indi.Children {
			if !individualEventTags[child.Tag] && !individualAttributeTags[child.Tag] && child.Tag != "RESI" {
				continue
			}
			e.extractAssociationRecords(ed, idx, indi, "INDI", child.Tag, child.ChildrenByTag("ASSO"))
		}
	}

	for _, fam := range e.doc.Families {
		if fam.Xref == "" {
			continue
		}

		e.extractAssociationRecords(ed, idx, fam, "FAM", "", fam.ChildrenByTag("ASSO"))

		for _, child := range fam.Children {
			if !familyEventTags[child.Tag] && !familyAttributeTags[child.Tag] && child.Tag != "RESI" {
				continue
			}
			e.extractAssociationRecords(ed, idx, fam, "FAM", child.Tag, child.ChildrenByTag("ASSO"))
		}
	}
}

func (e *enricherState) extractAssociationRecords(
	ed *EnrichedDocument,
	idx map[string]*gedcom.GedcomRecord,
	owner gedcom.GedcomRecord,
	ownerType string,
	ownerEventType string,
	assoRecords []gedcom.GedcomRecord,
) {
	for _, asso := range assoRecords {
		target := strings.TrimSpace(asso.Value)
		if target == "" {
			continue
		}
		// GEDCOM 5.5 association pointers are defined for INDI targets.
		targetRec := idx[target]
		if targetRec == nil || targetRec.Tag != "INDI" {
			continue
		}

		ed.Associates = append(ed.Associates, AssociateEdge{
			OwnerXref:      owner.Xref,
			OwnerType:      ownerType,
			AssociateXref:  target,
			Relationship:   strings.TrimSpace(asso.ChildValue("RELA")),
			SourceTag:      "ASSO",
			OwnerEventType: ownerEventType,
		})
	}
}

func (e *enricherState) buildRelationshipEdges(ed *EnrichedDocument) {
	indiSet := make(map[string]bool, len(e.doc.Individuals))
	for _, indi := range e.doc.Individuals {
		if indi.Xref != "" {
			indiSet[indi.Xref] = true
		}
	}

	for _, fam := range e.doc.Families {
		if fam.Xref == "" {
			continue
		}

		husbXref := fam.ChildValue("HUSB")
		wifeXref := fam.ChildValue("WIFE")
		hasHusband := husbXref != "" && indiSet[husbXref]
		hasWife := wifeXref != "" && indiSet[wifeXref]

		if hasHusband && hasWife {
			ed.Spouses = append(ed.Spouses,
				SpouseEdge{IndividualXref: husbXref, SpouseXref: wifeXref, FamilyXref: fam.Xref},
				SpouseEdge{IndividualXref: wifeXref, SpouseXref: husbXref, FamilyXref: fam.Xref},
			)
		}

		childRecs := fam.ChildrenByTag("CHIL")
		for i, chilRec := range childRecs {
			childXref := chilRec.Value
			if childXref == "" || !indiSet[childXref] {
				continue
			}

			ed.FamilyChildren = append(ed.FamilyChildren, FamilyChildEdge{
				FamilyXref: fam.Xref,
				ChildXref:  childXref,
				BirthOrder: i + 1,
			})

			childIndi := e.doc.FindByXref(childXref)
			famcLink := findFamcRecord(childIndi, fam.Xref)
			noteText := firstInlineNoteUnderRecord(famcLink)
			bioX, adoptX, fromNote := pedigreedoc.ParseMixedParentageNote(noteText)

			if fromNote && hasHusband && hasWife {
				hn := pedigreedoc.NormalizeXrefPointer(husbXref)
				wn := pedigreedoc.NormalizeXrefPointer(wifeXref)
				valid := (pedigreedoc.XrefEqual(bioX, hn) && pedigreedoc.XrefEqual(adoptX, wn)) ||
					(pedigreedoc.XrefEqual(bioX, wn) && pedigreedoc.XrefEqual(adoptX, hn))
				if valid {
					pedH, rtH := pedigreeAndRelTypeForParent(husbXref, bioX, adoptX)
					pedW, rtW := pedigreeAndRelTypeForParent(wifeXref, bioX, adoptX)
					ed.ParentChild = append(ed.ParentChild, ParentChildEdge{
						ParentXref: husbXref, ChildXref: childXref, FamilyXref: fam.Xref,
						ParentType: "father", RelationshipType: rtH, Pedigree: pedH,
					})
					ed.ParentChild = append(ed.ParentChild, ParentChildEdge{
						ParentXref: wifeXref, ChildXref: childXref, FamilyXref: fam.Xref,
						ParentType: "mother", RelationshipType: rtW, Pedigree: pedW,
					})
					continue
				}
			}

			pedigree := findPedigree(e.doc, childXref, fam.Xref)
			relType := "biological"
			if pedigree == "adopted" || pedigree == "foster" || pedigree == "sealing" {
				relType = pedigree
			}

			if hasHusband {
				ed.ParentChild = append(ed.ParentChild, ParentChildEdge{
					ParentXref: husbXref, ChildXref: childXref, FamilyXref: fam.Xref,
					ParentType: "father", RelationshipType: relType, Pedigree: pedigree,
				})
			}
			if hasWife {
				ed.ParentChild = append(ed.ParentChild, ParentChildEdge{
					ParentXref: wifeXref, ChildXref: childXref, FamilyXref: fam.Xref,
					ParentType: "mother", RelationshipType: relType, Pedigree: pedigree,
				})
			}
		}
	}
}

func findFamcRecord(indi *gedcom.GedcomRecord, famXref string) *gedcom.GedcomRecord {
	if indi == nil {
		return nil
	}
	for i := range indi.Children {
		ch := &indi.Children[i]
		if ch.Tag == "FAMC" && pedigreedoc.XrefEqual(ch.Value, famXref) {
			return ch
		}
	}
	return nil
}

func firstInlineNoteUnderRecord(rec *gedcom.GedcomRecord) string {
	if rec == nil {
		return ""
	}
	for i := range rec.Children {
		c := &rec.Children[i]
		if c.Tag != "NOTE" {
			continue
		}
		v := strings.TrimSpace(c.Value)
		if v == "" {
			continue
		}
		if strings.HasPrefix(v, "@") && strings.HasSuffix(v, "@") && len(v) > 2 {
			continue
		}
		return flattenNoteWithCont(*c)
	}
	return ""
}

func flattenNoteWithCont(note gedcom.GedcomRecord) string {
	var b strings.Builder
	b.WriteString(note.Value)
	for _, ch := range note.Children {
		if ch.Tag == "CONT" {
			b.WriteByte('\n')
			b.WriteString(ch.Value)
		}
	}
	return b.String()
}

func pedigreeAndRelTypeForParent(parentXref, bioXref, adoptXref string) (pedigree string, relType string) {
	if pedigreedoc.XrefEqual(parentXref, bioXref) {
		return "birth", "biological"
	}
	if pedigreedoc.XrefEqual(parentXref, adoptXref) {
		return "adopted", "adopted"
	}
	return "", "biological"
}

func findPedigree(doc *gedcom.GedcomDocument, childXref, famXref string) string {
	if p := findPedigreeFromIndiFamc(doc, childXref, famXref); p != "" {
		return p
	}
	famRec := doc.FindByXref(famXref)
	if famRec == nil || famRec.Tag != "FAM" {
		return ""
	}
	return findPedigreeFromFamCHIL(famRec, childXref)
}

func findPedigreeFromIndiFamc(doc *gedcom.GedcomDocument, childXref, famXref string) string {
	child := doc.FindByXref(childXref)
	if child == nil {
		return ""
	}
	for _, famc := range child.ChildrenByTag("FAMC") {
		if pedigreedoc.XrefEqual(famc.Value, famXref) {
			return famc.ChildValue("PEDI")
		}
	}
	return ""
}

func findPedigreeFromFamCHIL(fam *gedcom.GedcomRecord, childXref string) string {
	for _, chil := range fam.ChildrenByTag("CHIL") {
		if pedigreedoc.XrefEqual(chil.Value, childXref) {
			return chil.ChildValue("PEDI")
		}
	}
	return ""
}

func extractSurnameFromFullName(fullName string) string {
	start := strings.Index(fullName, "/")
	if start < 0 {
		return ""
	}
	end := strings.Index(fullName[start+1:], "/")
	if end < 0 {
		return strings.TrimSpace(fullName[start+1:])
	}
	return strings.TrimSpace(fullName[start+1 : start+1+end])
}

func extractGivenFromFullName(fullName string) string {
	idx := strings.Index(fullName, "/")
	if idx < 0 {
		return strings.TrimSpace(fullName)
	}
	return strings.TrimSpace(fullName[:idx])
}
