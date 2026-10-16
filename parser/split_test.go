package parser

import (
	"fmt"
	"strings"
	"testing"
)

func splitAll(t *testing.T, s string) []string {
	t.Helper()
	b := []byte(s)
	var out []string
	for pos := 0; pos < len(b); {
		var ln []byte
		ln, pos = nextLine(b, pos)
		out = append(out, string(ln))
	}
	return out
}

func TestSplitLineEndings(t *testing.T) {
	toks := splitAll(t, "0 HEAD\r1 GEDC\r\n2 VERS 5.5.1\n0 TRLR")
	if len(toks) != 4 {
		t.Fatalf("want 4 tokens, got %d: %q", len(toks), toks)
	}
	for _, tk := range toks {
		if strings.ContainsAny(tk, "\r\n") {
			t.Errorf("token %q contains a terminator", tk)
		}
	}
}

func TestSplitMixedLineEndings(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 1000; i++ {
		b.WriteString(fmt.Sprintf("1 NOTE line %d", i))
		if i%10 == 0 {
			b.WriteString("\r")
		} else {
			b.WriteString("\r\n")
		}
	}
	if toks := splitAll(t, b.String()); len(toks) != 1000 {
		t.Fatalf("want 1000 tokens, got %d", len(toks))
	}
}
