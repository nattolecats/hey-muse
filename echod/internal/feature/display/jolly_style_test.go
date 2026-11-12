//go:build !dot && !spot

package display

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/lib/avatar"
)

// standInJolly is a set to test the style with, since the real one is nobody's to keep here: every
// frame of every animation a solid square of standInColor.
func standInJolly(t testing.TB) *avatar.Set {
	t.Helper()
	dir := t.TempDir()
	manifest := `{"cell": 64, "fps": 12, "animations": {`
	for i, name := range jollyAnimations {
		if i > 0 {
			manifest += ","
		}
		manifest += fmt.Sprintf(`%q: {"frames": 2, "columns": 2, "loop": true}`, name)
		img := image.NewPaletted(image.Rect(0, 0, 128, 64), color.Palette{standInColor})
		writeAvatarFile(t, filepath.Join(dir, name+".png"), func(f *os.File) error { return png.Encode(f, img) })
	}
	writeAvatarFile(t, filepath.Join(dir, "manifest.json"), func(f *os.File) error {
		_, err := f.WriteString(manifest + "}}")
		return err
	})
	set, err := avatar.LoadFor(dir, jollyAnimations)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

// withJolly has the style draw set until the test is over.
func withJolly(t testing.TB, set *avatar.Set) {
	old := jollySet
	t.Cleanup(func() { jollySet = old })
	jollySet = func() *avatar.Set { return set }
}

func seededWander(seed uint64) wander {
	return wander{rng: rand.New(rand.NewPCG(seed, 7))}
}

// The character keeps to the floor it can be seen whole on, faces the way it is going, stands facing
// out, and from the same start lives the same life. Over ten minutes it walks, stands, goes to the
// back of the room and comes to the glass.
func TestTheCharacterWandersTheFloor(t *testing.T) {
	const aspect = 2.0
	start := time.Date(2026, 11, 12, 9, 30, 0, 0, time.Local)
	a, b := seededWander(1), seededWander(1)
	var walked, stood, visited, deep bool
	seen := map[string]bool{}
	for i := range 10 * 60 * 12 {
		now := start.Add(time.Duration(i) * time.Second / 12)
		px, pz := a.x, a.z
		a.move(now, aspect)
		b.move(now, aspect)
		if a.x != b.x || a.z != b.z || a.facing != b.facing {
			t.Fatalf("two characters from one start are apart after %v", now.Sub(start))
		}
		if a.z < jollyVisit-1e-9 || a.z > 1+1e-9 {
			t.Fatalf("the character is at depth %v", a.z)
		}
		if reach := jollyReach(a.z, aspect); a.x < -reach-1e-9 || a.x > reach+1e-9 {
			t.Fatalf("the character is %v across at depth %v, where the floor reaches %v", a.x, a.z, reach)
		}
		box, feet := jollyPlace(a.x, a.z, 960, 480)
		if wide := int(float64(box.Dx()) * jollyWide / 2); feet.X-wide < -1 || feet.X+wide > 961 {
			t.Fatalf("the character at %v, %v is drawn off the side of the screen: %v", a.x, a.z, box)
		}
		seen[a.facing] = true
		dx, dz := a.x-px, a.z-pz
		if i == 0 || (dx == 0 && dz == 0) {
			stood = stood || !a.walking
			continue
		}
		walked = true
		visited = visited || a.z < 0
		deep = deep || a.z > 0.8
		switch a.facing {
		case jollyLeft:
			if dx >= 0 {
				t.Fatalf("facing left, it moved %v across", dx)
			}
		case jollyRight:
			if dx <= 0 {
				t.Fatalf("facing right, it moved %v across", dx)
			}
		case jollyFront:
			if dz >= 0 {
				t.Fatalf("facing out, it moved %v back", dz)
			}
		case jollyBack:
			if dz <= 0 {
				t.Fatalf("facing the wall, it moved %v back", dz)
			}
		case jollyIdle:
			// The step that brought it to where it was going.
			if a.walking {
				t.Fatal("it is walking with the picture of it at rest")
			}
		}
	}
	if !walked || !stood || !visited || !deep {
		t.Errorf("in ten minutes: walked %v, stood %v, came to the glass %v, went to the back %v", walked, stood, visited, deep)
	}
	for _, name := range jollyAnimations {
		if !seen[name] {
			t.Errorf("in ten minutes it was never drawn as %s", name)
		}
	}
}

// A page that was up for a while leaves the character where it was, and a clock set back does not
// send it anywhere.
func TestTheCharacterPicksUpWhereItWas(t *testing.T) {
	now := time.Date(2026, 11, 12, 9, 30, 0, 0, time.Local)
	w := seededWander(3)
	for range 200 {
		now = now.Add(80 * time.Millisecond)
		w.move(now, 2)
	}
	x, z := w.x, w.z
	w.move(now.Add(time.Hour), 2)
	if far := jollySpeed * jollyMaxStep.Seconds(); w.x < x-far || w.x > x+far || w.z < z-far || w.z > z+far {
		t.Errorf("an hour away moved it from %v, %v to %v, %v", x, z, w.x, w.z)
	}
	w.move(now, 2)
	if w.z < jollyVisit || w.z > 1 {
		t.Errorf("the clock set back put it at depth %v", w.z)
	}
}

// jollyBox is where the stand-in character is on the canvas, and how many pixels of it there are.
func jollyBox(img *image.RGBA) (box image.Rectangle, n int) {
	for y := range img.Rect.Dy() {
		for x := range img.Rect.Dx() {
			if c := img.RGBAAt(x, y); c.R == standInColor.R && c.G == standInColor.G && c.B == standInColor.B {
				box = box.Union(image.Rect(x, y, x+1, y+1))
				n++
			}
		}
	}
	return box, n
}

// The style draws the character smaller and higher up at the back of the room than at the glass, with
// the time on the wall, and wants its next frame as soon as the character's is due.
func TestTheJollyStyleDrawsTheCharacterInItsRoom(t *testing.T) {
	now := time.Date(2026, 11, 12, 9, 30, 0, 0, time.Local)
	withJolly(t, standInJolly(t))
	for _, panel := range []struct{ wide, high int }{{showWide, showHigh}, {show8Wide, show8High}} {
		r := newRenderer(image.NewRGBA(image.Rect(0, 0, panel.wide, panel.high)))
		s := scene{now: now}
		s.style.kind, s.style.chosen = styleJolly, true
		at := func(x, z float64) (image.Rectangle, int) {
			r.jolly = seededWander(1)
			r.jolly.at, r.jolly.x, r.jolly.z, r.jolly.facing, r.jolly.until = now, x, z, jollyIdle, now.Add(time.Minute)
			r.draw(s)
			if !r.jollyDrawn {
				t.Fatal("a frame with the character in it does not say so")
			}
			return jollyBox(r.dst)
		}
		near, nearN := at(0, 0)
		far, farN := at(0, 1)
		if nearN == 0 || farN == 0 || near.Dy() < 2*far.Dy() || far.Max.Y >= near.Max.Y {
			t.Errorf("%dx%d: at the glass the character is %v, at the wall %v", panel.wide, panel.high, near, far)
		}
		if near.Min.Y < 0 || float64(near.Dy()) > float64(panel.high)*jollyCell+1 {
			t.Errorf("%dx%d: at the glass the character is %v", panel.wide, panel.high, near)
		}
		if mid := (far.Min.X + far.Max.X) / 2; mid < panel.wide/2-2 || mid > panel.wide/2+2 {
			t.Errorf("%dx%d: in the middle of the floor the character is drawn at %v", panel.wide, panel.high, far)
		}
		if wait := r.jollyUntil(now); wait <= 0 || wait > time.Second/12 {
			t.Errorf("the next frame is wanted in %v", wait)
		}
		if countOf(r.dst, cream) == 0 {
			t.Errorf("%dx%d: the time is not on the wall", panel.wide, panel.high)
		}
	}
}

// With no character installed the style is the room and the time, drawn once a second like any clock,
// and a character that is installed changes no other style.
func TestTheJollyStyleWithoutACharacter(t *testing.T) {
	now := time.Date(2026, 11, 12, 9, 30, 0, 0, time.Local)
	withJolly(t, nil)
	r := newRenderer(image.NewRGBA(image.Rect(0, 0, showWide, showHigh)))
	s := scene{now: now}
	s.style.kind, s.style.chosen = styleJolly, true
	r.draw(s)
	if r.jollyDrawn || countOf(r.dst, cream) == 0 || countOf(r.dst, jollyFaint) == 0 {
		t.Errorf("with no character: drawn %v, %d pixels of the time, %d of the words saying so",
			r.jollyDrawn, countOf(r.dst, cream), countOf(r.dst, jollyFaint))
	}

	classic := scene{now: now}
	classic.style.kind, classic.style.chosen = styleClassic, true
	plain := newRenderer(image.NewRGBA(image.Rect(0, 0, showWide, showHigh)))
	plain.draw(classic)
	withJolly(t, standInJolly(t))
	with := newRenderer(image.NewRGBA(image.Rect(0, 0, showWide, showHigh)))
	with.draw(classic)
	if with.jollyDrawn || string(plain.dst.Pix) != string(with.dst.Pix) {
		t.Error("the classic clock is drawn differently with a character installed")
	}
}

// JOLLY_PREVIEW names a folder to write a minute of the style to, a frame a file, to look at without
// a device; JOLLY_SPRITES a set made by tools/muse-avatar/make_roam_sprites.py to draw it with.
func TestJollyPreview(t *testing.T) {
	out := os.Getenv("JOLLY_PREVIEW")
	if out == "" {
		t.Skip("JOLLY_PREVIEW is not set")
	}
	set := standInJolly(t)
	if dir := os.Getenv("JOLLY_SPRITES"); dir != "" {
		var err error
		if set, err = avatar.LoadFor(dir, jollyAnimations); err != nil {
			t.Fatal(err)
		}
	}
	withJolly(t, set)
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	r := newRenderer(image.NewRGBA(image.Rect(0, 0, showWide, showHigh)))
	r.jolly = seededWander(5)
	start := time.Date(2026, 11, 12, 9, 30, 0, 0, time.Local)
	for i := range 60 * 12 {
		s := scene{now: start.Add(time.Duration(i) * time.Second / 12)}
		s.style.kind, s.style.chosen = styleJolly, true
		r.draw(s)
		writeAvatarFile(t, filepath.Join(out, fmt.Sprintf("%04d.png", i)), func(f *os.File) error { return png.Encode(f, r.dst) })
	}
}

// One frame of the style with the character at the glass, where it is largest.
func BenchmarkJollyFrame(b *testing.B) {
	withJolly(b, standInJolly(b))
	r := newRenderer(image.NewRGBA(image.Rect(0, 0, showWide, showHigh)))
	s := scene{now: time.Now()}
	s.style.kind, s.style.chosen = styleJolly, true
	for i := range b.N {
		s.now = s.now.Add(80 * time.Millisecond * time.Duration(i%2))
		r.draw(s)
	}
}
