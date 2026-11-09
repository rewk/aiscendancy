package main

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// The day's end warning (docs/screens.md "Combat mode"): Space, the
// mouse's forward button and the starmap's turn buttons ask askDay,
// which, when ships of the player's with usable energy stand in a fight
// (restless), puts an app-level prompt up instead of the day: "End the
// day" runs it, "Cancel" keeps it, a click on a star's line cancels and
// opens that star's system screen. The app routes Update and Draw to it
// while up, as the quit prompt's.

// dayWarning is the warning up: the stars it names and the day's end it
// holds back (nextDay or skipDays).
type dayWarning struct {
	stars []int
	then  func()
}

// askDay runs then (a day's end), unless the warning applies: then it
// comes up holding then.
func (a *app) askDay(then func()) {
	if st := a.restless(); len(st) > 0 {
		a.warn = &dayWarning{stars: st, then: then}
		return
	}
	then()
}

// nextDay is one day's end, reported (the starmap's next-day button,
// Space).
func (a *app) nextDay() {
	a.report(a.endDay())
	a.saveLast()
}

// skipDays is Skip: days until an event (runUntilEvent), reported.
func (a *app) skipDays() {
	_, ev := a.runUntilEvent()
	a.report(ev)
	a.saveLast()
}

// warnPrompt is the warning's prompt and the stars its lines 1.. name:
// as many as fit above the answers, the last line "and N more" when
// they do not all fit.
func (a *app) warnPrompt() (prompt, []int) {
	box, _, _ := quitPrompt.rects()
	n := (dlgBar.Y0 - box.Y0 - dlgTitleY - int(bigFace.Size)) / (zoom * promptLineH) // lines past the first
	st := a.warn.stars
	more := ""
	if len(st) > n {
		st = st[:n-1]
		more = fmt.Sprintf("and %d more", len(a.warn.stars)-len(st))
	}
	lines := []string{"Ships with energy left in combat:"}
	for _, s := range st {
		lines = append(lines, a.cl.Stars[s].Name)
	}
	if more != "" {
		lines = append(lines, more)
	}
	return prompt{lines, "End the day (Enter)", "Cancel (Esc)"}, st
}

// warnStar is the star whose line is under (x, y), −1 none.
func (a *app) warnStar(x, y float64) int {
	p, st := a.warnPrompt()
	if k := p.lineAt(x, y); k >= 1 && k <= len(st) {
		return st[k-1]
	}
	return -1
}

// answerWarn closes the warning: yes runs the day's end it held.
func (a *app) answerWarn(yes bool) {
	then := a.warn.then
	a.warn = nil
	if yes {
		then()
	}
}

// warnClick is a left click at (x, y) on a star's line: the warning
// cancelled, that star's system screen pushed (push brings an open one
// up, as the starmap's click); false off the lines.
func (a *app) warnClick(x, y float64) bool {
	st := a.warnStar(x, y)
	if st < 0 {
		return false
	}
	a.answerWarn(false)
	a.push(newSystemScreen(st))
	return true
}

// warnUpdate reads the warning's clicks and keys while it is up: a
// star's line first (its rows end above the answers), then the prompt's.
func (a *app) warnUpdate() error {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if x, y := a.mouse(); a.warnClick(x, y) {
			return nil
		}
	}
	p, _ := a.warnPrompt()
	if ok, yes := p.update(a); ok {
		a.answerWarn(yes)
	}
	return nil
}

// drawWarn paints the warning, the hovered star's line yellow.
func (a *app) drawWarn(frame *ebiten.Image) {
	p, st := a.warnPrompt()
	mx, my := a.mouse()
	hot := p.lineAt(mx, my)
	if hot < 1 || hot > len(st) {
		hot = -1
	}
	p.drawHot(a, frame, hot)
}
