// Package gedcom defines the canonical intermediate representation for GEDCOM data.
//
// GedcomDocument is the central type through which all data flows:
//
//	GEDCOM file → parser → GedcomDocument → exporter → GEDCOM / JSON / CSV
//
// The format is a lossless, JSON-serializable representation of GEDCOM files
// that preserves the hierarchical structure and original ordering.
package gedcom

import "strings"

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
	// Encoding records how the file's bytes were decoded.
	Encoding EncodingInfo `json:"encoding"`
	// LineCount is the number of lines in the file, empty lines included.
	LineCount int `json:"lineCount"`
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
	// Line is the 1-based line number of this node in the original file.
	// It is 0 for nodes that were not parsed from a file.
	Line int `json:"line,omitempty"`
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

// ChildText returns the value of the first child with tag, with that
// child's CONC lines joined verbatim and its CONT lines joined with "\n", in
// file order: a text value split across lines, read whole. Use it for the
// text tags of sources and citations (TITL, AUTH, ABBR, PUBL, TEXT, PAGE).
func (r *GedcomRecord) ChildText(tag string) string {
	child := r.FirstChildByTag(tag)
	if child == nil {
		return ""
	}
	v := child.Value
	for _, c := range child.Children {
		switch c.Tag {
		case "CONC":
			v += c.Value
		case "CONT":
			v += "\n" + c.Value
		}
	}
	return v
}

// TrimmedValue returns Value without leading and trailing whitespace. The
// parser keeps values verbatim, so "1 HUSB  @I1@" has Value " @I1@"; read
// pointers, SEX, NAME, DATE, PLAC and other code-like values with this.
// CONC, CONT, NOTE and TEXT values are text: read them verbatim.
func (r GedcomRecord) TrimmedValue() string {
	return strings.TrimSpace(r.Value)
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
// Last record wins and empty xrefs are skipped; for FindByXref's first-wins
// behaviour use RecordsByXref.
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

// RecordsByXref returns a map of xref -> *GedcomRecord with the same lookup result
// as FindByXref for every xref: when several records share an xref, the first
// one in FindByXref's order wins (Individuals, Families, Sources, Notes,
// Repositories, Media, Submitters). Records with an empty xref are included
// under "" so that FindByXref("") and RecordsByXref()[""] agree. This differs from
// XRefIndex, where the last record wins and empty xrefs are skipped.
//
// The pointers point into the document's slices. Build the index after
// parsing and do not append to those slices while it is in use.
func (d *GedcomDocument) RecordsByXref() map[string]*GedcomRecord {
	n := len(d.Individuals) + len(d.Families) + len(d.Sources) + len(d.Notes) +
		len(d.Repositories) + len(d.Media) + len(d.Submitters)
	idx := make(map[string]*GedcomRecord, n)
	add := func(recs []GedcomRecord) {
		for i := range recs {
			if _, dup := idx[recs[i].Xref]; !dup {
				idx[recs[i].Xref] = &recs[i]
			}
		}
	}
	add(d.Individuals)
	add(d.Families)
	add(d.Sources)
	add(d.Notes)
	add(d.Repositories)
	add(d.Media)
	add(d.Submitters)
	return idx
}
