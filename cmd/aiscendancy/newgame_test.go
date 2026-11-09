package main

import (
	"os"
	"testing"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/cob"

	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// ngHoleCentre is the centre of bottom hole k, native px.
func ngHoleCentre(k int) (float64, float64) {
	r := ngHoles[k]
	return float64(r.X0+r.X1) / 2, float64(r.Y0+r.Y1) / 2
}

// ngLineY is the middle of list line i, native px.
func ngLineY(i int) float64 { return float64(ngList.Y0+padY) + (float64(i)+0.5)*ngPitch }

// The density hole cycles Vacuous..Very Dense and wraps; the empires hole
// 3..7 and wraps; the atmosphere hole Neutral, Hostile, Peaceful, round.
func TestNewGameCycles(t *testing.T) {
	s := newNewGameScreen()
	a := &app{}
	if s.density != game.Average || s.empires != 3 || s.species != game.Snovemdomas || s.seed != 1 {
		t.Fatalf("defaults: %+v", s)
	}
	x, y := ngHoleCentre(densityHole)
	want := []game.Density{game.Dense, game.VeryDense, game.Vacuous, game.Sparse, game.Average}
	for _, d := range want {
		s.click(a, x, y)
		if s.density != d {
			t.Fatalf("density %s, want %s", s.density, d)
		}
	}
	x, y = ngHoleCentre(empiresHole)
	for _, n := range []int{4, 5, 6, 7, 3} {
		s.click(a, x, y)
		if s.empires != n {
			t.Fatalf("empires %d, want %d", s.empires, n)
		}
	}
	x, y = ngHoleCentre(atmosHole)
	for _, at := range []game.Atmosphere{game.Hostile, game.Peaceful, game.Neutral} {
		s.click(a, x, y)
		if s.atmos != at {
			t.Fatalf("atmosphere %s, want %s", s.atmos, at)
		}
	}
	if a.g != nil || len(a.stack) != 0 {
		t.Fatal("a cycle began a game")
	}
}

// A click on list line i picks species i; the strip above the list and
// the list's blank bottom pick nothing.
func TestNewGamePickSpecies(t *testing.T) {
	s := newNewGameScreen()
	x := float64(ngList.X0+ngList.X1) / 2
	for i := range int(game.NumSpecies) {
		s.click(nil, x, ngLineY(i))
		if s.species != game.Species(i) {
			t.Fatalf("line %d picked %s", i, s.species)
		}
		if l, _ := s.pick(x, ngLineY(i)); l != i {
			t.Fatalf("line %d hovers %d", i, l)
		}
	}
	for _, y := range []float64{float64(ngTitle.Y0 + 5), ngLineY(int(game.NumSpecies)), float64(ngList.Y1 - 1)} {
		if l, h := s.pick(x, y); l >= 0 || h >= 0 {
			t.Fatalf("y %.0f picks line %d hole %d", y, l, h)
		}
	}
}

// A click in the preview draws a new seed in 1..maxSeed; a click in the
// species' part (right of original x 330) does not.
func TestNewGameReroll(t *testing.T) {
	s := newNewGameScreen()
	changed := false
	for range 5 {
		old := s.seed
		s.click(nil, ngCamX, ngCamY)
		if s.seed < 1 || s.seed > maxSeed {
			t.Fatalf("seed %d", s.seed)
		}
		changed = changed || s.seed != old
	}
	if !changed {
		t.Fatal("five re-rolls kept the seed")
	}
	before := *s
	s.click(nil, float64(ngSpecies.X0)+1, ngCamY)
	if *s != before {
		t.Fatalf("a click right of the preview changed %+v to %+v", before, *s)
	}
}

// The preview is the cluster Begin rolls, NewCluster(seed, density):
// rolled again on a density cycle and a re-roll, kept on a species pick.
func TestNewGamePreview(t *testing.T) {
	s := newNewGameScreen()
	same := func(cl *game.Cluster) {
		t.Helper()
		want := game.NewCluster(s.seed, s.density)
		if len(cl.Stars) != len(want.Stars) || cl.Stars[0].Pos != want.Stars[0].Pos || cl.Density != s.density {
			t.Fatalf("preview of %d stars, star 0 at %v; want %d at %v", len(cl.Stars), cl.Stars[0].Pos,
				len(want.Stars), want.Stars[0].Pos)
		}
	}
	p := s.preview()
	same(p)
	if s.preview() != p {
		t.Fatal("an unchanged screen rolled the preview again")
	}
	s.click(nil, float64(ngList.X0+ngList.X1)/2, ngLineY(int(game.Orfa)))
	if s.species != game.Orfa || s.preview() != p {
		t.Fatal("a species pick rolled the preview again")
	}
	x, y := ngHoleCentre(densityHole)
	s.click(nil, x, y)
	q := s.preview()
	if q == p {
		t.Fatal("a density cycle kept the preview")
	}
	same(q)
	s.click(nil, ngCamX, ngCamY)
	if s.seed == 1 { // the re-roll drew seed 1 again: once more
		s.click(nil, ngCamX, ngCamY)
	}
	r := s.preview()
	if r == q {
		t.Fatal("a re-roll kept the preview")
	}
	same(r)
	if r.Stars[0].Pos == q.Stars[0].Pos {
		t.Fatal("a new seed rolled the same cluster")
	}
}

// The summary under the preview, at first.
func TestNewGameSummaryLines(t *testing.T) {
	if got, want := newNewGameScreen().summaryLines(), "Average Star Cluster\nThree Species\nseed 1"; got != want {
		t.Fatalf("%q, want %q", got, want)
	}
	s := newNewGameScreen()
	s.density, s.empires, s.seed = game.VeryDense, 7, 42
	if got, want := s.summaryLines(), "Very Dense Star Cluster\nSeven Species\nseed 42"; got != want {
		t.Fatalf("%q, want %q", got, want)
	}
}

// A click on a flag of the Player Color hole picks its colour, one
// between flags changes nothing; every hole's label sits above its
// picture (its line ends before the frame's row ngPicTop), the flags
// under it and inside the hole.
func TestNewGamePlayerColour(t *testing.T) {
	s := newNewGameScreen()
	a := &app{}
	s.preview()
	for c := game.NumColours - 1; c >= 0; c-- {
		x, y := float64(ngColX0+c*ngColDX+zoom*flagW/2), float64(ngColY+zoom*flagH/2)
		if colourAt(x, y) != c {
			t.Fatalf("colourAt flag %d: %d", c, colourAt(x, y))
		}
		s.click(a, x, y)
		if s.colour != c || a.g != nil {
			t.Fatalf("click on flag %d: colour %d", c, s.colour)
		}
	}
	before := *s
	s.click(a, float64(ngColX0+zoom*flagW+zoom/2), float64(ngColY+zoom*flagH/2)) // the gap
	if *s != before {
		t.Fatalf("the gap changed %+v to %+v", before, *s)
	}
	h := ngHoles[colourHole]
	last := ngColX0 + (game.NumColours-1)*ngColDX + zoom*flagW
	if ngColX0 < h.X0 || last > h.X1 || ngColY < int(ngLabelY(colourHole)+smallFace.Size*1.25) || ngColY+zoom*flagH > h.Y1 {
		t.Fatalf("flags x [%d,%d) y [%d,%d) outside hole %v or over its label", ngColX0, last, ngColY, ngColY+zoom*flagH, h)
	}
	for k := range ngLabels {
		top, bottom := ngLabelY(k), ngLabelY(k)+smallFace.Size*1.25
		if top < float64(ngHoles[k].Y0) || bottom > float64(ngHoles[k].Y0+zoom*ngPicTop) {
			t.Fatalf("hole %d label in y [%.1f,%.1f), picture from %d", k, top, bottom, ngHoles[k].Y0+zoom*ngPicTop)
		}
	}
}

// Every species' description and ability fit the right part whole: the
// row budget cuts only the home line.
func TestNewGameFactsFit(t *testing.T) {
	for sp := range game.NumSpecies {
		in := sp.Info()
		lines := ngFactsFit(in)
		all := ngFacts(in, int(float64(ngDescX1-ngDescX0)/ui.Width("m", smallFace)), 99)
		if len(all)-len(lines) > 1 {
			t.Errorf("%s: %d of %d lines fit", in.Name, len(lines), len(all))
		}
		for _, l := range lines {
			if w := ui.Width(l, smallFace); w > ngDescX1-ngDescX0 {
				t.Errorf("%s: %q is %.0f px wide", in.Name, l, w)
			}
		}
	}
}

// Begin makes the game of the screen's choices on an app with none: the
// player's species first, the density, a bot for every empire but the
// player's, a fresh starmap under the species' introduction.
func TestNewGameBegin(t *testing.T) {
	a := &app{}
	a.open([]screen{newNewGameScreen()})
	if a.inGame() {
		t.Fatal("a game before begin")
	}
	s := top[*newGameScreen](t, a)
	s.seed, s.density, s.species, s.colour = 7, game.Sparse, game.Orfa, game.Blue
	x := float64(ngList.X0+ngList.X1) / 2
	s.click(a, x, ngLineY(int(game.Mebes)))
	hx, hy := ngHoleCentre(empiresHole)
	s.click(a, hx, hy) // 4 empires
	hx, hy = ngHoleCentre(beginHole)
	s.click(a, hx, hy)

	if !a.inGame() || a.g.Seed != 7 || len(a.g.Empires) != 4 || a.g.Cluster.Density != game.Sparse {
		t.Fatalf("game: %+v", a.g)
	}
	if a.e != a.g.Empires[0] || a.e.Species != game.Mebes || a.cl != a.g.Cluster {
		t.Fatalf("the player: %v", a.e.Species)
	}
	want := game.PickSpecies(7, game.Mebes, 4)
	for i, e := range a.g.Empires {
		if e.Species != want[i] {
			t.Fatalf("empire %d is %s, want %s", i, e.Species, want[i])
		}
	}
	cols := game.PickColours(7, game.Green, 4) // the seed's draw, Blue's holder taking Green
	for i, e := range a.g.Empires {
		w := cols[i]
		switch w {
		case game.Green:
			w = game.Blue
		case game.Blue:
			w = game.Green
		}
		if e.Colour != w {
			t.Fatalf("empire %d colour %d, want %d", i, e.Colour, w)
		}
	}
	if len(a.bots) != 4 || a.bots[0] != nil {
		t.Fatalf("bots: %v", a.bots)
	}
	for i, b := range a.bots[1:] {
		if b == nil || b.G != a.g || b.Cfg.Genome != nil {
			t.Fatalf("bot %d: %+v", i+1, b)
		}
	}
	if len(a.stack) != 2 {
		t.Fatalf("%d screens", len(a.stack))
	}
	if _, ok := a.stack[0].(*starmapScreen); !ok {
		t.Fatalf("under the introduction: %T", a.stack[0])
	}
	if in := top[*introScreen](t, a); in.species != game.Mebes || in.colour != game.Blue {
		t.Fatalf("introduction of %s in colour %d", in.species, in.colour)
	}
	if a.log != nil || a.toast != "" { // the introduction says the choices
		t.Fatalf("log %v, toast %q", a.log, a.toast)
	}
	if b, c := introTone(game.Brown), colourText[game.Brown]; b.R <= c.R || b.G <= c.G || b.B <= c.B || introTone(game.Blue) != colourText[game.Blue] {
		t.Errorf("intro tones: brown %v from %v, blue %v", b, c, introTone(game.Blue))
	}
	for i, b := range a.bots[1:] {
		if b.Cfg.Difficulty != bot.Original {
			t.Fatalf("bot %d plays %v", i+1, b.Cfg.Difficulty)
		}
	}
	// -genome: every bot of the next game plays it; the Difficulty line
	// cycles Original, Hard, Original (the seed kept: not a preview click).
	a.genome = bot.Neutral()
	a.open([]screen{newNewGameScreen()})
	s = top[*newGameScreen](t, a)
	dx, dy := float64(ngDiff.X0+ngDiff.X1)/2, float64(ngDiff.Y0+ngDiff.Y1)/2
	seed := s.seed
	for i, want := range []bot.Difficulty{bot.Hard, bot.Original, bot.Hard} {
		if s.click(a, dx, dy); s.diff != want || s.seed != seed {
			t.Fatalf("click %d: %v, seed %d", i, s.diff, s.seed)
		}
	}
	if !ngPreview.has(dx, dy) {
		t.Error("the Difficulty line is not on the preview")
	}
	hx, hy = ngHoleCentre(beginHole)
	s.click(a, hx, hy)
	for i, b := range a.bots[1:] {
		if b == nil || b.Cfg.Genome != a.genome || b.Cfg.Difficulty != bot.Hard {
			t.Fatalf("-genome, Hard, bot %d: %+v", i+1, b)
		}
	}
}

// colourText is each colour ramp's lightest entry in game.pal, and a flag
// in colour c draws that ramp's entries where the green flag draws the
// green ramp's, every other pixel as drawn.
func TestColourRamps(t *testing.T) {
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	as, err := assets.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer as.Close()
	b, err := as.Bank(flagBank)
	if err != nil {
		t.Fatal(err)
	}
	green := b.Frames[0].RGBA(as.Palette())
	for c := range game.NumColours {
		if got := as.Ramp(c, 3); got != colourText[c] {
			t.Errorf("colour %d: text %v, game.pal %v", c, colourText[c], got)
		}
		img := b.Frames[0].RGBA(as.PaletteIn(c))
		f := b.Frames[0]
		w := f.Box.Dx()
		for k, ix := range f.Index {
			if !f.Opaque[k] {
				continue
			}
			x, y := f.Box.Min.X+k%w, f.Box.Min.Y+k/w
			want := green.RGBAAt(x, y)
			if ix >= 16 && ix <= 19 {
				want = as.Ramp(c, int(ix)-16)
			}
			if got := img.RGBAAt(x, y); got != want {
				t.Fatalf("colour %d, pixel (%d,%d) index %d: %v, want %v", c, x, y, ix, got, want)
			}
		}
	}
	if _, err := as.SpriteIn(flagBank, 0, 60); err == nil {
		t.Error("colour 60 accepted")
	}
}
