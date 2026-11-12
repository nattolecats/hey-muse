package avatar

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
)

// Set is a character ready to draw: its manifest, and every sheet decoded once. The sheets stay as
// they are stored, one byte a pixel at the character's own size, and are scaled as they are drawn;
// kept at the size they are shown they would be twenty-five times the memory.
type Set struct {
	Manifest
	sheets map[string]*sheet
}

// sheet is one animation's frames, with its colors worked out for drawing: ink for a pixel, shade
// for the grid's darker edge of it, and cover for how much of what is behind the color hides, from
// none of it to all.
type sheet struct {
	pix   *image.Paletted
	ink   [256]color.RGBA
	shade [256]color.RGBA
	cover [256]uint8
}

// Load reads the set in dir. An error that Is os.ErrNotExist means there is no set there at all.
func Load(dir string) (*Set, error) { return load(dir, Parse) }

// LoadFor is Load for a set that plays something other than a turn: need names the animations it has
// to have.
func LoadFor(dir string, need []string) (*Set, error) {
	return load(dir, func(b []byte) (Manifest, error) { return ParseFor(b, need) })
}

func load(dir string, parse func([]byte) (Manifest, error)) (*Set, error) {
	f, err := os.Open(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	b, err := io.ReadAll(io.LimitReader(f, manifestMax))
	f.Close()
	if err != nil {
		return nil, err
	}
	m, err := parse(b)
	if err != nil {
		return nil, err
	}
	s := &Set{Manifest: m, sheets: make(map[string]*sheet, len(m.Animations))}
	for name, a := range m.Animations {
		sh, err := loadSheet(filepath.Join(dir, name+".png"), a.Size(m.Cell), m.Grid)
		if err != nil {
			return nil, fmt.Errorf("%s.png: %w", name, err)
		}
		s.sheets[name] = sh
	}
	return s, nil
}

// loadSheet decodes one sheet, which has to be the size its manifest says: the header is read first,
// so a picture that is not is turned away before any of it is decoded.
func loadSheet(path string, want image.Point, grid float64) (*sheet, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cfg, err := png.DecodeConfig(f)
	if err != nil {
		return nil, err
	}
	if cfg.Width != want.X || cfg.Height != want.Y {
		return nil, fmt.Errorf("is %dx%d, and its manifest says %dx%d", cfg.Width, cfg.Height, want.X, want.Y)
	}
	if _, ok := cfg.ColorModel.(color.Palette); !ok {
		return nil, errors.New("is not a paletted PNG")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	img, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	pix := img.(*image.Paletted)
	sh := &sheet{pix: pix}
	if grid == 0 {
		grid = 1
	}
	for i, c := range pix.Palette {
		n := color.NRGBAModel.Convert(c).(color.NRGBA)
		// A drawn character's pixels are clear or solid. One cut out of a video has a soft edge, and
		// its sheet's palette says how soft: the color is laid over what is there by that much.
		sh.cover[i] = n.A
		sh.ink[i] = color.RGBA{n.R, n.G, n.B, 0xff}
		sh.shade[i] = color.RGBA{uint8(float64(n.R) * grid), uint8(float64(n.G) * grid), uint8(float64(n.B) * grid), 0xff}
	}
	return sh, nil
}

// Bytes is how much memory the decoded sheets take.
func (s *Set) Bytes() int {
	n := 0
	for _, sh := range s.sheets {
		n += len(sh.pix.Pix)
	}
	return n
}

// Scale is the largest whole number of screen pixels to a sprite pixel at which a frame fits in a
// square of side, and never less than one: whole, so every pixel of the character stays square.
func (s *Set) Scale(side int) int {
	return max(1, side/s.Cell)
}

// Draw puts frame i of the named animation on dst with its top left corner at at, every pixel of it
// scale wide. Clear pixels leave dst as it was, and soft ones are laid over it. A frame that would not land wholly on dst is not
// drawn.
func (s *Set) Draw(dst *image.RGBA, at image.Point, scale int, name string, i int) {
	sh, a := s.sheets[name], s.Animations[name]
	box := image.Rectangle{Min: at, Max: at.Add(image.Pt(s.Cell*scale, s.Cell*scale))}
	if sh == nil || scale < 1 || i < 0 || i >= a.Frames || !box.In(dst.Rect) {
		return
	}
	src := a.Rect(s.Cell, i)
	// Too small to see a grid through: at one or two pixels across, the edge would be the pixel.
	edge := scale
	if scale >= 3 && s.Grid > 0 && s.Grid < 1 {
		edge = scale - 1
	}
	for sy := range s.Cell {
		row := sh.pix.Pix[sh.pix.PixOffset(src.Min.X, src.Min.Y+sy):]
		for sx := range s.Cell {
			c := row[sx]
			cover := uint32(sh.cover[c])
			if cover == 0 {
				continue
			}
			ink, shade := sh.ink[c], sh.shade[c]
			if cover < 0xff {
				for dy := range scale {
					o := dst.PixOffset(at.X+sx*scale, at.Y+sy*scale+dy)
					px := dst.Pix[o : o+scale*4 : o+scale*4]
					for dx := range scale {
						px[dx*4] = uint8((uint32(ink.R)*cover + uint32(px[dx*4])*(0xff-cover) + 0x7f) / 0xff)
						px[dx*4+1] = uint8((uint32(ink.G)*cover + uint32(px[dx*4+1])*(0xff-cover) + 0x7f) / 0xff)
						px[dx*4+2] = uint8((uint32(ink.B)*cover + uint32(px[dx*4+2])*(0xff-cover) + 0x7f) / 0xff)
						px[dx*4+3] = 0xff
					}
				}
				continue
			}
			for dy := range scale {
				o := dst.PixOffset(at.X+sx*scale, at.Y+sy*scale+dy)
				px := dst.Pix[o : o+scale*4 : o+scale*4]
				for dx := range scale {
					v := ink
					if dx >= edge || dy >= edge {
						v = shade
					}
					px[dx*4], px[dx*4+1], px[dx*4+2], px[dx*4+3] = v.R, v.G, v.B, 0xff
				}
			}
		}
	}
}

// DrawScaled puts frame i of the named animation on dst so that it fills box, whatever size box is:
// what a character that comes nearer and goes further away is drawn with. The frame is smoothed as it
// is scaled, no grid is drawn through it, and the part of box that is off dst is left out.
func (s *Set) DrawScaled(dst *image.RGBA, box image.Rectangle, name string, i int) {
	sh, a := s.sheets[name], s.Animations[name]
	if sh == nil || i < 0 || i >= a.Frames || box.Empty() {
		return
	}
	src := a.Rect(s.Cell, i)
	on := box.Intersect(dst.Rect)
	if on.Empty() {
		return
	}
	// Where in the frame each of box's pixels looks, in 1/256ths of a sprite pixel: the middle of the
	// screen pixel, less the half that makes a sprite pixel's own middle its whole number.
	const one = 256
	stepX, stepY := s.Cell*one/box.Dx(), s.Cell*one/box.Dy()
	last := s.Cell - 1
	stride := sh.pix.Stride
	base := sh.pix.PixOffset(src.Min.X, src.Min.Y)
	for y := on.Min.Y; y < on.Max.Y; y++ {
		v := max((y-box.Min.Y)*stepY+stepY/2-one/2, 0)
		y0 := min(v/one, last)
		y1, fy := min(y0+1, last), uint32(v%one)
		row0, row1 := sh.pix.Pix[base+y0*stride:], sh.pix.Pix[base+y1*stride:]
		o := dst.PixOffset(on.Min.X, y)
		for x := on.Min.X; x < on.Max.X; x, o = x+1, o+4 {
			u := max((x-box.Min.X)*stepX+stepX/2-one/2, 0)
			x0 := min(u/one, last)
			x1, fx := min(x0+1, last), uint32(u%one)
			c := [4]uint8{row0[x0], row0[x1], row1[x0], row1[x1]}
			if sh.cover[c[0]]|sh.cover[c[1]]|sh.cover[c[2]]|sh.cover[c[3]] == 0 {
				continue
			}
			w := [4]uint32{(one - fx) * (one - fy), fx * (one - fy), (one - fx) * fy, fx * fy}
			// Each color counts by how much of it there is, so a clear pixel's black stays out of the edge.
			var r, g, b, n, cover uint32
			for k, ci := range c {
				ca := uint32(sh.cover[ci]) * w[k]
				ink := sh.ink[ci]
				r, g, b, n, cover = r+uint32(ink.R)*(ca>>8), g+uint32(ink.G)*(ca>>8), b+uint32(ink.B)*(ca>>8), n+ca>>8, cover+ca
			}
			if n == 0 {
				continue
			}
			r, g, b = (r+n/2)/n, (g+n/2)/n, (b+n/2)/n
			al := cover / (one * one) // 0 to 255
			px := dst.Pix[o : o+4 : o+4]
			px[0] = uint8((r*al + uint32(px[0])*(0xff-al) + 0x7f) / 0xff)
			px[1] = uint8((g*al + uint32(px[1])*(0xff-al) + 0x7f) / 0xff)
			px[2] = uint8((b*al + uint32(px[2])*(0xff-al) + 0x7f) / 0xff)
			px[3] = 0xff
		}
	}
}
