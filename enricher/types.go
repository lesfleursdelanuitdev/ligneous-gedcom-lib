// Package enricher takes a raw GedcomDocument and produces an EnrichedDocument
// containing all normalized/deduplicated data that mirrors the Go API's
// PostgreSQL schema: unique dates, places, given names, surnames, events,
// notes, sources, repositories, media, and relationship edges.
//
// Usage:
//
//	doc, _, _ := parser.Parse(reader)
//	enriched := enricher.Enrich(doc)
//	enricher.GenerateIDs(enriched) // optional: assign UUIDs for DB storage
//	fmt.Println(enriched.Stats)
package enricher

import "github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/gedcom"

// EnrichedDocument wraps a raw GedcomDocument with all extracted,
// normalized data ready for database storage or further processing.
type EnrichedDocument struct {
	Document *gedcom.GedcomDocument

	// Structured entity summaries
	Individuals []EnrichedIndividual
	Families    []EnrichedFamily

	// Normalized lookup tables (deduplicated)
	Dates      []ParsedDate
	Places     []ParsedPlace
	Surnames   []Surname
	GivenNames []GivenName
	Events     []Event

	// Top-level records (extracted and structured)
	Notes        []EnrichedNote
	Sources      []EnrichedSource
	Repositories []EnrichedRepository
	Media        []EnrichedMedia

	// Relationship edges
	Spouses        []SpouseEdge
	ParentChild    []ParentChildEdge
	FamilyChildren []FamilyChildEdge
	Associates     []AssociateEdge

	// Junction/link tables — names
	NameForms          []NameForm
	NameFormGivenNames []NameFormGivenNameLink
	NameFormSurnames   []NameFormSurnameLink
	FamilySurnames     []FamilySurnameLink

	// Junction/link tables — events
	IndividualEvents []IndividualEventLink
	FamilyEvents     []FamilyEventLink

	// Junction/link tables — notes
	IndividualNotes []IndividualNoteLink
	FamilyNotes     []FamilyNoteLink
	EventNotes      []EventNoteLink
	SourceNotes     []SourceNoteLink

	// Junction/link tables — sources
	IndividualSources []IndividualSourceLink
	FamilySources     []FamilySourceLink
	EventSources      []EventSourceLink

	// Junction/link tables — repositories
	SourceRepositories []SourceRepositoryLink

	// Junction/link tables — media
	IndividualMedia []IndividualMediaLink
	FamilyMedia     []FamilyMediaLink
	SourceMedia     []SourceMediaLink
	EventMedia      []EventMediaLink

	Stats Stats
}

// ---------------------------------------------------------------------------
// Structured entity summaries
// ---------------------------------------------------------------------------

// EnrichedIndividual mirrors gedcom_individuals_v2 with denormalized FK indexes.
type EnrichedIndividual struct {
	ID                string   `json:"id,omitempty"`
	Xref              string   `json:"xref"`
	FullName          string   `json:"full_name"`
	FullNameLower     string   `json:"full_name_lower"`
	Sex               string   `json:"sex,omitempty"`
	BirthDateIndex    int      `json:"birth_date_index"`
	BirthPlaceIndex   int      `json:"birth_place_index"`
	DeathDateIndex    int      `json:"death_date_index"`
	DeathPlaceIndex   int      `json:"death_place_index"`
	PrimarySurnameLower string `json:"primary_surname_lower,omitempty"`
	BirthCountry        string `json:"birth_country,omitempty"`
	BirthCountryLower   string `json:"birth_country_lower,omitempty"`
	DeathCountry        string `json:"death_country,omitempty"`
	DeathCountryLower   string `json:"death_country_lower,omitempty"`
	AgeAtDeath          *int   `json:"age_at_death,omitempty"`
	OccupationValues  []string `json:"occupation_values,omitempty"`
	NationalityValues []string `json:"nationality_values,omitempty"`
	Religion          string   `json:"religion,omitempty"`
	Gender            string   `json:"gender,omitempty"`
}

// EnrichedFamily mirrors gedcom_families_v2 with denormalized FK indexes.
type EnrichedFamily struct {
	ID                 string `json:"id,omitempty"`
	Xref               string `json:"xref"`
	HusbandXref        string `json:"husband_xref,omitempty"`
	WifeXref           string `json:"wife_xref,omitempty"`
	MarriageDateIndex  int    `json:"marriage_date_index"`
	MarriagePlaceIndex int    `json:"marriage_place_index"`
	ChildrenCount      int    `json:"children_count"`
}

// ---------------------------------------------------------------------------
// Normalized lookup tables
// ---------------------------------------------------------------------------

// DateType classifies how a GEDCOM date should be interpreted.
type DateType string

const (
	DateExact      DateType = "EXACT"
	DateAbout      DateType = "ABOUT"
	DateBefore     DateType = "BEFORE"
	DateAfter      DateType = "AFTER"
	DateBetween    DateType = "BETWEEN"
	DateFromTo     DateType = "FROM_TO"
	DateCalculated DateType = "CALCULATED"
	DateEstimated  DateType = "ESTIMATED"
	DateUnknown    DateType = "UNKNOWN"
)

// ParsedDate mirrors gedcom_dates_v2. Hash is used for deduplication.
type ParsedDate struct {
	ID       string   `json:"id,omitempty"`
	Original string   `json:"original"`
	Type     DateType `json:"type"`
	Calendar string   `json:"calendar"`
	Year     int      `json:"year,omitempty"`
	Month    int      `json:"month,omitempty"`
	Day      int      `json:"day,omitempty"`
	EndYear  int      `json:"end_year,omitempty"`
	EndMonth int      `json:"end_month,omitempty"`
	EndDay   int      `json:"end_day,omitempty"`
	Hash     string   `json:"hash"`
}

// ParsedPlace mirrors gedcom_places_v2. Hash is used for deduplication.
type ParsedPlace struct {
	ID        string  `json:"id,omitempty"`
	Original  string  `json:"original"`
	Name      string  `json:"name"`
	County    string  `json:"county,omitempty"`
	State     string  `json:"state,omitempty"`
	Country   string  `json:"country,omitempty"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
	Hash      string  `json:"hash"`
}

// Surname mirrors gedcom_surnames_v2.
type Surname struct {
	ID        string `json:"id,omitempty"`
	Value     string `json:"value"`
	Lower     string `json:"lower"`
	Frequency int    `json:"frequency"`
}

// GivenName mirrors gedcom_given_names_v2.
type GivenName struct {
	ID        string `json:"id,omitempty"`
	Value     string `json:"value"`
	Lower     string `json:"lower"`
	Frequency int    `json:"frequency"`
}

// Event mirrors gedcom_events_v2. DateIndex and PlaceIndex are indexes into
// the EnrichedDocument.Dates and Places slices (-1 if none).
type Event struct {
	ID         string `json:"id,omitempty"`
	Index      int    `json:"index"`
	EventType  string `json:"event_type"`
	CustomType string `json:"custom_type,omitempty"`
	EventLabel string `json:"event_label,omitempty"`
	DateIndex  int    `json:"date_index"`
	PlaceIndex int    `json:"place_index"`
	Value      string `json:"value,omitempty"`
	Cause      string `json:"cause,omitempty"`
	Agency     string `json:"agency,omitempty"`
	OwnerXref  string `json:"owner_xref"`
	OwnerType  string `json:"owner_type"`
	SortOrder  int    `json:"sort_order"`
}

// ---------------------------------------------------------------------------
// Top-level record types
// ---------------------------------------------------------------------------

// EnrichedNote mirrors gedcom_notes_v2.
type EnrichedNote struct {
	ID         string `json:"id,omitempty"`
	Xref       string `json:"xref,omitempty"`
	Content    string `json:"content"`
	IsTopLevel bool   `json:"is_top_level"`
}

// EnrichedSource mirrors gedcom_sources_v2.
type EnrichedSource struct {
	ID             string `json:"id,omitempty"`
	Xref           string `json:"xref"`
	Title          string `json:"title,omitempty"`
	Author         string `json:"author,omitempty"`
	Abbreviation   string `json:"abbreviation,omitempty"`
	Publication    string `json:"publication,omitempty"`
	Text           string `json:"text,omitempty"`
	RepositoryXref string `json:"repository_xref,omitempty"`
	CallNumber     string `json:"call_number,omitempty"`
}

// EnrichedRepository mirrors gedcom_repositories_v2.
type EnrichedRepository struct {
	ID      string `json:"id,omitempty"`
	Xref    string `json:"xref"`
	Name    string `json:"name,omitempty"`
	Address string `json:"address,omitempty"`
	City    string `json:"city,omitempty"`
	State   string `json:"state,omitempty"`
	Country string `json:"country,omitempty"`
	Phone   string `json:"phone,omitempty"`
	Email   string `json:"email,omitempty"`
	Website string `json:"website,omitempty"`
}

// EnrichedMedia mirrors gedcom_media_v2.
type EnrichedMedia struct {
	ID          string `json:"id,omitempty"`
	Xref        string `json:"xref,omitempty"`
	File        string `json:"file,omitempty"`
	Form        string `json:"form,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"` // inline NOTE bodies under OBJE (not xref pointers)
}

// ---------------------------------------------------------------------------
// Relationship edges
// ---------------------------------------------------------------------------

// SpouseEdge mirrors gedcom_spouses_v2 (stored bidirectionally).
type SpouseEdge struct {
	ID             string `json:"id,omitempty"`
	IndividualXref string `json:"individual_xref"`
	SpouseXref     string `json:"spouse_xref"`
	FamilyXref     string `json:"family_xref"`
}

// ParentChildEdge mirrors gedcom_parent_child_v2.
type ParentChildEdge struct {
	ID               string `json:"id,omitempty"`
	ParentXref       string `json:"parent_xref"`
	ChildXref        string `json:"child_xref"`
	FamilyXref       string `json:"family_xref"`
	ParentType       string `json:"parent_type"`
	RelationshipType string `json:"relationship_type"`
	Pedigree         string `json:"pedigree,omitempty"`
}

// FamilyChildEdge mirrors gedcom_family_children_v2.
type FamilyChildEdge struct {
	ID         string `json:"id,omitempty"`
	FamilyXref string `json:"family_xref"`
	ChildXref  string `json:"child_xref"`
	BirthOrder int    `json:"birth_order"`
}

// AssociateEdge mirrors GEDCOM ASSO/RELA links between records.
type AssociateEdge struct {
	ID             string `json:"id,omitempty"`
	OwnerXref      string `json:"owner_xref"`
	OwnerType      string `json:"owner_type"`
	AssociateXref  string `json:"associate_xref"`
	Relationship   string `json:"relationship,omitempty"`
	SourceTag      string `json:"source_tag,omitempty"`
	OwnerEventType string `json:"owner_event_type,omitempty"`
}

// ---------------------------------------------------------------------------
// Junction/link types — names
// ---------------------------------------------------------------------------

// NameForm groups given names and surnames by type (birth, maiden, married, etc.).
type NameForm struct {
	ID             string `json:"id,omitempty"`
	IndividualXref string `json:"individual_xref"`
	NameType       string `json:"name_type"`
	IsPrimary      bool   `json:"is_primary"`
	SortOrder      int    `json:"sort_order"`
}

// NameFormGivenNameLink links a name form to a given name.
type NameFormGivenNameLink struct {
	ID             string `json:"id,omitempty"`
	NameFormIndex  int    `json:"name_form_index"`
	GivenNameIndex int    `json:"given_name_index"`
	Position       int    `json:"position"`
}

// NameFormSurnameLink links a name form to a surname.
type NameFormSurnameLink struct {
	ID            string `json:"id,omitempty"`
	NameFormIndex int    `json:"name_form_index"`
	SurnameIndex  int    `json:"surname_index"`
	Position      int    `json:"position"`
}

// FamilySurnameLink mirrors gedcom_family_surnames_v2.
type FamilySurnameLink struct {
	ID           string `json:"id,omitempty"`
	FamilyXref   string `json:"family_xref"`
	SurnameIndex int    `json:"surname_index"`
	IsPrimary    bool   `json:"is_primary"`
}

// ---------------------------------------------------------------------------
// Junction/link types — events
// ---------------------------------------------------------------------------

// IndividualEventLink mirrors gedcom_individual_events_v2.
type IndividualEventLink struct {
	ID             string `json:"id,omitempty"`
	IndividualXref string `json:"individual_xref"`
	EventIndex     int    `json:"event_index"`
	Role           string `json:"role"`
}

// FamilyEventLink mirrors gedcom_family_events_v2.
type FamilyEventLink struct {
	ID         string `json:"id,omitempty"`
	FamilyXref string `json:"family_xref"`
	EventIndex int    `json:"event_index"`
}

// ---------------------------------------------------------------------------
// Junction/link types — notes
// ---------------------------------------------------------------------------

// IndividualNoteLink mirrors gedcom_individual_notes_v2.
type IndividualNoteLink struct {
	ID             string `json:"id,omitempty"`
	IndividualXref string `json:"individual_xref"`
	NoteIndex      int    `json:"note_index"`
}

// FamilyNoteLink mirrors gedcom_family_notes_v2.
type FamilyNoteLink struct {
	ID         string `json:"id,omitempty"`
	FamilyXref string `json:"family_xref"`
	NoteIndex  int    `json:"note_index"`
}

// EventNoteLink mirrors gedcom_event_notes_v2.
type EventNoteLink struct {
	ID         string `json:"id,omitempty"`
	EventIndex int    `json:"event_index"`
	NoteIndex  int    `json:"note_index"`
}

// SourceNoteLink mirrors gedcom_source_notes_v2.
type SourceNoteLink struct {
	ID          string `json:"id,omitempty"`
	SourceIndex int    `json:"source_index"`
	NoteIndex   int    `json:"note_index"`
}

// ---------------------------------------------------------------------------
// Junction/link types — sources
// ---------------------------------------------------------------------------

// IndividualSourceLink mirrors gedcom_individual_sources_v2.
type IndividualSourceLink struct {
	ID             string `json:"id,omitempty"`
	IndividualXref string `json:"individual_xref"`
	SourceIndex    int    `json:"source_index"`
	Page           string `json:"page,omitempty"`
	Quality        int    `json:"quality,omitempty"`
	CitationText   string `json:"citation_text,omitempty"`
}

// FamilySourceLink mirrors gedcom_family_sources_v2.
type FamilySourceLink struct {
	ID           string `json:"id,omitempty"`
	FamilyXref   string `json:"family_xref"`
	SourceIndex  int    `json:"source_index"`
	Page         string `json:"page,omitempty"`
	Quality      int    `json:"quality,omitempty"`
	CitationText string `json:"citation_text,omitempty"`
}

// EventSourceLink mirrors gedcom_event_sources_v2.
type EventSourceLink struct {
	ID           string `json:"id,omitempty"`
	EventIndex   int    `json:"event_index"`
	SourceIndex  int    `json:"source_index"`
	Page         string `json:"page,omitempty"`
	Quality      int    `json:"quality,omitempty"`
	CitationText string `json:"citation_text,omitempty"`
}

// ---------------------------------------------------------------------------
// Junction/link types — repositories
// ---------------------------------------------------------------------------

// SourceRepositoryLink mirrors gedcom_source_repositories_v2.
type SourceRepositoryLink struct {
	ID              string `json:"id,omitempty"`
	SourceIndex     int    `json:"source_index"`
	RepositoryIndex int    `json:"repository_index"`
	CallNumber      string `json:"call_number,omitempty"`
}

// ---------------------------------------------------------------------------
// Junction/link types — media
// ---------------------------------------------------------------------------

// IndividualMediaLink mirrors gedcom_individual_media_v2.
type IndividualMediaLink struct {
	ID             string `json:"id,omitempty"`
	IndividualXref string `json:"individual_xref"`
	MediaIndex     int    `json:"media_index"`
}

// FamilyMediaLink mirrors gedcom_family_media_v2.
type FamilyMediaLink struct {
	ID         string `json:"id,omitempty"`
	FamilyXref string `json:"family_xref"`
	MediaIndex int    `json:"media_index"`
}

// SourceMediaLink mirrors gedcom_source_media_v2.
type SourceMediaLink struct {
	ID          string `json:"id,omitempty"`
	SourceIndex int    `json:"source_index"`
	MediaIndex  int    `json:"media_index"`
}

// EventMediaLink mirrors gedcom_event_media_v2.
type EventMediaLink struct {
	ID         string `json:"id,omitempty"`
	EventIndex int    `json:"event_index"`
	MediaIndex int    `json:"media_index"`
}

// ---------------------------------------------------------------------------
// Statistics
// ---------------------------------------------------------------------------

// Stats holds aggregate counts produced during enrichment.
type Stats struct {
	Individuals  int `json:"individuals"`
	Families     int `json:"families"`
	Dates        int `json:"dates"`
	Places       int `json:"places"`
	Surnames     int `json:"surnames"`
	GivenNames   int `json:"given_names"`
	Events       int `json:"events"`
	Notes        int `json:"notes"`
	Sources      int `json:"sources"`
	Repositories int `json:"repositories"`
	Media        int `json:"media"`
	EventMedia   int `json:"event_media"`
}
