package render

import "strings"

// replacement stands in for a control character removed from document text.
const replacement = '�'

// isControl reports whether r is a C0 or C1 control character. Tab is
// excluded: expandTabs and the wrapper both treat it as ordinary whitespace.
func isControl(r rune) bool {
	return (r < 0x20 && r != '\t') || r == 0x7f || (r >= 0x80 && r <= 0x9f)
}

// sanitize replaces control characters in document text with U+FFFD.
//
// A markdown file is untrusted input — a README from a freshly cloned
// repository is the common case — and the terminal acts on escape sequences
// it is handed. Left alone, a document can write the clipboard with OSC 52,
// or set the window title and request it back so the terminal types it onto
// the shell's stdin. Controls also break layout, since runewidth counts the
// printable tail of an escape ("[31m") as four columns.
//
// Text with no controls is returned unchanged, allocating nothing.
//
// Sanitize is the exported form, for the few places outside the renderer
// that put document-derived text on screen: the TOC popup and the status
// bar, which shows link targets and filenames.
func Sanitize(s string) string { return sanitize(s) }

func sanitize(s string) string {
	if strings.IndexFunc(s, isControl) < 0 {
		return s
	}
	return strings.Map(func(r rune) rune {
		if isControl(r) {
			return replacement
		}
		return r
	}, s)
}

// sanitizeLink percent-encodes control characters in an OSC 8 target. A raw
// ESC in a link destination would close the sequence early and hand the rest
// of the destination to the terminal as input, so escaping matters even
// though the destination is never displayed. Percent-encoding is what the
// OSC 8 specification expects of a URI in any case.
func sanitizeLink(s string) string {
	if strings.IndexFunc(s, isControl) < 0 {
		return s
	}
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if !isControl(r) {
			b.WriteRune(r)
			continue
		}
		// C1 controls are two bytes in UTF-8; percent-encoding is per byte.
		for _, c := range []byte(string(r)) {
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0x0f])
		}
	}
	return b.String()
}
