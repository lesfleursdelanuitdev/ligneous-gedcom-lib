package parser

import "fmt"

// ParseError represents an error encountered while parsing a GEDCOM file.
type ParseError struct {
	Line    int
	Message string
	Context string
	Err     error
	// Code is a stable machine-readable identifier (see the Code constants).
	Code string
	// Severity is "error", "warning" or "hint".
	Severity string
	// Details carries extra values for explaining the finding (may be nil).
	Details map[string]string `json:"Details,omitempty"`
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

// Codes carried by findings the parser produces.
const (
	CodeEmptyLine         = "EMPTY_LINE"
	CodeLineTooLong       = "LINE_TOO_LONG"
	CodeLeadingWhitespace = "LEADING_WHITESPACE"
	CodeBadLine           = "BAD_LINE"
	CodeBadLevel          = "BAD_LEVEL"
	CodeXrefWithoutTag    = "XREF_WITHOUT_TAG"
	CodeTooDeep           = "TOO_DEEP"
	CodeReadError         = "READ_ERROR"

	CodeLinesBeforeHead = "LINES_BEFORE_HEAD"
	CodeLevelJump       = "LEVEL_JUMP"
	CodeUnknownRecord   = "UNKNOWN_RECORD"

	CodeEncodingNotUTF8    = "ENCODING_NOT_UTF8"
	CodeEncodingMislabeled = "ENCODING_MISLABELED"
	CodeEncodingUndeclared = "ENCODING_UNDECLARED"
	CodeEncodingBadBytes   = "ENCODING_BAD_BYTES"
	CodeEncodingStrayBytes = "ENCODING_STRAY_BYTES"
)

var codeSeverity = map[string]string{
	CodeEmptyLine:         "warning",
	CodeLineTooLong:       "warning",
	CodeLeadingWhitespace: "warning",
	CodeBadLine:           "error",
	CodeBadLevel:          "error",
	CodeXrefWithoutTag:    "error",
	CodeTooDeep:           "error",
	CodeReadError:         "error",

	CodeLinesBeforeHead: "warning",
	CodeLevelJump:       "warning",
	CodeUnknownRecord:   "hint",

	CodeEncodingNotUTF8:    "hint",
	CodeEncodingMislabeled: "warning",
	CodeEncodingUndeclared: "hint",
	CodeEncodingBadBytes:   "warning",
	CodeEncodingStrayBytes: "warning",
}

func newParseError(code string, line int, message, ctx string) *ParseError {
	return &ParseError{Line: line, Message: message, Context: ctx, Code: code, Severity: codeSeverity[code]}
}

func wrapParseError(code string, line int, message, ctx string, err error) *ParseError {
	return &ParseError{Line: line, Message: message, Context: ctx, Err: err, Code: code, Severity: codeSeverity[code]}
}
