# Fix plan: failing tests

Three packages have failing tests. All failures are in the enricher or exporter.

---

## 1. `TestFromEnriched_ScalarOccupationWithoutEvent` — exporter (simplest, standalone)

**File:** `exporter/from_enriched.go`, `buildIndividualRecord`

**Problem:** The function emits attribute records sourced from `groupAttributesByOwner`, but never reads `indi.OccupationValues` directly. If a caller sets `OccupationValues` without a corresponding attribute row (e.g. after enrichment + manual mutation), no `OCCU` tag is emitted.

**Fix:** After the existing `attrs` loop in `buildIndividualRecord`, iterate `indi.OccupationValues` and emit a `1 OCCU <value>` child for any value not already covered by an emitted attribute record.

---

## 2. `TestEnrichNestedNotes` — enricher (two sub-failures)

**File:** `enricher/notes.go`

### 2a. IndividualNotes count: 3 instead of 2

**Problem:** `extractRecordNotesRecursive` (Phase 2) passes `individualEventTags` as the skip set. `RESI` and `individualAttributeTags` are not in that set, so the function recurses into `RESI > ADDR > NOTE` and attribute subtrees and wrongly attributes their inline notes to `IndividualNotes`.

**Fix:** Build a combined skip set for Phase 2 calls:
```go
indiSkip := mergeMaps(individualEventTags, individualAttributeTags)
indiSkip["RESI"] = true
```
Pass `indiSkip` instead of `individualEventTags` in the Phase 2 call at `notes.go:44`.

Do the same for Phase 3 (family notes), merging `familyEventTags`, `familyAttributeTags`, and `"RESI"`.

### 2b. EventNotes count: 1 instead of 2

**Problem:** Phase 4 iterates `ed.Events`. Residences are stored in `ed.Residences` — `findEventRecord` only searches `individualEventTags` children, so RESI records are never processed and their notes never reach `EventNotes`.

**Fix:** Add Phase 4b after Phase 4 that iterates `ed.Residences`, locates each residence's raw `GedcomRecord` (by owner xref + sort order, searching for `RESI` children), extracts its notes with `extractRecordNotes(nil)`, and appends `EventNoteLink` entries. Use a sentinel `EventIndex` of `-(residenceIndex + 1)` to distinguish from real event indices, or add a `ResidenceNoteLink` type — the latter is cleaner but requires updating callers. The test only checks `len(ed.EventNotes)`, so the sentinel approach is sufficient to pass the test without schema changes.

---

## 3. `TestEnrichGEDCOM55EventWhitelist` — enricher (root cause shared with #2)

**File:** `enricher/enricher.go`

**Problem:** The test expects 6 entries in `ed.Events` (BIRT + FACT + BAPL on the individual; CENS + RESI + SLGS on the family). Currently only 4 are captured: `FACT` goes to `ed.Attributes` (it is in `individualAttributeTags`, not `individualEventTags`) and family `RESI` goes to `ed.Residences`.

**Decision required:** Two options with different trade-offs.

**Option A — Move FACT into Events (recommended)**
- Add `"FACT"` to `individualEventTags` and remove it from `individualAttributeTags`.
- Add `"RESI"` to both `individualEventTags` and `familyEventTags`, and remove the separate `case child.Tag == "RESI"` branch.
- Update `findEventRecord` to count RESI and FACT children in its sort-order walk.
- This makes residences and FACT attributes first-class events in `ed.Events`, which also resolves 2b naturally (Phase 4 then finds them without needing Phase 4b).
- **Downside:** `ed.Residences` and `ed.Attributes` become redundant for RESI/FACT — callers that rely on those tables need to migrate or we keep dual-storage.

**Option B — Keep FACT/RESI separate, change the test**
- If the architectural decision is that FACT and RESI belong in their own tables (not Events), the test is wrong.
- Update `TestEnrichGEDCOM55EventWhitelist` to check `ed.Events` (BIRT + BAPL + CENS + SLGS = 4) + `ed.Attributes` (FACT = 1) + `ed.Residences` (RESI = 1).
- This is a test correction, not a code fix.

**Recommended path:** Option A for FACT (it has dates and behaves like an event); Option B's reasoning applies only to RESI if residences need their own table for address fields. Decide per entity before implementing.

---

## Implementation order

1. Fix `TestFromEnriched_ScalarOccupationWithoutEvent` (exporter) — isolated, no dependencies.
2. Decide Option A vs B for FACT/RESI — this drives whether 2b needs Phase 4b at all.
3. Fix Phase 2/3 skip sets (`TestEnrichNestedNotes` sub-failure 2a) — safe regardless of the Option A/B decision.
4. Fix Phase 4b or update `findEventRecord` for RESI (`TestEnrichNestedNotes` sub-failure 2b) — depends on step 2 decision.
