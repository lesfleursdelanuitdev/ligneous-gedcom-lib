# Intermediate Format Documentation

This document describes the two core data structures produced by `ligneous-gedcom-lib`:

1. **GedcomDocument** — the raw, lossless parse tree
2. **EnrichedDocument** — the normalized, database-ready extraction

## Pipeline Overview

```
GEDCOM file
    │
    ▼
parser.Parse()  ──►  GedcomDocument   (lossless tree)
                          │
                          ▼
                    validator.Validate()  ──►  []ValidationError
                          │
                          ▼
                    enricher.Enrich()  ──►  EnrichedDocument  (normalized)
                                                │
                                                ▼
                                        enricher.GenerateIDs()  (optional: assigns UUIDs)
                                                │
                                        ┌───────┴───────┐
                                        ▼               ▼
                                  DB storage       exporter.*
```

---

## 1. GedcomDocument

**Package:** `gedcom`  
**File:** `gedcom/types.go`

The raw parse tree. Every GEDCOM line becomes a `GedcomRecord` node, preserving original ordering and hierarchy. This enables lossless round-trip: parse a GEDCOM file, then export it back to identical GEDCOM text.

### Structure

```go
type GedcomDocument struct {
    Header       GedcomRecord   // 0 HEAD
    Individuals  []GedcomRecord // 0 @Ix@ INDI
    Families     []GedcomRecord // 0 @Fx@ FAM
    Sources      []GedcomRecord // 0 @Sx@ SOUR
    Notes        []GedcomRecord // 0 @Nx@ NOTE
    Repositories []GedcomRecord // 0 @Rx@ REPO
    Media        []GedcomRecord // 0 @Mx@ OBJE
    Submitters   []GedcomRecord // 0 @Ux@ SUBM
    Trailer      GedcomRecord   // 0 TRLR
}
```

### GedcomRecord

```go
type GedcomRecord struct {
    Level    int            // 0, 1, 2, ...
    Tag      string         // "INDI", "NAME", "BIRT", etc.
    Xref     string         // "@I1@" (top-level records only)
    Value    string         // "John /Doe/", "1 JAN 1950", etc.
    Children []GedcomRecord // ordered sub-records
}
```

**Key properties:**
- Children are ordered — preserves the original file order
- Xref is only set on level-0 records that have cross-reference identifiers
- Value may be empty (e.g., `1 BIRT` has no value; the date/place are children)

### Helper Methods

| Method | Description |
|--------|-------------|
| `ChildrenByTag(tag)` | All direct children with the given tag |
| `FirstChildByTag(tag)` | First child with the tag, or nil |
| `ChildValue(tag)` | Value of the first child with the tag, or "" |
| `FindByXref(xref)` | Search all record slices for a record |
| `XRefIndex()` | Build a map[string]*GedcomRecord for fast lookups |

### JSON Example

```json
{
  "header": {
    "level": 0, "tag": "HEAD",
    "children": [
      {"level": 1, "tag": "GEDC", "children": [
        {"level": 2, "tag": "VERS", "value": "5.5.1"}
      ]}
    ]
  },
  "individuals": [
    {
      "level": 0, "tag": "INDI", "xref": "@I1@",
      "children": [
        {"level": 1, "tag": "NAME", "value": "John /Doe/",
         "children": [
           {"level": 2, "tag": "GIVN", "value": "John"},
           {"level": 2, "tag": "SURN", "value": "Doe"}
         ]},
        {"level": 1, "tag": "SEX", "value": "M"},
        {"level": 1, "tag": "BIRT", "children": [
          {"level": 2, "tag": "DATE", "value": "1 JAN 1950"},
          {"level": 2, "tag": "PLAC", "value": "New York, USA"}
        ]}
      ]
    }
  ]
}
```

---

## 2. EnrichedDocument

**Package:** `enricher`  
**File:** `enricher/types.go`

The normalized extraction. Converts the tree structure into flat, deduplicated tables that map directly to PostgreSQL rows. Cross-references use GEDCOM XREFs (for individuals/families) and integer slice indexes (for lookup tables).

### Top-Level Structure

```go
type EnrichedDocument struct {
    Document *gedcom.GedcomDocument  // original parse tree (retained)

    // Entity summaries
    Individuals []EnrichedIndividual
    Families    []EnrichedFamily

    // Normalized lookup tables (deduplicated)
    Dates      []ParsedDate
    Places     []ParsedPlace
    Surnames   []Surname
    GivenNames []GivenName
    Events     []Event

    // Top-level records
    Notes        []EnrichedNote
    Sources      []EnrichedSource
    Repositories []EnrichedRepository
    Media        []EnrichedMedia

    // Relationship edges
    Spouses        []SpouseEdge
    ParentChild    []ParentChildEdge
    FamilyChildren []FamilyChildEdge

    // Junction links — names
    IndividualSurnames   []IndividualSurnameLink
    IndividualGivenNames []IndividualGivenNameLink
    FamilySurnames       []FamilySurnameLink

    // Junction links — events
    IndividualEvents []IndividualEventLink
    FamilyEvents     []FamilyEventLink

    // Junction links — notes
    IndividualNotes []IndividualNoteLink
    FamilyNotes     []FamilyNoteLink
    EventNotes      []EventNoteLink
    SourceNotes     []SourceNoteLink

    // Junction links — sources
    IndividualSources []IndividualSourceLink
    FamilySources     []FamilySourceLink
    EventSources      []EventSourceLink

    // Junction links — repositories
    SourceRepositories []SourceRepositoryLink

    // Junction links — media
    IndividualMedia []IndividualMediaLink
    FamilyMedia     []FamilyMediaLink
    SourceMedia     []SourceMediaLink

    Stats Stats
}
```

---

## 3. Entity Types

### EnrichedIndividual

Maps to `gedcom_individuals_v2`.

| Field | Type | Description |
|-------|------|-------------|
| ID | string | UUID (populated by GenerateIDs) |
| Xref | string | GEDCOM xref (e.g. "@I1@") |
| FullName | string | Raw NAME value (e.g. "John /Doe/") |
| FullNameLower | string | Lowercased full name for searching |
| Sex | string | "M", "F", or "" |
| BirthDateIndex | int | Index into Dates (-1 if none) |
| BirthPlaceIndex | int | Index into Places (-1 if none) |
| DeathDateIndex | int | Index into Dates (-1 if none) |
| DeathPlaceIndex | int | Index into Places (-1 if none) |

### EnrichedFamily

Maps to `gedcom_families_v2`.

| Field | Type | Description |
|-------|------|-------------|
| ID | string | UUID |
| Xref | string | GEDCOM xref (e.g. "@F1@") |
| HusbandXref | string | Xref of husband individual |
| WifeXref | string | Xref of wife individual |
| MarriageDateIndex | int | Index into Dates (-1 if none) |
| MarriagePlaceIndex | int | Index into Places (-1 if none) |
| ChildrenCount | int | Number of CHIL records |

### ParsedDate

Maps to `gedcom_dates_v2`. Deduplicated by SHA256 hash.

| Field | Type | Description |
|-------|------|-------------|
| ID | string | UUID |
| Original | string | Raw GEDCOM date string |
| Type | DateType | EXACT, ABOUT, BEFORE, AFTER, BETWEEN, FROM_TO, CALCULATED, ESTIMATED, UNKNOWN |
| Calendar | string | "GREGORIAN" (default), "JULIAN", "HEBREW", "FRENCH_R" |
| Year, Month, Day | int | Start date components (0 if absent) |
| EndYear, EndMonth, EndDay | int | End date for ranges (0 if not a range) |
| Hash | string | SHA256 for deduplication |

### ParsedPlace

Maps to `gedcom_places_v2`. Deduplicated by SHA256 hash.

| Field | Type | Description |
|-------|------|-------------|
| ID | string | UUID |
| Original | string | Raw GEDCOM place string |
| Name | string | Most specific place name |
| County | string | County/district (from 4-part places) |
| State | string | State/province |
| Country | string | Country |
| Latitude, Longitude | float64 | Coordinates (0 if not available) |
| Hash | string | SHA256 for deduplication |

### Surname

Maps to `gedcom_surnames_v2`.

| Field | Type | Description |
|-------|------|-------------|
| ID | string | UUID |
| Value | string | Original casing |
| Lower | string | Lowercased for dedup/search |
| Frequency | int | Number of individual bearers |

### GivenName

Maps to `gedcom_given_names_v2`. Same structure as Surname.

### Event

Maps to `gedcom_events_v2`.

| Field | Type | Description |
|-------|------|-------------|
| ID | string | UUID |
| Index | int | Position in the Events slice |
| EventType | string | GEDCOM tag: BIRT, DEAT, MARR, etc. |
| CustomType | string | TYPE sub-tag value (for EVEN tags) |
| DateIndex | int | Index into Dates (-1 if none) |
| PlaceIndex | int | Index into Places (-1 if none) |
| Value | string | Event value |
| Cause | string | CAUS sub-tag |
| Agency | string | AGNC sub-tag |
| OwnerXref | string | Xref of the owning individual or family |
| OwnerType | string | "INDI" or "FAM" |
| SortOrder | int | Order within the owner's events |

### EnrichedNote

Maps to `gedcom_notes_v2`.

| Field | Type | Description |
|-------|------|-------------|
| ID | string | UUID |
| Xref | string | GEDCOM xref (empty for inline notes) |
| Content | string | Full note text (with CONT/CONC merged) |
| IsTopLevel | bool | True for top-level NOTE records |

### EnrichedSource

Maps to `gedcom_sources_v2`.

| Field | Type | Description |
|-------|------|-------------|
| ID | string | UUID |
| Xref | string | GEDCOM xref |
| Title | string | TITL sub-tag |
| Author | string | AUTH sub-tag |
| Abbreviation | string | ABBR sub-tag |
| Publication | string | PUBL sub-tag |
| Text | string | TEXT sub-tag |
| RepositoryXref | string | Referenced REPO xref |
| CallNumber | string | CALN from the REPO reference |

### EnrichedRepository

Maps to `gedcom_repositories_v2`.

| Field | Type | Description |
|-------|------|-------------|
| ID | string | UUID |
| Xref | string | GEDCOM xref |
| Name | string | NAME sub-tag |
| Address | string | ADDR value |
| City | string | ADDR.CITY |
| State | string | ADDR.STAE |
| Country | string | ADDR.CTRY |
| Phone | string | PHON sub-tag |
| Email | string | EMAIL sub-tag |
| Website | string | WWW sub-tag |

### EnrichedMedia

Maps to `gedcom_media_v2`.

| Field | Type | Description |
|-------|------|-------------|
| ID | string | UUID |
| Xref | string | GEDCOM xref (empty for inline media) |
| File | string | FILE sub-tag value |
| Form | string | FORM sub-tag (jpg, gif, etc.) |
| Title | string | TITL sub-tag |

---

## 4. Relationship Edges

### SpouseEdge

Maps to `gedcom_spouses_v2`. Stored **bidirectionally** — if A married B, there are two edges: A→B and B→A.

| Field | Type |
|-------|------|
| ID | string (UUID) |
| IndividualXref | string |
| SpouseXref | string |
| FamilyXref | string |

### ParentChildEdge

Maps to `gedcom_parent_child_v2`.

| Field | Type | Description |
|-------|------|-------------|
| ID | string | UUID |
| ParentXref | string | Parent's xref |
| ChildXref | string | Child's xref |
| FamilyXref | string | Family context |
| ParentType | string | "father" or "mother" |
| RelationshipType | string | "biological", "adopted", "foster", "sealing" |
| Pedigree | string | Raw PEDI value from FAMC |

### FamilyChildEdge

Maps to `gedcom_family_children_v2`.

| Field | Type | Description |
|-------|------|-------------|
| ID | string | UUID |
| FamilyXref | string | Family's xref |
| ChildXref | string | Child's xref |
| BirthOrder | int | 1-based order within family |

---

## 5. Junction/Link Types

All junction types have an `ID string` field for UUID assignment.

### Name Junctions

| Type | Key Fields | Maps To |
|------|-----------|---------|
| IndividualSurnameLink | IndividualXref, SurnameIndex, NameType, IsPrimary | gedcom_individual_surnames_v2 |
| IndividualGivenNameLink | IndividualXref, GivenNameIndex, Position, IsPrimary | gedcom_individual_given_names_v2 |
| FamilySurnameLink | FamilyXref, SurnameIndex, IsPrimary | gedcom_family_surnames_v2 |

### Event Junctions

| Type | Key Fields | Maps To |
|------|-----------|---------|
| IndividualEventLink | IndividualXref, EventIndex, Role | gedcom_individual_events_v2 |
| FamilyEventLink | FamilyXref, EventIndex | gedcom_family_events_v2 |

### Note Junctions

| Type | Key Fields | Maps To |
|------|-----------|---------|
| IndividualNoteLink | IndividualXref, NoteIndex | gedcom_individual_notes_v2 |
| FamilyNoteLink | FamilyXref, NoteIndex | gedcom_family_notes_v2 |
| EventNoteLink | EventIndex, NoteIndex | gedcom_event_notes_v2 |
| SourceNoteLink | SourceIndex, NoteIndex | gedcom_source_notes_v2 |

### Source Junctions

| Type | Key Fields | Maps To |
|------|-----------|---------|
| IndividualSourceLink | IndividualXref, SourceIndex, Page, Quality, CitationText | gedcom_individual_sources_v2 |
| FamilySourceLink | FamilyXref, SourceIndex, Page, Quality, CitationText | gedcom_family_sources_v2 |
| EventSourceLink | EventIndex, SourceIndex, Page, Quality, CitationText | gedcom_event_sources_v2 |

### Repository Junction

| Type | Key Fields | Maps To |
|------|-----------|---------|
| SourceRepositoryLink | SourceIndex, RepositoryIndex, CallNumber | gedcom_source_repositories_v2 |

### Media Junctions

| Type | Key Fields | Maps To |
|------|-----------|---------|
| IndividualMediaLink | IndividualXref, MediaIndex | gedcom_individual_media_v2 |
| FamilyMediaLink | FamilyXref, MediaIndex | gedcom_family_media_v2 |
| SourceMediaLink | SourceIndex, MediaIndex | gedcom_source_media_v2 |

---

## 6. UUID Assignment

UUIDs are **not** assigned during enrichment. This is intentional — the enricher produces a pure in-memory structure usable for analysis without database overhead.

To assign UUIDs for database storage:

```go
ed := enricher.Enrich(doc)
enricher.GenerateIDs(ed)
// Now every entity, edge, and junction link has a UUID in its ID field
```

`GenerateIDs` walks every slice in the EnrichedDocument and assigns `uuid.New().String()` to each element's `ID` field.

---

## 7. Database Mapping

### Entity → Table

| EnrichedDocument Field | PostgreSQL Table |
|----------------------|-----------------|
| Individuals | gedcom_individuals_v2 |
| Families | gedcom_families_v2 |
| Dates | gedcom_dates_v2 |
| Places | gedcom_places_v2 |
| Surnames | gedcom_surnames_v2 |
| GivenNames | gedcom_given_names_v2 |
| Events | gedcom_events_v2 |
| Notes | gedcom_notes_v2 |
| Sources | gedcom_sources_v2 |
| Repositories | gedcom_repositories_v2 |
| Media | gedcom_media_v2 |

### Edge/Junction → Table

| EnrichedDocument Field | PostgreSQL Table |
|----------------------|-----------------|
| Spouses | gedcom_spouses_v2 |
| ParentChild | gedcom_parent_child_v2 |
| FamilyChildren | gedcom_family_children_v2 |
| IndividualSurnames | gedcom_individual_surnames_v2 |
| IndividualGivenNames | gedcom_individual_given_names_v2 |
| FamilySurnames | gedcom_family_surnames_v2 |
| IndividualEvents | gedcom_individual_events_v2 |
| FamilyEvents | gedcom_family_events_v2 |
| IndividualNotes | gedcom_individual_notes_v2 |
| FamilyNotes | gedcom_family_notes_v2 |
| EventNotes | gedcom_event_notes_v2 |
| SourceNotes | gedcom_source_notes_v2 |
| IndividualSources | gedcom_individual_sources_v2 |
| FamilySources | gedcom_family_sources_v2 |
| EventSources | gedcom_event_sources_v2 |
| SourceRepositories | gedcom_source_repositories_v2 |
| IndividualMedia | gedcom_individual_media_v2 |
| FamilyMedia | gedcom_family_media_v2 |
| SourceMedia | gedcom_source_media_v2 |

### FK Resolution

Junction links use **integer slice indexes** to reference lookup tables. When inserting into the database:

1. Insert all lookup entities (Dates, Places, Surnames, GivenNames) first
2. Build a mapping from slice index → database UUID
3. Insert junction rows using the resolved UUIDs

For individual/family references, use the GEDCOM xref to find the corresponding database row.

---

## 8. Statistics

```go
type Stats struct {
    Individuals  int
    Families     int
    Dates        int  // unique dates
    Places       int  // unique places
    Surnames     int  // unique surnames
    GivenNames   int  // unique given names
    Events       int
    Notes        int
    Sources      int
    Repositories int
    Media        int
}
```

---

## 9. Usage Examples

### Basic: Parse and Enrich

```go
package main

import (
    "fmt"
    "os"

    "github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/enricher"
    "github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/parser"
    "github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/validator"
)

func main() {
    f, _ := os.Open("family.ged")
    defer f.Close()

    doc, warnings, err := parser.Parse(f)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Parsed with %d warnings\n", len(warnings))

    errors := validator.Validate(doc)
    fmt.Printf("Validation: %d errors\n", len(errors))

    ed := enricher.Enrich(doc)
    fmt.Printf("Individuals: %d\n", ed.Stats.Individuals)
    fmt.Printf("Unique dates: %d\n", ed.Stats.Dates)
    fmt.Printf("Unique places: %d\n", ed.Stats.Places)
    fmt.Printf("Events: %d\n", ed.Stats.Events)
    fmt.Printf("Notes: %d\n", ed.Stats.Notes)
    fmt.Printf("Sources: %d\n", ed.Stats.Sources)
}
```

### With UUIDs for Database Storage

```go
ed := enricher.Enrich(doc)
enricher.GenerateIDs(ed)

for _, indi := range ed.Individuals {
    fmt.Printf("INSERT INTO gedcom_individuals_v2 (id, xref, full_name) VALUES ('%s', '%s', '%s')\n",
        indi.ID, indi.Xref, indi.FullName)
}
```

### Accessing Relationships

```go
ed := enricher.Enrich(doc)

// Find all children of a specific individual
for _, pc := range ed.ParentChild {
    if pc.ParentXref == "@I1@" {
        fmt.Printf("Child: %s (type: %s)\n", pc.ChildXref, pc.RelationshipType)
    }
}

// Find marriage date for a family
for _, fam := range ed.Families {
    if fam.MarriageDateIndex >= 0 {
        date := ed.Dates[fam.MarriageDateIndex]
        fmt.Printf("Family %s married: %s\n", fam.Xref, date.Original)
    }
}
```

### Resolving FK Indexes

```go
for _, evt := range ed.Events {
    dateStr := "(no date)"
    if evt.DateIndex >= 0 {
        dateStr = ed.Dates[evt.DateIndex].Original
    }
    placeStr := "(no place)"
    if evt.PlaceIndex >= 0 {
        placeStr = ed.Places[evt.PlaceIndex].Original
    }
    fmt.Printf("%s: %s at %s\n", evt.EventType, dateStr, placeStr)
}
```
