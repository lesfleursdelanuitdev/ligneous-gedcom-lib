package reconciliation

import (
	"encoding/json"
	"testing"
)

func TestMergeReconcileOptionsFromJSON_EmptyUsesDefaults(t *testing.T) {
	o, err := MergeReconcileOptionsFromJSON(nil)
	if err != nil || o == nil {
		t.Fatal(err)
	}
	if !o.EnableSoftIndividualMatching {
		t.Fatalf("soft: %+v", o)
	}
	if o.SoftMinAlignScore != 0.74 || o.SoftMinHintScore != 0.55 || o.MaxSoftComparisonsPerSide != 40 {
		t.Fatalf("thresholds: %+v", o)
	}
}

func TestMergeReconcileOptionsFromJSON_ExplicitFalse(t *testing.T) {
	raw := json.RawMessage(`{"enableSoftIndividualMatching":false}`)
	o, err := MergeReconcileOptionsFromJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if o.EnableSoftIndividualMatching {
		t.Fatalf("expected false, got %+v", o)
	}
}

func TestMergeReconcileOptionsFromJSON_PartialOverlay(t *testing.T) {
	raw := json.RawMessage(`{"softMinAlignScore":0.9,"maxSoftComparisonsPerSide":12}`)
	o, err := MergeReconcileOptionsFromJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !o.EnableSoftIndividualMatching {
		t.Fatal("soft should remain default true")
	}
	if o.SoftMinAlignScore != 0.9 || o.MaxSoftComparisonsPerSide != 12 {
		t.Fatalf("got %+v", o)
	}
	if o.SoftMinHintScore != 0.55 {
		t.Fatalf("hint default: %v", o.SoftMinHintScore)
	}
}
