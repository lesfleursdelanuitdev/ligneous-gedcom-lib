package enricher

import (
	"strings"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/gedcom"
)

var individualEventTags = map[string]bool{
	"BIRT": true, "CHR": true, "DEAT": true, "BURI": true, "CREM": true,
	"ADOP": true, "BAPM": true, "BARM": true, "BASM": true, "BLES": true,
	"CHRA": true, "CONF": true, "FCOM": true, "ORDN": true, "NATU": true,
	"EMIG": true, "IMMI": true, "CENS": true, "PROB": true, "WILL": true,
	"GRAD": true, "RETI": true, "EVEN": true, "RESI": true,
}

var familyEventTags = map[string]bool{
	"MARR": true, "ANUL": true, "DIV": true, "DIVF": true, "ENGA": true,
	"MARB": true, "MARC": true, "MARL": true, "MARS": true, "EVEN": true,
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
		Notes:        len(ed.Notes),
		Sources:      len(ed.Sources),
		Repositories: len(ed.Repositories),
		Media:        len(ed.Media),
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

		// Extract names
		e.extractIndividualNames(ed, indi)

		// Build structured individual summary
		fullName := ""
		nameRecs := indi.ChildrenByTag("NAME")
		if len(nameRecs) > 0 {
			fullName = nameRecs[0].Value
		}

		ei := EnrichedIndividual{
			Xref:            indi.Xref,
			FullName:        fullName,
			FullNameLower:   strings.ToLower(fullName),
			Sex:             indi.ChildValue("SEX"),
			BirthDateIndex:  -1,
			BirthPlaceIndex: -1,
			DeathDateIndex:  -1,
			DeathPlaceIndex: -1,
		}

		// Extract events and capture birth/death FK indexes
		sortOrder := 0
		for _, child := range indi.Children {
			if !individualEventTags[child.Tag] {
				continue
			}

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
		}

		ed.Individuals = append(ed.Individuals, ei)
	}
}

func (e *enricherState) extractIndividualNames(ed *EnrichedDocument, indi gedcom.GedcomRecord) {
	nameRecs := indi.ChildrenByTag("NAME")
	if len(nameRecs) == 0 {
		return
	}

	nameRec := nameRecs[0]
	fullName := nameRec.Value

	surname := nameRec.ChildValue("SURN")
	if surname == "" {
		surname = extractSurnameFromFullName(fullName)
	}
	if surnIdx := e.getOrCreateSurname(surname, true); surnIdx >= 0 {
		ed.IndividualSurnames = append(ed.IndividualSurnames, IndividualSurnameLink{
			IndividualXref: indi.Xref,
			SurnameIndex:   surnIdx,
			NameType:       "birth",
			IsPrimary:      true,
		})
	}

	givenName := nameRec.ChildValue("GIVN")
	if givenName == "" {
		givenName = extractGivenFromFullName(fullName)
	}
	givenParts := strings.Fields(givenName)
	for i, part := range givenParts {
		if givenIdx := e.getOrCreateGivenName(part, true); givenIdx >= 0 {
			ed.IndividualGivenNames = append(ed.IndividualGivenNames, IndividualGivenNameLink{
				IndividualXref: indi.Xref,
				GivenNameIndex: givenIdx,
				Position:       i + 1,
				IsPrimary:      i == 0,
			})
		}
	}
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

		// Extract events and capture marriage FK indexes
		sortOrder := 0
		for _, child := range fam.Children {
			if !familyEventTags[child.Tag] {
				continue
			}

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

func findPedigree(doc *gedcom.GedcomDocument, childXref, famXref string) string {
	child := doc.FindByXref(childXref)
	if child == nil {
		return ""
	}
	for _, famc := range child.ChildrenByTag("FAMC") {
		if famc.Value == famXref {
			return famc.ChildValue("PEDI")
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
