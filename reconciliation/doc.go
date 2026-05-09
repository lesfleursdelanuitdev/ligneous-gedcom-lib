// Package reconciliation compares two enricher.EnrichedDocument snapshots (canonical
// JSON from the Go enricher pipeline) and produces a declarative MergePlan.
//
// Phases implemented:
//   - Stage 1: stable id (individual.id / family.id)
//   - Stage 2: normalized xref alignment when ids absent or consistent
//   - Stages 3–5 (optional, Options.EnableSoftIndividualMatching): name + birth
//     similarity, parent/spouse/child neighbor multiset (Jaccard), fuzzy bucket;
//     greedy one-to-one assignment within blocking keys + capped comparisons
//   - Order-insensitive multiset diffs for events, notes, media, sources on aligned individuals
//   - MergeConflict generation for aligned pairs (birth year tolerance, sex, full name)
//
// Persistence and DB apply are out of scope: use ReconciliationSession as a JSON
// envelope for your API or job store. The ligneous-gedcom-lib-api exposes
// POST /api/v1/reconcile/merge-plan and POST /api/v1/reconcile/session.

package reconciliation
