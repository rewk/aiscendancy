package main

import (
	"fmt"
	"image/color"
	"log"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/rewk/aiscendancy/flic"
	"github.com/rewk/aiscendancy/sound"
)

// The introduction, the opening's Play Introduction: data/intro65a.flc
// (640x480, 1143 frames, package flic) at zoom, timed and scored by
// subtitle.txt. That file is "KEY value" lines in blank-line blocks; a
// block with FRAMENUMBER n acts at frame n (1-based): FRAME_RATE
// the frames per second from there, TUNEID the music.txt tune (by its
// index) started as the jukebox's stinger; a SUBTITLE block's TEXT
// (underscores for spaces) shows from its FRAME_START to the next one's.
// Left out: FADEMUSIC (intro00's fade before intro01 cuts it), DELAY,
// the subtitle font (INTRO.FNT: ours is Go Mono, white, centred in
// TITLERECT). Any click or key but F3 and F12, or the movie's end, goes
// back to the opening; the jukebox is held meanwhile, a theme after.
const (
	movieFile    = "data/intro65a.flc"
	movieScriptF = "subtitle.txt"
	movieFPS     = 12 // until the script's first FRAME_RATE
)

// movieSubRect is TITLERECT, where the subtitles go.
var movieSubRect = orig(0, 379, 640, 420)

// movieCue is a value acting from frame on.
type movieCue struct{ frame, v int }

// movieSub is a subtitle shown from frame start.
type movieSub struct {
	start int
	text  string
}

// movieScript is subtitle.txt's timing, tunes and subtitles, each in
// frame order.
type movieScript struct {
	rates, tunes []movieCue
	subs         []movieSub
}

// parseMovieScript reads subtitle.txt, block by block: a block's keys
// come in any order (FRAMENUMBER before FRAME_RATE).
func parseMovieScript(s string) (movieScript, error) {
	var ms movieScript
	blk := map[string]string{}
	flush := func() error {
		defer clear(blk)
		num := func(k string) (int, bool) {
			v, err := strconv.Atoi(blk[k])
			return v, err == nil
		}
		at, ok := num("FRAMENUMBER")
		switch _, sub := blk["SUBTITLE"]; {
		case sub:
			start, ok := num("FRAME_START")
			if !ok || blk["TEXT"] == "" {
				return fmt.Errorf("%s: subtitle %v", movieScriptF, blk)
			}
			ms.subs = append(ms.subs, movieSub{start, strings.ReplaceAll(blk["TEXT"], "_", " ")})
		case !ok:
		default:
			if v, ok := num("FRAME_RATE"); ok && v > 0 {
				ms.rates = append(ms.rates, movieCue{at, v})
			}
			if v, ok := num("TUNEID"); ok {
				ms.tunes = append(ms.tunes, movieCue{at, v})
			}
		}
		return nil
	}
	for l := range strings.SplitSeq(s, "\n") {
		f := strings.Fields(strings.Trim(l, "\r\x1a"))
		if len(f) == 0 || f[0] == "END" {
			if err := flush(); err != nil {
				return ms, err
			}
			if len(f) > 0 {
				break
			}
			continue
		}
		blk[f[0]] = strings.Join(f[1:], " ")
	}
	return ms, flush()
}

// subAt is the subtitle shown at frame n, "" none.
func (ms movieScript) subAt(n int) string {
	t := ""
	for _, s := range ms.subs {
		if s.start <= n {
			t = s.text
		}
	}
	return t
}

// movieScreen plays the introduction; m nil (no assets, a read failure)
// leaves at the first update.
type movieScreen struct {
	m      *flic.Movie
	script movieScript
	tunes  []string // music.txt's names
	fps    int
	acc    float64 // frames due, in fractions
	img    *ebiten.Image
	rgba   []byte
	done   bool
	esc    bool // Esc came (cancel): leave at the next update
}

func newMovieScreen() *movieScreen { return &movieScreen{fps: movieFPS} }

// Enter reads the movie, the script and the tune names; holds the music.
func (s *movieScreen) Enter(a *app) {
	if a.a == nil {
		return
	}
	if err := s.load(a); err != nil {
		log.Printf("introduction: %v", err)
		s.m = nil
		return
	}
	a.music.hold(true)
}

func (s *movieScreen) load(a *app) error {
	g := a.a.Game
	b, err := g.Read(movieScriptF)
	if err != nil {
		return err
	}
	if s.script, err = parseMovieScript(string(b)); err != nil {
		return err
	}
	lib, err := sound.OpenLibrary(g)
	if err != nil {
		return err
	}
	s.tunes = lib.Names()
	if b, err = g.Read(movieFile); err != nil {
		return err
	}
	s.m, err = flic.Decode(b)
	return err
}

// leave goes back to the opening, a theme playing.
func (s *movieScreen) leave(a *app) {
	if !s.done {
		s.done = true
		a.music.hold(false)
		a.pop()
	}
}

// cancel is Esc's (app.back), which has no app to leave with: the next
// update leaves.
func (s *movieScreen) cancel() bool {
	s.esc = true
	return true
}

func (s *movieScreen) Update(a *app) error {
	if s.m == nil || s.esc {
		s.leave(a)
		return nil
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		s.leave(a)
		return nil
	}
	if movieLeaves(inpututil.AppendJustPressedKeys(nil)) {
		s.leave(a)
		return nil
	}
	s.acc += float64(s.fps) / float64(ebiten.TPS())
	drawn := false
	for ; s.acc >= 1; s.acc-- {
		if s.m.Next() != nil { // the end, or a broken frame
			s.leave(a)
			return nil
		}
		n := s.m.Frame()
		for _, c := range s.script.rates {
			if c.frame == n {
				s.fps = c.v
			}
		}
		for _, c := range s.script.tunes {
			if c.frame == n && c.v < len(s.tunes) {
				a.music.tune(s.tunes[c.v])
			}
		}
		drawn = true
	}
	if drawn {
		s.paint()
	}
	return nil
}

// movieLeaves reports a key among keys ends the movie: any but the
// app-level F3 (the music) and F12 (a shot of the frame, taken at the
// next Draw: leaving would snap the opening).
func movieLeaves(keys []ebiten.Key) bool {
	for _, k := range keys {
		if k != ebiten.KeyF3 && k != ebiten.KeyF12 {
			return true
		}
	}
	return false
}

// paint turns the canvas into img through the palette.
func (s *movieScreen) paint() {
	if s.img == nil {
		s.img = ebiten.NewImage(s.m.W, s.m.H)
		s.rgba = make([]byte, 4*s.m.W*s.m.H)
	}
	for i, ix := range s.m.Pix {
		c := s.m.Palette[ix].(color.RGBA)
		s.rgba[4*i], s.rgba[4*i+1], s.rgba[4*i+2], s.rgba[4*i+3] = c.R, c.G, c.B, 255
	}
	s.img.WritePixels(s.rgba)
}

func (s *movieScreen) Render(a *app, sc *ebiten.Image) {
	sc.Fill(color.Black)
	if s.img == nil {
		return
	}
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(zoom, zoom)
	sc.DrawImage(s.img, &op)
}

func (s *movieScreen) Text(a *app, frame *ebiten.Image) {
	if s.m == nil {
		return
	}
	if t := s.script.subAt(s.m.Frame()); t != "" {
		r := movieSubRect
		centred(frame, t, bigFace, r, float64(r.Y0+r.Y1)/2-bigFace.Size/2, color.White)
	}
}
