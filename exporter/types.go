package exporter

// DenormalizedJSON is the API-friendly flattened representation of GEDCOM data.
// Names are resolved, events are inline, and relationships include names (not just xrefs).
type DenormalizedJSON struct {
	File        FileMetadata              `json:"file"`
	Individuals []DenormalizedIndividual  `json:"individuals"`
	Families    []DenormalizedFamily      `json:"families"`
	Notes       map[string]DenormalizedNote   `json:"notes"`
	Sources     map[string]DenormalizedSource `json:"sources"`
}

// FileMetadata contains document-level information.
type FileMetadata struct {
	IndividualsCount int `json:"individuals_count"`
	FamiliesCount    int `json:"families_count"`
}

// DenormalizedIndividual represents a person with resolved fields.
type DenormalizedIndividual struct {
	Xref       string                     `json:"xref"`
	Name       string                     `json:"name"`
	GivenNames []string                   `json:"given_names"`
	Surnames   []string                   `json:"surnames"`
	Sex        string                     `json:"sex"`
	Birth      *DenormalizedEvent         `json:"birth,omitempty"`
	Death      *DenormalizedEvent         `json:"death,omitempty"`
	Parents    []DenormalizedRelationship `json:"parents"`
	Spouses    []DenormalizedRelationship `json:"spouses"`
	Children   []DenormalizedRelationship `json:"children"`
	Events     []DenormalizedEvent        `json:"events"`
	Notes      []DenormalizedNoteRef      `json:"notes"`
	Sources    []DenormalizedSourceRef    `json:"sources"`
}

// DenormalizedFamily represents a family unit with resolved fields.
type DenormalizedFamily struct {
	Xref     string                     `json:"xref"`
	Husband  *DenormalizedRelationship  `json:"husband,omitempty"`
	Wife     *DenormalizedRelationship  `json:"wife,omitempty"`
	Children []DenormalizedRelationship `json:"children"`
	Marriage *DenormalizedEvent         `json:"marriage,omitempty"`
	Divorce  *DenormalizedEvent         `json:"divorce,omitempty"`
	Events   []DenormalizedEvent        `json:"events"`
	Notes    []DenormalizedNoteRef      `json:"notes"`
	Sources  []DenormalizedSourceRef    `json:"sources"`
}

// DenormalizedEvent represents a life event.
type DenormalizedEvent struct {
	Type     string `json:"type"`
	Date     string `json:"date,omitempty"`
	Place    string `json:"place,omitempty"`
	DateYear int    `json:"date_year,omitempty"`
}

// DenormalizedRelationship represents a cross-reference with optional resolved name.
type DenormalizedRelationship struct {
	Xref         string `json:"xref"`
	Name         string `json:"name,omitempty"`
	FamilyXref   string `json:"family_xref,omitempty"`
	Relationship string `json:"relationship,omitempty"`
}

// DenormalizedNote represents a note record.
type DenormalizedNote struct {
	Xref    string `json:"xref"`
	Content string `json:"content"`
}

// DenormalizedNoteRef is a reference to a note.
type DenormalizedNoteRef struct {
	Xref string `json:"xref"`
}

// DenormalizedSource represents a source record.
type DenormalizedSource struct {
	Xref        string `json:"xref"`
	Title       string `json:"title,omitempty"`
	Author      string `json:"author,omitempty"`
	Publication string `json:"publication,omitempty"`
}

// DenormalizedSourceRef is a reference to a source.
type DenormalizedSourceRef struct {
	Xref string `json:"xref"`
}
