package enricher

import "github.com/google/uuid"

// GenerateIDs assigns a new UUID to every entity, edge, and junction link
// in the EnrichedDocument. This is optional -- consumers who need database-ready
// IDs call this after Enrich().
func GenerateIDs(ed *EnrichedDocument) {
	for i := range ed.Individuals {
		ed.Individuals[i].ID = uuid.New().String()
	}
	for i := range ed.Families {
		ed.Families[i].ID = uuid.New().String()
	}
	for i := range ed.Dates {
		ed.Dates[i].ID = uuid.New().String()
	}
	for i := range ed.Places {
		ed.Places[i].ID = uuid.New().String()
	}
	for i := range ed.Surnames {
		ed.Surnames[i].ID = uuid.New().String()
	}
	for i := range ed.GivenNames {
		ed.GivenNames[i].ID = uuid.New().String()
	}
	for i := range ed.Events {
		ed.Events[i].ID = uuid.New().String()
	}
	for i := range ed.Notes {
		ed.Notes[i].ID = uuid.New().String()
	}
	for i := range ed.Sources {
		ed.Sources[i].ID = uuid.New().String()
	}
	for i := range ed.Repositories {
		ed.Repositories[i].ID = uuid.New().String()
	}
	for i := range ed.Media {
		ed.Media[i].ID = uuid.New().String()
	}

	// Edges
	for i := range ed.Spouses {
		ed.Spouses[i].ID = uuid.New().String()
	}
	for i := range ed.ParentChild {
		ed.ParentChild[i].ID = uuid.New().String()
	}
	for i := range ed.FamilyChildren {
		ed.FamilyChildren[i].ID = uuid.New().String()
	}

	// Junction links — names
	for i := range ed.NameForms {
		ed.NameForms[i].ID = uuid.New().String()
	}
	for i := range ed.NameFormGivenNames {
		ed.NameFormGivenNames[i].ID = uuid.New().String()
	}
	for i := range ed.NameFormSurnames {
		ed.NameFormSurnames[i].ID = uuid.New().String()
	}
	for i := range ed.FamilySurnames {
		ed.FamilySurnames[i].ID = uuid.New().String()
	}

	// Junction links — events
	for i := range ed.IndividualEvents {
		ed.IndividualEvents[i].ID = uuid.New().String()
	}
	for i := range ed.FamilyEvents {
		ed.FamilyEvents[i].ID = uuid.New().String()
	}

	// Junction links — notes
	for i := range ed.IndividualNotes {
		ed.IndividualNotes[i].ID = uuid.New().String()
	}
	for i := range ed.FamilyNotes {
		ed.FamilyNotes[i].ID = uuid.New().String()
	}
	for i := range ed.EventNotes {
		ed.EventNotes[i].ID = uuid.New().String()
	}
	for i := range ed.SourceNotes {
		ed.SourceNotes[i].ID = uuid.New().String()
	}

	// Junction links — sources
	for i := range ed.IndividualSources {
		ed.IndividualSources[i].ID = uuid.New().String()
	}
	for i := range ed.FamilySources {
		ed.FamilySources[i].ID = uuid.New().String()
	}
	for i := range ed.EventSources {
		ed.EventSources[i].ID = uuid.New().String()
	}

	// Junction links — repositories
	for i := range ed.SourceRepositories {
		ed.SourceRepositories[i].ID = uuid.New().String()
	}

	// Junction links — media
	for i := range ed.IndividualMedia {
		ed.IndividualMedia[i].ID = uuid.New().String()
	}
	for i := range ed.FamilyMedia {
		ed.FamilyMedia[i].ID = uuid.New().String()
	}
	for i := range ed.SourceMedia {
		ed.SourceMedia[i].ID = uuid.New().String()
	}
}
