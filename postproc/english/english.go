// Package english provides English-specific post-processors for the
// grammar package. Hosts opt in by passing them to GenerateWith; the
// core grammar package itself has no language opinions.
package english

import (
	"strings"
	"unicode"
)

// UnderscoreToSpace replaces every underscore with a space. Useful for
// gluing compound words past inflection boundaries: a rule entry can
// write "fire_truck" so the pluralising form-default treats it as one
// token, then this post-processor restores the gap.
func UnderscoreToSpace(s string) string {
	return strings.ReplaceAll(s, "_", " ")
}

// AAn rewrites a standalone "a" or "A" to "an"/"An" when the next word
// starts with a vowel letter.
//
// The check is purely lexical and ASCII-only: it triggers on the
// presence of an ASCII vowel-letter (a/e/i/o/u) at the start of the
// following word. It does not understand phonetics, so consonant-sound
// exceptions like "an hour" and vowel-letter-with-consonant-sound
// exceptions like "a university" are out of scope. Non-ASCII vowels are
// not treated as vowels. Hosts that need such precision can layer their
// own post-processor on top.
func AAn(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	i := 0
	for i < len(s) {
		// Detect a standalone "a" or "A" — preceded by start-of-string
		// or whitespace, followed by whitespace and a word.
		if (s[i] == 'a' || s[i] == 'A') && atWordStart(s, i) && i+1 < len(s) && isSpace(s[i+1]) {
			// Find the next non-space character after the article.
			j := i + 2
			for j < len(s) && isSpace(s[j]) {
				j++
			}
			// Skip leading punctuation that isn't part of the word but
			// could appear before it: "a (apple)" should still become
			// "an (apple)".
			k := j
			for k < len(s) && isLeadingPunct(s[k]) {
				k++
			}
			if k < len(s) && isVowelLetter(s[k]) {
				if s[i] == 'a' {
					sb.WriteString("an")
				} else {
					sb.WriteString("An")
				}
				i++
				continue
			}
		}
		sb.WriteByte(s[i])
		i++
	}
	return sb.String()
}

// atWordStart reports whether s[i] is at a word boundary — start of
// string or preceded by whitespace. Avoids rewriting the "a" inside
// "banana".
func atWordStart(s string, i int) bool {
	if i == 0 {
		return true
	}
	return isSpace(s[i-1])
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n'
}

func isVowelLetter(b byte) bool {
	switch b {
	case 'a', 'e', 'i', 'o', 'u', 'A', 'E', 'I', 'O', 'U':
		return true
	}
	return false
}

func isLeadingPunct(b byte) bool {
	r := rune(b)
	if r >= 0x80 {
		return false
	}
	return unicode.IsPunct(r) || unicode.IsSymbol(r)
}
