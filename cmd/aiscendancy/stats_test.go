package main

import (
	"math"
	"slices"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// statsApp is seed 1 with three empires, the player having met empire 1
// only, days played (game.EndDay: the History grows), the Stats screen
// over the starmap.
func statsApp(t *testing.T, days int) (*app, *statsScreen) {
	t.Helper()
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa, game.Mebes}, game.Average)
	a := testApp(g)
	a.open([]screen{newStarmapScreen()})
	a.record(g.Contact(0, 1))
	for range days {
		g.EndDay()
	}
	s := newStatsScreen()
	a.push(s)
	return a, s
}

// The lines are the player's and its ally's Histories, each in its
// empire's colour, the player's thicker; empire 2, never met, draws none
// though it has a History too.
func TestStatsHidden(t *testing.T) {
	a, _ := statsApp(t, 3*game.HistoryEvery)
	ally(t, a.g)
	if got := statsEmpires(a); !slices.Equal(got, []int{0, 1}) {
		t.Fatalf("empires drawn %v", got)
	}
	hidden := a.g.Empires[2].History
	if len(hidden) == 0 {
		t.Fatal("fixture: the unmet empire has no History")
	}
	for k := range statSeries {
		ls := statsLines(a, k)
		if len(ls) != 2 {
			t.Fatalf("series %d: %d lines", k, len(ls))
		}
		for i, j := range []int{0, 1} {
			l, e := ls[i], a.g.Empires[j]
			if &l.h[0] != &e.History[0] || len(l.h) != len(e.History) || l.c != empireColour(colourOf(a.g, j)) {
				t.Errorf("series %d line %d: not empire %d's History in its colour", k, i, j)
			}
			if l.of(e.History[len(e.History)-1]) != statSeries[k].of(e.History[len(e.History)-1]) {
				t.Errorf("series %d line %d reads another series", k, i)
			}
			if &l.h[0] == &hidden[0] {
				t.Errorf("series %d: the unmet empire's History drawn", k)
			}
		}
		if ls[0].w <= ls[1].w {
			t.Errorf("series %d: the player's line %v not thicker than %v", k, ls[0].w, ls[1].w)
		}
	}
}

// statsEmpires: the player, then the empires met that are allied with it
// or defeated, in empire order; one met at peace or war is a rival
// (its curves would be cheating), one never met stays fogged even
// defeated.
func TestStatsEmpires(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa, game.Mebes, game.Govorom, game.Capelons, game.Dubtaks}, game.Average)
	a := testApp(g)
	for j := 1; j <= 4; j++ {
		a.record(g.Contact(0, j))
	}
	if got := statsEmpires(a); !slices.Equal(got, []int{0}) {
		t.Fatalf("met at peace: %v, want the player alone", got)
	}
	if _, err := g.OfferAlliance(2, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Answer(g.Offers[0], true); err != nil {
		t.Fatal(err)
	}
	if _, err := g.DeclareWar(0, 4); err != nil {
		t.Fatal(err)
	}
	g.Empires[3].Lost = g.Day + 1
	g.Empires[5].Lost = g.Day + 1 // never met
	if got := statsEmpires(a); !slices.Equal(got, []int{0, 2, 3}) {
		t.Errorf("empires drawn %v, want [0 2 3]", got)
	}
	if n := len(statsLines(a, 0)); n != 3 {
		t.Errorf("%d lines", n)
	}
}

// The series buttons lie in a row in the big area, apart, their labels
// fitting; a click on one shows its series, a click elsewhere none.
func TestStatsSeries(t *testing.T) {
	a, s := statsApp(t, 0)
	for k, st := range statSeries {
		r := statsButton(k)
		if r.X0 < mapArea.X0 || r.X1 > mapArea.X1 || r.Y0 < mapArea.Y0 || r.Y1 > statsChart().Y0 {
			t.Errorf("button %d %+v outside the area above the chart", k, r)
		}
		if k > 0 && r.X0 < statsButton(k-1).X1 {
			t.Errorf("button %d overlaps %d", k, k-1)
		}
		if w := ui.Width(st.name, bigFace); w > float64(r.X1-r.X0-2*zoom) {
			t.Errorf("%q: %.0f px in %d", st.name, w, r.X1-r.X0)
		}
	}
	for k := len(statSeries) - 1; k >= 0; k-- {
		r := statsButton(k)
		s.click(float64(r.X0+r.X1)/2, float64(r.Y0+r.Y1)/2)
		if s.series != k {
			t.Fatalf("click on button %d: series %d", k, s.series)
		}
	}
	c := statsChart()
	s.click(float64(c.X0+c.X1)/2, float64(c.Y0+c.Y1)/2)
	if s.series != 0 {
		t.Errorf("a click in the chart picked series %d", s.series)
	}
	if a.top() != screen(s) {
		t.Errorf("top %T", a.top())
	}
}

// Day 0: no sample anywhere; the frame still spans (no division by
// zero) and the chart and the screen's text draw.
func TestStatsEmpty(t *testing.T) {
	a, s := statsApp(t, 0)
	for _, e := range a.g.Empires {
		if len(e.History) != 0 {
			t.Fatalf("empire %d: History at day 0", e.Index)
		}
	}
	ls := statsLines(a, 0)
	sp := spanOf(ls)
	if sp.d1 <= sp.d0 || sp.v1 <= sp.v0 || sp.step <= 0 {
		t.Fatalf("empty span %+v", sp)
	}
	img := ebiten.NewImage(screenW, screenH)
	defer img.Deallocate()
	drawChart(img, statsChart(), ls, true)
	s.Text(a, img)
}

// One sample: the frame spans a day and a unit at least, its point lands
// inside the chart; more samples: every point in the frame, v1 a
// multiple of the step at or over the largest value.
func TestStatsSpan(t *testing.T) {
	r := statsChart()
	one := []game.Sample{{Day: 0, Pop: 7}}
	for _, v := range []int{0, 7} {
		one[0].Pop = v
		sp := spanOf([]chartLine{{h: one, of: statSeries[1].of}})
		x, y := sp.at(r, 0, v)
		if sp.d1 <= sp.d0 || sp.v1 <= sp.v0 || math.IsNaN(float64(x)) || math.IsNaN(float64(y)) ||
			x < float32(r.X0) || x > float32(r.X1) || y < float32(r.Y0) || y > float32(r.Y1) {
			t.Errorf("one sample of %d: span %+v, point (%v, %v) in %+v", v, sp, x, y, r)
		}
	}
	a, _ := statsApp(t, 12*game.HistoryEvery)
	for k, st := range statSeries {
		ls := statsLines(a, k)
		sp := spanOf(ls)
		top := 0
		for _, l := range ls {
			for _, x := range l.h {
				top = max(top, st.of(x))
				if px, py := sp.at(r, x.Day, st.of(x)); px < float32(r.X0) || px > float32(r.X1) || py < float32(r.Y0) || py > float32(r.Y1) {
					t.Errorf("%s day %d: (%v, %v) outside %+v", st.name, x.Day, px, py, r)
				}
			}
		}
		if sp.v1%sp.step != 0 || sp.v1 < top || sp.d0 != 0 || sp.d1 != a.g.Day {
			t.Errorf("%s: span %+v, top %d, day %d", st.name, sp, top, a.g.Day)
		}
	}
}

// niceStep is the least 1, 2, 5 × 10^k at least n.
func TestNiceStep(t *testing.T) {
	nice := func(m int) bool {
		for m%10 == 0 {
			m /= 10
		}
		return m == 1 || m == 2 || m == 5
	}
	for n := -1; n <= 2000; n++ {
		m := niceStep(n)
		if m < max(n, 1) || !nice(m) {
			t.Fatalf("niceStep(%d) = %d", n, m)
		}
		for k := max(n, 1); k < m; k++ {
			if nice(k) {
				t.Fatalf("niceStep(%d) = %d, %d is nice", n, m, k)
			}
		}
	}
}

// H opens the Stats screen and closes it again (the real Update); the
// species screen's Stats box opens it with the mouse.
func TestStatsOpen(t *testing.T) {
	a, _ := statsApp(t, 0)
	a.pop()
	step := func() {
		t.Helper()
		pressing(t, ebiten.KeyH)
		if err := a.Update(); err != nil {
			t.Fatal(err)
		}
	}
	step()
	top[*statsScreen](t, a)
	step()
	top[*starmapScreen](t, a)
	sp := newSpeciesScreen(-1)
	a.push(sp)
	sp.click(a, float64(statsBox.X0+statsBox.X1)/2, float64(statsBox.Y0+statsBox.Y1)/2)
	top[*statsScreen](t, a)
	if a.keysSection() != keysLists {
		t.Errorf("keys section %q", a.keysSection())
	}
}
