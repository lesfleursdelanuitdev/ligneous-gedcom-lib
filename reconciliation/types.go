package reconciliation

// MatchStage follows the staged certainty model (1 = strongest).
type MatchStage int

const (
	StageStableID MatchStage = 1 // UUID / stable row id on both sides
	StageXref     MatchStage = 2 // Normalized GEDCOM xref match
	StageNameDate MatchStage = 3 // Strong name + birth/death similarity (same snapshot family not required)
	StageFamily   MatchStage = 4 // Neighbor multiset (parents/spouses/children fingerprints) similarity
	StageFuzzy    MatchStage = 5 // Weaker combined score; requires review
)

// DefaultScoringProfileID is embedded in plans for reproducible reruns.
const DefaultScoringProfileID = "rec-p1-2026-05"

// Evidence is one machine-readable factor in a scorecard.
type Evidence struct {
	Code   string         `json:"code"`
	Weight float64        `json:"weight"`
	Detail map[string]any `json:"detail,omitempty"`
}

// MatchScorecard carries explainable match metadata.
type MatchScorecard struct {
	Score             float64    `json:"score"`
	Stage             MatchStage `json:"stage"`
	ScoringProfileID  string     `json:"scoringProfileId"`
	Reasons           []string   `json:"reasons"`
	Evidence          []Evidence `json:"evidence"`
}

// AlignmentConfidence is coarse UI policy; "certain" still requires human approval by default policy.
type AlignmentConfidence string

const (
	ConfidenceCertain AlignmentConfidence = "certain"
	ConfidenceHigh    AlignmentConfidence = "high"
	ConfidenceMedium  AlignmentConfidence = "medium"
	ConfidenceLow     AlignmentConfidence = "low"
)

// ProposedAlignment is one proposed entity correspondence after blocking + assignment.
type ProposedAlignment struct {
	Kind       string              `json:"kind"` // individual | family | ...
	LeftID     string              `json:"leftId"`
	RightID    string              `json:"rightId"`
	Confidence AlignmentConfidence `json:"confidence"`
	Scorecard  MatchScorecard      `json:"scorecard"`
}

// MergeOperation is a declarative safe operation (v0: usually empty; reserved for strict auto-merge policy).
type MergeOperation struct {
	ID        string         `json:"id"`
	Kind      string         `json:"kind"`
	Target    map[string]any `json:"target"`
	Payload   map[string]any `json:"payload,omitempty"`
	Rationale []string       `json:"rationale"`
	PolicyID  string         `json:"policyId"`
}

// AlignmentCandidateEntry is one row inside PossibleMatch.
type AlignmentCandidateEntry struct {
	LeftID    string         `json:"leftId"`
	RightID   string         `json:"rightId"`
	Scorecard MatchScorecard `json:"scorecard"`
}

// PossibleMatch groups ambiguous alternatives (future expansion).
type PossibleMatch struct {
	Kind             string                    `json:"kind"`
	ResolutionHint   string                    `json:"resolutionHint"`
	Candidates       []AlignmentCandidateEntry `json:"candidates,omitempty"`
}

// MergeConflict records incompatible facts for aligned entities.
type MergeConflict struct {
	ID        string         `json:"id"`
	Kind      string         `json:"kind"`
	LeftRef   map[string]any `json:"leftRef"`
	RightRef  map[string]any `json:"rightRef"`
	FieldPath string         `json:"fieldPath"`
	LeftValue any            `json:"leftValue"`
	RightValue any            `json:"rightValue"`
	Severity  string         `json:"severity"` // blocking | warning
	Explain   MatchScorecard `json:"explain"`
}

// UnresolvedEntity is an entity on one side with no alignment in this pass.
type UnresolvedEntity struct {
	Side       string           `json:"side"` // left | right
	EntityType string           `json:"entityType"`
	EntityID   string           `json:"entityId"`
	Xref       string           `json:"xref,omitempty"`
	Reason     string           `json:"reason"`
	Hints      []MatchScorecard `json:"hints,omitempty"`
}

// IndividualDiffSummary is a compact per-pair diff for UI (order-insensitive event keys).
type IndividualDiffSummary struct {
	LeftID            string   `json:"leftId"`
	RightID           string   `json:"rightId"`
	FullNameEqual     bool     `json:"fullNameEqual"`
	SexEqual          bool     `json:"sexEqual"`
	BirthYearEqual    *bool    `json:"birthYearEqual,omitempty"`
	DeathYearEqual    *bool    `json:"deathYearEqual,omitempty"`
	EventKeysLeft     []string `json:"eventKeysLeft"`
	EventKeysRight    []string `json:"eventKeysRight"`
	EventKeysOnlyLeft []string `json:"eventKeysOnlyLeft"`
	EventKeysOnlyRight []string `json:"eventKeysOnlyRight"`

	// Order-insensitive attachment fingerprints (notes / media / sources linked to individual).
	NoteKeysLeft       []string `json:"noteKeysLeft,omitempty"`
	NoteKeysRight      []string `json:"noteKeysRight,omitempty"`
	NoteKeysOnlyLeft   []string `json:"noteKeysOnlyLeft,omitempty"`
	NoteKeysOnlyRight  []string `json:"noteKeysOnlyRight,omitempty"`
	MediaKeysLeft      []string `json:"mediaKeysLeft,omitempty"`
	MediaKeysRight     []string `json:"mediaKeysRight,omitempty"`
	MediaKeysOnlyLeft  []string `json:"mediaKeysOnlyLeft,omitempty"`
	MediaKeysOnlyRight []string `json:"mediaKeysOnlyRight,omitempty"`
	SourceKeysLeft     []string `json:"sourceKeysLeft,omitempty"`
	SourceKeysRight    []string `json:"sourceKeysRight,omitempty"`
	SourceKeysOnlyLeft []string `json:"sourceKeysOnlyLeft,omitempty"`
	SourceKeysOnlyRight []string `json:"sourceKeysOnlyRight,omitempty"`
}

// FamilyDiffSummary compares aligned families by spouse xrefs and child multiset.
type FamilyDiffSummary struct {
	LeftID              string   `json:"leftId"`
	RightID             string     `json:"rightId"`
	HusbandXrefEqual    bool     `json:"husbandXrefEqual"`
	WifeXrefEqual       bool     `json:"wifeXrefEqual"`
	ChildrenCountLeft   int    `json:"childrenCountLeft"`
	ChildrenCountRight  int    `json:"childrenCountRight"`
	ChildXrefOnlyLeft   []string `json:"childXrefOnlyLeft"`
	ChildXrefOnlyRight  []string `json:"childXrefOnlyRight"`
}

// MergePlan is the primary reconciliation output (declarative IR).
type MergePlan struct {
	Version            int    `json:"version"`
	ScoringProfileID   string `json:"scoringProfileId"`
	GeneratedAt       string `json:"generatedAt,omitempty"`

	SafeAutoMerges   []MergeOperation         `json:"safeAutoMerges"`
	PossibleMatches  []PossibleMatch          `json:"possibleMatches"`
	Conflicts        []MergeConflict          `json:"conflicts"`
	Unresolved       []UnresolvedEntity       `json:"unresolved"`
	Alignments       struct {
		Individuals []ProposedAlignment `json:"individuals"`
		Families    []ProposedAlignment `json:"families"`
	} `json:"alignments"`

	IndividualDiffs []IndividualDiffSummary `json:"individualDiffs"`
	FamilyDiffs     []FamilyDiffSummary     `json:"familyDiffs"`
}

// Options tune reconciliation behaviour.
type Options struct {
	ScoringProfileID  string `json:"scoringProfileId,omitempty"`
	MaxBirthYearDelta int    `json:"maxBirthYearDelta,omitempty"` // inclusive; years further apart → conflict

	// EnableSoftIndividualMatching runs stages 3–5 after id/xref alignment (blocking + greedy assignment).
	EnableSoftIndividualMatching bool `json:"enableSoftIndividualMatching,omitempty"`
	// SoftMinAlignScore is the minimum combined score (0–1) to emit a proposed alignment. Default 0.74.
	SoftMinAlignScore float64 `json:"softMinAlignScore,omitempty"`
	// SoftMinHintScore lists unmatched pairs as hints on Unresolved when in [softMinHintScore, softMinAlignScore). Default 0.55.
	SoftMinHintScore float64 `json:"softMinHintScore,omitempty"`
	// MaxSoftComparisonsPerSide caps scoring calls per unmatched individual (deterministic). Default 40.
	MaxSoftComparisonsPerSide int `json:"maxSoftComparisonsPerSide,omitempty"`

	// UseHungarianAssignment runs a min-cost / max-weight assignment per blocking bucket instead of greedy global sort.
	UseHungarianAssignment bool `json:"useHungarianAssignment,omitempty"`
	// HungarianMaxMatrix caps max(nL,nR) per bucket for Hungarian (falls back to greedy slice if exceeded). Default 64.
	HungarianMaxMatrix int `json:"hungarianMaxMatrix,omitempty"`
}

// ReconciliationSession is a portable wrapper for persistence (DB, file, or job store) — not written by this package.
type ReconciliationSession struct {
	ID          string    `json:"id"`
	CreatedAt   string    `json:"createdAt"`
	Status      string    `json:"status"` // draft | in_review | approved | rejected
	MergePlan   *MergePlan `json:"mergePlan"`
	InputSummary struct {
		LeftIndividualCount  int `json:"leftIndividualCount"`
		RightIndividualCount int `json:"rightIndividualCount"`
	} `json:"inputSummary"`
}

func (o *Options) scoringProfile() string {
	if o != nil && o.ScoringProfileID != "" {
		return o.ScoringProfileID
	}
	return DefaultScoringProfileID
}

func (o *Options) maxBirthYearDelta() int {
	if o != nil && o.MaxBirthYearDelta > 0 {
		return o.MaxBirthYearDelta
	}
	return 1
}

func (o *Options) softMinAlignScore() float64 {
	if o != nil && o.SoftMinAlignScore > 0 {
		return o.SoftMinAlignScore
	}
	return 0.74
}

func (o *Options) softMinHintScore() float64 {
	if o != nil && o.SoftMinHintScore > 0 {
		return o.SoftMinHintScore
	}
	return 0.55
}

func (o *Options) maxSoftComparisonsPerSide() int {
	if o != nil && o.MaxSoftComparisonsPerSide > 0 {
		return o.MaxSoftComparisonsPerSide
	}
	return 40
}

func (o *Options) hungarianMaxMatrix() int {
	if o != nil && o.HungarianMaxMatrix > 0 {
		return o.HungarianMaxMatrix
	}
	return 64
}
