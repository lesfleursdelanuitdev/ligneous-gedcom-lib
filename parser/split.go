package parser

import "bytes"

// nextLine returns the line starting at b[pos] and the offset of the next
// line. A line ends at "\r\n", "\n" or a lone "\r"; each terminator counts
// once, and a final unterminated line counts too. There is no limit on line
// length. Call it only while pos < len(b).
//
// repair.splitLines follows the same rule; repair's TestSplitLinesMatchesParser
// guards against drift.
func nextLine(b []byte, pos int) (line []byte, next int) {
	i := bytes.IndexAny(b[pos:], "\r\n")
	if i < 0 {
		return b[pos:], len(b)
	}
	line = b[pos : pos+i]
	next = pos + i + 1
	if b[pos+i] == '\r' && next < len(b) && b[next] == '\n' {
		next++
	}
	return line, next
}
