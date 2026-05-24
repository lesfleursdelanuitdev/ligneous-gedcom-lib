# Testing guide — ligneous-gedcom-lib

## Running the tests

```bash
# All packages (from the repo root)
go test ./...

# Single package
go test ./parser/
go test ./validator/
go test ./enricher/
go test ./exporter/
go test ./reconciliation/
go test ./pedigreedoc/
```

## Current status

| Package | Status | Notes |
|---|---|---|
| `parser` | ✅ all pass | |
| `validator` | ✅ all pass | |
| `pedigreedoc` | ✅ all pass | |
| `reconciliation` | ✅ all pass | |
| `enricher` | ❌ 2 failing | `TestEnrichNestedNotes`, `TestEnrichGEDCOM55EventWhitelist` |
| `exporter` | ❌ 1 failing | `TestFromEnriched_ScalarOccupationWithoutEvent` |

## Test inventory

### `parser`

File: [parser/parser_test.go](../parser/parser_test.go)

| Test | What it covers |
|---|---|
| `TestParseMinimal` | Full field check on a minimal valid GEDCOM: header, trailer, single individual with NAME/SEX/BIRT |
| `TestParseFamilies` | FAM record with HUSB/WIFE/CHIL and a MARR event |
| `TestParseEmptyLine` | Empty lines produce a parser warning, not a fatal error |
| `TestParseInvalidLevel` | Non-numeric level token returns an error |
| `TestParseMissingTag` | Line with only a level number (no tag) returns an error |
| `TestParseNotes` | NOTE records with CONT/CONC continuations |
| `TestParseSources` | SOUR records |
| `TestParseXRefIndex` | `doc.XRefIndex()` map covers INDI, FAM, SOUR xrefs |
| `TestParseBOM` | UTF-8 BOM at file start is stripped cleanly |
| `TestParseRealFiles` | Parses every `testdata/*.ged` file without a fatal error |
| `TestParseMalformedFiles` | Graceful handling of missing-header and invalid-level files |

### `validator`

File: [validator/validator_test.go](../validator/validator_test.go)

| Test | What it covers |
|---|---|
| `TestValidateMinimalDoc` | Valid minimal doc produces zero findings |
| `TestValidateMissingHeader` | `MISSING_HEADER` error when HEAD is absent |
| `TestValidateMissingTrailer` | `MISSING_TRAILER` warning when TRLR is absent |
| `TestValidateMissingName` | `MISSING_NAME` warning for INDI without a NAME tag |
| `TestValidateInvalidSex` | `INVALID_SEX` error for unrecognised SEX value |
| `TestValidateEmptyFamily` | `EMPTY_FAMILY` warning for FAM with no HUSB/WIFE/CHIL |
| `TestValidateBrokenXRef` | `ORPHANED_FAMC` / `ORPHANED_FAMS` errors for dangling family xrefs |
| `TestValidateAssociations` | `ORPHANED_ASSO` error for ASSO pointing to non-existent individual |
| `TestValidateDateConsistency` | `DEATH_BEFORE_BIRTH` warning when DEAT date precedes BIRT date |
| `TestValidateSwappedOppositeSexSpouses` | `SWAPPED_SPOUSES` warning when HUSB is F or WIFE is M |
| `TestValidateNoSwappedWarningForSameSex` | No false positive for same-sex couples |
| `TestValidateChildBornBeforeFather` | `CHILD_BORN_BEFORE_PARENT` warning |
| `TestMotherTooYoungAssociatedXrefsOrder` | `AssociatedXrefs` contains both parent and child xrefs |
| `TestValidateOrphanedFamcCode` | Confirms error code is `ORPHANED_FAMC` (not a generic string) |
| `TestValidateAgeAtDeathExceeds120Code` | `AGE_AT_DEATH_EXCEEDS_120` warning code |
| `TestValidateRealFiles` | Validates every `testdata/*.ged` file without panicking |

### `enricher`

Files: [enricher/enricher_test.go](../enricher/enricher_test.go), [date_format_test.go](../enricher/date_format_test.go), [event_label_test.go](../enricher/event_label_test.go)

| Test | What it covers |
|---|---|
| `TestFormatGEDCOMDate_*` | Date formatting: exact, empty, about-month-year, between |
| `TestParseDateExact` | Exact date parsing: day-month-year, month-year, year-only |
| `TestParseDateModifiers` | ABT / BEF / AFT / CAL / EST prefixes (case-insensitive) |
| `TestParseDateRange` | BET…AND and FROM…TO ranges |
| `TestParseDateDedup` / `TestParseDateEmpty` / `TestParseDateCalendar` | Edge cases |
| `TestParsePlace*` | Place parsing with one to four comma-separated parts, dedup |
| `TestExtractSurnameFromFullName` / `TestExtractGivenFromFullName` | Name field decomposition |
| `TestEnrichFull` | Full enrichment of a multi-individual document |
| `TestEnrichNestedNotes` | ❌ Notes nested under events — currently failing |
| `TestEnrichMixedParentageFamcNote` | FAMC pedigree note (biological vs adopted) |
| `TestEnrichGEDCOM55EventWhitelist` | ❌ GEDCOM 5.5 event tag whitelist — currently failing |
| `TestEnrichAssociations` | ASSO records become typed associations |
| `TestEnrichNestedNoteDedup` | Identical nested notes are deduplicated |
| `TestGenerateIDs` | Stable UUID generation for all enriched entities |
| `TestEnrichMultipleFamilies` | Individual appearing in multiple families |
| `TestEnrichRealFiles` | Enriches every `testdata/*.ged` file without panicking |
| `TestEventCatalogTag_EvenCustom` | Custom EVEN tags go through the catalog |
| `TestEventLabelFor` | Human-readable label lookup for standard event tags |

### `exporter`

Files: [exporter/from_enriched_test.go](../exporter/from_enriched_test.go), [gedcom_test.go](../exporter/gedcom_test.go), [gedcom_line_wrap_test.go](../exporter/gedcom_line_wrap_test.go), [csv_test.go](../exporter/csv_test.go), [json_test.go](../exporter/json_test.go)

| Test | What it covers |
|---|---|
| `TestFromEnriched_Minimal` | Round-trip from minimal EnrichedDocument back to GEDCOM |
| `TestFromEnriched_NoteXrefPointerFormatting` | NOTE xref pointers use the correct `@N1@` delimiter form |
| `TestFromEnriched_TopLevelNoteXrefWithoutDelimiters` | Top-level NOTE xrefs without surrounding `@` are normalised |
| `TestFromEnriched_DateUsesGedcomMonthToken` | Dates export using uppercase GEDCOM month tokens (JAN, FEB, …) |
| `TestFromEnriched_MediaOBJELinks` | OBJE links are emitted for media attachments |
| `TestFromEnriched_MediaDescriptionAsInlineNote` | Media description becomes an inline NOTE |
| `TestFromEnriched_DeathCause` | CAUS tag emitted under DEAT |
| `TestFromEnriched_FamilySurnameNote` | Family surname note emitted as NOTE under FAM |
| `TestFromEnriched_EventSourceAndMediaUnderEvent` | SOUR and OBJE emitted as children of the event record |
| `TestFromEnriched_ScalarOccupationWithoutEvent` | ❌ Scalar OCCU without a linked event — currently failing |
| `TestFromEnriched_NoteReferences` | NOTE xref pointers resolve in exported output |
| `TestFromEnriched_RealFiles` | Export all `testdata/*.ged` files via parse→enrich→export round-trip |
| `TestFromEnriched_SingleParentEmitsFAMS` | Single parent (no spouse) still emits a FAMS link |
| `TestFromEnriched_MixedParentageFamcExport` | Adopted/biological FAMC pedigree note survives round-trip |
| `TestFromEnriched_Associations` | ASSO records survive parse→enrich→export |
| `TestMaxPhysicalLineEnrichedNoteRoundTrip` | Long note values are split at the 255-byte GEDCOM line limit |
| `TestToGEDCOM_NormalizesLevel0XrefDelimiters` | Level-0 xrefs are normalised to `@XREF@ TAG` form |
| `TestToGEDCOM_Minimal` | Raw-AST-to-GEDCOM serialisation |
| `TestToGEDCOM_RoundTrip` | Parse then re-serialise produces structurally equivalent output |
| `TestToGEDCOM_RealFiles` | Raw serialisation of all `testdata/*.ged` files |
| `TestToCSV_Minimal` / `TestToCSV_FamilyRelationships` | CSV columns and family relationship rows |
| `TestToCSV_RealFiles` | CSV export of all testdata files |
| `TestToJSON_*` | JSON serialisation including nested notes and media |

### `reconciliation`

Files: [reconciliation/reconciliation_test.go](../reconciliation/reconciliation_test.go), [hungarian_test.go](../reconciliation/hungarian_test.go), [options_json_test.go](../reconciliation/options_json_test.go)

| Test | What it covers |
|---|---|
| `TestMinCostAssignment_2x2` | Hungarian algorithm on a 2×2 cost matrix |
| `TestHungarianPairsFromRectangular` | Rectangular (non-square) cost matrix |
| `TestMaxWeightBipartiteSquareSkipsDummy` | Dummy rows inserted for non-square case are excluded from output |
| `TestMergeReconcileOptionsFromJSON_EmptyUsesDefaults` | Empty JSON produces default options |
| `TestMergeReconcileOptionsFromJSON_ExplicitFalse` | Explicit `false` in JSON overrides a `true` default |
| `TestMergeReconcileOptionsFromJSON_PartialOverlay` | Partial JSON only overrides the specified fields |
| `TestBuildMergePlan_StableIDAlignment` | Individuals with matching stable UUIDs are paired |
| `TestBuildMergePlan_BirthYearConflict` | Birth-year mismatch lowers match confidence |
| `TestBuildMergePlan_XrefAlignmentWithoutID` | Xref-only alignment when no stable ID is present |
| `TestBuildMergePlan_XrefUUIDMismatchPossibleMatch` | Xref matches UUID mismatch → `possible` status |
| `TestBuildMergePlan_SoftMatchDifferentXref` | Name+date soft match with different xref |
| `TestNewReconciliationSession` | Session initialisation and state |
| `TestIndividualDiff_AttachmentFingerprints` | Attachment fingerprinting for diff output |

### `pedigreedoc`

File: [pedigreedoc/pedigreedoc_test.go](../pedigreedoc/pedigreedoc_test.go)

| Test | What it covers |
|---|---|
| `TestFormatParseRoundTrip` | Pedigree document survives a format→parse round-trip |
| `TestParseCaseInsensitivePrefix` | Pedigree type prefix is case-insensitive |
| `TestIsMixedBiologicalAdoptivePair` | Detects mixed biological/adoptive parentage pairs |

## Testdata

`testdata/` holds real-world GEDCOM files used by every package's `TestXxx_RealFiles` test:

| File | Description |
|---|---|
| `minimal55.ged` | Bare-minimum GEDCOM 5.5 file |
| `minimal551.ged` | Bare-minimum GEDCOM 5.5.1 file |
| `comprehensive551.ged` | Wide coverage of 5.5.1 tags |
| `gracis.ged` | Real family tree (small) |
| `tree1.ged` | Real family tree (small) |
| `xavier.ged` | Real family tree (small) |
| `royal92.ged` | British royal family (mid-size) |
| `pres2020.ged` | US presidents (mid-size) |

`testdata/malformed/` holds intentionally broken files for negative-path tests:

| File | Defect |
|---|---|
| `missing-header.ged` | File starts with INDI instead of HEAD |
| `missing-xref.ged` | Level-0 record with no xref |
| `invalid-xref.ged` | Malformed xref delimiter |
| `invalid-level.ged` | Non-numeric level token |
| `duplicate-xref.ged` | Two records with the same xref |
| `circular-reference.ged` | FAMC/FAMS cycle |

## Stress tests

File: [parser/parser_stress_test.go](../parser/parser_stress_test.go)

Stress tests use large synthetic GEDCOM files placed at the repository root (not committed — generated on demand). The files follow the naming pattern `ENFANnn.GED` where `nn` is the bit-width of the individual count (e.g. ENFAN22 = 2²² − 1 = 4,194,303 individuals).

```bash
# Run the functional stress test (skips if the file is absent)
go test -v -run TestStressRealFiles ./parser/

# Run the benchmark (requires the file to be present)
go test -bench BenchmarkParseENFAN22 -benchmem -benchtime=5s ./parser/
```

Latest results on ENFAN22.GED (4,194,303 individuals, 2,097,151 families):

| Metric | Value |
|---|---|
| Parse time | ~26s |
| Throughput | ~48 MB/s |
| Memory allocated | ~29.5 GB |
| Allocations | ~132M (~21 allocs/record) |
| Parser warnings | 0 |

Throughput is stable up to ENFAN18 (~262k individuals, ~42 MB/s) and begins to degrade beyond that due to GC pressure. Allocation count scales linearly with record count at ~21 allocs/record. Real-world trees rarely exceed 200k individuals, so ENFAN16/17 are the operationally relevant range.

## Benchmarks

### `enricher`

```bash
go test -bench=. -benchmem ./enricher/
```

| Benchmark | What it measures |
|---|---|
| `BenchmarkParseOnly` | Parser throughput on each testdata file |
| `BenchmarkEnrichOnly` | Enricher throughput (parser output pre-warmed) |
| `BenchmarkGenerateIDs` | UUID generation throughput across all enriched entities |

### `validator`

```bash
go test -bench=. -benchmem ./validator/
```

| Benchmark | What it measures |
|---|---|
| `BenchmarkParseAndValidate` | Combined parse + validate throughput |
| `BenchmarkParseValidateEnrich` | Combined parse + validate + enrich throughput |

## Manual file validation

`cmd/gedvalidate` runs the parser and validator against any file and prints a human-readable summary:

```bash
go run ./cmd/gedvalidate path/to/file.ged
```

Output format:

```
file: path/to/file.ged
parser warnings: 0
[warn] MISSING_TRAILER: GEDCOM file should end with a TRLR record (xref="")
[error] ORPHANED_FAMC: FAMC reference to non-existent record @F1@ (xref="@I1@")
summary: errors=1 warnings=1 hints=0
```

Exits with code 0 if no errors, 1 if any errors are found, 2 on a fatal parse failure.
