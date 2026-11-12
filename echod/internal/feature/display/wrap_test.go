//go:build !dot

package display

import (
	"slices"
	"strings"
	"testing"
)

// Text with spaces is its words, as it always was, and goes back together as it was written.
func TestWordsBreakAtTheirSpaces(t *testing.T) {
	for _, s := range []string{
		"Tomorrow will be sunny, with a high of 74 and a low of 51.",
		"  Wie ist   das Wetter?  ",
		"¿Qué tiempo hace? — 21° … ok",
		"",
	} {
		got := breakable(s)
		if !slices.Equal(got, strings.Fields(s)) {
			t.Errorf("%q breaks into %q", s, got)
		}
		line := ""
		for _, w := range got {
			line = joinWord(line, w)
		}
		if line != strings.Join(strings.Fields(s), " ") {
			t.Errorf("%q goes back together as %q", s, line)
		}
	}
}

// Japanese breaks after any character, but a line never starts with a mark that closes or ends with
// one that opens, and a Latin word among it stays whole. Put back together it has no spaces added.
func TestJapaneseBreaksAnywhereButAtItsMarks(t *testing.T) {
	for s, want := range map[string][]string{
		"明日は晴れです。":           {"明", "日", "は", "晴", "れ", "で", "す。"},
		"「はい」と言った":           {"「は", "い」", "と", "言っ", "た"},
		"最高気温は21°、Tokyoは晴れ！": {"最", "高", "気", "温", "は", "21°、", "Tokyo", "は", "晴", "れ！"},
		"きょう, 9:30 に":        {"きょ", "う,", "9:30", "に"},
	} {
		got := breakable(s)
		if !slices.Equal(got, want) {
			t.Errorf("%q breaks into %q, want %q", s, got, want)
		}
		line := ""
		for i, w := range got {
			first, last := []rune(w)[0], []rune(w)[len([]rune(w))-1]
			if i > 0 && strings.ContainsRune(noStart, first) {
				t.Errorf("%q: a line could start with %q", s, w)
			}
			if i < len(got)-1 && strings.ContainsRune(noEnd, last) {
				t.Errorf("%q: a line could end with %q", s, w)
			}
			line = joinWord(line, w)
		}
		if strings.ReplaceAll(line, " ", "") != strings.ReplaceAll(s, " ", "") {
			t.Errorf("%q goes back together as %q", s, line)
		}
	}
	if got := joinWord("Tokyo", "は"); got != "Tokyoは" {
		t.Errorf("a Latin word and a Japanese one join as %q", got)
	}
	if got := joinWord("is", "sunny"); got != "is sunny" {
		t.Errorf("two words join as %q", got)
	}
}

// A long Japanese sentence is wrapped to the width it is given, where before it was one line.
func TestAJapaneseSentenceIsWrapped(t *testing.T) {
	r := &paint{}
	face := r.faces().label
	const s = "明日の東京は晴れ時々くもりで、最高気温は21度、最低気温は12度の見込みです。傘は必要ありません。"
	lines := r.wrap(face, s, 400)
	if len(lines) < 3 {
		t.Fatalf("wrapped into %d lines: %q", len(lines), lines)
	}
	for _, l := range lines {
		if w := r.width(face, l); w > 400 {
			t.Errorf("the line %q is %d wide", l, w)
		}
	}
	if strings.Join(lines, "") != s {
		t.Errorf("the lines are %q", lines)
	}
}
