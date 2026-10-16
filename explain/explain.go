// Package explain holds the plain-English text shown for each finding code.
package explain

import "regexp"

// Entry is the explanation of one finding code. Title, Meaning and HowToFix
// may contain {placeholders} that the page fills in from the finding.
type Entry struct{ Code, Group, Title, Meaning, HowToFix string }

var byCode = func() map[string]Entry {
	m := make(map[string]Entry, len(catalog))
	for _, e := range catalog {
		m[e.Code] = e
	}
	return m
}()

// Lookup returns the entry for a finding code.
func Lookup(code string) (Entry, bool) {
	e, ok := byCode[code]
	return e, ok
}

// All returns every entry in catalog order.
func All() []Entry { return append([]Entry(nil), catalog...) }

var placeholderRe = regexp.MustCompile(`\{([a-z_]+)\}`)

// Placeholders returns the names inside {…} across Title, Meaning and
// HowToFix, deduplicated, in order of first appearance.
func Placeholders(e Entry) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range []string{e.Title, e.Meaning, e.HowToFix} {
		for _, m := range placeholderRe.FindAllStringSubmatch(s, -1) {
			if !seen[m[1]] {
				seen[m[1]] = true
				out = append(out, m[1])
			}
		}
	}
	return out
}
