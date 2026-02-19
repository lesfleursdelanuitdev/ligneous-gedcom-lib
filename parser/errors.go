package parser

import "fmt"

// ParseError represents an error encountered while parsing a GEDCOM file.
type ParseError struct {
	Line    int
	Message string
	Context string
	Err     error
}

func (e *ParseError) Error() string {
	if e.Context != "" {
		return fmt.Sprintf("line %d: %s (context: %q)", e.Line, e.Message, e.Context)
	}
	return fmt.Sprintf("line %d: %s", e.Line, e.Message)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

func newParseError(line int, message, ctx string) *ParseError {
	return &ParseError{Line: line, Message: message, Context: ctx}
}

func wrapParseError(line int, message, ctx string, err error) *ParseError {
	return &ParseError{Line: line, Message: message, Context: ctx, Err: err}
}
