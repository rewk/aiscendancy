// Package sound plays the original's music from its COB archives.
//
// music.txt (ascend00.cob) lists the tunes: a count line, then that many
// DOS paths (`data\theme00.raw`), CRLF. Every .raw is headerless PCM,
// 8-bit unsigned, mono, 22050 Hz. A Track streams one as the s16le
// stereo the device takes, converted on the fly from the raw bytes
// (never a decoded copy, nothing written to disk), silences included
// (the themes sound odd without them): the themes
// end on ~30 s of silence, theme00's four pieces are ~30 s apart.
//
// The Device (device.go) is oto's, under Ebiten's audio package, used
// directly: Ebiten's audio returns a device failure from RunGame and
// ends the game, oto reports it through Err.
package sound

import (
	"encoding/binary"
	"errors"
	"io"
	"time"
)

const (
	// SampleRate is the .raw files' rate, the device's: no resampling.
	SampleRate = 22050
	// FrameBytes is one output sample: s16le, left then right.
	FrameBytes = 4
)

// Track is one tune as an io.ReadSeeker of s16le stereo frames: output
// sample o is raw sample o converted to (b−128)<<8 on both channels.
type Track struct {
	raw []byte
	pos int64 // output bytes read
}

// NewTrack wraps raw, kept, not copied.
func NewTrack(raw []byte) *Track { return &Track{raw: raw} }

// Len is the stream's length in bytes.
func (t *Track) Len() int64 { return int64(len(t.raw)) * FrameBytes }

// Duration is how long the track plays.
func (t *Track) Duration() time.Duration {
	return time.Duration(len(t.raw)) * time.Second / SampleRate
}

func (t *Track) Read(p []byte) (int, error) {
	if t.pos >= t.Len() {
		return 0, io.EOF
	}
	off := int(t.pos % FrameBytes)
	r := int(t.pos / FrameBytes)
	n := 0
	for n < len(p) && t.pos < t.Len() {
		var f [FrameBytes]byte
		binary.LittleEndian.PutUint16(f[0:], uint16(int16(int(t.raw[r])-128)<<8))
		f[2], f[3] = f[0], f[1]
		c := copy(p[n:], f[off:])
		n += c
		t.pos += int64(c)
		if off += c; off < FrameBytes {
			break // p is full mid-frame
		}
		off = 0
		r++
	}
	return n, nil
}

// Seek moves the stream to any byte offset, mid-frame too; past the end
// Read returns io.EOF.
func (t *Track) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset += t.pos
	case io.SeekEnd:
		offset += t.Len()
	default:
		return t.pos, errors.New("sound: bad whence")
	}
	if offset < 0 {
		return t.pos, errors.New("sound: negative position")
	}
	t.pos = offset
	return t.pos, nil
}
