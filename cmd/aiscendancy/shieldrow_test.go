package main

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/cob"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// Our panel's design line: shields up, an icon per ShieldHits, shieldGap
// apart after "<design>  ", then " <Absorb>"; off, dark or short of a
// hit's draw one dimmed icon and a word; no shield part, the design
// alone; too many icons for the width, one and " x<n> <absorb>".
func TestShieldRowOurs(t *testing.T) {
	_, _, colony, gun, _ := panelFixture(t)
	x, w := float64(panelX0), panelWidth()
	lay := func(sh *game.Starship) shieldLine { return layShields(sh.Design.Name, shipShields(sh), x, w) }
	if !gun.Shields || gun.ShieldsDown || gun.ShieldHits() < 1 {
		t.Fatalf("fixture: shields %v down %v, %d hits", gun.Shields, gun.ShieldsDown, gun.ShieldHits())
	}
	head := gun.Design.Name + "  "
	x0 := x + math.Floor(ui.Width(head, smallFace))
	for _, p := range []int{gun.Design.ShieldDraw(), 2 * gun.Design.ShieldDraw(), 3*gun.Design.ShieldDraw() + 1} {
		gun.Power = p
		n := gun.ShieldHits()
		l := lay(gun)
		if l.head != head || len(l.icons) != n || l.f != 1 || l.tail != fmt.Sprintf(" %d", gun.Absorb()) {
			t.Errorf("energy %d: head %q, %d icons (want %d), dim %v, tail %q", p, l.head, len(l.icons), n, l.f, l.tail)
			continue
		}
		for i, ix := range l.icons {
			if ix != x0+float64(i*(shieldIconW+shieldGap)) {
				t.Errorf("energy %d: icon %d at %v", p, i, ix)
			}
		}
		if l.tx != l.icons[n-1]+shieldIconW {
			t.Errorf("energy %d: tail at %v, last icon ends %v", p, l.tx, l.icons[n-1]+shieldIconW)
		}
	}
	dimmed := func(name, word string) {
		t.Helper()
		if l := lay(gun); l.head != head || len(l.icons) != 1 || l.f != shieldDim || l.tail != " "+word {
			t.Errorf("%s: head %q, %d icons, dim %v, tail %q; want one dimmed, %q", name, l.head, len(l.icons), l.f, l.tail, word)
		}
	}
	gun.Power = gun.Design.ShieldDraw() - 1
	dimmed("short of a draw", noPower)
	gun.ShieldsDown = true
	dimmed("down", "down")
	gun.ShieldsDown, gun.Shields = false, false
	dimmed("off", "off")
	if colony.Design.ShieldScore() != 0 {
		t.Fatal("fixture: the colony ship has a shield")
	}
	if l := lay(colony); l.head != colony.Design.Name || l.icons != nil || l.tail != "" {
		t.Errorf("no shield part: %+v", l)
	}
	// Overflow: more hits than the line holds.
	gun.Shields, gun.Power = true, 40*gun.Design.ShieldDraw()
	n := gun.ShieldHits()
	if l := lay(gun); len(l.icons) != 1 || l.tail != fmt.Sprintf(" x%d %d", n, gun.Absorb()) || l.tx+ui.Width(l.tail, smallFace) > x+w {
		t.Errorf("%d hits: %d icons, tail %q", n, len(l.icons), l.tail)
	}
	// A shield drawing nothing never falls: one icon, "x∞".
	ws := &game.Starship{Design: game.Design{Name: "W", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.WaveScatterer}}, Shields: true}
	if ws.Design.ShieldScore() == 0 || ws.Design.ShieldDraw() != 0 {
		t.Fatalf("fixture: Wave Scatterer score %d draw %d", ws.Design.ShieldScore(), ws.Design.ShieldDraw())
	}
	if l := lay(ws); ws.ShieldHits() != game.NoShieldFall || len(l.icons) != 1 || l.tail != fmt.Sprintf(" x∞ %d", ws.Absorb()) {
		t.Errorf("no draw: %d hits, %+v", ws.ShieldHits(), l)
	}
}

// A long design name is cut so the shortest row still fits the width.
func TestShieldRowCutsDesign(t *testing.T) {
	_, _, _, gun, _ := panelFixture(t)
	long := strings.Repeat("Q", 80)
	w := panelWidth()
	l := layShields(long, shipShields(gun), 0, w)
	if len(l.head) >= len(long) || !strings.HasSuffix(l.head, "  ") || l.tx+ui.Width(l.tail, smallFace) > w {
		t.Errorf("head %q, tail %q at %v of %v", l.head, l.tail, l.tx, w)
	}
}

// The foreign panel's design line at the three scan levels: unscanned
// none; scanned without Parts one icon by the switch, no number; x-rayed
// ours' row from the Scan's ShieldHits and Absorb, nothing for no shield
// part.
func TestShieldRowForeign(t *testing.T) {
	sc := &game.Scan{Design: game.Design{Name: "D", Hull: game.Small}, Shields: true}
	if st := scanShieldState(sc); st != (shieldState{shown: true, lit: true, hits: 1, absorb: -1}) {
		t.Errorf("no Parts, up: %+v", st)
	}
	sc.ShieldsDown = true
	if st := scanShieldState(sc); st.lit || st.word != "down" {
		t.Errorf("no Parts, down: %+v", st)
	}
	sc.Shields, sc.ShieldsDown = false, false
	if st := scanShieldState(sc); st.lit || st.word != "off" {
		t.Errorf("no Parts, off: %+v", st)
	}
	sc.Parts = true
	if st := scanShieldState(sc); st.shown {
		t.Errorf("Parts, no shield part: %+v", st)
	}
	_, _, _, gun, _ := panelFixture(t)
	sc = &game.Scan{Design: gun.Design, Parts: true, Shields: true, Absorb: gun.Absorb(), ShieldHits: gun.ShieldHits()}
	if st := scanShieldState(sc); st != shipShields(gun) {
		t.Errorf("Parts: %+v, ours %+v", st, shipShields(gun))
	}
}

// The panels render with the game's art: ours with its shield row, a
// foreign one inspected; the icon bank is shipicon.shp's 13x13 shield
// sphere (frame 8, the third of the 13x13 set).
func TestShieldRowRenders(t *testing.T) {
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	as, err := assets.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer as.Close()
	b, err := as.Bank(shipIconBank)
	if err != nil {
		t.Fatal(err)
	}
	if f := b.Frames[shieldIcon]; len(b.Frames) != 12 || f.Box.Dx() != shieldIconW || f.Box.Dy() != shieldIconW || b.Frames[shieldIcon-6].Box.Dx() != 7 {
		t.Fatalf("%d frames, frame %d %v, want 13x13 (frame %d: 7x7)", len(b.Frames), shieldIcon, f.Box, shieldIcon-6)
	}
	a, v, _, gun, raider := panelFixture(t)
	a.a = as
	a.preloadGame()
	frame := ebiten.NewImage(screenW, screenH)
	v.sel = gun
	v.Render(a, frame)
	v.Text(a, frame)
	v.inspect(shipOf(v, raider))
	if v.box() != boxForeign {
		t.Fatalf("box %d, want the foreign panel", v.box())
	}
	v.Render(a, frame)
	v.Text(a, frame)
}
