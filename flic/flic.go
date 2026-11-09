// Package flic decodes Autodesk FLC animations (magic 0xAF12), the
// original's introduction (data/intro65a.flc in ascend02.cob).
//
// A file is a 128-byte header (size u32, magic u16, frames u16, width
// u16, height u16, depth u16, flags u16, speed u32 ms per frame, ...,
// oframe1 u32 at 80: the first frame's offset), then frames: size u32,
// type 0xF1FA, chunk count u16, 10 bytes, the chunks. Other top-level
// blocks (the 0xF100 prefix) are skipped by their size. Chunk: size u32,
// type u16, data. Decoded here, all on one 8-bit indexed canvas:
//
//	4  COLOR_256  packets: skip u8, count u8 (0 = 256), count RGB 8-bit
//	11 COLOR_64   the same, 6-bit RGB
//	13 BLACK      the canvas to index 0
//	15 BYTE_RUN   per line a packet count u8 (ignored), then i8 n until
//	              the line is full: n > 0 the next byte n times, n < 0
//	              -n literal bytes
//	16 COPY       the canvas raw
//	7  DELTA_FLC  line count u16; per line words: 00xx.. the packet
//	              count, 11xx.. (int16 < 0) -w lines skipped, 10xx..
//	              the line's last byte; then packets: column skip u8,
//	              i8 n: n > 0 n words literal, n < 0 one word -n times
//	12 DELTA_FLI  first line u16, line count u16; per line a packet
//	              count u8, packets: skip u8, i8 n: n > 0 n literal
//	              bytes, n < 0 one byte -n times
//
// Others (18 PSTAMP, the thumbnail) are skipped. The frame after the
// last, the ring frame back to the first, is never played.
package flic

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image/color"
	"io"
	"time"
)

const (
	magicFLC   = 0xAF12
	frameMagic = 0xF1FA
	headerLen  = 128
	frameHead  = 16
	chunkHead  = 6
	maxSide    = 2048 // a header's width or height past it is refused: a canvas is W×H bytes
)

// Movie is an FLC being played: Next decodes one frame onto Pix (W×H
// palette indices, row by row) and Palette.
type Movie struct {
	W, H    int
	Frames  int           // the frames played, the ring frame left out
	Speed   time.Duration // the header's delay between two frames
	Pix     []uint8
	Palette color.Palette // 256 entries, opaque
	data    []byte
	off     int // the next frame's offset
	frame   int // frames decoded
}

// Decode reads the header of the FLC in b, kept (not copied).
func Decode(b []byte) (*Movie, error) {
	if len(b) < headerLen {
		return nil, errors.New("flic: short header")
	}
	le := binary.LittleEndian
	if m := le.Uint16(b[4:]); m != magicFLC {
		return nil, fmt.Errorf("flic: magic %#x, want %#x", m, magicFLC)
	}
	if d := le.Uint16(b[12:]); d != 8 {
		return nil, fmt.Errorf("flic: depth %d", d)
	}
	if w, h := le.Uint16(b[8:]), le.Uint16(b[10:]); w == 0 || h == 0 || w > maxSide || h > maxSide {
		return nil, fmt.Errorf("flic: size %dx%d", w, h)
	}
	m := &Movie{
		W: int(le.Uint16(b[8:])), H: int(le.Uint16(b[10:])),
		Frames: int(le.Uint16(b[6:])),
		Speed:  time.Duration(le.Uint32(b[16:])) * time.Millisecond,
		data:   b, off: int(le.Uint32(b[80:])),
	}
	if m.off < headerLen || m.off > len(b) {
		m.off = headerLen
	}
	m.Pix = make([]uint8, m.W*m.H)
	m.Palette = make(color.Palette, 256)
	for i := range m.Palette {
		m.Palette[i] = color.RGBA{A: 255}
	}
	return m, nil
}

// Frame is the count of frames decoded so far.
func (m *Movie) Frame() int { return m.frame }

// Next decodes the next frame; io.EOF after the last.
func (m *Movie) Next() error {
	if m.frame >= m.Frames {
		return io.EOF
	}
	le := binary.LittleEndian
	for {
		if m.off+frameHead > len(m.data) {
			return fmt.Errorf("flic: frame %d past the end", m.frame)
		}
		size, typ := int(le.Uint32(m.data[m.off:])), le.Uint16(m.data[m.off+4:])
		if size < frameHead || m.off+size > len(m.data) {
			return fmt.Errorf("flic: frame %d: size %d", m.frame, size)
		}
		blk := m.data[m.off : m.off+size]
		m.off += size
		if typ != frameMagic {
			continue // a prefix block
		}
		n := int(le.Uint16(blk[6:]))
		p := frameHead
		for range n {
			if p+chunkHead > len(blk) {
				return fmt.Errorf("flic: frame %d: chunk past the frame", m.frame)
			}
			cs, ct := int(le.Uint32(blk[p:])), le.Uint16(blk[p+4:])
			if cs < chunkHead || p+cs > len(blk) {
				return fmt.Errorf("flic: frame %d: chunk %d size %d", m.frame, ct, cs)
			}
			if err := m.chunk(ct, blk[p+chunkHead:p+cs]); err != nil {
				return fmt.Errorf("flic: frame %d: chunk %d: %w", m.frame, ct, err)
			}
			p += cs
		}
		m.frame++
		return nil
	}
}

var errShort = errors.New("short data")

func (m *Movie) chunk(typ uint16, d []byte) (err error) {
	defer func() { // a corrupt chunk indexes past d or Pix
		if recover() != nil {
			err = errShort
		}
	}()
	switch typ {
	case 4, 11:
		m.colours(d, typ == 11)
	case 13:
		clear(m.Pix)
	case 15:
		m.byteRun(d)
	case 16:
		copy(m.Pix, d)
	case 7:
		m.deltaFLC(d)
	case 12:
		m.deltaFLI(d)
	}
	return nil
}

func (m *Movie) colours(d []byte, six bool) {
	n := int(binary.LittleEndian.Uint16(d))
	p, i := 2, 0
	for range n {
		i += int(d[p])
		c := int(d[p+1])
		if c == 0 {
			c = 256
		}
		p += 2
		for range c {
			r, g, b := d[p], d[p+1], d[p+2]
			if six {
				r, g, b = r<<2|r>>4, g<<2|g>>4, b<<2|b>>4
			}
			m.Palette[i&255] = color.RGBA{r, g, b, 255}
			i++
			p += 3
		}
	}
}

func (m *Movie) byteRun(d []byte) {
	p := 0
	for y := range m.H {
		row := m.Pix[y*m.W : (y+1)*m.W]
		p++ // the packet count, unreliable
		for x := 0; x < m.W; {
			n := int(int8(d[p]))
			p++
			if n > 0 {
				for range n {
					row[x] = d[p]
					x++
				}
				p++
			} else {
				p += copy(row[x:x-n], d[p:p-n])
				x -= n
			}
		}
	}
}

func (m *Movie) deltaFLC(d []byte) {
	le := binary.LittleEndian
	lines := int(le.Uint16(d))
	p, y := 2, 0
	for ; lines > 0; lines-- {
		var packets int
		for {
			w := le.Uint16(d[p:])
			p += 2
			if w&0xC000 == 0xC000 {
				y -= int(int16(w))
				continue
			}
			if w&0xC000 == 0x8000 {
				m.Pix[y*m.W+m.W-1] = uint8(w)
				continue
			}
			packets = int(w)
			break
		}
		row := m.Pix[y*m.W : (y+1)*m.W]
		x := 0
		for range packets {
			x += int(d[p])
			n := int(int8(d[p+1]))
			p += 2
			if n > 0 {
				p += copy(row[x:x+2*n], d[p:p+2*n])
				x += 2 * n
			} else {
				for range -n {
					row[x], row[x+1] = d[p], d[p+1]
					x += 2
				}
				p += 2
			}
		}
		y++
	}
}

func (m *Movie) deltaFLI(d []byte) {
	le := binary.LittleEndian
	y, lines := int(le.Uint16(d)), int(le.Uint16(d[2:]))
	p := 4
	for ; lines > 0; lines-- {
		row := m.Pix[y*m.W : (y+1)*m.W]
		packets := int(d[p])
		p++
		x := 0
		for range packets {
			x += int(d[p])
			n := int(int8(d[p+1]))
			p += 2
			if n > 0 {
				p += copy(row[x:x+n], d[p:p+n])
				x += n
			} else {
				for range -n {
					row[x] = d[p]
					x++
				}
				p++
			}
		}
		y++
	}
}
