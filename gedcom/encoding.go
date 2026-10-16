package gedcom

// EncodingInfo records how the bytes of a GEDCOM file were read.
type EncodingInfo struct {
	// Detected is the encoding the bytes were decoded with: "UTF-8", "UTF-16LE",
	// "UTF-16BE", "ANSEL", "Windows-1252", "CP437", "MacRoman" or "ASCII".
	Detected string `json:"detected"`
	// Declared is HEAD.CHAR as written, or "" when the file has none.
	Declared string `json:"declared"`
	// Reason says why Detected was chosen, e.g. "byte order mark" or "header CHAR".
	Reason string `json:"reason"`
}
