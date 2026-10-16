package parser

import "context"

// Options configures parser behavior.
type Options struct {
	// Context allows cancellation and timeout control.
	Context context.Context

	// StrictMode rejects non-standard tags and extensions.
	StrictMode bool

	// MaxNestingDepth limits hierarchical depth (default: 100).
	MaxNestingDepth int

	// Lenient turns line-level errors (BAD_LINE, BAD_LEVEL, XREF_WITHOUT_TAG,
	// TOO_DEEP) into findings and skips the line instead of failing the parse.
	Lenient bool
}

// DefaultOptions returns sensible default parsing options.
func DefaultOptions() *Options {
	return &Options{
		Context:         context.Background(),
		StrictMode:      false,
		MaxNestingDepth: 100,
	}
}
