package sound

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// voc builds a VOC of one type 9 block (rate, bits, channels, codec 0)
// holding pcm, then a type 2 continuation holding more.
func voc(rate uint32, bits, ch byte, pcm, more []byte) []byte {
	b := append([]byte(vocMagic), 26, 0, 0x14, 0x01, 0x1f, 0x11)
	n := 12 + len(pcm)
	b = append(b, 9, byte(n), byte(n>>8), byte(n>>16))
	b = binary.LittleEndian.AppendUint32(b, rate)
	b = append(b, bits, ch, 0, 0, 0, 0, 0, 0)
	b = append(b, pcm...)
	if more != nil {
		b = append(b, 2, byte(len(more)), 0, 0)
		b = append(b, more...)
	}
	return append(b, 0)
}

func TestDecodeVOC(t *testing.T) {
	pcm, err := DecodeVOC(voc(SampleRate, 8, 1, []byte{1, 2, 3}, []byte{4, 5}))
	if err != nil || !bytes.Equal(pcm, []byte{1, 2, 3, 4, 5}) {
		t.Fatalf("pcm %v, err %v", pcm, err)
	}
	for _, b := range [][]byte{
		voc(11025, 8, 1, []byte{1}, nil),
		voc(SampleRate, 8, 2, []byte{1, 2}, nil),
		voc(SampleRate, 16, 1, []byte{1, 2}, nil),
		[]byte("Creative Voice File\x1a"),
		voc(SampleRate, 8, 1, []byte{1, 2, 3}, nil)[:40],
	} {
		if _, err := DecodeVOC(b); err == nil {
			t.Errorf("%d bytes decoded", len(b))
		}
	}
}

// The three effects played (blueexit headerless, trust01/02 VOC) open and
// last about as measured: blueexit 1.53 s, trust01 1.94 s, trust02 1.36 s.
func TestEffects(t *testing.T) {
	l := openTestLibrary(t)
	for name, ms := range map[string]int{"blueexit": 1530, "trust01": 1940, "trust02": 1360} {
		tr, err := Effect(l.g, name)
		if err != nil {
			t.Fatal(err)
		}
		if d := int(tr.Duration().Milliseconds()); d < ms-10 || d > ms+10 {
			t.Errorf("%s lasts %d ms, want ~%d", name, d, ms)
		}
	}
	if _, err := Effect(l.g, "nosuch"); err == nil {
		t.Error("nosuch opened")
	}
}
