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
}

// DefaultOptions returns sensible default parsing options.
func DefaultOptions() *Options {
	return &Options{
		Context:         context.Background(),
		StrictMode:      false,
		MaxNestingDepth: 100,
	}
}
