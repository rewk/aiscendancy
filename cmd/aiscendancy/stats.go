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

// The Stats screen: the empires' History (game/history.go) as curves on
// the starmap's chrome, one series at a time (a row of buttons over the
// chart), the player against its allies and the empires defeated
// (statsEmpires: a rival's curves would be cheating), each in its
// empire's colour; the legend in the news box.
// Key H anywhere in a game, or the species screen's Stats button.
type statsScreen struct {
	series int // statSeries index shown
	btn    int // series button under the cursor, −1 none
}

func newStatsScreen() *statsScreen { return &statsScreen{btn: -1} }

func (s *statsScreen) Enter(*app) {}

// stat is a graphed series: its label and its value in a Sample.
type stat struct {
	name string
	of   func(game.Sample) int
}

// statSeries are the series graphed, the buttons' order.
var statSeries = [...]stat{
	{"Colonies", func(x game.Sample) int { return x.Colonies }},
	{"Pop", func(x game.Sample) int { return x.Pop }},
	{"Industry", func(x game.Sample) int { return x.Industry }},
	{"Research", func(x game.Sample) int { return x.Research }},
	{"Prosperity", func(x game.Sample) int { return x.Prosperity }},
	{"Fleet", func(x game.Sample) int { return x.Fleet }},
	{"Techs", func(x game.Sample) int { return x.Techs }},
	{"Stars", func(x game.Sample) int { return x.Stars }},
}

// chartLine is one curve: a History read through of, in colour c, w
// native px wide. Nothing is copied out of the History.
type chartLine struct {
	h  []game.Sample
	of func(game.Sample) int
	c  color.RGBA
	w  float32
}

// chartSpan is the lines' frame: days d0..d1 and values v0..v1, both
// spans > 0 (one sample or none still spans a day and a unit), v0 ≤ 0,
// v1 a multiple of step (niceStep of a quarter of the values' span).
type chartSpan struct{ d0, d1, v0, v1, step int }

func spanOf(ls []chartLine) chartSpan {
	sp := chartSpan{d0: -1}
	lo, hi := 0, 0
	for _, l := range ls {
		for _, x := range l.h {
			if sp.d0 < 0 || x.Day < sp.d0 {
				sp.d0 = x.Day
			}
			sp.d1 = max(sp.d1, x.Day)
			lo, hi = min(lo, l.of(x)), max(hi, l.of(x))
		}
	}
	sp.d0 = max(sp.d0, 0)
	if sp.d1 <= sp.d0 {
		sp.d1 = sp.d0 + 1
	}
	sp.step = niceStep(max(hi-lo, 1) / 4)
	sp.v0 = -ceilTo(-lo, sp.step)
	sp.v1 = max(ceilTo(hi, sp.step), sp.v0+sp.step)
	return sp
}

// niceStep is the least of 1, 2, 5 × 10^k at least n (1 for n ≤ 1).
func niceStep(n int) int {
	for p := 1; ; p *= 10 {
		for _, m := range [3]int{1, 2, 5} {
			if m*p >= n {
				return m * p
			}
		}
	}
}

// ceilTo is n rounded up to a multiple of step (n ≥ 0).
func ceilTo(n, step int) int { return (n + step - 1) / step * step }

// at is (day, v)'s native point in r.
func (sp chartSpan) at(r rect, day, v int) (float32, float32) {
	x := float64(r.X0) + float64(r.X1-r.X0)*float64(day-sp.d0)/float64(sp.d1-sp.d0)
	y := float64(r.Y1) - float64(r.Y1-r.Y0)*float64(v-sp.v0)/float64(sp.v1-sp.v0)
	return float32(x), float32(y)
}

// chartGrid is the grid's colour, the zero line's brighter.
var chartGrid = color.RGBA{40, 50, 60, 255}

// drawChart paints ls in r over the zero line and, with ticks, the value
// grid (labels left of r) and the day ticks (labels under r), smallFace.
// A one-sample line is a dot. It reads the lines, allocates nothing.
func drawChart(dst *ebiten.Image, r rect, ls []chartLine, ticks bool) {
	sp := spanOf(ls)
	if ticks {
		for v := sp.v0; v <= sp.v1; v += sp.step {
			_, y := sp.at(r, sp.d0, v)
			vector.FillRect(dst, float32(r.X0), y, float32(r.X1-r.X0), 1, chartGrid, false)
			l := fmt.Sprint(v)
			ui.Print(dst, l, smallFace, float64(r.X0)-ui.Width(l, smallFace)-4, float64(y)-smallFace.Size*0.6, dim(textColour, 0.7))
		}
		dstep := niceStep((sp.d1 - sp.d0) / 6)
		for d := ceilTo(sp.d0, dstep); d <= sp.d1; d += dstep {
			x, _ := sp.at(r, d, sp.v0)
			vector.FillRect(dst, x, float32(r.Y1), 1, 4, chartGrid, false)
			l := fmt.Sprint(d)
			ui.Print(dst, l, smallFace, float64(x)-ui.Width(l, smallFace)/2, float64(r.Y1)+4, dim(textColour, 0.7))
		}
	}
	x0, y0 := sp.at(r, sp.d0, 0)
	vector.FillRect(dst, x0, y0, float32(r.X1-r.X0), 1, dim(textColour, 0.4), false)
	for _, l := range ls {
		if len(l.h) == 1 {
			x, y := sp.at(r, l.h[0].Day, l.of(l.h[0]))
			vector.FillRect(dst, x-l.w, y-l.w, 2*l.w, 2*l.w, l.c, false)
			continue
		}
		for i := 1; i < len(l.h); i++ {
			ax, ay := sp.at(r, l.h[i-1].Day, l.of(l.h[i-1]))
			bx, by := sp.at(r, l.h[i].Day, l.of(l.h[i]))
			vector.StrokeLine(dst, ax, ay, bx, by, l.w, l.c, true)
		}
	}
}

// lastOf is h's last value through of, and false for an empty h.
func lastOf(h []game.Sample, of func(game.Sample) int) (int, bool) {
	if len(h) == 0 {
		return 0, false
	}
	return of(h[len(h)-1]), true
}

// statsEmpires are the empires drawn: the player, then those met that
// are defeated or allied with it (in empire order); none without a game.
func statsEmpires(a *app) []int {
	if a.g == nil || a.e == nil {
		return nil
	}
	out := []int{a.player}
	for _, j := range met(a) {
		if !a.g.Empires[j].Alive() || a.e.StanceWith(j) == game.Alliance {
			out = append(out, j)
		}
	}
	return out
}

// statsLines are series k's curves, statsEmpires' order, the player's
// thicker.
func statsLines(a *app, k int) []chartLine {
	var ls []chartLine
	for _, j := range statsEmpires(a) {
		w := float32(zoom)
		if j == a.player {
			w = 2 * zoom
		}
		ls = append(ls, chartLine{a.g.Empires[j].History, statSeries[k].of, empireColour(colourOf(a.g, j)), w})
	}
	return ls
}

// The layout, native px: the series buttons in a row at the big area's
// top, the chart under them, room left for the value labels and under
// it for the days'.
const (
	statsBtnH   = 18 * zoom
	statsBtnGap = 4 * zoom
	statsLabelW = 64 // "100000" in smallFace, with its gap
)

// statsButton is series button k's rect.
func statsButton(k int) rect {
	n := len(statSeries)
	w := (mapArea.X1 - mapArea.X0 - 2*padX - (n-1)*statsBtnGap) / n
	x := mapArea.X0 + padX + k*(w+statsBtnGap)
	y := mapArea.Y0 + padY
	return rect{x, y, x + w, y + statsBtnH}
}

// statsButtonAt is the series button under (x, y), −1 none.
func statsButtonAt(x, y float64) int {
	for k := range statSeries {
		if statsButton(k).has(x, y) {
			return k
		}
	}
	return -1
}

// statsChart is the chart's rect: under the buttons and the title line.
func statsChart() rect {
	lh := int(bigFace.Size * 1.25)
	return rect{mapArea.X0 + padX + statsLabelW, statsButton(0).Y1 + 2*lh, mapArea.X1 - 2*padX,
		mapArea.Y1 - padY - 2*int(smallFace.Size*1.25)}
}

// click picks the series under (x, y), if any.
func (s *statsScreen) click(x, y float64) {
	if k := statsButtonAt(x, y); k >= 0 {
		s.series = k
	}
}

func (s *statsScreen) Update(a *app) error {
	x, y := a.mouse()
	if onSpeciesKey(a, inpututil.IsKeyJustPressed(ebiten.KeyS)) {
		return nil
	}
	s.btn = statsButtonAt(x, y)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		s.click(x, y)
		a.pressTurn(turnButton(x, y))
	}
	return nil
}

// toggleStats pops the Stats screen when on top, else pushes it.
func (a *app) toggleStats() error {
	if _, ok := a.top().(*statsScreen); ok {
		return a.pop()
	}
	a.push(newStatsScreen())
	return nil
}

// Render paints the chrome, the legend's box and the chart.
func (s *statsScreen) Render(a *app, sc *ebiten.Image) {
	drawMapChrome(a, sc, 2)
	drawNewsBox(sc, newsBox)
	if a.g == nil {
		return
	}
	drawChart(sc, statsChart(), statsLines(a, s.series), true)
}

const statsHelp = "click: a series\nH, Esc: back"

// Text writes the buttons, the chart's title, the legend (each empire's
// last value) and the boxes.
func (s *statsScreen) Text(a *app, frame *ebiten.Image) {
	for k, st := range statSeries {
		if k == s.series {
			strokeRect(frame, statsButton(k), yellow)
			r := statsButton(k)
			drawButton(frame, rect{r.X0 + zoom, r.Y0 + zoom, r.X1 - zoom, r.Y1 - zoom}, st.name, true)
			continue
		}
		drawButton(frame, statsButton(k), st.name, k == s.btn)
	}
	putBox(frame, mapStack[0], statsHelp, smallFace, textColour)
	if a.g == nil {
		return
	}
	putBox(frame, mapStack[1], fmt.Sprintf("Day %d\na sample every %d", a.g.Day, game.HistoryEvery), smallFace, textColour)
	c := statsChart()
	ui.Print(frame, statSeries[s.series].name+" by day", bigFace, float64(c.X0), float64(statsButton(0).Y1)+bigFace.Size*0.5, textColour)
	lh := smallFace.Size * 1.25
	x, y := float64(newsBox.X0+padX), float64(newsBox.Y0+padY)
	for _, j := range statsEmpires(a) {
		e := a.g.Empires[j]
		col := empireColour(colourOf(a.g, j))
		vector.FillRect(frame, float32(x), float32(y+lh/2-3), 12, 6, col, false)
		l := e.Species.String()
		if v, ok := lastOf(e.History, statSeries[s.series].of); ok {
			l += fmt.Sprintf("  %d", v)
		}
		if !e.Alive() {
			l += "  (gone)"
		}
		ui.Print(frame, fitWidth(l, float64(newsBox.X1-padX)-x-18, smallFace), smallFace, x+18, y, col)
		y += lh
	}
}
