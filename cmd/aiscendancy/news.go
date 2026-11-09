package main

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// newsView is the news: a mini-log in newsBox on every map-canvas screen
// and the system screen, each calling drawNewsBox in Render, input in
// Update, text in Text. One smallFace line per entry (newsEntries),
// coloured by age; the wheel over the box scrolls it, a press there is
// the news' alone. Rows map to entries from off: no log index is kept.
// A hovered line too wide for the box scrolls left (newsSlide).
type newsView struct {
	off     int // rows scrolled: the top row shows the off-th entry
	hover   int // row under the cursor, −1 none
	hoverAt int // a.tick the hover (row and entry) began: newsSlide's clock
	hovered int // log index under the cursor, −1 none
}

const (
	newsDays  = 10
	newsMin   = 20
	newsNotch = 3
)

func newNewsView() newsView { return newsView{hover: -1, hovered: -1} }

// newsPause is the ticks a sliding line rests at each end, newsPace the
// native px it moves a tick.
const (
	newsPause = 60
	newsPace  = 1
)

// newsSlide is how far left a hovered line of textW px slides in a box
// of boxW px, t ticks into the hover: 0 when it fits; else a newsPause
// rest, newsPace px a tick up to textW − boxW, a rest there, and again.
func newsSlide(t int, textW, boxW float64) float64 {
	over := textW - boxW
	if over <= 0 || t < 0 {
		return 0
	}
	run := int(over/newsPace) + 1 // ticks from 0 to over, the last capped
	t %= newsPause + run + newsPause
	return min(float64(max(t-newsPause, 0))*newsPace, over)
}

// newsLineH is a news line's height, native px.
var newsLineH = smallFace.Size * 1.25

// newsEntries are the log indices listed, newest first: the last
// newsDays days' entries, at least newsMin when the log holds them.
func newsEntries(a *app) []int {
	if a.e == nil {
		return nil
	}
	n := 0
	for i := len(a.log) - 1; i >= 0 && a.log[i].Day >= a.e.Day-newsDays+1; i-- {
		n++
	}
	n = max(n, min(newsMin, len(a.log)))
	out := make([]int, n)
	for k := range out {
		out[k] = len(a.log) - 1 - k
	}
	return out
}

// rows is how many lines rect r holds.
func (n *newsView) rows(r rect) int { return linesFit(float64(r.Y1-r.Y0), smallFace) }

// row is the row of r under (x, y), −1 outside its lines.
func (n *newsView) row(r rect, x, y float64) int {
	if !r.has(x, y) {
		return -1
	}
	k := int((y - float64(r.Y0+padY)) / newsLineH)
	if y < float64(r.Y0+padY) || k >= n.rows(r) {
		return -1
	}
	return k
}

// entry is the log index row k shows, −1 none.
func (n *newsView) entry(a *app, r rect, k int) int {
	es := newsEntries(a)
	if k < 0 || k >= n.rows(r) || n.off+k >= len(es) {
		return -1
	}
	return es[n.off+k]
}

// scroll moves by d rows (positive: older), clamped.
func (n *newsView) scroll(a *app, r rect, d int) {
	n.off = min(max(n.off+d, 0), max(0, len(newsEntries(a))-n.rows(r)))
}

// wheel scrolls a notch with the cursor over r: true then.
func (n *newsView) wheel(a *app, r rect, x, y, wy float64) bool {
	if wy == 0 || !r.has(x, y) {
		return false
	}
	n.scroll(a, r, notch(wy, newsNotch))
	return true
}

// update re-clamps the scroll (entries change with the days) and sets
// the hovered row.
func (n *newsView) update(a *app, r rect, x, y float64) {
	n.scroll(a, r, 0)
	n.hover = n.row(r, x, y)
	if i := n.entry(a, r, n.hover); i != n.hovered {
		n.hovered, n.hoverAt = i, a.tick
	}
}

// click is a left press: true in r, a linked line's screens pushed.
func (n *newsView) click(a *app, r rect, x, y float64) bool {
	if !r.has(x, y) {
		return false
	}
	if i := n.entry(a, r, n.row(r, x, y)); i >= 0 {
		for _, sc := range eventScreens(a, a.log[i].Ev) {
			a.push(sc)
		}
	}
	return true
}

// input is the frame's mouse over newsBox, without Ebiten: wheeled and
// pressed say the wheel or the press was the news' (the caller ignores
// it).
func (n *newsView) input(a *app, x, y, wy float64, press bool) (wheeled, pressed bool) {
	wheeled = n.wheel(a, newsBox, x, y, wy)
	n.update(a, newsBox, x, y)
	return wheeled, press && n.click(a, newsBox, x, y)
}

func (n *newsView) text(a *app, frame *ebiten.Image) { n.draw(a, frame, newsBox) }

// newsRim is the soft border's rings, inside out, one native px each.
var newsRim = [2]color.RGBA{{0, 0, 0, 168}, {0, 0, 0, 84}}

// drawNewsBox paints r black and its rings outside it, each as four
// strips (no pixel painted twice under alpha).
func drawNewsBox(sc *ebiten.Image, r rect) {
	fillRect(sc, r)
	for k, c := range newsRim {
		d := float32(k)
		x0, y0, x1, y1 := float32(r.X0)-d-1, float32(r.Y0)-d-1, float32(r.X1)+d+1, float32(r.Y1)+d+1
		vector.FillRect(sc, x0, y0, x1-x0, 1, c, false)
		vector.FillRect(sc, x0, y1-1, x1-x0, 1, c, false)
		vector.FillRect(sc, x0, y0+1, 1, y1-y0-2, c, false)
		vector.FillRect(sc, x1-1, y0+1, 1, y1-y0-2, c, false)
	}
}

// draw writes the lines in r, then the scroll cues.
func (n *newsView) draw(a *app, frame *ebiten.Image, r rect) {
	if a.e == nil {
		return
	}
	x, w := float64(r.X0+padX), float64(r.X1-r.X0-2*padX)
	for k := range n.rows(r) {
		i := n.entry(a, r, k)
		if i < 0 {
			break
		}
		l := a.log[i]
		c := lineColour(l, a.e.Day)
		if k == n.hover && linked(l.Ev) {
			c = yellow
		}
		y := float64(r.Y0+padY) + float64(k)*newsLineH
		if tw := ui.Width(l.Ev.Text, smallFace); k == n.hover && tw > w {
			// Through the line's strip of the box: nothing spills.
			clip := frame.SubImage(image.Rect(int(x), int(y), int(x+w), int(y+newsLineH))).(*ebiten.Image)
			ui.Print(clip, l.Ev.Text, smallFace, x-newsSlide(a.tick-n.hoverAt, tw, w), y, c)
			continue
		}
		ui.Print(frame, fitWidth(l.Ev.Text, w, smallFace), smallFace, x, y, c)
	}
	drawScrollCues(a, frame, r, n.off, n.rows(r), len(newsEntries(a)))
}

// eventScreens is what opening ev pushes, in order: with no place, the
// research screen for a discovery, the species screen (Other selected)
// for a diplomatic event or the ability's, else nothing; with a place
// (an ability's too: the Govorom's rich world), its system, then its
// planet.
func eventScreens(a *app, ev game.Event) []screen {
	if ev.Kind == game.Discovered && ev.Star < 0 {
		return []screen{newResearchScreen()}
	}
	if ev.Star < 0 && ev.Other >= 0 {
		return []screen{newSpeciesScreen(ev.Other)}
	}
	if ev.Star < 0 && (ev.Kind == game.AbilityUsed || ev.Kind == game.AbilityReady) {
		return []screen{newSpeciesScreen(-1)} // the ability button
	}
	if ev.Star < 0 {
		return nil
	}
	out := []screen{newSystemScreen(ev.Star)}
	if ev.Planet >= 0 {
		out = append(out, newPlanetScreen(ev.Star, ev.Planet))
	}
	return out
}
