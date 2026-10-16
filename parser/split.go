package parser

import "bytes"

// splitGedcomLines is a bufio.SplitFunc that ends a line at "\r\n", "\n" or a
// lone "\r". Each terminator counts once, so the number of tokens equals the
// number of line terminators (plus a final unterminated line, if any).
func splitGedcomLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	i := bytes.IndexAny(data, "\r\n")
	if i < 0 {
		if atEOF {
			return len(data), data, nil
		}
		return 0, nil, nil
	}
	if data[i] == '\r' {
		if i+1 < len(data) {
			if data[i+1] == '\n' {
				return i + 2, data[:i], nil
			}
			return i + 1, data[:i], nil
		}
		// "\r" is the last byte we have: it may be the start of "\r\n".
		if !atEOF {
			return 0, nil, nil
		}
	}
	return i + 1, data[:i], nil
}
