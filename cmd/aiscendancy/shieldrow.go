package main

import (
	"fmt"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// A ship panel's design line: "<design>  " then the
// shields as icons (shipicon.shp's shield sphere, shieldIcon, pixel for
// pixel) one per hit they can still take (ShieldHits), shieldGap apart,
// then " <absorb>" in smallFace. Off, dark or with no energy for a hit:
// one icon dimmed (shieldDim) and " off", " down" or " no power". No
// shield part: nothing after the design. Icons that would pass the
// line's width: one icon and " x<n> <absorb>" (n "∞" for shields that
// draw nothing, game.NoShieldFall). The icons are drawn on the Render
// side, the text on the Text side, both from shieldLine.
const (
	shieldGap = 2   // px between two icons
	shieldDim = 0.4 // a dimmed icon's brightness
)

// shieldState is what a design line shows of the shields: nothing
// (!shown), lit icons (hits of them, absorb printed when known, −1 not),
// or one dimmed icon and word.
type shieldState struct {
	shown, lit bool
	hits       int // lit: the icons, game.NoShieldFall unlimited
	absorb     int // lit: the number, −1 unknown (a scan without Parts)
	word       string
}

// shipShields is our ship's shield row.
func shipShields(sh *game.Starship) shieldState {
	if sh.Design.ShieldScore() == 0 {
		return shieldState{}
	}
	return shieldsOf(sh.Shields, sh.ShieldsDown, sh.ShieldHits(), sh.Absorb())
}

// scanShieldState is a scanned foreign ship's shield row: with Parts as
// ours, from the Scan's ShieldHits and Absorb; without, one icon lit or
// dimmed by Scan.Shields and ShieldsDown, no number. A scan without
// Parts cannot tell a ship without a shield part: it shows the switch.
func scanShieldState(sc *game.Scan) shieldState {
	switch {
	case sc.Parts && sc.Design.ShieldScore() == 0:
		return shieldState{}
	case sc.Parts:
		return shieldsOf(sc.Shields, sc.ShieldsDown, sc.ShieldHits, sc.Absorb)
	case sc.Shields && !sc.ShieldsDown:
		return shieldState{shown: true, lit: true, hits: 1, absorb: -1}
	}
	return shieldsOf(sc.Shields, sc.ShieldsDown, 0, 0)
}

// shieldsOf is a known shield's row: lit with hits > 0 (or unlimited),
// else dimmed off, down or with no power for a hit.
func shieldsOf(on, down bool, hits, absorb int) shieldState {
	switch {
	case !on:
		return shieldState{shown: true, word: "off"}
	case down:
		return shieldState{shown: true, word: "down"}
	case hits == 0:
		return shieldState{shown: true, word: noPower}
	}
	return shieldState{shown: true, lit: true, hits: hits, absorb: absorb}
}

// shieldLine is a design line laid out from x, w px wide: head (the
// design, cut so the row fits) at x, the icons' left edges, their
// brightness, the tail's text and left edge.
type shieldLine struct {
	head  string
	icons []float64
	f     float64
	tail  string
	tx    float64
}

// layShields lays the design line of design and st out at x, w wide.
func layShields(design string, st shieldState, x, w float64) shieldLine {
	if !st.shown {
		return shieldLine{head: fitWidth(design, w, smallFace)}
	}
	num := ""
	if st.absorb >= 0 {
		num = fmt.Sprintf(" %d", st.absorb)
	}
	n, f, tail := 1, 1.0, num
	if !st.lit {
		f, tail = shieldDim, " "+st.word
	} else {
		n = st.hits
	}
	compact := " x∞" + num
	if n > 0 {
		compact = fmt.Sprintf(" x%d%s", n, num)
	}
	room := max(ui.Width(compact, smallFace), ui.Width(tail, smallFace)) // the shortest row's tail
	head := fitWidth(design, w-ui.Width("  ", smallFace)-shieldIconW-room, smallFace) + "  "
	x0 := x + math.Floor(ui.Width(head, smallFace)) // whole px, never past the room kept
	if n < 0 || n > 1 && x0+float64(n*shieldIconW+(n-1)*shieldGap)+ui.Width(tail, smallFace) > x+w {
		n, tail = 1, compact
	}
	l := shieldLine{head: head, f: f, tail: tail}
	for i := range n {
		l.icons = append(l.icons, x0+float64(i*(shieldIconW+shieldGap)))
	}
	l.tx = l.icons[n-1] + shieldIconW
	return l
}

// designLineY is a ship panel's design line top: under the title.
func designLineY() float64 { return float64(sysInfo.Y0+6*zoom) + bigFace.Size*1.25 }

// drawShieldRow draws l's icons on the design line (Render side).
func drawShieldRow(a *app, dst *ebiten.Image, l shieldLine) {
	y := math.Round(designLineY() + (smallFace.Size*1.25-shieldIconW)/2)
	for _, ix := range l.icons {
		drawIcon(dst, a.spr(shipIconBank, shieldIcon), ix, y, l.f)
	}
}

// shieldRowText prints l's head and tail on the design line (Text side).
func shieldRowText(frame *ebiten.Image, l shieldLine, x float64) {
	y := designLineY()
	ui.Print(frame, l.head, smallFace, x, y, textColour)
	if l.tail != "" {
		ui.Print(frame, l.tail, smallFace, l.tx, y, textColour)
	}
}
