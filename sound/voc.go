package sound

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/rewk/aiscendancy/cob"
)

// The sound effects (soundfx.txt's list, ascend01.cob's data/*.voc) are
// Creative Voice Files, the Sound Blaster's: a 26-byte header ("Creative
// Voice File", 0x1A, the first block's offset at 20), then blocks of a
// type byte and a 24-bit little-endian length, ended by type 0. The ones
// played here hold one type-9 block: rate u32, bits, channels, codec u16,
// 4 reserved, then the samples, 8-bit unsigned mono at SampleRate.
// Some (blueexit, the gun sounds gbNN, ghNN...) carry no header: raw
// 8-bit unsigned mono like the music's .raw, their rate assumed the same
// (OUR ASSUMPTION; the lengths come out at about a second).

// DecodeVOC is a VOC's samples as raw bytes for NewTrack: type 9 blocks
// (and type 2 continuations) at SampleRate, 8-bit unsigned mono PCM;
// anything else is refused.
func DecodeVOC(b []byte) ([]byte, error) {
	if len(b) < 26 || !isVOC(b) {
		return nil, errors.New("voc: no header")
	}
	var pcm []byte
	for p := int(binary.LittleEndian.Uint16(b[20:])); p < len(b) && b[p] != 0; {
		if p+4 > len(b) {
			return nil, errors.New("voc: truncated block")
		}
		t, n := b[p], int(b[p+1])|int(b[p+2])<<8|int(b[p+3])<<16
		if p+4+n > len(b) {
			return nil, errors.New("voc: truncated block")
		}
		d := b[p+4 : p+4+n]
		switch t {
		case 9:
			if n < 12 {
				return nil, errors.New("voc: short type 9 block")
			}
			rate, bits, ch, codec := binary.LittleEndian.Uint32(d), d[4], d[5], binary.LittleEndian.Uint16(d[6:])
			if rate != SampleRate || bits != 8 || ch != 1 || codec != 0 {
				return nil, fmt.Errorf("voc: %d Hz, %d bits, %d channels, codec %d", rate, bits, ch, codec)
			}
			pcm = append(pcm, d[12:]...)
		case 2:
			pcm = append(pcm, d...)
		default:
			return nil, fmt.Errorf("voc: block type %d", t)
		}
		p += 4 + n
	}
	if len(pcm) == 0 {
		return nil, errors.New("voc: no samples")
	}
	return pcm, nil
}

// Effect is sound effect name ("blueexit": data/blueexit.voc) from g, a
// file without the VOC header taken as raw samples.
func Effect(g *cob.Game, name string) (*Track, error) {
	b, err := g.Read(cob.Normalise(`data\` + name + ".voc"))
	if err != nil {
		return nil, err
	}
	if !isVOC(b) {
		return NewTrack(b), nil
	}
	pcm, err := DecodeVOC(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return NewTrack(pcm), nil
}

const vocMagic = "Creative Voice File\x1a"

func isVOC(b []byte) bool { return len(b) >= len(vocMagic) && string(b[:len(vocMagic)]) == vocMagic }
