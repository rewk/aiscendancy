package main

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The quit prompt (Esc on the last screen): a prompt plus quick save
// (F5), quick load (F9), Save..., Load... (the slots screen), Screenshot
// (F12) and Settings (F2, settings.go) buttons, so the game plays with the mouse alone.

// quitPrompt is the question; quitSave, quitLoad its buttons' first row,
// quitSaveAs, quitLoadAs the second, quitShot and quitSet the third, between the
// title and the answers.
var (
	quitPrompt               = prompt{[]string{"Quit AIscendancy?"}, "Yes (Enter)", "No (Esc)"}
	quitBox, quitYes, quitNo = quitPrompt.rects()
	quitSave, quitLoad       = quitButtons(0)
	quitSaveAs, quitLoadAs   = quitButtons(1)
	quitShot, quitSet        = quitButtons(2)
)

// quitButtons lays row k of the buttons out: discOKW x discOKH, 2 x
// discGap apart, the three rows' block centred halfway between the title
// and the answers.
func quitButtons(k int) (left, right rect) {
	cx := (quitBox.X0 + quitBox.X1) / 2
	cy := (quitBox.Y0 + dlgTitleY + zoom*promptLineH + dlgBar.Y0) / 2
	y0 := cy - (3*discOKH+2*2*discGap)/2 + k*(discOKH+2*discGap)
	y1 := y0 + discOKH
	return rect{cx - discGap - discOKW, y0, cx - discGap, y1}, rect{cx + discGap, y0, cx + discGap + discOKW, y1}
}

// back is Esc or the back button: the top screen's mode is cancelled,
// else it pops; on the last screen the quit prompt comes up. A system screen leaving with a climb due
// (navDue) climbs from the screen below, else the climb would never run.
func (a *app) back() error {
	if c, ok := a.top().(interface{ cancel() bool }); ok && c.cancel() {
		return nil
	}
	if len(a.stack) <= 1 {
		a.quitting = true
		return nil
	}
	v, ok := a.top().(*systemScreen)
	var due []game.Event
	if ok && v.replay != nil {
		due = v.replay.navDue
	}
	if err := a.pop(); err != nil || len(due) == 0 {
		return err
	}
	a.navigate(due)
	return nil
}

// answerQuit closes the prompt: quit (ebiten.Termination) or stay.
func (a *app) answerQuit(yes bool) error {
	a.quitting = false
	if yes {
		return ebiten.Termination
	}
	return nil
}

// quitUpdate reads the prompt's keys and clicks while it is up.
func (a *app) quitUpdate() error {
	click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	x, y := a.mouse()
	if a.quitSaveLoad(inpututil.IsKeyJustPressed(ebiten.KeyF5) || click && quitSave.has(x, y),
		inpututil.IsKeyJustPressed(ebiten.KeyF9) || click && quitLoad.has(x, y)) {
		return nil
	}
	if click && a.openSlots(quitSaveAs.has(x, y), quitLoadAs.has(x, y)) {
		return nil
	}
	if click && quitShot.has(x, y) { // F12 is Update's, everywhere
		a.snapDue = true
		return nil
	}
	if click && quitSet.has(x, y) {
		a.quitting = false
		a.openSettings()
		return nil
	}
	if ok, yes := quitPrompt.update(a); ok {
		return a.answerQuit(yes)
	}
	return nil
}

// quitSaveLoad is the quick save (the prompt stays) or load (it closes
// once loaded), without Ebiten; true when one was pressed.
func (a *app) quitSaveLoad(save, load bool) bool {
	switch {
	case save:
		if a.inGame() {
			a.save()
		}
	case load:
		if a.load() {
			a.quitting = false
		}
	default:
		return false
	}
	return true
}

// openSlots closes the prompt over the slots screen, Save (in a game) or
// Load; true when one was pressed.
func (a *app) openSlots(saving, loading bool) bool {
	switch {
	case saving && a.inGame(), loading:
		a.quitting = false
		a.push(newSlotsScreen(saving))
	case !saving:
		return false
	}
	return true
}

// drawQuit paints the prompt and its buttons.
func (a *app) drawQuit(frame *ebiten.Image) {
	quitPrompt.draw(a, frame)
	mx, my := a.mouse()
	if a.inGame() {
		drawButton(frame, quitSave, "Quick save (F5)", quitSave.has(mx, my))
		drawButton(frame, quitSaveAs, "Save…", quitSaveAs.has(mx, my))
	} else {
		drawButtonOff(frame, quitSave, "Quick save (F5)")
		drawButtonOff(frame, quitSaveAs, "Save…")
	}
	drawButton(frame, quitLoad, "Quick load (F9)", quitLoad.has(mx, my))
	drawButton(frame, quitLoadAs, "Load…", quitLoadAs.has(mx, my))
	drawButton(frame, quitShot, "Screenshot (F12)", quitShot.has(mx, my))
	drawButton(frame, quitSet, "Settings (F2)", quitSet.has(mx, my))
}

// prompt is a yes/no question drawn as the structure dialog: help.shp's
// box at dlgBox, a line every promptLineH original px, the answers the
// button bar's halves (no "": yes alone, the whole bar). Enter/Y yes,
// Esc/N/back button no. Its owner routes Update to it: the screen under
// it gets no input.
type prompt struct {
	lines   []string
	yes, no string
}

const promptLineH = 20

// rects are the box and its two buttons, native px.
func (p prompt) rects() (box, yes, no rect) {
	if p.no == "" {
		return dlgBox, dlgBar, rect{}
	}
	mid := (dlgBar.X0 + dlgBar.X1) / 2
	return dlgBox, rect{dlgBar.X0, dlgBar.Y0, mid, dlgBar.Y1}, rect{mid, dlgBar.Y0, dlgBar.X1, dlgBar.Y1}
}

// update reads the keys and clicks: answered, and the answer.
func (p prompt) update(a *app) (answered, yes bool) {
	switch {
	case keyJustPressed(ebiten.KeyEnter), keyJustPressed(ebiten.KeyY):
		return true, true
	case keyJustPressed(ebiten.KeyEscape), keyJustPressed(ebiten.KeyN),
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButton3):
		return true, false
	case inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft):
		x, y := a.mouse()
		_, by, bn := p.rects()
		switch {
		case by.has(x, y):
			return true, true
		case bn.has(x, y):
			return true, false
		}
	}
	return false, false
}

// lineAt is the line under (x, y), −1 none: its row promptLineH tall
// from where draw prints it, across the box, cut at the button bar (no
// row overlaps an answer). An owner with clickable lines reads it before
// update.
func (p prompt) lineAt(x, y float64) int {
	b, _, _ := p.rects()
	for k := range p.lines {
		y0 := b.Y0 + dlgTitleY + zoom*promptLineH*k
		r := rect{b.X0, y0, b.X1, min(y0+zoom*promptLineH, dlgBar.Y0)}
		if r.Y0 < r.Y1 && r.has(x, y) {
			return k
		}
	}
	return -1
}

// draw paints the prompt: the lines centred, the hovered answer yellow.
func (p prompt) draw(a *app, frame *ebiten.Image) { p.drawHot(a, frame, -1) }

// drawHot is draw with line hot yellow (a clickable line hovered), −1
// none.
func (p prompt) drawHot(a *app, frame *ebiten.Image, hot int) {
	mx, my := a.mouse()
	b, by, bn := p.rects()
	fillRectWith(frame, b, dlgFill)
	drawSprite(frame, a.spr(dlgBank, 0), float64(b.X0), float64(b.Y0))
	centre := func(r rect, s string, y float64, c color.RGBA) {
		s = fitWidth(s, float64(r.X1-r.X0-4*padX), bigFace)
		ui.Print(frame, s, bigFace, float64(r.X0+r.X1)/2-ui.Width(s, bigFace)/2, y, c)
	}
	for k, l := range p.lines {
		c := textColour
		if k == hot {
			c = yellow
		}
		centre(b, l, float64(b.Y0+dlgTitleY+zoom*promptLineH*k), c)
	}
	for _, btn := range []struct {
		r   rect
		txt string
	}{{by, p.yes}, {bn, p.no}} {
		if btn.txt == "" {
			continue
		}
		c := textColour
		if btn.r.has(mx, my) {
			c = yellow
		}
		centre(btn.r, btn.txt, float64(btn.r.Y0+btn.r.Y1)/2-bigFace.Size*0.6, c)
	}
}
