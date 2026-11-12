// Package cjk gives the screen's faces Japanese: the screen's own font, Go, has Latin letters only,
// so a face made from it is wrapped in one that draws kana, kanji and full width punctuation from Noto
// Sans JP and everything else exactly as before.
//
// The font here is a part of Noto Sans JP, one weight of it and the characters of JIS X 0208, made by
// tools/fonts/make_jp_font.py. It is under the SIL Open Font License, in OFL.txt beside it.
package cjk

import (
	_ "embed"
	"image"
	"log/slog"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

//go:embed NotoSansJP-subset.ttf
var ttf []byte

// parsed is the font, read the first time a face has a Japanese character to draw: a device that
// never shows one never pays for it.
var parsed = sync.OnceValue(func() *sfnt.Font {
	f, err := opentype.Parse(ttf)
	if err != nil {
		slog.Error("the Japanese font cannot be read", "err", err)
		return nil
	}
	return f
})

// first is the first character the Japanese font is asked for. Everything below it is the screen's
// own font's, drawn or not as it always was.
const first = 0x3000

// Wide is whether r is a character that is written with no spaces between it and the next, so that a
// line may be broken after it: kana, kanji, and their punctuation and full width forms.
func Wide(r rune) bool {
	switch {
	case r >= 0x3000 && r <= 0x30ff, // punctuation and kana
		r >= 0x3400 && r <= 0x4dbf, r >= 0x4e00 && r <= 0x9fff, r >= 0xf900 && r <= 0xfaff, // kanji
		r >= 0xff00 && r <= 0xff60, r >= 0xffe0 && r <= 0xffe6: // full width forms
		return true
	}
	return false
}

// Face is primary with Japanese: a face that draws what primary draws, and the characters primary's
// font has no glyph for, the Japanese ones, from the Japanese font at px pixels. Its metrics are
// primary's, so a line of it stands where a line of primary did. Like the faces it is made of, it is
// for one goroutine at a time.
func Face(primary font.Face, px float64) font.Face {
	if primary == nil {
		return nil
	}
	return &face{primary: primary, px: px}
}

type face struct {
	primary font.Face
	px      float64
	once    sync.Once
	wide    font.Face // nil until asked for, and after that if it could not be made
}

// other is the Japanese face, if r is its to draw.
func (f *face) other(r rune) font.Face {
	if r < first {
		return nil
	}
	f.once.Do(func() {
		src := parsed()
		if src == nil {
			return
		}
		made, err := opentype.NewFace(src, &opentype.FaceOptions{Size: f.px, DPI: 72, Hinting: font.HintingNone})
		if err != nil {
			slog.Error("making a Japanese face failed", "size", f.px, "err", err)
			return
		}
		f.wide = made
	})
	if f.wide == nil {
		return nil
	}
	// A character the Japanese font has not got either is the primary's to draw as it would.
	if _, ok := f.wide.GlyphAdvance(r); !ok {
		return nil
	}
	return f.wide
}

func (f *face) of(r rune) font.Face {
	if o := f.other(r); o != nil {
		return o
	}
	return f.primary
}

func (f *face) Glyph(dot fixed.Point26_6, r rune) (image.Rectangle, image.Image, image.Point, fixed.Int26_6, bool) {
	return f.of(r).Glyph(dot, r)
}

func (f *face) GlyphBounds(r rune) (fixed.Rectangle26_6, fixed.Int26_6, bool) {
	return f.of(r).GlyphBounds(r)
}

func (f *face) GlyphAdvance(r rune) (fixed.Int26_6, bool) { return f.of(r).GlyphAdvance(r) }

// Kern is the primary's between two of its own characters, and nothing beside a Japanese one.
func (f *face) Kern(r0, r1 rune) fixed.Int26_6 {
	if r0 >= first || r1 >= first {
		return 0
	}
	return f.primary.Kern(r0, r1)
}

func (f *face) Metrics() font.Metrics { return f.primary.Metrics() }

func (f *face) Close() error {
	if f.wide != nil {
		_ = f.wide.Close()
	}
	return f.primary.Close()
}
