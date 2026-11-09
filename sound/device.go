package sound

import (
	"github.com/ebitengine/oto/v3"
)

// Device is the sound card at SampleRate, s16le stereo. oto opens it in
// the background (ALSA on Linux): a failure shows up later through Err.
// One per process (oto refuses a second).
type Device struct {
	ctx    *oto.Context
	volume float64
}

// Open starts opening the device; volume (0..1) is every player's.
func Open(volume float64) (*Device, error) {
	ctx, _, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   SampleRate,
		ChannelCount: 2,
		Format:       oto.FormatSignedInt16LE,
	})
	if err != nil {
		return nil, err
	}
	return &Device{ctx: ctx, volume: volume}, nil
}

// Err is the device's failure, if any (no device, no ALSA).
func (d *Device) Err() error { return d.ctx.Err() }

// SetVolume is the volume of the players started from now (Play).
func (d *Device) SetVolume(v float64) { d.volume = v }

// Play starts t on a new player. A player stops by itself at t's end
// (IsPlaying turns false); Pause keeps its place for Play.
func (d *Device) Play(t *Track) *oto.Player { return d.PlayAt(t, d.volume) }

// PlayAt is Play at volume (0..1) instead of the device's.
func (d *Device) PlayAt(t *Track, volume float64) *oto.Player {
	p := d.ctx.NewPlayer(t)
	p.SetVolume(volume)
	p.Play()
	return p
}
