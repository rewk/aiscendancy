// Package shp decodes Ascendancy's SHP sprite banks and VGA palettes.
//
// A bank is "1.10", a u32 frame count, then count × (u32 frame offset, u32
// palette offset — always 0 in the shipped files: the palette comes from a
// .pal). Each frame is a 24-byte header and an RLE stream:
//
//	u16 height-1, u16 width-1   the canvas (320x200 panels, 40x40 bullets…)
//	u16 cy, u16 cx              the origin inside the canvas, y first like the
//	                            size (0,0 for panels, the hot spot for ships,
//	                            bullets, explosions)
//	i32 x0, y0, x1, y1          the drawn box, INCLUSIVE, relative to (cx,cy)
//
// The stream is one row per box line, each ended by 0x00:
//
//	0x00        end of row (the rest of the row is transparent)
//	0x01 n      n transparent pixels
//	odd  b      (b>>1) literal palette indices follow
//	even b  c   (b>>1) pixels of index c
//
// Frames with a box outside the canvas (garbage coordinates) are empty.
// A palette file is 256 × (r,g,b) in 6-bit VGA units.
package shp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
)

// Frame is one decoded sprite. Index and Opaque cover Box only, row-major.
type Frame struct {
	Width, Height int             // the canvas
	Center        image.Point     // the origin in canvas coordinates
	Box           image.Rectangle // drawn area, canvas coordinates, half-open
	Index         []uint8         // palette indices, Box.Dx()*Box.Dy()
	Opaque        []bool          // false where the frame is transparent
}

// Empty reports a frame with nothing to draw.
func (f *Frame) Empty() bool { return f.Box.Empty() }

// RGBA paints the frame on a transparent canvas-sized image.
func (f *Frame) RGBA(pal color.Palette) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, f.Width, f.Height))
	f.Draw(img, pal)
	return img
}

// Draw paints the opaque pixels of the frame onto img at Box.
func (f *Frame) Draw(img *image.RGBA, pal color.Palette) {
	w := f.Box.Dx()
	for y := 0; y < f.Box.Dy(); y++ {
		for x := 0; x < w; x++ {
			k := y*w + x
			if !f.Opaque[k] {
				continue
			}
			c := pal[f.Index[k]].(color.RGBA)
			img.SetRGBA(f.Box.Min.X+x, f.Box.Min.Y+y, c)
		}
	}
}

// Bank is a decoded SHP file.
type Bank struct {
	Frames []*Frame
}

var magic = []byte("1.10")

// Decode parses a whole SHP file.
func Decode(b []byte) (*Bank, error) {
	if len(b) < 8 || string(b[:4]) != string(magic) {
		return nil, errors.New("shp: bad magic")
	}
	n := int(binary.LittleEndian.Uint32(b[4:]))
	if n < 0 || 8+8*n > len(b) {
		return nil, fmt.Errorf("shp: implausible frame count %d", n)
	}
	bank := &Bank{}
	for i := 0; i < n; i++ {
		off := int(binary.LittleEndian.Uint32(b[8+8*i:]))
		end := len(b)
		if i+1 < n {
			end = int(binary.LittleEndian.Uint32(b[8+8*(i+1):]))
		}
		if off < 8+8*n || end < off+24 || end > len(b) {
			return nil, fmt.Errorf("shp: frame %d: offset %d out of range", i, off)
		}
		f, err := decodeFrame(b[off:end])
		if err != nil {
			return nil, fmt.Errorf("shp: frame %d: %w", i, err)
		}
		bank.Frames = append(bank.Frames, f)
	}
	return bank, nil
}

func decodeFrame(b []byte) (*Frame, error) {
	le := binary.LittleEndian
	f := &Frame{
		Height: int(le.Uint16(b[0:])) + 1,
		Width:  int(le.Uint16(b[2:])) + 1,
		Center: image.Pt(int(le.Uint16(b[6:])), int(le.Uint16(b[4:]))),
	}
	x0, y0 := int(int32(le.Uint32(b[8:]))), int(int32(le.Uint32(b[12:])))
	x1, y1 := int(int32(le.Uint32(b[16:]))), int(int32(le.Uint32(b[20:])))
	if x1 < x0 || y1 < y0 || x0 > f.Width || y0 > f.Height ||
		x0 < -f.Width || y0 < -f.Height || x1 > 2*f.Width || y1 > 2*f.Height {
		return f, nil // an empty slot: the box holds garbage
	}
	f.Box = image.Rect(x0, y0, x1+1, y1+1).Add(f.Center)
	w, h := f.Box.Dx(), f.Box.Dy()
	f.Index = make([]uint8, w*h)
	f.Opaque = make([]bool, w*h)
	p := 24
	for y := 0; y < h; y++ {
		x := 0
		row := f.Index[y*w : (y+1)*w]
		op := f.Opaque[y*w : (y+1)*w]
		for {
			if p >= len(b) {
				return nil, fmt.Errorf("row %d: stream ends early", y)
			}
			c := int(b[p])
			p++
			if c == 0 {
				break
			}
			if c == 1 {
				if p >= len(b) {
					return nil, fmt.Errorf("row %d: truncated skip", y)
				}
				x += int(b[p])
				p++
				continue
			}
			n := c >> 1
			if c&1 == 1 {
				if p+n > len(b) {
					return nil, fmt.Errorf("row %d: truncated literal", y)
				}
				for k := 0; k < n; k++ {
					if x < w {
						row[x] = b[p+k]
						op[x] = true
					}
					x++
				}
				p += n
			} else {
				if p >= len(b) {
					return nil, fmt.Errorf("row %d: truncated run", y)
				}
				v := b[p]
				p++
				for k := 0; k < n; k++ {
					if x < w {
						row[x] = v
						op[x] = true
					}
					x++
				}
			}
		}
	}
	return f, nil
}

// Palette parses a 768-byte VGA palette (6-bit components).
func Palette(b []byte) (color.Palette, error) {
	if len(b) != 768 {
		return nil, fmt.Errorf("palette: %d bytes, want 768", len(b))
	}
	pal := make(color.Palette, 256)
	for i := range pal {
		r, g, bl := b[3*i], b[3*i+1], b[3*i+2]
		// 6-bit to 8-bit with the top bits folded down, so 63 maps to 255.
		pal[i] = color.RGBA{r<<2 | r>>4, g<<2 | g>>4, bl<<2 | bl>>4, 255}
	}
	return pal, nil
}
