package enricher

import (
	"testing"
	"time"
)

// TestEnrichScalesLinearly guards against O(events x records) lookups: 40,000
// people with events, notes, citations and families must enrich in seconds.
func TestEnrichScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("scaling test skipped with -short")
	}
	doc := genLargeDoc(40000)
	start := time.Now()
	ed := Enrich(doc)
	elapsed := time.Since(start)
	t.Logf("Enrich of %d people: %v (events=%d)", len(doc.Individuals), elapsed, len(ed.Events))
	if len(ed.Events) < 80000 {
		t.Fatalf("expected >= 80000 events, got %d", len(ed.Events))
	}
	if elapsed > 3*time.Second {
		t.Fatalf("Enrich took %v, want < 3s", elapsed)
	}
}

func BenchmarkEnrichLarge(b *testing.B) {
	for _, n := range []int{5000, 20000} {
		doc := genLargeDoc(n)
		b.Run(itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				Enrich(doc)
			}
		})
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var s []byte
	for ; n > 0; n /= 10 {
		s = append([]byte{byte('0' + n%10)}, s...)
	}
	return string(s)
}
