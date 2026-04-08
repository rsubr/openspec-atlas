package internal

import (
	"bytes"
	"regexp"
)

// scriptBlockRe matches a <script> or <script setup> block in a Vue SFC.
// The inner capture group holds the script content.
var scriptBlockRe = regexp.MustCompile(`(?s)<script(?:[^>]*)>(.*?)</script>`)

// extractVueScript returns the content of the first <script> block in a Vue
// SFC source file. Leading newlines are inserted to preserve the original line
// numbers of the script content so that Symbol.Line values remain accurate.
// If no script block is found, an empty slice is returned.
func extractVueScript(src []byte) []byte {
	loc := scriptBlockRe.FindSubmatchIndex(src)
	if loc == nil {
		return []byte{}
	}
	contentStart, contentEnd := loc[2], loc[3]

	// Count newlines in the bytes before the script content so we can
	// prepend the same number of blank lines, keeping line numbers intact.
	lineOffset := bytes.Count(src[:contentStart], []byte("\n"))

	result := make([]byte, lineOffset, lineOffset+(contentEnd-contentStart))
	for i := 0; i < lineOffset; i++ {
		result[i] = '\n'
	}
	return append(result, src[contentStart:contentEnd]...)
}
