// Package gedcom defines the canonical intermediate representation for GEDCOM data.
//
// GedcomDocument is the central type through which all data flows:
//
//	GEDCOM file → parser → GedcomDocument → exporter → GEDCOM / JSON / CSV
//
// The format is a lossless, JSON-serializable representation of GEDCOM files
// that preserves the hierarchical structure and original ordering.
package gedcom

// GedcomDocument is the canonical intermediate representation of a GEDCOM file.
// All parsing produces this type; all exporters consume it.
type GedcomDocument struct {
	Header       GedcomRecord   `json:"header"`
	Individuals  []GedcomRecord `json:"individuals"`
	Families     []GedcomRecord `json:"families"`
	Sources      []GedcomRecord `json:"sources"`
	Notes        []GedcomRecord `json:"notes"`
	Repositories []GedcomRecord `json:"repositories,omitempty"`
	Media        []GedcomRecord `json:"media,omitempty"`
	Submitters   []GedcomRecord `json:"submitters,omitempty"`
	Trailer      GedcomRecord   `json:"trailer"`
}

// GedcomRecord represents a single GEDCOM record (line + children).
// Children are stored as an ordered slice to preserve original file ordering,
// which is required for lossless round-trip to GEDCOM text.
type GedcomRecord struct {
	Level    int            `json:"level"`
	Tag      string         `json:"tag"`
	Xref     string         `json:"xref,omitempty"`
	Value    string         `json:"value,omitempty"`
	Children []GedcomRecord `json:"children,omitempty"`
}

// ChildrenByTag returns all direct children with the given tag.
func (r *GedcomRecord) ChildrenByTag(tag string) []GedcomRecord {
	var result []GedcomRecord
	for _, child := range r.Children {
		if child.Tag == tag {
			result = append(result, child)
		}
	}
	return result
}

// FirstChildByTag returns the first direct child with the given tag, or nil.
func (r *GedcomRecord) FirstChildByTag(tag string) *GedcomRecord {
	for i := range r.Children {
		if r.Children[i].Tag == tag {
			return &r.Children[i]
		}
	}
	return nil
}

// ChildValue returns the value of the first child with the given tag, or "".
func (r *GedcomRecord) ChildValue(tag string) string {
	if child := r.FirstChildByTag(tag); child != nil {
		return child.Value
	}
	return ""
}

// HasChildren returns true if the record has any children.
func (r *GedcomRecord) HasChildren() bool {
	return len(r.Children) > 0
}

// AddChild appends a child record.
func (r *GedcomRecord) AddChild(child GedcomRecord) {
	r.Children = append(r.Children, child)
}

// NewRecord creates a new GedcomRecord with the given level and tag.
func NewRecord(level int, tag string) GedcomRecord {
	return GedcomRecord{Level: level, Tag: tag}
}

// NewRecordWithValue creates a new GedcomRecord with level, tag, and value.
func NewRecordWithValue(level int, tag, value string) GedcomRecord {
	return GedcomRecord{Level: level, Tag: tag, Value: value}
}

// NewRecordWithXref creates a new GedcomRecord with level, xref, and tag.
func NewRecordWithXref(level int, xref, tag string) GedcomRecord {
	return GedcomRecord{Level: level, Tag: tag, Xref: xref}
}

// IndividualCount returns the number of individuals in the document.
func (d *GedcomDocument) IndividualCount() int {
	return len(d.Individuals)
}

// FamilyCount returns the number of families in the document.
func (d *GedcomDocument) FamilyCount() int {
	return len(d.Families)
}

// FindByXref searches all record slices for a record with the given xref.
func (d *GedcomDocument) FindByXref(xref string) *GedcomRecord {
	for i := range d.Individuals {
		if d.Individuals[i].Xref == xref {
			return &d.Individuals[i]
		}
	}
	for i := range d.Families {
		if d.Families[i].Xref == xref {
			return &d.Families[i]
		}
	}
	for i := range d.Sources {
		if d.Sources[i].Xref == xref {
			return &d.Sources[i]
		}
	}
	for i := range d.Notes {
		if d.Notes[i].Xref == xref {
			return &d.Notes[i]
		}
	}
	for i := range d.Repositories {
		if d.Repositories[i].Xref == xref {
			return &d.Repositories[i]
		}
	}
	for i := range d.Media {
		if d.Media[i].Xref == xref {
			return &d.Media[i]
		}
	}
	for i := range d.Submitters {
		if d.Submitters[i].Xref == xref {
			return &d.Submitters[i]
		}
	}
	return nil
}

// XRefIndex builds and returns a map of xref -> *GedcomRecord for fast lookups.
func (d *GedcomDocument) XRefIndex() map[string]*GedcomRecord {
	idx := make(map[string]*GedcomRecord)
	for i := range d.Individuals {
		if d.Individuals[i].Xref != "" {
			idx[d.Individuals[i].Xref] = &d.Individuals[i]
		}
	}
	for i := range d.Families {
		if d.Families[i].Xref != "" {
			idx[d.Families[i].Xref] = &d.Families[i]
		}
	}
	for i := range d.Sources {
		if d.Sources[i].Xref != "" {
			idx[d.Sources[i].Xref] = &d.Sources[i]
		}
	}
	for i := range d.Notes {
		if d.Notes[i].Xref != "" {
			idx[d.Notes[i].Xref] = &d.Notes[i]
		}
	}
	for i := range d.Repositories {
		if d.Repositories[i].Xref != "" {
			idx[d.Repositories[i].Xref] = &d.Repositories[i]
		}
	}
	for i := range d.Media {
		if d.Media[i].Xref != "" {
			idx[d.Media[i].Xref] = &d.Media[i]
		}
	}
	for i := range d.Submitters {
		if d.Submitters[i].Xref != "" {
			idx[d.Submitters[i].Xref] = &d.Submitters[i]
		}
	}
	return idx
}
