//go:build !dot && !spot

package display

import (
	"image"
	"image/color"
	"log/slog"
	"math"
	"math/rand/v2"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/layout"
	"github.com/HuskerMinion/techo5/echod/internal/lib/avatar"
	"github.com/HuskerMinion/techo5/echod/internal/lib/locale"
)

// The Jolly clock style: a character walking about a room, with the time in a corner of the wall
// behind it. The room has depth: the character is drawn smaller and higher the further back it is,
// turned the way it is going, and now and then it comes up to the glass. The character is a set of
// sprite sheets its owner makes from a picture of it (tools/muse-avatar/make_roam_sprites.py) and puts
// in jollyDir. The software carries none, and without one the style is the room and the time.

const styleJolly = "jolly"

func init() {
	clockStyles = append(clockStyles, struct{ label, value string }{"Jolly", styleJolly})
}

// jollyDir is where an installed set lives: with the device's own state, which an update leaves alone.
const jollyDir = layout.StateDir + "/jolly"

// The animations a set has to have: the character at rest, facing out of the screen, and walking each
// of the four ways it can be seen going.
const (
	jollyIdle  = "idle"
	jollyFront = "walk_front" // toward whoever is looking
	jollyBack  = "walk_back"
	jollyLeft  = "walk_left"
	jollyRight = "walk_right"
)

var jollyAnimations = []string{jollyIdle, jollyFront, jollyBack, jollyLeft, jollyRight}

// jollyLoaded is the installed set, read once as the screen starts (loadJolly); jollySet is where a
// frame gets it, and a test puts its own there.
var (
	jollyLoaded *avatar.Set
	jollySet    = func() *avatar.Set { return jollyLoaded }
)

// loadJolly reads the installed set, if there is one, and says which it was.
func loadJolly() {
	set, err := avatar.LoadFor(jollyDir, jollyAnimations)
	if err != nil {
		slog.Info("no Jolly character installed: the Jolly clock style is the room alone", "dir", jollyDir, "err", err)
		return
	}
	slog.Info("Jolly character loaded", "dir", jollyDir, "bytes", set.Bytes())
	jollyLoaded = set
}

// The room, as shares of the screen and in the floor's own measures. The floor runs from the glass
// (z 0) back to the wall (z 1); a thing at z is drawn 1/(1+jollyDeep*z) the size it is at the glass.
const (
	jollyHorizon = 0.42 // the foot of the wall, down the screen
	jollyNear    = 0.97 // where feet at the glass stand, down the screen
	jollyDeep    = 1.6
	jollyCell    = 0.90 // a frame's side at the glass, of the screen's height
	// jollyFoot and jollyWide are where the sheets' maker puts the character in a frame: its feet this
	// far down it, and no wider than this share of it.
	jollyFoot = 0.95
	jollyWide = 0.92
	// jollyVisit is how far past the glass the character comes when it comes to look.
	jollyVisit = -0.1

	jollySpeed   = 0.42 // floor a second, the depth counted double: far across is about five seconds
	jollyMaxStep = time.Second
)

var (
	jollyWall   = color.RGBA{0x17, 0x16, 0x15, 0xff}
	jollyFloorA = color.RGBA{0x2b, 0x28, 0x24, 0xff} // the floor at the wall
	jollyFloorB = color.RGBA{0x3a, 0x35, 0x2e, 0xff} // and at the glass
	jollyFaint  = color.RGBA{0x87, 0x86, 0x7f, 0xff}
)

// wander is the character's life on the floor: where it is, where it is going, and what it does when
// it gets there. It runs by the clock, so a frame drawn late moves it further and not slower.
type wander struct {
	rng        *rand.Rand
	at         time.Time // when it was last moved
	x, z       float64   // on the floor: x across in half screens at the glass, z back from it
	tx, tz     float64   // where it is walking to
	walking    bool
	until      time.Time // standing: when it sets off again
	facing     string    // the animation it is drawn with
	sinceVisit int       // walks since it last came to the glass
}

// persp is how large a thing at depth z is drawn, 1 at the glass.
func jollyPersp(z float64) float64 { return 1 / (1 + jollyDeep*z) }

// reach is how far to either side the floor can be walked at depth z with the character whole on a
// screen aspect times as wide as it is high.
func jollyReach(z, aspect float64) float64 {
	p := jollyPersp(z)
	half := aspect / 2
	return max(0, (half-jollyCell*jollyWide/2*p)/(half*p))
}

// start puts the character somewhere on the floor, standing.
func (w *wander) start(now time.Time, aspect float64) {
	if w.rng == nil {
		n := uint64(now.UnixNano())
		w.rng = rand.New(rand.NewPCG(n, n>>17|1))
	}
	w.at = now
	w.z = 0.2 + 0.5*w.rng.Float64()
	w.x = (w.rng.Float64()*2 - 1) * jollyReach(w.z, aspect)
	w.walking, w.facing = false, jollyIdle
	w.until = now.Add(time.Second)
}

// choose picks where it walks to next: usually anywhere on the floor, now and then up to the glass,
// off to the right where it does not stand in front of the time.
func (w *wander) choose(aspect float64) {
	w.sinceVisit++
	if w.sinceVisit >= 3 && w.rng.Float64() < 0.3 {
		w.sinceVisit = 0
		w.tz = jollyVisit
		w.tx = (0.25 + 0.5*w.rng.Float64()) * jollyReach(w.tz, aspect)
	} else {
		w.tz = 0.02 + 0.98*w.rng.Float64()
		w.tx = (w.rng.Float64()*2 - 1) * jollyReach(w.tz, aspect)
	}
	w.walking = true
	dx, dz := w.tx-w.x, w.tz-w.z
	switch {
	case math.Abs(dx) >= 2*math.Abs(dz) && dx < 0:
		w.facing = jollyLeft
	case math.Abs(dx) >= 2*math.Abs(dz):
		w.facing = jollyRight
	case dz < 0:
		w.facing = jollyFront
	default:
		w.facing = jollyBack
	}
}

// move brings the character to where it is at now.
func (w *wander) move(now time.Time, aspect float64) {
	if w.at.IsZero() || now.Before(w.at) {
		w.start(now, aspect)
		return
	}
	// A long gap is another page having been up: it carries on from where it was.
	left := min(now.Sub(w.at), jollyMaxStep)
	w.at = now
	if !w.walking {
		if now.Before(w.until) {
			return
		}
		w.choose(aspect)
	}
	dx, dz := w.tx-w.x, 2*(w.tz-w.z)
	far := math.Hypot(dx, dz)
	if step := jollySpeed * left.Seconds(); step < far {
		w.x += dx / far * step
		w.z += dz / far * step / 2
		return
	}
	w.x, w.z = w.tx, w.tz
	w.walking, w.facing = false, jollyIdle
	// It stands a while, and longer at the glass, where it came to look.
	stand := 2*time.Second + time.Duration(w.rng.Float64()*float64(5*time.Second))
	if w.z <= 0 {
		stand = 4*time.Second + time.Duration(w.rng.Float64()*float64(3*time.Second))
	}
	w.until = now.Add(stand)
}

// jollyPlace is where the character at x, z is drawn on a screen w by h: its frame's box, and where
// its feet are.
func jollyPlace(x, z float64, w, h int) (box image.Rectangle, feet image.Point) {
	p := jollyPersp(z)
	fw, fh := float64(w), float64(h)
	side := jollyCell * fh * p
	fx := fw/2 + x*fw/2*p
	fy := (jollyHorizon + (jollyNear-jollyHorizon)*p) * fh
	left, top := int(math.Round(fx-side/2)), int(math.Round(fy-side*jollyFoot))
	n := int(math.Round(side))
	return image.Rect(left, top, left+n, top+n), image.Pt(int(math.Round(fx)), int(math.Round(fy)))
}

// jollyStyle draws the whole screen.
func (r *renderer) jollyStyle(s scene) {
	r.jollyRoom()
	r.timeAndDateAt(s.now, r.s(100), "", -1, r.margin)

	set := jollySet()
	if set == nil {
		none := locale.NoCharacter(screenLang())
		r.text(r.small, none, (r.w-r.width(r.small, none))/2, r.h*3/4, jollyFaint)
	} else {
		r.jolly.move(s.now, float64(r.w)/float64(r.h))
		box, feet := jollyPlace(r.jolly.x, r.jolly.z, r.w, r.h)
		r.jollyShadow(feet, box.Dx())
		name := r.jolly.facing
		set.DrawScaled(r.dst, box, name, r.jollyAv.Frame(set.Manifest, name, s.now))
		r.jollyDrawn = true
	}
	r.timersLine(s, r.h-r.s(18))
}

// jollyRoom is the wall and the floor: the floor lighter toward the glass, as if lit from this side.
func (r *renderer) jollyRoom() {
	horizon := int(jollyHorizon * float64(r.h))
	for y := range r.h {
		c := jollyWall
		if y >= horizon {
			c = lerp(jollyFloorA, jollyFloorB, float64(y-horizon)/float64(max(r.h-1-horizon, 1)))
		}
		row := r.dst.Pix[r.dst.PixOffset(0, y) : r.dst.PixOffset(0, y)+r.w*4]
		for i := 0; i < len(row); i += 4 {
			row[i], row[i+1], row[i+2], row[i+3] = c.R, c.G, c.B, 0xff
		}
	}
}

// jollyShadow is the character's shadow on the floor under its feet, for a frame side wide: soft at
// its edge, and no part of it off the screen.
func (r *renderer) jollyShadow(feet image.Point, side int) {
	rx, ry := float64(side)*0.21, float64(side)*0.045
	if rx < 1 || ry < 1 {
		return
	}
	const dark = 0.45
	b := image.Rect(feet.X-int(rx)-1, feet.Y-int(ry)-1, feet.X+int(rx)+2, feet.Y+int(ry)+2).Intersect(r.dst.Rect)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		dy := (float64(y-feet.Y) + 0.5) / ry
		o := r.dst.PixOffset(b.Min.X, y)
		for x := b.Min.X; x < b.Max.X; x, o = x+1, o+4 {
			dx := (float64(x-feet.X) + 0.5) / rx
			in := 1 - (dx*dx + dy*dy)
			if in <= 0 {
				continue
			}
			keep := uint32(256 * (1 - dark*min(1, in*1.6)))
			px := r.dst.Pix[o : o+3 : o+3]
			px[0], px[1], px[2] = uint8(uint32(px[0])*keep>>8), uint8(uint32(px[1])*keep>>8), uint8(uint32(px[2])*keep>>8)
		}
	}
}

// jollyUntil is how long from now the character's next frame is due.
func (r *renderer) jollyUntil(now time.Time) time.Duration {
	set := jollySet()
	if set == nil {
		return idleFrame
	}
	return r.jollyAv.Until(set.Manifest, now)
}
