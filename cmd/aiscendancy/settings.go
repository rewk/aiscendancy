package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"io/fs"
	"math"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/rewk/aiscendancy/ui"
)

// The settings panel (F2, the opening's Settings, the quit prompt's
// button): the config file's sound keys, modal over any screen like the
// key help. Every change applies at once (the music's volume to the
// playing tune, an effect played as the effects' slider is let go);
// Done, Esc, Enter, F2 or the back button keep them and write them into
// the config file (writeConfig), Cancel restores the panel's opening
// values. game and save are shown, edited in the file only.

// settings are the panel's values, the flags' of the same names.
type settings struct {
	music      bool
	volume     float64 // the music's, 0..1
	sfx        float64 // the effects', 0..1
	gap        time.Duration
	game, save string // shown only
}

// The steps: the sliders snap to volStep, the gap goes by gapStep up to
// gapMax.
const (
	volStep = 0.05
	gapStep = 30 * time.Second
	gapMax  = 10 * time.Minute
)

// settingsPanel is the panel up: was the values when it opened (Cancel),
// drag the slider held (setVolume, setSfx), setNone none.
type settingsPanel struct {
	was  settings
	drag int
}

// The panel's rows, top to bottom.
const (
	setMusic = iota
	setVolume
	setSfx
	setGap
	setGame
	setSave
	setConfig
	numSetRows
	setNone = -1
)

var setLabels = [numSetRows]string{"Music", "Music volume", "Effects volume", "Silence between tunes", "Game", "Save", "Settings file"}

// The panel's layout, native px: setBox centred, a row every setRowH from
// setTop, the labels at setLabelX, the controls setCtrlW wide at
// setCtrlX, the value beside; Cancel and Done at the bottom.
const (
	setW      = 760
	setH      = 520
	setTop    = 80
	setRowH   = 52
	setLabelX = 32
	setCtrlX  = 330
	setCtrlW  = 300
	setBtnW   = 140
	setBtnH   = 44
)

var (
	setBox    = rect{(screenW - setW) / 2, (screenH - setH) / 2, (screenW + setW) / 2, (screenH + setH) / 2}
	setCancel = rect{setBox.X0 + setW/2 - 16 - setBtnW, setBox.Y1 - 28 - setBtnH, setBox.X0 + setW/2 - 16, setBox.Y1 - 28}
	setDone   = rect{setBox.X0 + setW/2 + 16, setCancel.Y0, setBox.X0 + setW/2 + 16 + setBtnW, setCancel.Y1}
)

// setCtrl is row k's control: a button, a slider's track, the gap's
// value between its − and + (setGapButtons).
func setCtrl(k int) rect {
	y := setBox.Y0 + setTop + k*setRowH
	return rect{setBox.X0 + setCtrlX, y, setBox.X0 + setCtrlX + setCtrlW, y + setRowH - 14}
}

// setGapButtons are the gap row's − and +.
func setGapButtons() (minus, plus rect) {
	r := setCtrl(setGap)
	h := r.Y1 - r.Y0
	return rect{r.X0, r.Y0, r.X0 + h, r.Y1}, rect{r.X1 - h, r.Y0, r.X1, r.Y1}
}

// openSettings puts the panel up over whatever is up.
func (a *app) openSettings() {
	a.setUp = &settingsPanel{was: a.set, drag: setNone}
}

// applySettings makes s the window's: the music on or off, its volume
// and gap, the effects' volume.
func (a *app) applySettings(s settings) {
	a.set = s
	a.music.setOn(s.music)
	a.music.setVolume(s.volume)
	a.music.setGap(s.gap)
	if a.sfx != nil {
		a.sfx.volume = s.sfx
	}
}

// closeSettings takes the panel down: kept, the values go to the config
// file (none in tests: configPath ""), a failure toasted; else the
// opening values come back.
func (a *app) closeSettings(keep bool) {
	p := a.setUp
	a.setUp = nil
	if !keep {
		a.applySettings(p.was)
		return
	}
	if a.configPath == "" || a.set == p.was {
		return
	}
	if err := writeConfig(a.configPath, map[string]any{
		"music": a.set.music, "volume": a.set.volume, "sfx": a.set.sfx, "music-gap": a.set.gap.String(),
	}); err != nil {
		a.toastFor(fmt.Sprintf("settings: %v", err), true)
		return
	}
	a.toastFor("settings saved in "+a.configPath, false)
}

// settingsUpdate is the panel's Update, with Ebiten's input.
func (a *app) settingsUpdate() {
	x, y := a.mouse()
	a.settingsInput(keyJustPressed(ebiten.KeyF2),
		keyJustPressed(ebiten.KeyEscape) || keyJustPressed(ebiten.KeyEnter) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButton3),
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft), ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft), x, y)
}

// settingsInput is the panel's input, without Ebiten: f2 opens it; up,
// it takes the tick (true): f2 or done closes it kept, a press acts on
// what it lands on, held drags the slider pressed.
func (a *app) settingsInput(f2, done, press, held bool, x, y float64) bool {
	p := a.setUp
	if p == nil {
		if f2 {
			a.openSettings()
		}
		return f2
	}
	if f2 || done {
		a.closeSettings(true)
		return true
	}
	s := a.set
	minus, plus := setGapButtons()
	switch {
	case press && setDone.has(x, y):
		a.closeSettings(true)
		return true
	case press && setCancel.has(x, y):
		a.closeSettings(false)
		return true
	case press && setCtrl(setMusic).has(x, y):
		s.music = !s.music
	case press && minus.has(x, y):
		s.gap = max(s.gap-gapStep, 0)
	case press && plus.has(x, y):
		s.gap = min(s.gap+gapStep, gapMax)
	case press && setCtrl(setVolume).has(x, y):
		p.drag = setVolume
	case press && setCtrl(setSfx).has(x, y):
		p.drag = setSfx
	}
	if p.drag != setNone && !held {
		if p.drag == setSfx {
			a.sfx.play(laneSound)
		}
		p.drag = setNone
	}
	switch p.drag {
	case setVolume:
		s.volume = sliderAt(setCtrl(setVolume), x)
	case setSfx:
		s.sfx = sliderAt(setCtrl(setSfx), x)
	}
	if s != a.set {
		a.applySettings(s)
	}
	return true
}

// sliderAt is track r's value at x, snapped to volStep.
func sliderAt(r rect, x float64) float64 {
	f := min(max((x-float64(r.X0))/float64(r.X1-r.X0), 0), 1)
	return math.Round(f/volStep) * volStep
}

// drawSettings paints the panel over a dimmed frame.
func (a *app) drawSettings(frame *ebiten.Image) {
	vector.FillRect(frame, 0, 0, screenW, screenH, color.RGBA{0, 0, 0, 160}, false)
	fillRectWith(frame, setBox, dlgFill)
	b := setBox
	vector.StrokeRect(frame, float32(b.X0), float32(b.Y0), float32(b.X1-b.X0), float32(b.Y1-b.Y0), zoom, btnLight, false)
	mx, my := a.mouse()
	title := "Settings"
	ui.Print(frame, title, bigFace, float64(b.X0+b.X1)/2-ui.Width(title, bigFace)/2, float64(b.Y0+28), footColour)
	s := a.set
	mid := func(r rect) float64 { return float64(r.Y0+r.Y1)/2 - bigFace.Size*1.25/2 }
	for k, l := range setLabels {
		r := setCtrl(k)
		ui.Print(frame, l, bigFace, float64(b.X0+setLabelX), mid(r), textColour)
		value := ""
		switch k {
		case setMusic:
			on := "Off"
			if s.music {
				on = "On"
			}
			drawButton(frame, r, on, r.has(mx, my))
		case setVolume, setSfx:
			v := s.volume
			if k == setSfx {
				v = s.sfx
			}
			drawSlider(frame, r, v)
			value = fmt.Sprintf("%3.0f%%", v*100)
		case setGap:
			minus, plus := setGapButtons()
			drawButton(frame, minus, "-", minus.has(mx, my))
			drawButton(frame, plus, "+", plus.has(mx, my))
			g := fmtGap(s.gap)
			ui.Print(frame, g, bigFace, float64(r.X0+r.X1)/2-ui.Width(g, bigFace)/2, mid(r), textColour)
		default:
			path := map[int]string{setGame: s.game, setSave: s.save, setConfig: a.configPath}[k]
			path = fitWidth(path, float64(b.X1-r.X0-setLabelX), smallFace)
			ui.Print(frame, path, smallFace, float64(r.X0), float64(r.Y0+r.Y1)/2-smallFace.Size*1.25/2, dim(textColour, 0.6))
		}
		if value != "" {
			ui.Print(frame, value, bigFace, float64(r.X1+16), mid(r), textColour)
		}
	}
	drawButton(frame, setCancel, "Cancel", setCancel.has(mx, my))
	drawButton(frame, setDone, "Done", setDone.has(mx, my))
}

// drawSlider paints track r filled to v, a handle at v.
func drawSlider(frame *ebiten.Image, r rect, v float64) {
	x, y, w, h := float32(r.X0), float32(r.Y0), float32(r.X1-r.X0), float32(r.Y1-r.Y0)
	vector.FillRect(frame, x, y+h/2-4, w, 8, btnDark, false)
	vector.FillRect(frame, x, y+h/2-4, w*float32(v), 8, btnLight, false)
	hx := x + w*float32(v)
	vector.FillRect(frame, hx-5, y+4, 10, h-8, textColour, false)
}

// fmtGap is d as "2m30s", "0s" none.
func fmtGap(d time.Duration) string {
	m, s := int(d/time.Minute), int(d%time.Minute/time.Second)
	switch {
	case m == 0:
		return fmt.Sprintf("%ds", s)
	case s == 0:
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dm%02ds", m, s)
}

// writeConfig sets keys in the config file at path, keeping its other
// keys; absent, it is made. The object is written sorted and indented,
// through a temporary file renamed over it; a file that does not parse
// is left alone.
func writeConfig(path string, keys map[string]any) error {
	m := map[string]json.RawMessage{}
	b, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return err
	default:
		if err := json.Unmarshal(b, &m); err != nil {
			return fmt.Errorf("%s: %v", path, err)
		}
	}
	for k, v := range keys {
		raw, err := json.Marshal(v)
		if err != nil {
			return err
		}
		m[k] = raw
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(out, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
