package main

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The log: the app's whole log, newest first, paler with age, a popup
// box over the dimmed screen under it in the stack (still a screen of
// the stack: Esc pops it, push keeps one). A linked line (eventScreens)
// closes it and opens its place. Row r shows the entry off+r from the
// newest: no index is kept, an append under it shifts nothing.
type logScreen struct {
	off   int // rows scrolled: the top row shows the off-th newest entry
	hover int // row under the cursor, −1 none
}

func newLogScreen() *logScreen { return &logScreen{hover: -1} }

func (s *logScreen) Enter(*app) {}

const logHelp = "wheel/arrows: scroll  click a line: go there  S: species  Esc or a click outside: close"

// logLineH is a bigFace line's height, native px (ui.Print's 1.25).
var logLineH = bigFace.Size * 1.25

// visibleLines is how many bigFace lines fit frameH, padY at both ends.
func visibleLines(frameH float64) int { return linesFit(frameH, bigFace) }

// logTop is the native y of the big area's row 0.
func logTop() float64 { return float64(mapArea.Y0 + padY) }

// logRowsN is the popup's lines; logW its width, native px.
const (
	logRowsN = 15
	logW     = 480 * zoom
)

func logRows() int { return logRowsN }

// logBox is the popup's box, centred: title, rows, help, padY around
// each.
func logBox() rect {
	h := int(2*padY + logLineH + padY + float64(logRowsN)*logLineH + padY + smallFace.Size*1.25 + padY)
	x0, y0 := (screenW-logW)/2, (screenH-h)/2
	return rect{x0, y0, x0 + logW, y0 + h}
}

// logRowsTop is row 0's native y, logRowsBox the rows' rect.
func logRowsTop() float64 { return float64(logBox().Y0+2*padY) + logLineH + padY }

func logRowsBox() rect {
	b := logBox()
	return rect{b.X0, int(logRowsTop()), b.X1, int(logRowsTop() + float64(logRowsN)*logLineH)}
}

// row is the row under (x, y), −1 outside the rows.
func (s *logScreen) row(x, y float64) int {
	if !logRowsBox().has(x, y) {
		return -1
	}
	if k := int((y - logRowsTop()) / logLineH); k < logRowsN {
		return k
	}
	return -1
}

// entry is the log index row r shows, −1 when none (past the oldest).
func (s *logScreen) entry(a *app, r int) int {
	if r < 0 {
		return -1
	}
	if i := len(a.log) - 1 - (s.off + r); i >= 0 {
		return i
	}
	return -1
}

func (s *logScreen) lineAt(a *app, x, y float64) int { return s.entry(a, s.row(x, y)) }

// scroll moves by d rows (positive: older), clamped.
func (s *logScreen) scroll(a *app, d int) {
	s.off = min(max(s.off+d, 0), max(0, len(a.log)-logRows()))
}

// linked reports a click on ev's line opens something.
func linked(ev game.Event) bool {
	return ev.Star >= 0 || ev.Kind == game.Discovered || ev.Other >= 0
}

// ageColour is a line's colour by its age: today's full, 1 to 5 days
// old at 70%, older at 45%.
func ageColour(day, today int) color.RGBA { return ageDim(textColour, day, today) }

// ageDim is c dimmed by a line's age as ageColour.
func ageDim(c color.RGBA, day, today int) color.RGBA {
	switch age := today - day; {
	case age <= 0:
		return c
	case age <= 5:
		return dim(c, 0.7)
	}
	return dim(c, 0.45)
}

// shipNews is a ship event's log colour, light blue; speciesNews
// another species' (Other set), green.
var (
	shipNews    = color.RGBA{130, 200, 255, 255}
	speciesNews = color.RGBA{110, 220, 110, 255}
)

// lineColour is entry l's colour on day today, dimmed by age: red for one
// that left its colony idle (a ship's launch too), else light blue for
// a ship's (shipKind), else green for one naming another species (Other:
// contact, diplomacy, their ability, their defeat), else textColour.
func lineColour(l logEntry, today int) color.RGBA {
	switch {
	case l.Idle:
		return ageDim(red, l.Day, today)
	case shipKind(l.Ev):
		return ageDim(shipNews, l.Day, today)
	case l.Ev.Other >= 0:
		return ageDim(speciesNews, l.Day, today)
	}
	return ageColour(l.Day, today)
}

func (s *logScreen) open(a *app, i int) []screen { return eventScreens(a, a.log[i].Ev) }

func (s *logScreen) Update(a *app) error {
	if _, wy := ebiten.Wheel(); wy != 0 {
		s.scroll(a, notch(wy, 3)) // up: toward the newest
	}
	s.off = keyScroll(s.off, logRows())
	s.scroll(a, 0) // the log may have been capped since
	if onSpeciesKey(a, inpututil.IsKeyJustPressed(ebiten.KeyS)) {
		return nil
	}
	x, y := a.mouse()
	s.hover = s.row(x, y)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return s.click(a, x, y)
	}
	return nil
}

// click is a left press, without Ebiten: outside the box the log
// closes; on a linked line it closes and the line's screens open.
func (s *logScreen) click(a *app, x, y float64) error {
	if !logBox().has(x, y) {
		return a.pop()
	}
	i := s.lineAt(a, x, y)
	if i < 0 {
		return nil
	}
	scs := s.open(a, i)
	if len(scs) == 0 {
		return nil
	}
	if err := a.pop(); err != nil {
		return err
	}
	for _, sc := range scs {
		a.push(sc)
	}
	return nil
}

// below is the screen under s in the stack, nil when none.
func (s *logScreen) below(a *app) screen {
	for i := len(a.stack) - 1; i > 0; i-- {
		if a.stack[i] == screen(s) {
			return a.stack[i-1]
		}
	}
	return nil
}

// Render paints the screen under the log (the popup is drawn in Text).
func (s *logScreen) Render(a *app, sc *ebiten.Image) {
	if b := s.below(a); b != nil {
		b.Render(a, sc)
	}
}

// Text writes the screen under it, dims it, then paints the box.
func (s *logScreen) Text(a *app, frame *ebiten.Image) {
	if b := s.below(a); b != nil {
		b.Text(a, frame)
	}
	vector.FillRect(frame, 0, 0, screenW, screenH, color.RGBA{0, 0, 0, 150}, false)
	box := logBox()
	fillRectWith(frame, box, dlgFill)
	strokeRect(frame, box, btnLight)
	x := float64(box.X0 + padX)
	w := float64(box.X1 - box.X0 - 2*padX)
	ui.Print(frame, fitWidth(fmt.Sprintf("Log, day %d: %d events", a.e.Day, len(a.log)), w, bigFace), bigFace, x, float64(box.Y0+2*padY), titleColour)
	for r := range logRows() {
		i := s.entry(a, r)
		if i < 0 {
			break
		}
		l := a.log[i]
		c := lineColour(l, a.e.Day)
		if r == s.hover && linked(l.Ev) {
			c = yellow
		}
		str := fitWidth(fmt.Sprintf("day %5d  %s", l.Day, l.Ev.Text), w, bigFace)
		ui.Print(frame, str, bigFace, x, logRowsTop()+float64(r)*logLineH, c)
	}
	drawScrollCues(a, frame, logRowsBox(), s.off, logRows(), len(a.log))
	hy := float64(box.Y1-padY) - smallFace.Size*1.25
	ui.Print(frame, fitWidth(logHelp, w, smallFace), smallFace, x, hy, dim(textColour, 0.7))
}
