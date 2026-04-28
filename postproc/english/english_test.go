package english

import "testing"

func TestUnderscoreToSpace(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"plain text", "plain text"},
		{"foo_bar", "foo bar"},
		{"a_b_c", "a b c"},
		{"_leading", " leading"},
		{"trailing_", "trailing "},
	}
	for _, c := range cases {
		got := UnderscoreToSpace(c.in)
		if got != c.want {
			t.Errorf("UnderscoreToSpace(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestAAn(t *testing.T) {
	cases := []struct{ in, want string }{
		// Bare "a" before vowel-letter starts.
		{"a apple", "an apple"},
		{"a egg", "an egg"},
		{"a igloo", "an igloo"},
		{"a orange", "an orange"},
		{"a umbrella", "an umbrella"},
		// "A" capitalised stays capitalised.
		{"A apple", "An apple"},
		// Consonant start: leave alone.
		{"a banana", "a banana"},
		{"A cat", "A cat"},
		// Already "an" before consonant: don't downgrade. (We aim to
		// only fix "a" -> "an" before vowels; downgrading "an" -> "a"
		// is out of scope for v1.)
		{"an apple", "an apple"},
		{"an banana", "an banana"},
		// Mid-sentence.
		{"I saw a owl in a tree", "I saw an owl in a tree"},
		// Don't munge words that happen to start with 'a'.
		{"about a hour", "about a hour"}, // 'h' is consonant by letter rule
		// Punctuation right after the article should not break detection.
		{"a (apple)", "an (apple)"},
		// At end of string with no following word: leave alone.
		{"a", "a"},
		{"a ", "a "},
		// Article at end of string after a preceding word: leave alone
		// (no following word to inspect).
		{"foo a", "foo a"},
		// Article preceded by a word and followed by a vowel-starting
		// word: still rewrite.
		{"foo a apple", "foo an apple"},
		// Don't touch "a" inside another word.
		{"banana", "banana"},
		{"data", "data"},
	}
	for _, c := range cases {
		got := AAn(c.in)
		if got != c.want {
			t.Errorf("AAn(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
