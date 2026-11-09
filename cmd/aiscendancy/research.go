package main

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The research screen: the original's 3D cylinder of bubbles
// (restree.go). A hovered bubble shows in the right box (the topic when
// none); a click on an available one studies it.
//
// The chrome is 20res.tmp frame 0 (640x480), measured: the big area
// [7,477)x[46,473) under a title strip, the right box [485,633)x[7,434),
// two button holes [524,555) and [563,594) x [442,473), left black.
// restree.shp frame t is tech t's picture, 50x50, origin the centre.
// Original px (orig).
var (
	resArea = orig(7, 46, 477, 473)
	resBox  = orig(485, 7, 633, 434)
	// resCurrent is the Select current button over both button holes, a
	// black box with the news box's soft rim hiding the chrome between.
	resCurrent = orig(524, 442, 594, 473)
)

const resCurrentLabel = "Select current"

// Native px.
const (
	resTitle = 242 * zoom // the x the title is centred on
	resPicY  = 60 * zoom  // from the box's top to the text under the picture
	resBank  = "restree"
)

var titleColour = color.RGBA{90, 220, 120, 255}

// resLineH is a line's height in native px: bigFace at ui.Print's 1.25.
var resLineH = bigFace.Size * 1.25

// knownColour is a known tech's name: a soft green, apart from the hover's.
var knownColour = color.RGBA{110, 200, 120, 255}

// pick is a click on t: a studiable tech becomes the topic.
func pick(e *game.Empire, t game.Tech) error {
	if !e.Research.CanStudy(t) {
		return nil
	}
	return e.Study(t)
}

// detail is the right box's text for t: the name (line 0), then
// "Researched" once known, else the days its remaining cost takes at
// today's rate: no points shown, as vague as the original. The
// prerequisites show in the tree, the unlock rows follow unannounced.
func detail(r *game.Research, t game.Tech, rate int) []string {
	out := []string{t.String()}
	switch d := r.Days(t, rate); {
	case r.Known[t]:
		out = append(out, "Researched")
	case d < 0:
		out = append(out, "never: no research")
	case d == 1:
		out = append(out, "1 day remaining")
	default:
		out = append(out, fmt.Sprintf("%d days remaining", d))
	}
	return out
}

// unlock is one thing a tech unlocks: its icon and name.
type unlock struct {
	name  string
	bank  string
	frame int
}

// unlocks are what t unlocks: items, then hulls, then ship parts. The
// Ship item is left out (its planitem frame is the Missile Base's
// picture): its hulls are listed instead, "<Hull> hull" as sp's ship.
func unlocks(r *game.Research, sp game.Species, t game.Tech) []unlock {
	items, gizmos := r.Unlocks(t)
	var out []unlock
	for _, it := range items {
		if it != game.Ship {
			out = append(out, unlock{it.String(), "planitem", int(it)})
		}
	}
	for h := range game.NumHulls {
		if h.Tech() == t {
			out = append(out, unlock{h.String() + " hull", shipBank(sp), int(h)})
		}
	}
	for _, g := range gizmos {
		out = append(out, unlock{g.String(), gizmoBank, int(g)})
	}
	return out
}

// unlockRow is one unlock laid out: its icon w x h centred in the icons'
// column at x, top y; the name wrapped beside it in face.
type unlockRow struct {
	unlock
	x, y, w, h int
	k          float64 // the icon's scale: zoom, less for a sprite too wide (a hull's)
	names      []string
	face       *text.GoTextFace
}

// resFace is the right box's text under the name: the detail, the unlocks'
// names (bigFace: the box has the room).
var resFace = bigFace

// Unlock rows: the icons' column (frames at most 70 wide), the row gap.
const unlockIconW, unlockGap = 70 * zoom, 4 * zoom

// unlockRows lays t's unlocks out from y top while they fit above bottom.
func unlockRows(a *app, t game.Tech, top, bottom int) []unlockRow {
	var out []unlockRow
	y := top
	nw := float64(resBox.X1-padX) - unlockNameX()
	for _, u := range unlocks(a.e.Research, a.e.Species, t) {
		// resFace, or smallFace when a word of the name is too long for it.
		face := resFace
		names := reflow(paragraph{false, u.name}, int(nw/ui.Width("m", face)))
		for _, l := range names {
			if ui.Width(l, face) > nw {
				face = smallFace
				names = reflow(paragraph{false, u.name}, int(nw/ui.Width("m", face)))
				break
			}
		}
		w, h, k := 0, int(float64(len(names))*face.Size*1.25), float64(zoom)
		if sp := a.spr(u.bank, u.frame); sp.Image != nil {
			b := sp.Image.Bounds()
			k = min(k, float64(unlockIconW)/float64(b.Dx()))
			w, h = int(k*float64(b.Dx())), max(h, int(k*float64(b.Dy())))
		}
		if y+h > bottom {
			break
		}
		out = append(out, unlockRow{u, resBox.X0 + padX, y, w, h, k, names, face})
		y += h + unlockGap
	}
	return out
}

// unlockTop is the rows' top under the detail lines; unlockBottom keeps
// two lines for the error.
func unlockTop(lines int) int {
	return resBox.Y0 + resPicY + int(resLineH) + int(float64(lines)*resFace.Size*1.25) + unlockGap
}

func unlockBottom() int { return resBox.Y1 - padY - int(2*smallFace.Size*1.25) }

// unlockNameX is the names' native x: right of the icons' column.
func unlockNameX() float64 { return float64(resBox.X0 + padX + unlockIconW + padX) }

// known counts the techs r knows.
func known(r *game.Research) int {
	n := 0
	for _, k := range r.Known {
		if k {
			n++
		}
	}
	return n
}

type researchScreen struct {
	hover    game.Tech // the bubble under the cursor, NoTech none
	sel      game.Tech // the tech clicked to read (not studiable), NoTech none
	unlock   *unlock   // the shown tech's unlock row under the cursor, nil none
	tick     int
	msg      string // last Study error, shown until msgUntil
	msgUntil int

	// The tree's view (restree.go): the cylinder's turn, the height
	// looked at, the zoom (Focal, 0 resFocal), the drag over the area.
	yaw, yc, focal float64
	drag           drag
}

func newResearchScreen() *researchScreen {
	return &researchScreen{hover: game.NoTech, sel: game.NoTech}
}

// Enter centres the tree on the topic (a.e nil in the screen tests).
func (s *researchScreen) Enter(a *app) {
	if a.e != nil {
		s.centre(a.e.Research)
	}
}

// shown is the right box's tech: the hovered one, else the selected,
// else the topic.
func (s *researchScreen) shown(r *game.Research) game.Tech {
	switch {
	case s.hover != game.NoTech:
		return s.hover
	case s.sel != game.NoTech:
		return s.sel
	}
	return r.Topic
}

// selectCurrent drops the selection and turns the tree's view back on
// the topic (the button, R): the right box shows the topic again.
func (s *researchScreen) selectCurrent(r *game.Research) {
	s.sel = game.NoTech
	s.centre(r)
}

// click is a click on tech t (ok false: on none): a studiable tech
// becomes the topic, any other is selected to read in the right box
// while the cursor goes there (a known tech's unlocks and their
// help without touching the topic); on none, the selection drops.
func (s *researchScreen) click(e *game.Empire, t game.Tech, ok bool) {
	s.sel = game.NoTech
	switch {
	case !ok:
	case e.Research.CanStudy(t):
		if err := pick(e, t); err != nil {
			s.msg, s.msgUntil = err.Error(), s.tick+120 // two seconds at 60 ticks
		}
	default:
		s.sel = t
	}
}

func (s *researchScreen) Update(a *app) error {
	s.tick++
	if onSpeciesKey(a, keyJustPressed(ebiten.KeyS)) {
		return nil
	}
	x, y := a.mouse()
	press := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	if press && resCurrent.has(x, y) || keyJustPressed(ebiten.KeyR) {
		s.selectCurrent(a.e.Research)
		return nil
	}
	key := func(k ebiten.Key) float64 {
		if ebiten.IsKeyPressed(k) {
			return 1
		}
		return 0
	}
	_, wy := ebiten.Wheel()
	s.treeInput(a, x, y, press, inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft),
		ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft), wy,
		key(ebiten.KeyArrowRight)-key(ebiten.KeyArrowLeft), key(ebiten.KeyArrowDown)-key(ebiten.KeyArrowUp))
	return nil
}

// hoverAt sets the hovered bubble (also returned, ok false off one) and the shown tech's unlock row under native (x,y).
func (s *researchScreen) hoverAt(a *app, x, y float64) (game.Tech, bool) {
	t, ok := s.treeAt(a.e.Research, x, y)
	s.hover = game.NoTech
	if ok {
		s.hover = t
	}
	s.unlock = nil
	if t := s.shown(a.e.Research); t < game.NumTechs && resBox.has(x, y) {
		for _, u := range unlockRows(a, t, unlockTop(len(detail(a.e.Research, t, a.e.ResearchRate()))-1), unlockBottom()) {
			if u.hovered(x, y) {
				s.unlock = &u.unlock
				break
			}
		}
	}
	return t, ok
}

// Render paints the chrome, the tree (its view), the shown tech's
// picture and unlock icons.
func (s *researchScreen) Render(a *app, sc *ebiten.Image) {
	sc.Fill(color.Black)
	drawSprite(sc, a.spr("20res.tmp", 0), 0, 0)
	fillRect(sc, resArea)
	fillRect(sc, resBox)
	s.drawTree(a, sc)
	drawNewsBox(sc, resCurrent)
	if t := s.shown(a.e.Research); t < game.NumTechs {
		drawSprite(sc, a.spr(resBank, int(t)), float64(resBox.X0+zoom*74), float64(resBox.Y0+zoom*30))
		for _, u := range unlockRows(a, t, unlockTop(len(detail(a.e.Research, t, a.e.ResearchRate()))-1), unlockBottom()) {
			sp := a.spr(u.bank, u.frame) // its drawn box's top-left where the row puts it
			var op ebiten.DrawImageOptions
			op.GeoM.Scale(u.k, u.k)
			op.GeoM.Translate(float64(u.x+(unlockIconW-u.w)/2)-u.k*float64(sp.Offset.X), float64(u.y)-u.k*float64(sp.Offset.Y))
			if u.k != zoom {
				op.Filter = ebiten.FilterLinear
			}
			sp.Draw(sc, &op)
		}
	}
}

// Text writes the title and the right box.
func (s *researchScreen) Text(a *app, frame *ebiten.Image) {
	r, rate := a.e.Research, a.e.ResearchRate()
	title := a.e.Species.String() + " Knowledge"
	ui.Print(frame, title, bigFace, resTitle-ui.Width(title, bigFace)/2, 12*zoom, titleColour)

	// The right box: the detail (or the no-topic hint), then the error.
	bx := float64(resBox.X0 + padX)
	bw := float64(resBox.X1 - resBox.X0 - 2*padX)
	y := float64(resBox.Y0 + resPicY)
	bottom := float64(resBox.Y1 - padY)
	smallH := smallFace.Size * 1.25
	var head string
	var body []string
	if t := s.shown(r); t < game.NumTechs {
		d := detail(r, t, rate)
		head, body = d[0], d[1:]
	} else {
		head, body = "no topic", []string{"click an available tech", "", "Esc back"}
	}
	// The name centred under the picture: green once known (the topic
	// never is), else white.
	hc := color.RGBA{255, 255, 255, 255}
	if t := s.shown(r); t < game.NumTechs && r.Known[t] {
		hc = knownColour
	}
	head = fitWidth(head, bw, bigFace)
	ui.Print(frame, head, bigFace, float64(resBox.X0+resBox.X1)/2-ui.Width(head, bigFace)/2, y, hc)
	y += resLineH
	// The detail lines centred as the name.
	for _, l := range body {
		l = fitWidth(l, bw, resFace)
		ui.Print(frame, l, resFace, float64(resBox.X0+resBox.X1)/2-ui.Width(l, resFace)/2, y, textColour)
		y += resFace.Size * 1.25
	}
	if t := s.shown(r); t < game.NumTechs {
		nx := unlockNameX()
		for _, u := range unlockRows(a, t, unlockTop(len(body)), unlockBottom()) {
			ny := float64(u.y+u.h/2) - float64(len(u.names))*u.face.Size*1.25/2
			ui.Print(frame, strings.Join(u.names, "\n"), u.face, nx, ny, textColour)
		}
	}
	if s.tick < s.msgUntil {
		ui.Print(frame, fitWidth(s.msg, bw, smallFace), smallFace, bx, bottom-smallH, errColour)
	}
	cc := textColour
	if x, y := a.mouse(); resCurrent.has(x, y) {
		cc = yellow
	}
	ui.Print(frame, resCurrentLabel, bigFace, float64(resCurrent.X0+resCurrent.X1)/2-ui.Width(resCurrentLabel, bigFace)/2,
		float64(resCurrent.Y0+resCurrent.Y1)/2-bigFace.Size*1.25/2, cc)
	if s.unlock != nil {
		ls := unlockHelpLines(a, *s.unlock)
		if len(ls) > 0 { // a band over the tree, up from the area's bottom
			top := float64(resArea.Y1-padY) - float64(len(ls))*resHelpFace.Size*1.25
			fillRect(frame, rect{resArea.X0, int(top) - padY, resArea.X1, resArea.Y1})
			ui.Print(frame, strings.Join(ls, "\n"), resHelpFace, float64(resArea.X0+padX), top, textColour)
		}
	}
}

// hovered reports (x,y) on row u, its icon's left to the box's edge.
func (u unlockRow) hovered(x, y float64) bool {
	return rect{u.x, u.y, resBox.X1, u.y + u.h}.has(x, y)
}

// resHelpFace is the unlock help's face over the tree: smallFace ×2,
// its band at most the area's lower half.
var resHelpFace = ui.Face(24)

// unlockHelpLines is u's name and paragraph wrapped in resHelpFace, at
// most the area's lower half; none for a hull.
func unlockHelpLines(a *app, u unlock) []string {
	var t string
	switch u.bank {
	case "planitem":
		t = a.itemHelp(game.Item(u.frame))
	case gizmoBank:
		t = a.gizmoHelp(game.Gizmo(u.frame))
	}
	if t == "" {
		return nil
	}
	face := resHelpFace
	w, n := float64(resArea.X1-resArea.X0-2*padX), int(float64(resArea.Y1-resArea.Y0)/2/(face.Size*1.25))
	ls := helpLinesIn(face, u.name, w, n)
	return append(ls, helpLinesIn(face, t, w, n-len(ls))...)
}
