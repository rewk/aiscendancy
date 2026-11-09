package main

import (
	"errors"
	"fmt"
	"image/color"
	"io/fs"
	"log"
	"math"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/cam"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/save"
	"github.com/rewk/aiscendancy/ui"
)

// The save and load screens (the original's Load Game, capture
// image0064), one screen in two modes on the starmap's chrome: the title
// in the day counter's box, a row per save in the big area. Row 0 is the
// quick save (F5/F9's file), pinned; under it the slots (save.Slots)
// scroll, then in Save mode a "New save" row. A row (geometry below):
// the player and the day, the fleet, colonies and discoveries counts,
// the empires met (flags with the war bursts or the alliance laurel,
// dimmed when dead). Every file is Peeked once, at Enter. A save closes
// the screen (or asks first over an occupied slot); a load restarts the
// app on the loaded game, the screen dropped with the stack. The right
// column's panel previews the hovered row (the cluster as its player
// knew it, its History in small graphs), each file read in full once, on
// its first hover.
type slotsScreen struct {
	saving  bool
	quick   slotRow
	slots   []slotRow // the slot files, by number
	fresh   slotRow   // the New save row: save.NextSlot's file
	off     int       // slots scrolled: row 1 shows entry off
	hover   int       // row under the cursor, −1 none
	confirm *prompt   // the overwrite question, nil none
	target  slotRow   // the row it asks about
	// previews are the hovered files' previews by path, built once (Enter
	// drops them: a file may have changed since).
	previews map[string]*svPreview
}

// slotRow is a save file as Peek found it.
type slotRow struct {
	n      int // the slot number, 0 the quick save
	path   string
	exists bool
	sum    *save.Summary // nil unless loadable
	why    string        // a refusal: other version, unreadable
	fresh  bool          // the New save row
}

func newSlotsScreen(saving bool) *slotsScreen { return &slotsScreen{saving: saving, hover: -1} }

// Enter peeks the quick save and every slot file.
func (s *slotsScreen) Enter(a *app) {
	s.quick = peekRow(0, a.savePath)
	ns, _ := save.Slots(a.savePath) // an unreadable directory lists nothing
	s.slots = s.slots[:0]
	for _, n := range ns {
		s.slots = append(s.slots, peekRow(n, save.SlotPath(a.savePath, n)))
	}
	n := save.NextSlot(ns)
	s.fresh = slotRow{n: n, path: save.SlotPath(a.savePath, n), fresh: true}
	s.off, s.confirm = 0, nil
	s.previews = map[string]*svPreview{}
}

// peekRow is slot n's row from path's head.
func peekRow(n int, path string) slotRow {
	r := slotRow{n: n, path: path, exists: true}
	v, sum, err := save.Peek(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		r.exists = false
	case err != nil:
		r.why = "unreadable, not loadable"
	case v != save.Version:
		r.why = fmt.Sprintf("version %d, not loadable", v)
	case !sumDrawable(sum):
		r.why = "bad summary, not loadable"
	default:
		r.sum = sum
	}
	return r
}

// sumDrawable reports every species and colour in s indexes the sprite
// banks (a hand-edited file could crash the sprite cache).
func sumDrawable(s *save.Summary) bool {
	ok := func(sp game.Species, c int) bool {
		return sp >= 0 && sp < game.NumSpecies && c >= 0 && c < game.NumColours
	}
	if s == nil || !ok(s.Species, s.Colour) {
		return false
	}
	for _, m := range s.Met {
		if !ok(m.Species, m.Colour) {
			return false
		}
	}
	return true
}

// saved re-peeks the row of path, just written under the screen (F5),
// and drops its preview.
func (s *slotsScreen) saved(path string) {
	if path == s.quick.path {
		s.quick = peekRow(0, path)
	}
	for i, r := range s.slots {
		if r.path == path {
			s.slots[i] = peekRow(r.n, path)
		}
	}
	delete(s.previews, path)
}

// entries is the scrolling list's length: the slots, then New save.
func (s *slotsScreen) entries() int {
	if s.saving {
		return len(s.slots) + 1
	}
	return len(s.slots)
}

// row is what screen row r shows: the quick save at 0, then entry off+r−1
// (a slot, or the New save row past them); ok false for none.
func (s *slotsScreen) row(r int) (slotRow, bool) {
	if r == 0 {
		return s.quick, true
	}
	if r >= slotsRows() {
		return slotRow{}, false
	}
	switch i := listIndex(s.off, r-1, s.entries()); {
	case i < 0:
		return slotRow{}, false
	case i < len(s.slots):
		return s.slots[i], true
	}
	return s.fresh, true
}

func (s *slotsScreen) scroll(d int) {
	s.off = min(max(s.off+d, 0), max(0, s.entries()-(slotsRows()-1)))
}

// typing holds the app's keys (Space, Tab, H, F5, F9) while the
// overwrite question is up: it answers first.
func (s *slotsScreen) typing() bool { return s.confirm != nil }

// cancel drops the overwrite question (app.back: Esc, the back button).
func (s *slotsScreen) cancel() bool {
	if s.confirm == nil {
		return false
	}
	s.confirm = nil
	return true
}

// press acts on screen row r. Load: a loadable row loads. Save (in a
// game): the quick row and New save write at once, an occupied slot asks
// first.
func (s *slotsScreen) press(a *app, r int) {
	row, ok := s.row(r)
	switch {
	case !ok:
	case !s.saving:
		if row.sum != nil {
			a.loadPath(row.path) // restarts: this screen is gone
		}
	case !a.inGame():
	case row.n == 0 || row.fresh:
		s.write(a, row)
	default:
		s.confirm = &prompt{[]string{fmt.Sprintf("Overwrite save %d?", row.n)}, "Yes (Enter)", "No (Esc)"}
		s.target = row
	}
}

// answer closes the question, writing on yes.
func (s *slotsScreen) answer(a *app, yes bool) {
	s.confirm = nil
	if yes {
		s.write(a, s.target)
	}
}

// write saves to row's file, then closes the screen; a failure is
// toasted and keeps it.
func (s *slotsScreen) write(a *app, row slotRow) {
	if a.saveTo(row.path) && len(a.stack) > 1 && a.top() == s {
		a.pop()
	}
}

func (s *slotsScreen) Update(a *app) error {
	if s.confirm != nil {
		if ok, yes := s.confirm.update(a); ok {
			s.answer(a, yes)
		}
		return nil
	}
	x, y := a.mouse()
	if _, wy := ebiten.Wheel(); wy != 0 {
		s.scroll(notch(wy, 1))
	}
	s.off = keyScroll(s.off, slotsRows()-1)
	s.scroll(0)
	s.hoverAt(x, y)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		s.press(a, s.hover)
	}
	return nil
}

// Row geometry, native px: the capture's 85 px row at zoom. Left to
// right, svGap apart: the number, the flag (raceflag at zoom), the text
// column, the portrait in its ring (x1, as the New Game's Begin box), three
// count blocks (a smallFace label over icons at x1, svPerLine a line),
// then the empires met in svMetW x svMetH cells (the flag at zoom, the
// laurel's 41 px wide), svMetCols a line.
const (
	svRowH    = 85 * zoom
	svPad     = 3 * zoom // a row's top margin
	svGap     = 8
	svNumW    = 28
	svTextW   = 156 // "2006-01-02 15:04" in bigFace
	svRingW   = 110 // racering's 109
	svBlockW  = 104 // "Discoveries 99" in smallFace
	svPitch   = 8   // the 14intel icons' (6-7 px)
	svPerLine = 10
	svMetW    = 43 * zoom
	svMetH    = 27 * zoom
	svMetCols = 2
)

// The 14intel frames: the count icons (tinted), the war bursts and the
// alliance laurel (drawn at a flag's corner: their origins centre them).
const (
	intelBank   = "14intel"
	intelFleet  = 5
	intelColony = 6
	intelTech   = 7
	intelWar    = 8
	intelAlly   = 9
)

// titleGreen is the title bar's, the capture's green.
var titleGreen = color.RGBA{0, 96, 48, 255}

func slotsRows() int { return (mapArea.Y1 - mapArea.Y0 - 2*padY) / svRowH }

func svY0(r int) int { return mapArea.Y0 + padY + r*svRowH }

// svRowAt is the screen row under native y, −1 none.
func svRowAt(y float64) int {
	if y < float64(svY0(0)) {
		return -1
	}
	if r := (int(y) - svY0(0)) / svRowH; r < slotsRows() {
		return r
	}
	return -1
}

func svNumX() int  { return mapArea.X0 + padX }
func svFlagX() int { return svNumX() + svNumW }
func svTextX() int { return svFlagX() + flagW*zoom + svGap }
func svRingX() int { return svTextX() + svTextW + svGap }

// svBlockX is count block k's left edge (k = 3: the empires' column).
func svBlockX(k int) int { return svRingX() + svRingW + svGap + k*(svBlockW+svGap) }

// svIconLines is how many icon lines a block holds under its label.
func svIconLines() int {
	return (svRowH - 2*svPad - svIconDY()) / svPitch
}

// svIconDY is the icons' offset under the row's top: the label's line.
func svIconDY() int { return svPad + int(smallFace.Size*1.25) + 4 }

// svMetCell is met empire k's cell in the row from y0.
func svMetCell(y0, k int) rect {
	x := svBlockX(3) + k%svMetCols*svMetW
	y := y0 + svPad + k/svMetCols*svMetH
	return rect{x, y, x + svMetW, y + svMetH}
}

// svMetMax is how many met empires a row shows.
const svMetMax = svMetCols * (svRowH - svPad) / svMetH

// intel is 14intel frame f in colour c.
func (a *app) intel(f, c int) *assets.Sprite {
	s, err := a.a.SpriteIn(intelBank, f, c)
	if err != nil {
		log.Fatal(err)
	}
	return s
}

// drawDim draws sp at zoom with its origin at (x, y), its colour scaled
// by k.
func drawDim(dst *ebiten.Image, sp *assets.Sprite, x, y float64, k float32) {
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(zoom, zoom)
	op.GeoM.Translate(x, y)
	op.ColorScale.Scale(k, k, k, 1)
	sp.Draw(dst, &op)
}

// svCounts are a summary's three blocks: the icon frame and the count.
func svCounts(sum *save.Summary) [3]struct{ frame, n int } {
	return [3]struct{ frame, n int }{{intelFleet, sum.Ships}, {intelColony, sum.Colonies}, {intelTech, sum.Techs}}
}

var svLabels = [3]string{"Fleet", "Colonies", "Discoveries"}

// Render paints the chrome, the right column's panel (its title bar, the
// hovered row's preview), then each loadable row's pictures.
func (s *slotsScreen) Render(a *app, sc *ebiten.Image) {
	sc.Fill(color.Black)
	drawSprite(sc, a.spr("1starmap.tmp", 0), 0, 0)
	fillRect(sc, mapArea)
	drawNewsBox(sc, svPanel)
	fillRectWith(sc, svTitle, titleGreen)
	s.drawPreview(sc, a.tick)
	dst := clipped(sc, mapArea)
	for r := range slotsRows() {
		row, ok := s.row(r)
		if !ok {
			break
		}
		if row.sum == nil {
			continue
		}
		sum, y0 := row.sum, svY0(r)
		drawSprite(dst, a.flag(sum.Species, sum.Colour), float64(svFlagX()), float64(y0+svPad))
		cx, cy := float64(svRingX()+svRingW/2), float64(y0+svRowH/2)
		a.spr(ringBank, sum.Colour).DrawAt(dst, cx, cy)
		a.spr(portraitBank, int(sum.Species)).DrawAt(dst, cx, cy)
		for k, b := range svCounts(sum) {
			sp := a.intel(b.frame, sum.Colour)
			for i := range min(b.n, svPerLine*svIconLines()) {
				sp.DrawAt(dst, float64(svBlockX(k)+i%svPerLine*svPitch), float64(y0+svIconDY()+i/svPerLine*svPitch))
			}
		}
		for k, m := range sum.Met[:min(len(sum.Met), svMetMax)] {
			c := svMetCell(y0, k)
			x, y := float64(c.X0+(svMetW-flagW*zoom)/2), float64(c.Y0)
			var lum float32 = 1
			if m.Dead {
				lum = 0.4
			}
			drawDim(dst, a.flag(m.Species, m.Colour), x, y, lum)
			switch m.Stance {
			case game.War:
				drawDim(dst, a.spr(intelBank, intelWar), x, y, lum)
			case game.Alliance:
				drawDim(dst, a.spr(intelBank, intelAlly), x, y, lum)
			}
		}
	}
	// The pinned quick row's foot, a dim rule.
	fillRectWith(sc, rect{mapArea.X0 + padX, svY0(1) - 1, mapArea.X1 - padX, svY0(1)}, dim(textColour, 0.3))
	drawScrollCues(a, sc, rect{mapArea.X0, svY0(1), mapArea.X1, mapArea.Y1}, s.off, slotsRows()-1, s.entries())
}

// svText is a row's text column: species, day, date; a refusal, or
// "empty"; the quick row's label under.
func svText(row slotRow) []string {
	var ls []string
	switch {
	case row.fresh:
		ls = []string{"New save"}
	case row.sum != nil:
		ls = []string{row.sum.Species.String(), fmt.Sprintf("Day %d", row.sum.Day), row.sum.Saved.Format("2006-01-02 15:04")}
	case row.why != "":
		ls = []string{row.why}
	default:
		ls = []string{"empty"}
	}
	if row.n == 0 {
		ls = append(ls, "Quick save  F5 / F9")
	}
	return ls
}

// svNum is a row's number column: Q for the quick save.
func svNum(row slotRow) string {
	if row.n == 0 {
		return "Q"
	}
	return fmt.Sprint(row.n)
}

func (s *slotsScreen) Text(a *app, frame *ebiten.Image) {
	title := "Load Game"
	if s.saving {
		title = "Save Game"
	}
	centred(frame, title, bigFace, svTitle, float64(svTitle.Y0+svTitle.Y1)/2-bigFace.Size*0.6, textColour)
	s.previewText(a, frame)
	lh := bigFace.Size * 1.25
	for r := range slotsRows() {
		row, ok := s.row(r)
		if !ok {
			break
		}
		y0 := float64(svY0(r) + svPad)
		c := textColour
		switch {
		case r == s.hover && (row.sum != nil || s.saving && a.inGame()):
			c = yellow
		case row.sum == nil && !row.fresh:
			c = dim(textColour, 0.5)
		}
		ui.Print(frame, svNum(row), bigFace, float64(svNumX()), y0, c)
		ls := svText(row)
		w := float64(svTextW)
		if row.sum == nil { // nothing right of it: the line may run on
			w = float64(mapArea.X1 - padX - svTextX())
		}
		for i, l := range ls {
			face, lc := bigFace, c
			if row.n == 0 && i == len(ls)-1 {
				face, lc = smallFace, dim(textColour, 0.6)
			}
			ui.Print(frame, fitWidth(l, w, face), face, float64(svTextX()), y0+float64(i)*lh, lc)
		}
		if row.sum == nil {
			continue
		}
		for k, b := range svCounts(row.sum) {
			ui.Print(frame, fmt.Sprintf("%s %d", svLabels[k], b.n), smallFace, float64(svBlockX(k)), y0, textColour)
		}
	}
	if s.confirm != nil {
		s.confirm.draw(a, frame)
	}
}

// The right column: one black panel with the news box's soft edge over
// the chrome's boxes, the title bar on top (mapTop's height), the preview
// under it: the map's square, then the small graphs, svGraphCols a line,
// clear of the "?" button.
var (
	svPanel = rect{mapTop.X0, mapTop.Y0, mapTop.X1, mapArea.Y1}
	svTitle = rect{svPanel.X0, svPanel.Y0, svPanel.X1, mapTop.Y1}
)

const (
	svGraphCols = 2
	svGraphGap  = 4 * zoom
	svMapPad    = 4 * zoom // the stars' margin inside the square
)

// svMap is the map's square under the title.
func svMap() rect {
	side := svPanel.X1 - svPanel.X0 - 2*padX
	x, y := svPanel.X0+padX, svTitle.Y1+padY
	return rect{x, y, x + side, y + side}
}

// svGraph is small graph k's rect: its label line, the plot under.
func svGraph(k int) rect {
	rows := (len(statSeries) + svGraphCols - 1) / svGraphCols
	top, bottom := svMap().Y1+svGraphGap, min(svPanel.Y1, keysButton.Y0)-padY
	w := (svPanel.X1 - svPanel.X0 - 2*padX - (svGraphCols-1)*svGraphGap) / svGraphCols
	h := (bottom - top - (rows-1)*svGraphGap) / rows
	x := svPanel.X0 + padX + k%svGraphCols*(w+svGraphGap)
	y := top + k/svGraphCols*(h+svGraphGap)
	return rect{x, y, x + w, y + h}
}

// svPlot is graph k's plot, under its label line.
func svPlot(k int) rect {
	r := svGraph(k)
	r.Y0 += int(smallFace.Size*1.25) + 2
	return r
}

// svPreview is a save's preview, built once from its full read: the stars
// its player (empire 0, as the Summary's) knows, the lanes between two of
// them (a lane to an unknown star would show where it lies), the player's
// colour and History.
type svPreview struct {
	why    string // the read's error, "" when built
	stars  []svStar
	lanes  []svLane
	colour int
	hist   []game.Sample
}

// svStar is a known star at its cluster position, its colonist's colour
// when settled.
type svStar struct {
	star    int
	pos     game.Vec3
	settled bool
	colour  color.RGBA
}

// svLane joins stars[a] and stars[b].
type svLane struct {
	a, b int
	red  bool
}

// readSave is save.Read behind a seam: the tests count the reads.
var readSave = save.Read

// newPreview reads path in full and builds its preview.
func newPreview(path string) *svPreview {
	f, err := readSave(path)
	if err != nil {
		return &svPreview{why: err.Error()}
	}
	return previewOf(f.Game)
}

// previewOf is g's preview for its empire 0: the fog is game/fog.go's
// Knows; a known star's colonist as the starmap's ring shows it.
func previewOf(g *game.Game) *svPreview {
	e := g.Empires[0]
	p := &svPreview{colour: e.Colour, hist: e.History}
	at := map[int]int{}
	for i, st := range g.Cluster.Stars {
		if !e.Knows(i) {
			continue
		}
		at[i] = len(p.stars)
		s := svStar{star: i, pos: st.Pos}
		if c := st.System.Colonist(); c >= 0 {
			s.settled, s.colour = true, empireColour(colourOf(g, c))
		}
		p.stars = append(p.stars, s)
	}
	for _, l := range g.Cluster.Lanes {
		a, okA := at[l.A]
		b, okB := at[l.B]
		if okA && okB {
			p.lanes = append(p.lanes, svLane{a, b, l.Red})
		}
	}
	return p
}

// hoverAt sets the row under (x, y) hovered and builds a loadable row's
// preview on its first hover (one read a file while the screen is up).
func (s *slotsScreen) hoverAt(x, y float64) {
	s.hover = -1
	if mapArea.has(x, y) {
		s.hover = svRowAt(y)
	}
	row, ok := s.row(s.hover)
	if s.hover < 0 || !ok || row.sum == nil {
		return
	}
	if s.previews == nil {
		s.previews = map[string]*svPreview{}
	}
	if s.previews[row.path] == nil {
		s.previews[row.path] = newPreview(row.path)
	}
}

// hovered is the hovered row and its preview (nil for a row with no
// summary); ok false with no row hovered.
func (s *slotsScreen) hovered() (slotRow, *svPreview, bool) {
	if s.hover < 0 {
		return slotRow{}, nil, false
	}
	row, ok := s.row(s.hover)
	if !ok {
		return slotRow{}, nil, false
	}
	if row.sum == nil {
		return row, nil, true
	}
	return row, s.previews[row.path], true
}

// The minimap turns: svTilt above the plane, a turn every svTurnTicks.
const (
	svTilt      = 30 * math.Pi / 180
	svTurnTicks = 40 * 60
)

// svCamera views the unit ball turned to tick's yaw, centred in m: Focal
// puts the nearest possible point (depth Dist−1) svMapPad inside the edge.
func svCamera(m rect, tick int) cam.Camera {
	c := cam.Default(float64(m.X0+m.X1)/2, float64(m.Y0+m.Y1)/2)
	c.Pitch, c.Yaw = svTilt, 2*math.Pi*float64(tick%svTurnTicks)/svTurnTicks
	c.Focal = float64(m.X1-m.X0-2*svMapPad) / 2 * (c.Dist - 1)
	return c
}

// svAt is st's native point through c.
func svAt(c *cam.Camera, st svStar) (float32, float32) {
	p := c.Project(st.pos)
	return float32(p.X), float32(p.Y)
}

// drawPreview paints the hovered preview: the lanes, the stars (a settled
// one a dot in its colonist's colour, the others small and grey), the
// small graphs' plots in the player's colour.
func (s *slotsScreen) drawPreview(sc *ebiten.Image, tick int) {
	_, p, _ := s.hovered()
	if p == nil || p.why != "" {
		return
	}
	m := svMap()
	cv := svCamera(m, tick)
	strokeRect(sc, m, dim(textColour, 0.25))
	dst := clipped(sc, m)
	for _, l := range p.lanes {
		c := dim(blueLane, 0.6)
		if l.red {
			c = dim(redLane, 0.6)
		}
		ax, ay := svAt(&cv, p.stars[l.a])
		bx, by := svAt(&cv, p.stars[l.b])
		vector.StrokeLine(dst, ax, ay, bx, by, 1, c, true)
	}
	for _, st := range p.stars {
		x, y := svAt(&cv, st)
		if st.settled {
			vector.FillCircle(dst, x, y, 2.5*zoom, st.colour, true)
			continue
		}
		vector.FillRect(dst, x-1, y-1, 3, 3, dim(textColour, 0.6), false)
	}
	for k, ser := range statSeries {
		r := svPlot(k)
		fillRectWith(sc, r, color.RGBA{10, 14, 18, 255})
		ls := [1]chartLine{{p.hist, ser.of, empireColour(p.colour), zoom}}
		drawChart(sc, r, ls[:], false)
	}
}

// previewText writes the panel's text: nothing hovered, the help and the
// save directory; a row with no summary, its reason; a failed read, its
// error; else each graph's label and last value.
func (s *slotsScreen) previewText(a *app, frame *ebiten.Image) {
	body := rect{svPanel.X0, svTitle.Y1, svPanel.X1, svPanel.Y1}
	cols := int(float64(body.X1-body.X0-2*padX) / ui.Width("M", smallFace))
	row, p, ok := s.hovered()
	switch {
	case !ok:
		help := "click: load it\nEsc: back"
		if s.saving {
			help = "click: save there\nEsc: back"
		}
		help += "\nhover a save: its map and graphs"
		dir := []rune(filepath.Dir(a.savePath)) // cut in lines of cols
		var ls []string
		for cols > 0 && len(dir) > cols {
			ls, dir = append(ls, string(dir[:cols])), dir[cols:]
		}
		putBox(frame, body, fmt.Sprintf("%s\n\n%d %s in\n%s", help, len(s.slots), countWord(len(s.slots), "slot"),
			strings.Join(append(ls, string(dir)), "\n")), smallFace, textColour)
	case p == nil && row.sum == nil:
		putBox(frame, body, wrap(svText(row)[0], cols), smallFace, dim(textColour, 0.7))
	case p == nil: // not read yet: Update reads it
	case p.why != "":
		putBox(frame, body, wrap(p.why, cols), smallFace, errColour)
	default:
		for k, ser := range statSeries {
			r := svGraph(k)
			l := ser.name
			if v, ok := lastOf(p.hist, ser.of); ok {
				l = fmt.Sprintf("%s %d", ser.name, v)
			}
			ui.Print(frame, fitWidth(l, float64(r.X1-r.X0), smallFace), smallFace, float64(r.X0), float64(r.Y0), textColour)
		}
	}
}
