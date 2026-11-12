package cjk

import (
	"image"
	"image/draw"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

func goFace(t *testing.T) font.Face {
	t.Helper()
	src, err := opentype.Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	f, err := opentype.NewFace(src, &opentype.FaceOptions{Size: 32, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func drawn(f font.Face, s string) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 600, 60))
	draw.Draw(img, img.Rect, image.Black, image.Point{}, draw.Src)
	(&font.Drawer{Dst: img, Src: image.White, Face: f, Dot: fixed.P(4, 44)}).DrawString(s)
	return img
}

// What the screen's own font draws is drawn as it was, to the pixel, and takes the room it took.
func TestLatinIsDrawnAsItWas(t *testing.T) {
	plain, with := goFace(t), Face(goFace(t), 32)
	const s = "Thursday, November 12 … 9:30 — Wärme, 21°"
	if string(drawn(plain, s).Pix) != string(drawn(with, s).Pix) {
		t.Error("a line of Latin is drawn differently")
	}
	if a, b := font.MeasureString(plain, s), font.MeasureString(with, s); a != b {
		t.Errorf("a line of Latin is %v wide, and was %v", b, a)
	}
	if plain.Metrics() != with.Metrics() {
		t.Error("the face's metrics are not its own font's")
	}
}

// Japanese is drawn: each character has a glyph of its own with ink in it, as wide as the face is
// tall or near it, where the screen's own font has nothing for it.
func TestJapaneseIsDrawn(t *testing.T) {
	plain, with := goFace(t), Face(goFace(t), 32)
	seen := map[string]bool{}
	for _, r := range "11月2日木曜晴れくもりアラーム、。「」！？鬱" {
		if r < first {
			continue
		}
		if _, ok := plain.GlyphAdvance(r); ok {
			t.Fatalf("the screen's own font draws %c: this test is of nothing", r)
		}
		adv, ok := with.GlyphAdvance(r)
		if !ok || adv.Round() < 28 || adv.Round() > 36 {
			t.Errorf("%c is %v wide, %v", r, adv, ok)
		}
		img := drawn(with, string(r))
		ink := 0
		for i := 0; i < len(img.Pix); i += 4 {
			if img.Pix[i] > 128 {
				ink++
			}
		}
		if ink < 8 {
			t.Errorf("%c is drawn with %d pixels of ink", r, ink)
		}
		if seen[string(img.Pix)] {
			t.Errorf("%c is drawn as another character is", r)
		}
		seen[string(img.Pix)] = true
	}
	// A character neither font has is left to the screen's own, as before.
	if a, ok := with.GlyphAdvance(0x1F600); ok {
		if b, _ := plain.GlyphAdvance(0x1F600); a != b {
			t.Errorf("a character neither font has is %v wide, and was %v", a, b)
		}
	}
}

func TestWhichCharactersALineBreaksAfter(t *testing.T) {
	for _, r := range "あア漢、。「！" {
		if !Wide(r) {
			t.Errorf("%c is not one a line breaks after", r)
		}
	}
	for _, r := range "aZ9 é…-" {
		if Wide(r) {
			t.Errorf("%c is one a line breaks after", r)
		}
	}
}
