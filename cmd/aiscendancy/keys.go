package main

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/rewk/aiscendancy/ui"
)

// The key help (F1): keyTable is THE list of the window's bindings, the
// overlay draws it as is; a binding changed in a screen's Update changes
// here too. The overlay is modal over any screen, popups included:
// F1, Esc, the mouse's back button or a left click close it, F3 and F12
// pass (app.Update reads them first), the rest is swallowed. It never
// runs a day.

// keysButton is the "?" button, the mouse's way to F1: the frame's
// bottom-right corner, over the chrome's border, drawn by app.Draw over
// any screen and read by app.Update before it.
var keysButton = rect{screenW - 28, screenH - 28, screenW - 4, screenH - 4}

// keyJustPressed is inpututil.IsKeyJustPressed behind a seam: the tests
// swap it to fake a key.
var keyJustPressed = inpututil.IsKeyJustPressed

type keyRow struct{ keys, what string }

type keySection struct {
	title string
	rows  []keyRow
}

// The sections keysSection names.
const (
	keysEverywhere = "Everywhere"
	keysStarmap    = "Starmap"
	keysSystem     = "System"
	keysPlanet     = "Planet"
	keysResearch   = "Research"
	keysDesign     = "Ship design"
	keysLists      = "Lists (planets, ships, log, news, saves, stats)"
	keysSpecies    = "Species"
	keysPopups     = "Popups"
)

var keyTable = []keySection{
	{keysEverywhere, []keyRow{
		{"Space, mouse forward", "end the day (warns if combat waits);"},
		{"", "skip a replay; next round in combat"},
		{"Esc, mouse back", "back, cancel; last screen: quit"},
		{"Tab", "log"},
		{"S", "species (not in design, negotiation)"},
		{"H", "stats: the history's graphs"},
		{"F1, ? (bottom right)", "this help"},
		{"F2", "settings: music, volumes (saved in the config file)"},
		{"F3", "music on / off"},
		{"F5 / F9", "quick save / load"},
		{"F12", "screenshot"},
		{"wheel", "scroll lists, zoom 3D views"},
	}},
	{keysStarmap, []keyRow{
		{"drag", "rotate"},
		{"wheel", "zoom"},
		{"arrows", "pan"},
		{"click a star", "its system"},
		{"Navigate…: click a star", "send the ship; right click: cancel"},
		{"L", "lanes on / off"},
		{"R", "reset camera"},
	}},
	{keysSystem, []keyRow{
		{"drag", "rotate"},
		{"wheel", "zoom; over the lists, scroll"},
		{"arrows", "pan"},
		{"G / E", "grid / elevation lines"},
		{"R", "reset camera"},
		{"click", "select; a selected planet: open"},
		{"ship: click", "ship selected: send it to a planet, lane end"},
		{"ship: click nothing", "ship selected: height step; click: go"},
		{"right click", "end the step or aim, deselect"},
		{"X", "cancel the order (the panel's Stop)"},
		{"N", "navigate: any system (the panel's Navigate…)"},
		{"M", "repeat the move"},
		{"A", "auto: every ordered ship"},
		{"F", "fire at the hovered ship"},
		{"click a weapon tile", "aim, then click the target"},
		{"D", "shields on / off"},
		{"Esc", "as right click; none left: back"},
	}},
	{keysPlanet, []keyRow{
		{"click", "build, a structure's or ship's dialog"},
		{"click the project panel", "build on several squares"},
		{"right click, Esc", "close the picker; end placing"},
		{"click outside, Esc", "close a dialog"},
	}},
	{keysResearch, []keyRow{
		{"drag, arrows", "turn; move along the tree"},
		{"wheel / R", "zoom / the topic (Select current)"},
		{"click / T, Table, Tree", "study / table or tree"},
	}},
	{keysDesign, []keyRow{
		{"click a part", "add it"},
		{"right click a slot", "remove its part"},
		{"click the hull", "next hull"},
		{"F2, click the title", "edit the name"},
		{"Backspace", "erase"},
		{"Enter", "build (shipyard); end the edit"},
		{"S", "save as template"},
	}},
	{keysLists, []keyRow{
		{"Up / Down", "a line"},
		{"PgUp / PgDn", "a page"},
		{"Home", "top"},
		{"click, right click", "go to the planet, ship, line"},
		{"click outside the log", "close it"},
		{"click a save", "save there / load it"},
		{"hover a save", "its map and graphs"},
		{"click a series", "graph it (stats)"},
	}},
	{keysSpecies, []keyRow{
		{"click a row", "talk to that species"},
		{"Up / Down", "scroll"},
		{"click a line", "say it (negotiation)"},
	}},
	{keysPopups, []keyRow{
		{"Enter, Esc, mouse back", "close (OK)"},
		{"R", "research (discovery)"},
		{"G", "go to the system (exploration)"},
		{"T", "talk (first contact, proposal)"},
		{"A / R", "accept / refuse (proposal)"},
		{"Enter, Y / Esc, N", "yes / no"},
		{"click a star's line", "its system (day's end warning)"},
		{"Space", "nothing: close the popup first"},
		{"any key, click", "skip the intro"},
	}},
}

// keysInput is the overlay's input, without Ebiten: F1 or button (a left
// press in keysButton) opens it; up, it takes the tick (true), closing on
// F1 or close (Esc, back, left click, the button's press included).
func (a *app) keysInput(f1, close, button bool) bool {
	if a.keysUp {
		if f1 || close {
			a.keysUp = false
		}
		return true
	}
	a.keysUp = f1 || button
	return a.keysUp
}

// keysSection is the section of what is up: a popup, else the top screen.
func (a *app) keysSection() string {
	if a.quitting || a.warn != nil || a.ending() || a.discovering() || a.exploring() || a.reporting() || a.contacting() || a.proposing() || a.charging() || a.winning() {
		return keysPopups
	}
	switch s := a.top().(type) {
	case *starmapScreen:
		return keysStarmap
	case *systemScreen:
		return keysSystem
	case *planetScreen:
		if s.confirm != nil {
			return keysPopups
		}
		return keysPlanet
	case *researchScreen:
		return keysResearch
	case *designScreen:
		return keysDesign
	case *slotsScreen:
		if s.confirm != nil {
			return keysPopups
		}
		return keysLists
	case *planetsScreen, *fleetScreen, *logScreen, *statsScreen:
		return keysLists
	case *speciesScreen, *negotiateScreen:
		return keysSpecies
	case *introScreen:
		return keysPopups
	}
	return keysEverywhere
}

// The overlay's layout, native px: two columns from keysX0 and keysX1,
// the keys then the what at keysWhat; sections flow down a column whole.
const (
	keysTop  = 40
	keysX0   = 40
	keysX1   = 660
	keysWhat = 250
	keysBot  = screenH - 10 // the "?" button sits right of any text
)

// drawKeys paints the dimmed backdrop and keyTable over the frame, the
// current section bright, the others dim.
func (a *app) drawKeys(frame *ebiten.Image) {
	vector.FillRect(frame, 0, 0, screenW, screenH, color.RGBA{0, 0, 0, 216}, false)
	lh := bigFace.Size * 1.25
	ui.Print(frame, "Keys   (F1, Esc or a click: close)", bigFace, keysX0, keysTop, footColour)
	cur := a.keysSection()
	x, y := float64(keysX0), keysTop+2*lh
	for _, sec := range keyTable {
		h := float64(len(sec.rows)+1) * lh
		if y+h > keysBot && x == keysX0 {
			x, y = keysX1, keysTop+2*lh
		}
		title, keys, what := textColour, dim(textColour, 0.6), dim(textColour, 0.6)
		if sec.title == cur {
			title, keys, what = yellow, yellow, footColour
		}
		ui.Print(frame, sec.title, bigFace, x, y, title)
		y += lh
		for _, r := range sec.rows {
			ui.Print(frame, r.keys, bigFace, x+12, y, keys)
			ui.Print(frame, r.what, bigFace, x+keysWhat, y, what)
			y += lh
		}
		y += lh / 2
	}
}
