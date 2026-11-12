//go:build !dot && !spot

package display

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/config"
)

// A turn whose words are Japanese is drawn with them wrapped to the screen: no line runs off its
// side. With SHOW_PREVIEW set, the pages are written there to look at.
func TestAJapaneseTurnFitsTheScreen(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	now := time.Date(2026, 11, 12, 9, 30, 0, 0, time.Local)
	const heard = "明日の東京の天気を教えて"
	const reply = "明日の東京は晴れ時々くもりで、最高気温は21度、最低気温は12度の見込みです。午後から風が強くなりますが、傘は必要ありません。"
	for name, s := range map[string]scene{
		"replying":  {now: now, phase: "replying", since: now, heard: heard, reply: reply},
		"lingering": {now: now, phase: "lingering", since: now, heard: heard, reply: reply + reply},
	} {
		r := newRenderer(image.NewRGBA(image.Rect(0, 0, showWide, showHigh)))
		r.draw(s)
		// The ground at the right edge is the ground all the way down: nothing was written through it.
		edge := r.dst.RGBAAt(showWide-2, 2)
		for y := range showHigh {
			if got := r.dst.RGBAAt(showWide-2, y); got != edge {
				t.Errorf("%s: something is drawn at the screen's right edge, at row %d", name, y)
				break
			}
		}
		if dir := os.Getenv("SHOW_PREVIEW"); dir != "" {
			writeAvatarFile(t, filepath.Join(dir, "ja-turn-"+name+".png"), func(f *os.File) error { return png.Encode(f, r.dst) })
		}
	}
}
