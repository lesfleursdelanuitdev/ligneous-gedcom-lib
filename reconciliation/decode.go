package reconciliation

import (
	"encoding/json"
	"fmt"

	"github.com/lesfleursdelanuitdev/ligneous-gedcom-lib/enricher"
)

// DecodeEnrichedDocument unmarshals canonical enriched JSON into EnrichedDocument.
// The raw document field is omitted in typical API payloads (nil is fine).
func DecodeEnrichedDocument(data []byte) (*enricher.EnrichedDocument, error) {
	var ed enricher.EnrichedDocument
	if err := json.Unmarshal(data, &ed); err != nil {
		return nil, fmt.Errorf("decode enriched document: %w", err)
	}
	return &ed, nil
}
