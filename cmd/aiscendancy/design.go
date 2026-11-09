package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The ship design screen edits a game.Template: a hull, its parts in
// slots, the allowed parts listed by category, led on the template screen
// by "Best <category>" (Template.Realise picks the day's best; building
// or refitting, a Best part comes in fixed). Only discovered material is
// listed or saved (AddTemplate), so the template realises whole. "Load
// template" lists the templates in the list's place; "Save as template"
// (S) and "Build" (Enter) open the naming box; the name's text mode (F2,
// a click on the title) takes S and Enter as text. Three modes: build (a
// colony), save only (no colony), refit (newRefitScreen: a ship at its
// Docks, the parts aboard edited, the hull locked; "Refit" (Enter) in
// Build's place starts Empire.Refit, "Use template" loads the template
// named as the ship's class when RefitTarget accepts it).
//
// The chrome is 8shipdes.tmp frame 0, measured as transparent runs: title
// box [7,307)x[7,106), part box [315,477)x[7,105), seven button holes
// (row 1 [485,516), [524,594) the 70-wide hull, [602,633) x [7,38); row 2
// [485,516), [524,555), [563,594), [602,633) x [46,77)), the list
// [485,633)x[85,473), the picture box [7,322)x[114,337), the slot hole an
// L: [330,477)x[114,473) and [7,477)x[345,473), overlapping.
// 8shipdes.shp: 0/1 the empty slot's diamond (74x37, origin its centre)
// plain/bright, 2-8 the buttons' pictures in reading order (3 the 70x31
// hull button). gizmos.shp frame g: part g's tile (70x53, origin (34,36)
// on its floor plate); dkshipNN.shp frame h: hull h, 315x223, the picture
// box exactly. Original px (orig).
var (
	desTitle   = orig(7, 7, 307, 106)
	desPart    = orig(315, 7, 477, 105)
	desHull    = orig(524, 7, 594, 38)
	desList    = orig(485, 85, 633, 473)
	desPicture = orig(7, 114, 322, 337)
	desRight   = orig(330, 114, 477, 473)
	desBottom  = orig(7, 345, 477, 473)
	// desSlots holds the L: tiles are clipped to it.
	desSlots = orig(7, 114, 477, 473)
	// The title box's buttons on its bottom row: Load and Save from the
	// left, Build (with a colony only) or Refit (refit mode) at the right,
	// Use template (refit mode) after Save.
	desLoad  = btnRect("Load template", desTitle.X0+padX)
	desSave  = btnRect("Save as template", desLoad.X1+btnGap)
	desBuild = btnRect("Build", desTitle.X1-padX-btnW("Build"))
	desRefit = btnRect("Refit", desTitle.X1-padX-btnW("Refit"))
	desUse   = btnRect(useLabel, desSave.X1+btnGap)
)

// useLabel is the refit mode's template button (it fits between Save
// and Refit: TestRefitButtonsFit).
const useLabel = "Use template"

// desBtnH is a title box button's height, btnGap the room between two.
const desBtnH, btnGap = 28, 12

// btnW is a button's width for label in bigFace, padX a side.
func btnW(label string) int { return int(math.Ceil(ui.Width(label, bigFace))) + 2*padX }

// btnRect is label's button from x on the title box's bottom row.
func btnRect(label string, x int) rect {
	y1 := desTitle.Y1 - padY
	return rect{x, y1 - desBtnH, x + btnW(label), y1}
}

// Button colours: the face, the bevel's light and dark edges.
var (
	btnFace  = color.RGBA{24, 44, 72, 255}
	btnLight = color.RGBA{110, 140, 180, 255}
	btnDark  = color.RGBA{6, 12, 22, 255}
)

// drawButton paints a raised button on frame (native px): r filled
// btnFace, a zoom-px bevel, label centred in it, yellow when hot.
func drawButton(frame *ebiten.Image, r rect, label string, hot bool) {
	x, y, w, h := float32(r.X0), float32(r.Y0), float32(r.X1-r.X0), float32(r.Y1-r.Y0)
	vector.FillRect(frame, x, y, w, h, btnFace, false)
	vector.FillRect(frame, x, y, w, zoom, btnLight, false)
	vector.FillRect(frame, x, y, zoom, h, btnLight, false)
	vector.FillRect(frame, x, y+h-zoom, w, zoom, btnDark, false)
	vector.FillRect(frame, x+w-zoom, y, zoom, h, btnDark, false)
	c := textColour
	if hot {
		c = yellow
	}
	tw := ui.Width(label, bigFace)
	ui.Print(frame, label, bigFace, float64(r.X0+r.X1)/2-tw/2, float64(r.Y0+r.Y1)/2-bigFace.Size*1.25/2, c)
}

// drawButtonOff paints a greyed button: the face and bevel, the label dim.
func drawButtonOff(frame *ebiten.Image, r rect, label string) {
	drawButton(frame, r, "", false)
	tw := ui.Width(label, bigFace)
	ui.Print(frame, label, bigFace, float64(r.X0+r.X1)/2-tw/2, float64(r.Y0+r.Y1)/2-bigFace.Size*1.25/2, dim(textColour, 0.4))
}

// catHole is a category button: its hole and its 8shipdes frame, in
// help.txt's order left to right, top to bottom.
type catHole struct {
	r     rect
	cat   game.GizmoCat
	frame int
}

var catHoles = []catHole{
	{orig(485, 7, 516, 38), game.Weapon, 2},
	{orig(602, 7, 633, 38), game.Shield, 4},
	{orig(485, 46, 516, 77), game.Drive, 5},
	{orig(524, 46, 555, 77), game.Scanner, 6},
	{orig(563, 46, 594, 77), game.Generator, 7},
	{orig(602, 46, 633, 77), game.Special, 8},
}

const (
	desBank   = "8shipdes"
	gizmoBank = "gizmos"
	hullFrame = 3  // the hull button's picture
	emptySlot = 0  // the empty slot's diamond, 1 bright
	nameCap   = 20 // runes in a design's name
	// bestLabelDY puts a Best slot's label on its tile's floor plate.
	bestLabelDY = 5 * zoom
)

// dkBank is the species' big hull pictures.
func dkBank(sp game.Species) string { return fmt.Sprintf("dkship%02d", int(sp)) }

// The slots lie on an iso lattice: slot (u,v)'s 70x34 diamond centred
// on (desSlotX0 + desSlotDU*u, desSlotY0 + desSlotDV*v), u+v even,
// measured on reference/shipdes-*-dosbox.png. Native px.
const (
	desSlotX0 = 44 * zoom
	desSlotY0 = 166 * zoom
	desSlotDU = 36 * zoom
	desSlotDV = 18 * zoom
	// slotHW, slotHH: the hit diamond's half axes, 35x17 orig px, keep
	// two lattice neighbours (36,18) apart.
	slotHW = 35 * zoom
	slotHH = 17 * zoom
)

// slotUV is a slot's lattice coordinates.
type slotUV struct{ u, v int }

// hullSlots is each hull's slots in slot order, by v then u, so painting
// in k order lays a lower tile over the upper one's top.
var hullSlots = [game.NumHulls][]slotUV{
	game.Small:  {{10, 0}, {9, 7}, {3, 13}, {11, 15}, {0, 16}},
	game.Medium: {{10, 0}, {9, 3}, {11, 7}, {11, 11}, {9, 13}, {8, 14}, {3, 15}, {5, 15}, {7, 15}, {0, 16}},
	game.Large: {{10, 0}, {10, 2}, {10, 4}, {11, 7}, {11, 9}, {9, 11}, {2, 12}, {6, 12}, {8, 12}, {5, 13},
		{0, 14}, {2, 14}, {5, 15}, {9, 15}, {0, 16}},
	game.Enormous: {{10, 0}, {11, 1}, {10, 2}, {9, 3}, {11, 5}, {10, 6}, {9, 7}, {11, 9}, {10, 10}, {9, 11},
		{0, 12}, {4, 12}, {3, 13}, {7, 13}, {9, 13}, {11, 13}, {2, 14}, {6, 14}, {1, 15}, {5, 15},
		{9, 15}, {11, 15}, {0, 16}, {4, 16}, {8, 16}},
}

// desSlotCentre is hull h's slot k's diamond centre.
func desSlotCentre(h game.Hull, k int) (float64, float64) {
	p := hullSlots[h][k]
	return float64(desSlotX0 + desSlotDU*p.u), float64(desSlotY0 + desSlotDV*p.v)
}

// slotAnchor is hull h's slot k's tile origin: the floor plate's centre
// is 2 orig px above it, so 2 under the diamond's centre.
func slotAnchor(h game.Hull, k int) (float64, float64) {
	x, y := desSlotCentre(h, k)
	return x, y + 2*zoom
}

// slotAt is hull h's slot whose diamond holds (x,y), -1 none.
func slotAt(h game.Hull, x, y float64) (int, bool) {
	for k := range hullSlots[h] {
		cx, cy := desSlotCentre(h, k)
		if math.Abs(x-cx)/slotHW+math.Abs(y-cy)/slotHH <= 1 {
			return k, true
		}
	}
	return 0, false
}

// The list: tiles at scale 1, listCols at listPitchX (4 x 74 = 296 px),
// rows listPitchY apart under the view toggles; 12 rows hold every
// category (the Specials' 44 the most): no scroll. The detailed view
// puts one entry a row, the tile then its name, numbers and help's
// start, detRows at a time, the wheel scrolling them. Native px.
const (
	listCols   = 4
	listPitchX = 74
	listPitchY = 60
	listCellW  = 70
	listCellH  = 54
	listCells  = 48
	listY0     = 36 // under the toggles; 36 + 12 x 60 = 756 of 776
	listOX     = 34 // the tile's origin in its cell
	listOY     = 36
	detRows    = 12
	detTextX   = listPitchX + 4 // the text's x in a row
)

// The view toggles on the list's top: Compact then Detailed.
var (
	desCompact  = rect{desList.X0 + padX, desList.Y0 + 6, desList.X0 + padX + 90, desList.Y0 + 28}
	desDetailed = rect{desCompact.X1 + 8, desCompact.Y0, desCompact.X1 + 8 + 90, desCompact.Y1}
)

// listCell is list cell i's tile box in the compact view.
func listCell(i int) rect {
	x := desList.X0 + listPitchX*(i%listCols)
	y := desList.Y0 + listY0 + listPitchY*(i/listCols)
	return rect{x, y, x + listCellW, y + listCellH}
}

// entryCell is list entry i's tile box and its hit box (the whole row in
// the detailed view); false when not shown (past the cells, scrolled
// off).
func (s *designScreen) entryCell(i int) (tile, hit rect, ok bool) {
	if !s.detailed {
		c := listCell(i)
		return c, c, i < listCells
	}
	k := i - s.listOff
	y := desList.Y0 + listY0 + listPitchY*k
	tile = rect{desList.X0, y, desList.X0 + listCellW, y + listCellH}
	return tile, rect{tile.X0, tile.Y0, desList.X1 - padX, tile.Y1}, k >= 0 && k < detRows
}

// listScroll moves the detailed list by the wheel over it, a row a notch,
// clamped (a category switch went back to the top).
func (s *designScreen) listScroll(a *app, r *game.Research) {
	n := len(s.list(r))
	if x, y := a.mouse(); s.detailed && desList.has(x, y) {
		if _, wy := ebiten.Wheel(); wy != 0 {
			s.listOff += notch(wy, 1)
		}
	}
	s.listOff = min(max(s.listOff, 0), max(0, n-detRows))
}

// The pick mode's template lines, pickPitch apart.
const (
	pickPitch = 40
	pickLines = 19 // (776 - padY) / 40
)

// pickTop is the native y of template line k's band.
func pickTop(k int) int { return desList.Y0 + padY + pickPitch*k }

type hoverKind uint8

const (
	hoverNone  hoverKind = iota
	hoverList            // index: the gizmo
	hoverSlot            // index: the slot
	hoverLoad            // the "Load template" button
	hoverBuild           // the "Build" button
	hoverSave            // the "Save as template" button
	hoverPick            // index: the template line
	hoverRefit           // the "Refit" button
	hoverUse             // the "Use template" button
)

type designScreen struct {
	t   game.Template // what is edited and saved
	d   game.Design   // t realised against the research (realise)
	col *game.Colony  // where Build builds, nil: the screen only saves
	// ship, when set, is shown read-only (the ship dialog's Show).
	ship *game.Starship
	// refit, when set, is the ship refitted (col its Docks' colony): the
	// hull is locked, Refit replaces Build.
	refit    *game.Starship
	editing  bool       // the name's text mode
	picking  bool       // the template pick mode: the list shows the templates
	box      *nameBox   // the naming box, nil when closed
	loaded   string     // the last template loaded or saved, "" none or since a hull switch
	helpOf   game.Gizmo // the part whose help shows (helpScroll), if helpOn
	helpOn   bool
	helpOff  int  // its first line shown
	detailed bool // the list's detailed view (app.desDetailed: kept while the game runs)
	// held is the part picked in the list, placed by a click on a slot
	// (several times), when holding; Esc or a right click drops it.
	held     game.Part
	holding  bool
	listOff  int // the detailed view's first row shown
	cat      game.GizmoCat
	hover    hoverKind
	index    int // hoverList: the entry in list, hoverSlot: the slot, hoverPick: the template
	tick     int
	msg      string // the last save's or add's verdict, shown until msgUntil
	msgErr   bool
	msgUntil int
}

// typing reports the name takes typed text (Space and Tab included).
func (s *designScreen) typing() bool { return s.editing || s.box != nil }

// cancel closes the naming box, else the pick mode, else the text
// mode, on Esc.
func (s *designScreen) cancel() bool {
	if s.box != nil {
		s.box = nil
		return true
	}
	if s.holding {
		s.holding = false
		return true
	}
	if s.picking {
		s.picking = false
		return true
	}
	if s.editing {
		s.editing = false
		return true
	}
	return false
}

// newDesignScreenFor edits a copy of t (parts cloned), building at col
// when not nil. The drives are listed first.
func newDesignScreenFor(col *game.Colony, t game.Template) *designScreen {
	t.Parts = slices.Clone(t.Parts)
	return &designScreen{t: t, col: col, cat: game.Drive}
}

// newShipViewScreen shows sh's parts aboard, read-only.
func newShipViewScreen(sh *game.Starship) *designScreen {
	t := sh.Aboard().Fixed()
	t.Name = sh.Name
	s := newDesignScreenFor(nil, t)
	s.ship = sh
	return s
}

// newRefitScreen edits sh's parts aboard (Starship.Aboard) for a refit at
// col's Docks, named as its class (the loaded template's name).
func newRefitScreen(col *game.Colony, sh *game.Starship) *designScreen {
	t := sh.Aboard().Fixed()
	t.Name = sh.Design.Name
	s := newDesignScreenFor(col, t)
	s.loaded, s.refit = sh.Design.Name, sh
	return s
}

// newDesignScreen starts a fresh template building nowhere.
func newDesignScreen() *designScreen { return newDesignScreenFor(nil, freshTemplate()) }

// freshTemplate is a Small hull, no parts, no name.
func freshTemplate() game.Template { return game.Template{Hull: game.Small} }

// Enter names an unnamed template (defaultName), takes the list's view
// and realises it.
func (s *designScreen) Enter(a *app) {
	s.detailed = a.desDetailed
	if s.t.Name == "" {
		s.t.Name = defaultName(a)
	}
	if a.e != nil {
		s.realise(a.e.Research)
	}
}

// defaultName is "Design N", N the next in the empire's list.
func defaultName(a *app) string {
	n := 1
	if a.e != nil {
		n = len(a.e.Templates) + 1
	}
	return fmt.Sprintf("Design %d", n)
}

// realise recomputes d from t at every change and tick; Realise's ok is
// moot: only discovered parts are held.
func (s *designScreen) realise(r *game.Research) { s.d, _ = s.t.Realise(r) }

// realisePart is the gizmo part p stands for today; ok false for a Best
// part with none allowed yet.
func realisePart(r *game.Research, p game.Part) (game.Gizmo, bool) {
	d, ok := game.Template{Hull: game.Small, Parts: []game.Part{p}}.Realise(r)
	if !ok {
		return 0, false
	}
	return d.Parts[0], true
}

// list is s.cat's entries: "Best <cat>" when research allows one (not
// the Specials, templating only), then the allowed parts in table order.
func (s *designScreen) list(r *game.Research) []game.Part {
	var out []game.Part
	if s.ship != nil { // read-only: the parts aboard of the category, each once
		for _, p := range s.t.Parts {
			if p.Cat == s.cat && !slices.Contains(out, p) {
				out = append(out, p)
			}
		}
		return out
	}
	if b := (game.Part{Best: true, Cat: s.cat}); s.cat != game.Special && s.templating() {
		if _, ok := realisePart(r, b); ok {
			out = append(out, b)
		}
	}
	for g := game.Gizmo(0); g < game.NumGizmos; g++ {
		if game.Gizmos[g].Cat == s.cat && r.AllowsGizmo(g) {
			out = append(out, game.Part{Gizmo: g, Cat: s.cat})
		}
	}
	return out
}

// entryName is "Best <cat>" or the gizmo's name.
func entryName(p game.Part) string {
	if p.Best {
		return "Best " + p.Cat.String()
	}
	return p.Gizmo.String()
}

// listAt is the list entry whose cell (row in the detailed view) holds
// (x,y).
func (s *designScreen) listAt(r *game.Research, x, y float64) (int, bool) {
	if !desList.has(x, y) {
		return 0, false
	}
	for i := range s.list(r) {
		if _, hit, ok := s.entryCell(i); ok && hit.has(x, y) {
			return i, true
		}
	}
	return 0, false
}

// setView switches the list to the detailed view or back, kept for the
// next design screens.
func (s *designScreen) setView(a *app, detailed bool) {
	s.detailed, a.desDetailed, s.listOff = detailed, detailed, 0
}

// viewClick flips the view on a toggle under (x,y); true when one was.
func (s *designScreen) viewClick(a *app, x, y float64) bool {
	switch {
	case desCompact.has(x, y):
		s.setView(a, false)
	case desDetailed.has(x, y):
		s.setView(a, true)
	default:
		return false
	}
	return true
}

// catClick switches the category on a button under (x,y), the detailed
// list back to its top.
func (s *designScreen) catClick(x, y float64) {
	for _, h := range catHoles {
		if h.r.has(x, y) {
			s.cat, s.listOff = h.cat, 0
		}
	}
}

// pickAt is the template line whose band holds (x,y), -1 none.
func (s *designScreen) pickAt(a *app, x, y float64) (int, bool) {
	if !desList.has(x, y) || y < float64(pickTop(0)) {
		return 0, false
	}
	k := int(y-float64(pickTop(0))) / pickPitch
	if k >= min(len(a.e.Templates), pickLines) {
		return 0, false
	}
	return k, true
}

// pickLine is a template's line in the pick mode.
func pickLine(t game.Template) string {
	return fmt.Sprintf("%s %s %d parts", t.Name, t.Hull, len(t.Parts))
}

// load puts a copy of t in the editor, ends the pick mode, keeps t's
// name for the build box.
func (s *designScreen) load(r *game.Research, t game.Template) {
	t.Parts = slices.Clone(t.Parts)
	if !s.templating() {
		t = fixedNow(r, t)
	}
	s.t, s.picking, s.loaded = t, false, t.Name
	s.realise(r)
}

// templating reports the screen edits templates only (no colony, not a
// ship's view): the one place the Best entries are listed (a Best part
// is a template's, a ship built or refitted now takes today's best as a
// fixed part).
func (s *designScreen) templating() bool { return s.col == nil && s.ship == nil }

// fixedNow is t with every Best part turned into today's gizmo (one
// research allows none is dropped, its slot left empty).
func fixedNow(r *game.Research, t game.Template) game.Template {
	d, _ := t.Realise(r)
	return d.Fixed()
}

// loadOrRefuse loads t, or in the refit mode refuses another hull's
// template (nothing loaded, the pick mode kept).
func (s *designScreen) loadOrRefuse(r *game.Research, t game.Template) {
	if s.refit != nil && t.Hull != s.t.Hull {
		s.setMsg(fmt.Sprintf("a refit keeps the hull: %s is %s", t.Name, t.Hull), true)
		return
	}
	s.load(r, t)
}

// useTemplate is the refit's template (Empire.RefitTarget accepts it: a
// template named as the ship's class, realisable, same hull), ok false
// when none or not in the refit mode.
func (s *designScreen) useTemplate(e *game.Empire) (game.Template, bool) {
	if s.refit == nil {
		return game.Template{}, false
	}
	if _, err := e.RefitTarget(s.refit); err != nil {
		return game.Template{}, false
	}
	i := slices.IndexFunc(e.Templates, func(t game.Template) bool { return t.Name == s.refit.Design.Name })
	return e.Templates[i], true // RefitTarget found it
}

// startRefit is the Refit button: Empire.Refit of the ship to the design
// under the title's name (its new class), reporting started; a refusal
// (CanRefit) is shown and the screen stays.
func (s *designScreen) startRefit(a *app) bool {
	if strings.TrimSpace(s.t.Name) == "" { // the class names the design: as submit
		s.setMsg("a name is needed", true)
		return false
	}
	d := s.d
	d.Name = s.t.Name
	if err := a.e.Refit(s.col, s.refit, d); err != nil {
		s.setMsg(err.Error(), true)
		return false
	}
	return true
}

// setMsg shows m for two seconds, in errColour when bad.
func (s *designScreen) setMsg(m string, bad bool) {
	s.msg, s.msgErr, s.msgUntil = m, bad, s.tick+120
}

// add puts p in the first empty slot (a Best part stays Best); refused
// when the hull is full.
func (s *designScreen) add(r *game.Research, p game.Part) error {
	for k := range s.t.Hull.Slots() {
		if _, full := s.t.PartAt(k); !full {
			s.place(r, p, k)
			return nil
		}
	}
	return fmt.Errorf("no free slot")
}

// place puts p in slot k, replacing the part there (Template.Place).
func (s *designScreen) place(r *game.Research, p game.Part, k int) {
	s.t = s.t.Place(p, k)
	s.realise(r)
}

// remove empties slot k, the others staying (Template.Unplace).
func (s *designScreen) remove(r *game.Research, k int) {
	s.t = s.t.Unplace(k)
	s.realise(r)
}

// cycleHull switches to the next allowed hull, dropping the parts past
// its slots; loaded is cleared (another hull's template never refits).
func (s *designScreen) cycleHull(r *game.Research) {
	for i := 1; i <= int(game.NumHulls); i++ {
		h := game.Hull((int(s.t.Hull) + i) % int(game.NumHulls))
		if r.Known[h.Tech()] {
			if h != s.t.Hull {
				s.loaded = ""
			}
			s.t.Hull = h
			break
		}
	}
	s.t = s.t.FitHull(s.t.Hull) // the parts past its slots go
	s.realise(r)
}

// leftClick reports the screen is done: the naming box takes every
// click; Build and Save open it, Load toggles the pick mode; else the
// title, a category, the hull button, a list entry.
func (s *designScreen) leftClick(a *app, x, y float64) bool {
	r := a.e.Research
	if s.ship != nil { // read-only: the category buttons and the view only, to browse the parts
		s.catClick(x, y)
		s.viewClick(a, x, y)
		return false
	}
	if s.box != nil {
		_, yes, no := s.box.prompt().rects()
		switch {
		case yes.has(x, y):
			return s.submit(a)
		case no.has(x, y):
			s.box = nil
		}
		return false
	}
	if s.refit != nil {
		if desRefit.has(x, y) {
			return s.startRefit(a)
		}
		if t, ok := s.useTemplate(a.e); ok && desUse.has(x, y) {
			s.load(r, t) // load fixes its Best parts (no Best off the template screen)
			return false
		}
	} else if s.col != nil && desBuild.has(x, y) {
		s.openBox(a, nameBuild)
		return false
	}
	if desSave.has(x, y) {
		s.openBox(a, nameSave)
		return false
	}
	if desLoad.has(x, y) {
		s.picking, s.editing = !s.picking, false
		return false
	}
	if s.picking {
		if k, ok := s.pickAt(a, x, y); ok {
			s.loadOrRefuse(r, a.e.Templates[k])
		}
		return false
	}
	if desTitle.has(x, y) {
		s.editing = !s.editing
	}
	s.catClick(x, y)
	if s.viewClick(a, x, y) {
		return false
	}
	if desHull.has(x, y) {
		if s.refit != nil {
			s.setMsg("a refit keeps the hull", true)
		} else {
			s.cycleHull(r)
		}
	}
	if k, ok := s.listAt(r, x, y); ok {
		p := s.list(r)[k]
		s.held, s.holding = p, !(s.holding && s.held == p)
	}
	if k, ok := slotAt(s.t.Hull, x, y); ok && s.holding && k < s.t.Hull.Slots() {
		s.place(r, s.held, k)
	}
	return false
}

// rightClick empties the slot under (x,y), else drops the held part.
func (s *designScreen) rightClick(r *game.Research, x, y float64) {
	if s.picking || s.ship != nil || s.box != nil {
		return
	}
	if k, ok := slotAt(s.t.Hull, x, y); ok {
		if _, full := s.t.PartAt(k); full {
			s.remove(r, k)
			return
		}
	}
	s.holding = false
}

// typeInto is name with in's printable runes appended, nameCap at most.
func typeInto(name string, in []rune) string {
	for _, c := range in {
		if unicode.IsPrint(c) && utf8.RuneCountInString(name) < nameCap {
			name += string(c)
		}
	}
	return name
}

// unType is name without its last rune.
func unType(name string) string {
	if r := []rune(name); len(r) > 0 {
		return string(r[:len(r)-1])
	}
	return name
}

// typed appends in's printable runes to the name.
func (s *designScreen) typed(in []rune) {
	s.t.Name = typeInto(s.t.Name, in)
	s.d.Name = s.t.Name
}

// backspace deletes the name's last rune.
func (s *designScreen) backspace() {
	s.t.Name = unType(s.t.Name)
	s.d.Name = s.t.Name
}

// nameMode is what the naming box's first button does.
type nameMode uint8

const (
	nameSave  nameMode = iota // save the template under the name
	nameBuild                 // build the design under the name
)

// nameBox is the modal naming box: the prompt's box and buttons
// (quit.go), a field under the title, the last refusal in red.
type nameBox struct {
	mode nameMode
	name string
	err  string
	at   string // nameBuild: the colony's planet, for the title
	// class is nameBuild's design name (a refit finds the template by
	// it); name is the ship's own.
	class string
}

// prompt is the box's title and buttons, for the geometry and drawing.
func (b *nameBox) prompt() prompt {
	if b.mode == nameBuild {
		return prompt{[]string{"Build a ship at " + b.at}, "Build (Enter)", "Cancel (Esc)"}
	}
	return prompt{[]string{"Save as template"}, "Save (Enter)", "Cancel (Esc)"}
}

// boxFieldY, boxErrY are the field's and the refusal's y: the
// prompt's second and third lines.
func boxFieldY() float64 { return float64(dlgBox.Y0 + dlgTitleY + zoom*promptLineH) }
func boxErrY() float64   { return float64(dlgBox.Y0 + dlgTitleY + 2*zoom*promptLineH) }

// openBox opens the naming box in mode m, ending the pick and text
// modes: the save box on the design's name, the build box on the class
// (the last loaded or saved template's, else the hull's) and the ship
// name the game would give (Empire.ShipNameFor).
func (s *designScreen) openBox(a *app, m nameMode) {
	s.picking, s.editing = false, false
	b := &nameBox{mode: m, name: s.t.Name}
	switch {
	case m == nameBuild:
		b.class, b.at = s.loaded, s.col.Planet.Name
		if b.class == "" {
			b.class = s.t.Hull.String()
		}
		b.name = a.e.ShipNameFor(b.class)
	case b.name == "":
		b.name = defaultName(a)
	}
	s.box = b
}

// submit is the box's first button, reporting built. A blank name is
// refused; save stores a copy under the name, the design taking it;
// build starts the design as its class at col, the ship bearing the
// typed name. A refusal stays in the box.
func (s *designScreen) submit(a *app) bool {
	b := s.box
	if strings.TrimSpace(b.name) == "" {
		b.err = "a name is needed"
		return false
	}
	t := s.t
	t.Name = b.name
	if b.mode == nameBuild {
		t.Name = b.class
	}
	var err error
	d, _ := t.Realise(a.e.Research)
	if b.mode == nameBuild {
		err = a.e.BuildShipNamed(s.col, d, b.name)
	} else {
		err = a.e.AddTemplate(t)
	}
	if err != nil {
		b.err = err.Error()
		return false
	}
	s.t.Name, s.d, s.box = t.Name, d, nil
	if b.mode == nameBuild {
		return true
	}
	s.loaded = t.Name // the design is that class now: the build box's name
	s.setMsg("saved", false)
	return false
}

// desKeys are the keys handleKeys acts on, as just pressed this tick.
type desKeys struct{ f2, enter, s, backspace bool }

// handleKeys is the keyboard without Ebiten: the chars go to the naming
// box's field or, in the text mode, the name; Enter submits or ends the
// mode; else F2 enters it, S opens the save box, Enter the build box
// (the refit mode: Enter refits, as the Refit button).
func (s *designScreen) handleKeys(a *app, chars []rune, k desKeys) (built bool) {
	if s.ship != nil {
		return false
	}
	if b := s.box; b != nil {
		b.name = typeInto(b.name, chars)
		if k.backspace {
			b.name = unType(b.name)
		}
		return k.enter && s.submit(a)
	}
	if s.editing {
		s.typed(chars)
		if k.backspace {
			s.backspace()
		}
		if k.enter || k.f2 {
			s.editing = false
		}
		return false
	}
	switch {
	case k.f2:
		s.editing = true
	case k.s:
		s.openBox(a, nameSave)
	case k.enter && s.refit != nil:
		return s.startRefit(a)
	case k.enter && s.col != nil:
		s.openBox(a, nameBuild)
	}
	return false
}

// hoverAt finds what is under (x,y): nothing under the naming box; the
// title buttons; a template line in the pick mode; a list entry or slot.
func (s *designScreen) hoverAt(a *app, x, y float64) {
	r := a.e.Research
	s.hover = hoverNone
	if s.box != nil {
		return
	}
	_, use := s.useTemplate(a.e)
	if s.refit != nil && desRefit.has(x, y) {
		s.hover = hoverRefit
	} else if use && desUse.has(x, y) {
		s.hover = hoverUse
	} else if s.refit == nil && s.col != nil && desBuild.has(x, y) {
		s.hover = hoverBuild
	} else if s.ship == nil && desLoad.has(x, y) {
		s.hover = hoverLoad
	} else if s.ship == nil && desSave.has(x, y) {
		s.hover = hoverSave
	} else if s.picking {
		if k, ok := s.pickAt(a, x, y); ok {
			s.hover, s.index = hoverPick, k
		}
	} else if k, ok := s.listAt(r, x, y); ok {
		s.hover, s.index = hoverList, k
	} else if k, ok := slotAt(s.t.Hull, x, y); ok && k < s.t.Hull.Slots() {
		s.hover, s.index = hoverSlot, k
	}
}

// hoveredPart is the hovered entry's or filled slot's part.
func (s *designScreen) hoveredPart(r *game.Research) (game.Part, bool) {
	switch l := s.list(r); {
	case s.hover == hoverList && s.index < len(l):
		return l[s.index], true
	case s.hover == hoverSlot:
		if i, ok := s.t.PartAt(s.index); ok {
			return s.t.Parts[i], true
		}
	}
	return game.Part{}, false
}

// shown is the gizmo the part box describes: the held part realised
// (its description stays, a hover changes nothing),
// else the hovered one.
func (s *designScreen) shown(r *game.Research) (game.Gizmo, bool) {
	p, ok := s.held, s.holding && s.ship == nil
	if !ok {
		p, ok = s.hoveredPart(r)
	}
	if !ok {
		return 0, false
	}
	return realisePart(r, p)
}

func (s *designScreen) Update(a *app) error {
	s.tick++
	r := a.e.Research
	s.realise(r) // the day may have moved the research
	x, y := a.mouse()
	s.listScroll(a, r)
	s.hoverAt(a, x, y)
	s.helpScroll(a, r)
	switch {
	case inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft):
		if s.leftClick(a, x, y) {
			return a.pop()
		}
	case inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight):
		s.rightClick(r, x, y)
	}
	// Enter, Backspace and Esc never come through AppendInputChars.
	k := desKeys{
		f2:        inpututil.IsKeyJustPressed(ebiten.KeyF2),
		enter:     inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter),
		s:         inpututil.IsKeyJustPressed(ebiten.KeyS),
		backspace: inpututil.IsKeyJustPressed(ebiten.KeyBackspace),
	}
	if s.handleKeys(a, ebiten.AppendInputChars(nil), k) {
		return a.pop()
	}
	s.hoverAt(a, x, y) // the click may have changed the slots
	return nil
}

// clipped is sc cut to r, so a spilling tile does not paint the chrome.
func clipped(sc *ebiten.Image, r rect) *ebiten.Image {
	return sc.SubImage(image.Rect(r.X0, r.Y0, r.X1, r.Y1)).(*ebiten.Image)
}

// desChrome paints black, the chrome, then every hole black.
func desChrome(a *app, sc *ebiten.Image) {
	sc.Fill(color.Black)
	drawSprite(sc, a.spr("8shipdes.tmp", 0), 0, 0)
	for _, r := range []rect{desTitle, desPart, desHull, desList, desPicture, desRight, desBottom} {
		fillRect(sc, r)
	}
	for _, h := range catHoles {
		fillRect(sc, h.r)
	}
}

// drawTile draws gizmo g's tile at zoom at hull h's slot k.
func drawTile(a *app, sc *ebiten.Image, h game.Hull, k int, g game.Gizmo) {
	drawTileAlpha(a, sc, h, k, g, 1)
}

// drawTileAlpha is drawTile at opacity al.
func drawTileAlpha(a *app, sc *ebiten.Image, h game.Hull, k int, g game.Gizmo, al float32) {
	x, y := slotAnchor(h, k)
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(zoom, zoom)
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleAlpha(al)
	a.spr(gizmoBank, int(g)).Draw(clipped(sc, desSlots), &op)
}

// drawEmptySlot draws hull h's slot k's diamond, bright when lit.
func drawEmptySlot(a *app, sc *ebiten.Image, h game.Hull, k int, lit bool) {
	f := emptySlot
	if lit {
		f++
	}
	x, y := desSlotCentre(h, k)
	drawSprite(clipped(sc, desSlots), a.spr(desBank, f), x, y)
}

// drawPicture paints the hull picture over its blacked box.
func drawPicture(a *app, sc *ebiten.Image, sp game.Species, h game.Hull) {
	fillRect(sc, desPicture)
	drawSprite(sc, a.spr(dkBank(sp), int(h)), float64(desPicture.X0), float64(desPicture.Y0))
}

// drawListTile draws list entry i, part p, as its realised tile.
func (s *designScreen) drawListTile(a *app, sc *ebiten.Image, r *game.Research, i int, p game.Part) {
	c, _, ok := s.entryCell(i)
	if g, real := realisePart(r, p); ok && real {
		a.spr(gizmoBank, int(g)).DrawAt(clipped(sc, desList), float64(c.X0+listOX), float64(c.Y0+listOY))
	}
}

// printEntry writes list entry i's text in the detailed view: its name,
// then dim its power (drawn, or supplied by a generator) and industry;
// ours, the original lists tiles only.
func (s *designScreen) printEntry(frame *ebiten.Image, r *game.Research, i int, p game.Part, c color.Color) {
	tile, hit, ok := s.entryCell(i)
	if !ok {
		return
	}
	x, w := float64(tile.X0+detTextX), float64(hit.X1-tile.X0-detTextX)
	y := float64(tile.Y0 + 2)
	ui.Print(frame, fitWidth(entryName(p), w, bigFace), bigFace, x, y, c)
	if g, real := realisePart(r, p); real {
		ui.Print(frame, fitWidth(entryCost(g), w, smallFace), smallFace, x, y+bigFace.Size*1.25, dim(textColour, 0.6))
	}
}

// entryCost is "power N  industry N" ("supplies N" for a generator, no
// power when it draws none).
func entryCost(g game.Gizmo) string {
	in := game.Gizmos[g]
	switch {
	case in.Cat == game.Generator:
		return fmt.Sprintf("supplies %d  industry %d", in.Power, in.Industry)
	case in.Power != 0:
		return fmt.Sprintf("power %d  industry %d", in.Power, in.Industry)
	}
	return fmt.Sprintf("industry %d", in.Industry)
}

// drawToggle paints a view toggle: framed green with its label when on.
func drawToggle(frame *ebiten.Image, r rect, label string, on bool) {
	c := dim(textColour, 0.5)
	if on {
		c = toggleOn
	}
	strokeRect(frame, r, c)
	w := ui.Width(label, smallFace)
	ui.Print(frame, label, smallFace, float64(r.X0+r.X1)/2-w/2, float64(r.Y0+r.Y1)/2-smallFace.Size*0.6, c)
}

// slotLabel writes label on a dark backing under slot k's tile origin.
func slotLabel(frame *ebiten.Image, h game.Hull, k int, label string, c color.Color) {
	ax, ay := slotAnchor(h, k)
	w := ui.Width(label, smallFace)
	lx, ly := ax-w/2, ay+bestLabelDY
	vector.FillRect(frame, float32(lx-3), float32(ly-1), float32(w+6), float32(smallFace.Size+3), color.RGBA{0, 0, 0, 200}, false)
	ui.Print(frame, label, smallFace, lx, ly, c)
}

// cellLabel writes label at the bottom of list cell i, on a dark backing.
func cellLabel(frame *ebiten.Image, i int, label string, c color.Color) {
	cl := listCell(i)
	w := ui.Width(label, smallFace)
	lx, ly := float64(cl.X0+cl.X1)/2-w/2, float64(cl.Y1)-smallFace.Size-2
	vector.FillRect(frame, float32(lx-2), float32(ly), float32(w+4), float32(smallFace.Size+2), color.RGBA{0, 0, 0, 200}, false)
	ui.Print(frame, label, smallFace, lx, ly, c)
}

// printPart writes gizmo g in the part box: name, facts, help from line
// off (the wheel scrolls a help too long for the box: helpScroll), the
// scroll cues.
func printPart(frame *ebiten.Image, a *app, g game.Gizmo, off int) {
	px := float64(desPart.X0 + padX)
	pw := float64(desPart.X1 - desPart.X0 - 2*padX)
	y := float64(desPart.Y0 + padY)
	ui.Print(frame, fitWidth(g.String(), pw, bigFace), bigFace, px, y, textColour)
	facts := partFacts(g)
	for i := range facts {
		facts[i] = fitWidth(facts[i], pw, smallFace)
	}
	ui.Print(frame, strings.Join(facts, "\n"), smallFace, px, y+bigFace.Size*1.25, textColour)
	ls, n := partHelpLines(a, g), partHelpRows(g)
	off = min(max(off, 0), max(0, len(ls)-n))
	ui.Print(frame, strings.Join(ls[off:min(len(ls), off+n)], "\n"), smallFace, px, partHelpTop(g), textColour)
	top := int(partHelpTop(g))
	drawScrollCues(a, frame, rect{desPart.X0, top, desPart.X1, desPart.Y1}, off, n, len(ls))
}

// partHelpRows is how many help lines the part box shows for g.
func partHelpRows(g game.Gizmo) int { return fitLines(partHelpTop(g), float64(desPart.Y1-padY)) }

// helpScroll moves the shown part's help by the wheel, a line a notch,
// back to its top when the part changes.
func (s *designScreen) helpScroll(a *app, r *game.Research) {
	g, ok := s.shown(r)
	if !ok || !s.helpOn || g != s.helpOf {
		s.helpOff, s.helpOf, s.helpOn = 0, g, ok
		return
	}
	if x, y := a.mouse(); s.detailed && desList.has(x, y) {
		return // the wheel scrolls the list (listScroll)
	}
	if _, wy := ebiten.Wheel(); wy != 0 {
		ls := len(partHelpLines(a, g))
		s.helpOff = min(max(s.helpOff+notch(wy, 1), 0), max(0, ls-partHelpRows(g)))
	}
}

// partHelpTop is the y of g's paragraph, after a blank line.
func partHelpTop(g game.Gizmo) float64 {
	return float64(desPart.Y0+padY) + bigFace.Size*1.25 + float64(len(partFacts(g))+1)*smallH
}

// partHelpLines is g's whole paragraph wrapped to the part box's width.
func partHelpLines(a *app, g game.Gizmo) []string {
	return helpLines(a.gizmoHelp(g), float64(desPart.X1-desPart.X0-2*padX), math.MaxInt)
}

// printHelp writes help in the part box, in smallFace.
func printHelp(frame *ebiten.Image, help string) {
	ui.Print(frame, help, smallFace, float64(desPart.X0+padX), float64(desPart.Y0+padY), textColour)
}

// Render paints the chrome, the buttons, the slots in k order, the hull
// over them and the list's tiles (none in the pick mode).
func (s *designScreen) Render(a *app, sc *ebiten.Image) {
	desChrome(a, sc)
	for _, h := range catHoles {
		drawSprite(sc, a.spr(desBank, h.frame), float64(h.r.X0), float64(h.r.Y0))
		if h.cat == s.cat {
			strokeRect(sc, h.r, yellow)
		}
	}
	drawSprite(sc, a.spr(desBank, hullFrame), float64(desHull.X0), float64(desHull.Y0))

	r := a.e.Research
	for k := range s.t.Hull.Slots() {
		var g game.Gizmo
		filled := false
		if i, ok := s.t.PartAt(k); ok {
			g, filled = realisePart(r, s.t.Parts[i])
		}
		hot := s.hover == hoverSlot && s.index == k
		if hg, ok := realisePart(r, s.held); hot && s.holding && ok {
			if !filled {
				drawEmptySlot(a, sc, s.t.Hull, k, true)
			}
			drawTileAlpha(a, sc, s.t.Hull, k, hg, 0.6) // where the held part would go
			continue
		}
		if filled {
			drawTile(a, sc, s.t.Hull, k, g)
			continue
		}
		// An empty slot: the bright diamond when hovered.
		drawEmptySlot(a, sc, s.t.Hull, k, hot)
	}
	drawPicture(a, sc, a.e.Species, s.t.Hull)

	if s.picking {
		return
	}
	l := s.list(r)
	for i := range l {
		s.drawListTile(a, sc, r, i, l[i])
	}
	if i := slices.Index(l, s.held); s.holding && i >= 0 {
		if _, hit, ok := s.entryCell(i); ok {
			strokeRect(sc, hit, toggleOn) // the held part, green
		}
	}
	if _, hit, ok := s.entryCell(s.index); s.hover == hoverList && s.index < len(l) && ok {
		strokeRect(sc, hit, yellow)
	}
}

// partFacts is part g's numbers by category, then its industry.
func partFacts(g game.Gizmo) []string {
	in := game.Gizmos[g]
	var f string
	switch in.Cat {
	case game.Weapon:
		f = fmt.Sprintf("damage %d  range %d  power %d  uses %d", in.Level, in.Range, in.Power, in.Uses)
	case game.Shield:
		f = fmt.Sprintf("strength %d  power %d", in.Level, in.Power) // reserved off the energy while on (Starship.SetShields)
	case game.Drive:
		f = fmt.Sprintf("reach %d cubes  power %d", in.Level, in.Power)
	case game.Scanner:
		f = fmt.Sprintf("range %d", in.Range)
	case game.Generator:
		f = fmt.Sprintf("supplies %d", in.Power)
	case game.Special:
		if in.Power != 0 {
			f = fmt.Sprintf("power %d", in.Power)
		}
	}
	out := []string{}
	if f != "" {
		out = append(out, f)
	}
	return append(out, fmt.Sprintf("industry %d", in.Industry))
}

// buildTime is "N days" at the colony's industry today (DaysFor),
// "never: no industry", "busy: " first while another project runs;
// "cost N" with no colony.
func (s *designScreen) buildTime() string {
	cost := s.d.Cost()
	if s.col == nil {
		return fmt.Sprintf("cost %d", cost)
	}
	n, ok := s.col.DaysFor(cost)
	days := fmt.Sprintf("%d days", n)
	switch {
	case !ok:
		return "never: no industry"
	case n == 1:
		days = "1 day"
	}
	if s.col.Running() { // Build is refused until it completes
		return "busy: " + days
	}
	return days
}

// refitTime is the refit mode's two lines, "refit +N industry" then
// "D days" (RefitCost against the parts aboard; at least 1 day: it
// completes at a day's close), DaysFor's wording as buildTime's.
func (s *designScreen) refitTime() []string {
	cost := game.RefitCost(s.refit, s.d)
	n, ok := s.col.DaysFor(cost)
	days := fmt.Sprintf("%d days", n)
	switch {
	case !ok:
		days = "never: no industry"
	case n <= 1:
		days = "1 day"
	}
	if ok && s.col.Running() { // Refit is refused until it completes
		days = "busy: " + days
	}
	return []string{fmt.Sprintf("refit +%d industry", cost), days}
}

// titleLines is the title box: name (cursor in text mode), hull, time
// to build (the refit's cost, then its time), the generators' supply
// against the parts' demand, slots and one move's reach, then the
// verdict (ok: buildable, refittable: CanRefit).
func (s *designScreen) titleLines(e *game.Empire) (lines []string, verdict string, ok bool) {
	r := e.Research
	sup, dem := s.d.Power()
	reach := int(math.Round(s.d.Reach() / game.SugarCube))
	cursor := ""
	if s.editing {
		cursor = "_"
	}
	lines = []string{fmt.Sprintf("%s%s %q", s.t.Name, cursor, s.t.Hull.String())}
	if s.refit != nil {
		lines = append(lines, s.refitTime()...)
	} else {
		lines = append(lines, s.buildTime())
	}
	lines = append(lines,
		fmt.Sprintf("power %d, demand %d", sup, dem),
		fmt.Sprintf("slots %d/%d, reach %d/move", len(s.t.Parts), s.t.Hull.Slots(), reach))
	if sh := s.ship; sh != nil { // the ship's state, not a verdict
		return lines, fmt.Sprintf("hull %d/%d  energy %d/%d", sh.Hull, sh.MaxHull(), sh.Power, sh.Design.PowerPerDay()), true
	}
	if s.refit != nil { // CanRefit checks Valid too
		if err := e.CanRefit(s.col, s.refit, s.d); err != nil {
			return lines, err.Error(), false
		}
		return lines, "valid", true
	}
	if err := s.d.Valid(r); err != nil {
		return lines, err.Error(), false
	}
	return lines, "valid", true
}

// The design's gauges, the original's five icon rows as bars, desGaugeW
// wide at the title box's right, between its top and the buttons.
const desGaugeW = 260

func desGaugeX() float64 { return float64(desTitle.X1-padX) - desGaugeW }

func desGaugeY(k int) float64 {
	top, bottom := float64(desTitle.Y0+padY), float64(desLoad.Y0-padY)
	return top + float64(k)*(bottom-top)/5
}

// Gauge colours past the hull's and the energy's.
var (
	barShield = color.RGBA{160, 120, 230, 255}
	barDrive  = color.RGBA{230, 150, 60, 255}
)

// desGaugeFull is the fixed sum a design gauge holds: four of the best
// generators (Nanotwirler, 10); more breaks the gauge (drawBar), as the
// original's icon rows run past their box.
const desGaugeFull = 40

// desGaugeTick is a graduation's step: the original's one icon per 4.
const desGaugeTick = 4

// designBars sums d's parts per category as the original's icon rows:
// the weapons' damage, the shields' strength, the drives' and scanners'
// Level, the generators' supply; one scale, desGaugeFull (frac past 1
// beyond it), graduated every desGaugeTick; the sum printed.
func designBars(d game.Design) [5]bar {
	cats := [5]game.GizmoCat{game.Weapon, game.Shield, game.Drive, game.Scanner, game.Generator}
	labels := [5]string{"weapon", "shield", "drive", "scan", "energy"}
	colours := [5]color.RGBA{barRed, barShield, barDrive, barGreen, barEnergy}
	value := func(g game.Gizmo) int {
		if in := game.Gizmos[g]; in.Cat == game.Generator {
			return in.Power
		}
		return game.Gizmos[g].Level
	}
	var out [5]bar
	for k, cat := range cats {
		sum := 0
		for _, g := range d.Parts {
			if game.Gizmos[g].Cat == cat {
				sum += value(g)
			}
		}
		frac := float64(sum) / desGaugeFull
		out[k] = bar{labels[k], frac, colours[k], fmt.Sprint(sum), float64(desGaugeTick) / desGaugeFull}
	}
	return out
}

// Text writes the boxes' text; the naming box last.
func (s *designScreen) Text(a *app, frame *ebiten.Image) {
	r := a.e.Research
	if s.box != nil {
		defer s.box.draw(a, frame)
	}
	lineH := bigFace.Size * 1.25

	// The text in the left column, the design's gauges in the right one.
	tx := float64(desTitle.X0 + padX)
	tw := desGaugeX() - padX - tx
	for k, b := range designBars(s.d) {
		drawBar(frame, b, desGaugeX(), desGaugeY(k), desGaugeW)
	}
	y := float64(desTitle.Y0 + padY)
	lines, verdict, ok := s.titleLines(a.e)
	for _, l := range lines {
		ui.Print(frame, fitWidth(l, tw, bigFace), bigFace, tx, y, textColour)
		y += lineH
	}
	c := knownColour
	if !ok {
		c = errColour
	}
	if s.tick < s.msgUntil {
		verdict, c = s.msg, textColour
		if s.msgErr {
			c = errColour
		}
	}
	// The verdict on two lines at most: an error names the design.
	vs := reflow(paragraph{false, verdict}, int(tw/ui.Width("m", bigFace)))
	for _, l := range vs[:min(len(vs), 2)] {
		ui.Print(frame, fitWidth(l, tw, bigFace), bigFace, tx, y, c)
		y += lineH
	}
	if s.ship == nil {
		drawButton(frame, desLoad, "Load template", s.hover == hoverLoad || s.picking)
		drawButton(frame, desSave, "Save as template", s.hover == hoverSave)
	}
	switch {
	case s.refit != nil:
		if _, ok := s.useTemplate(a.e); ok {
			drawButton(frame, desUse, useLabel, s.hover == hoverUse)
		}
		drawButton(frame, desRefit, "Refit", s.hover == hoverRefit)
	case s.col != nil:
		drawButton(frame, desBuild, "Build", s.hover == hoverBuild)
	}

	switch g, ok := s.shown(r); {
	case s.picking && len(a.e.Templates) == 0:
		printHelp(frame, "no template saved yet\nEsc: cancel")
	case s.picking:
		printHelp(frame, "click a template to load it\nEsc: cancel")
	case ok:
		printPart(frame, a, g, s.helpOff)
	case s.ship != nil:
		printHelp(frame, "hover a part to read it\nEsc: back")
	default:
		printHelp(frame, "click a part, then a slot to place it\nright click a slot to empty it")
	}

	lw := float64(desList.X1 - desList.X0 - 2*padX)
	if s.picking {
		for k := range min(len(a.e.Templates), pickLines) {
			c := textColour
			if s.hover == hoverPick && s.index == k {
				c = yellow
			}
			// The bigFace line (20 px) centred in its band.
			ly := float64(pickTop(k) + (pickPitch-20)/2)
			ui.Print(frame, fitWidth(pickLine(a.e.Templates[k]), lw, bigFace), bigFace, float64(desList.X0+padX), ly, c)
		}
	} else {
		drawToggle(frame, desCompact, "Compact", !s.detailed)
		drawToggle(frame, desDetailed, "Detailed", s.detailed)
		l := s.list(r)
		switch {
		case s.detailed:
			for i, p := range l {
				c := textColour
				if p.Best {
					c = yellow
				}
				s.printEntry(frame, r, i, p, c)
			}
			drawScrollCues(a, frame, rect{desList.X0, desList.Y0 + listY0, desList.X1, desList.Y1}, s.listOff, detRows, len(l))
		case len(l) > 0 && l[0].Best:
			cellLabel(frame, 0, "best", yellow)
		}
	}

	// A Best slot says so under its tile.
	for i, p := range s.t.Parts {
		if k := s.t.SlotOf(i); p.Best && k < len(hullSlots[s.t.Hull]) {
			slotLabel(frame, s.t.Hull, k, "best", yellow)
		}
	}
}

// clashWarn: saving under a template's name replaces it.
const clashWarn = "* a template of that name exists: Save replaces it"

// clash reports the save box's name is already one of e's templates'.
func (b *nameBox) clash(e *game.Empire) bool {
	return b.mode == nameSave && slices.ContainsFunc(e.Templates, func(t game.Template) bool { return t.Name == b.name })
}

// draw paints the naming box: the prompt, the field with the name and
// a cursor, "*" and clashWarn on a clash, the refusal in red.
func (b *nameBox) draw(a *app, frame *ebiten.Image) {
	b.prompt().draw(a, frame)
	f := boxField()
	strokeRect(frame, f, textColour)
	w := float64(f.X1 - f.X0 - 2*padX)
	ui.Print(frame, fitWidth(b.name+"_", w, bigFace), bigFace, float64(f.X0+padX), boxFieldY(), yellow)
	centre := func(str string, y float64, c color.Color) {
		str = fitWidth(str, float64(dlgBox.X1-dlgBox.X0-4*padX), bigFace)
		ui.Print(frame, str, bigFace, float64(dlgBox.X0+dlgBox.X1)/2-ui.Width(str, bigFace)/2, y, c)
	}
	y := boxErrY()
	if b.clash(a.e) {
		ui.Print(frame, "*", bigFace, float64(f.X1+padX), boxFieldY(), yellow)
		centre(clashWarn, y, yellow)
		y += float64(zoom * promptLineH)
	}
	if b.err != "" {
		centre(b.err, y, errColour)
	}
}

// boxField is the field's outline: nameCap characters wide, centred.
func boxField() rect {
	w := int(ui.Width(strings.Repeat("M", nameCap+1), bigFace)) + 2*padX
	cx := (dlgBox.X0 + dlgBox.X1) / 2
	y := int(boxFieldY())
	return rect{cx - w/2, y - 6, cx + w/2, y + 26}
}
