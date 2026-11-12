//go:build !dot

package display

import (
	"strings"
	"unicode/utf8"

	"github.com/HuskerMinion/techo5/echod/internal/lib/cjk"
)

// Where a line of text may be broken. Between words, at the spaces, as ever; and in Japanese, which
// has no spaces, after any character, with the two things a line must not do there: start with a
// mark that closes what came before it, or end with one that opens what comes after.

// noStart are marks a line does not start with: they stay on the end of the line before.
const noStart = "、。，．」』）】〕〉》！？・ー…‥゛゜ぁぃぅぇぉっゃゅょァィゥェォッャュョ,.!?:;)]}%°"

// noEnd are marks a line does not end with: they go down with what follows them.
const noEnd = "「『（【〔〈《"

// breakable is s as the pieces a line is built of: its words, and each Japanese character with the
// marks that stay beside it. joinWord puts them back together.
func breakable(s string) []string {
	var out []string
	for _, field := range strings.Fields(s) {
		start, held := 0, false // held is the piece so far ending in a mark that may not end a line
		prev := rune(-1)
		for i, r := range field {
			wide := cjk.Wide(r)
			// A piece ends before a Japanese character, and after one, unless a mark holds them together.
			if i > start && (wide || cjk.Wide(prev)) && !held && !strings.ContainsRune(noStart, r) {
				out = append(out, field[start:i])
				start = i
			}
			held = strings.ContainsRune(noEnd, r)
			prev = r
		}
		out = append(out, field[start:])
	}
	return out
}

// joinWord is line with word after it: a space between them, unless either side of the join is
// Japanese, which is written without.
func joinWord(line, word string) string {
	if line == "" {
		return word
	}
	last, _ := utf8.DecodeLastRuneInString(line)
	first, _ := utf8.DecodeRuneInString(word)
	if cjk.Wide(last) || cjk.Wide(first) {
		return line + word
	}
	return line + " " + word
}
