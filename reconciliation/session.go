package reconciliation

import (
	"time"

	"github.com/google/uuid"
)

// NewReconciliationSession wraps a merge plan for persistence or UI workflows.
// The caller stores the returned value (JSON, database row, job queue payload).
func NewReconciliationSession(plan *MergePlan) *ReconciliationSession {
	if plan == nil {
		return nil
	}
	s := &ReconciliationSession{
		ID:        uuid.NewString(),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Status:    "draft",
		MergePlan: plan,
	}
	return s
}

// WithInputSummary fills optional counts for auditing (caller may overwrite).
func (s *ReconciliationSession) WithInputSummary(leftCount, rightCount int) *ReconciliationSession {
	if s == nil {
		return nil
	}
	s.InputSummary.LeftIndividualCount = leftCount
	s.InputSummary.RightIndividualCount = rightCount
	return s
}
