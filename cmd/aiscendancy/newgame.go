package main

import (
	"fmt"
	"image/color"
	"math"
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/cam"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The New Game screen (docs/screens.md, "New game"): the program opens
// on it unless -load gives a save. No game exists while it is up (a.g
// nil): the app runs only Esc, F9 and this screen's Update.
//
// The chrome is 3cfgnew.tmp frame 0 (640x480), its holes measured: the
// big area, the species list (one ngFace name a line), an opaque strip
// above the list ("Player Species"), five bottom holes 116x92.
//
// The big area splits at x 330. Left (ngPreview): the cluster Begin will
// roll, stars only, under ngCamera, a three-line summary under it; a
// click draws a new seed, 1..maxSeed, but on the bots' Difficulty at its
// top-left (ngDiff), which a click cycles. Right (ngSpecies): the chosen
// species' name, portrait (smraces.shp 100x100, origin the centre) in
// the colour ring (racering.shp frame = colour, 109x109, centred), home
// world (planets.shp at scale 1) with its flag (raceflag.shp 15x25,
// origin top-left, recoloured), its newgame.txt description, then its
// ability and home world, in the colour's text tone.
//
// 3cfgnew.shp's 13 frames are 116x92, origin (0,0), drawn at a hole's
// top-left; their picture fills rows 25-80, the label goes above:
//
//	hole 0  "Star Density": frames 0-4 (Vacuous..Very Dense); a click cycles it
//	hole 1  "Species": frames 5-9 (3..7 flags); a click cycles 3..7,
//	        capped by the density's star count
//	hole 2  "Atmosphere": frames 10-12 (Peaceful, Neutral, Hostile);
//	        Game.Atmosphere, the bots' bias
//	hole 3  "Player Color": the flag in the seven colours at ngColX0 +
//	        k·ngColDX, ngColY, the chosen one outlined white; the others'
//	        colours are drawn by the seed (game.PickColours)
//	hole 4  the portrait at scale 1 in its ring and "Begin New Game"
//
// Original px (orig).
var (
	ngArea    = orig(7, 7, 494, 373)
	ngPreview = rect{ngArea.X0, ngArea.Y0, zoom * 330, ngArea.Y1}
	ngSpecies = rect{zoom * 330, ngArea.Y0, ngArea.X1, ngArea.Y1}
	ngDiff    = rect{ngPreview.X0 + padX, ngPreview.Y0 + padY, ngPreview.X0 + padX + 100*zoom, ngPreview.Y0 + padY + 12*zoom} // outside the preview's ball
	ngList    = orig(502, 45, 633, 373)
	ngTitle   = orig(502, 7, 633, 45)
	ngHoles   = [5]rect{
		orig(7, 381, 123, 473), orig(131, 381, 247, 473), orig(255, 381, 371, 473),
		orig(379, 381, 494, 473), orig(502, 381, 633, 473),
	}
)

const (
	ngChrome               = "3cfgnew.tmp"
	ngBank                 = "3cfgnew"
	ngLabelDY              = 10 * zoom // a hole's label, native px from its top (the original's y 391)
	ngPicTop, ngPicBottom  = 25, 80    // the frame's picture rows, original px
	minEmpires, maxEmpires = 3, 7
	maxSeed                = 999999
)

// The bottom holes, by index in ngHoles.
const (
	densityHole = iota
	empiresHole
	atmosHole
	colourHole
	beginHole
)

// Frames of 3cfgnew.shp.
const (
	densityFrame0 = 0  // + density
	empiresFrame0 = 5  // + empires − minEmpires
	neutralFrame  = 11 // + the atmosphere (−1..1)
)

// The species' banks: portraits and rings, by species and empire index.
const (
	portraitBank = "smraces"
	ringBank     = "racering"
)

// The right part's places, measured (centres unless said).
const (
	ngNameX, ngNameY       = 411 * zoom, 16 * zoom
	ngPortX, ngPortY       = 408 * zoom, 96 * zoom
	ngGlobeX, ngGlobeY     = 408 * zoom, 216 * zoom
	ngFlagX, ngFlagY       = 441 * zoom, 205 * zoom // top-left
	flagW, flagH           = 15, 25                 // raceflag's frames, original px
	ngDescX0, ngDescX1     = 335 * zoom, 482 * zoom
	ngDescY                = 265 * zoom
	ngSumX, ngSumY         = 170 * zoom, 325 * zoom // the summary's centre x, first line's top
	ngBeginX, ngBeginPortY = 567 * zoom, 418 * zoom // the fifth hole's portrait centre
	ngBeginY               = 455 * zoom
	ngColX0, ngColY        = 381 * zoom, 433 * zoom // hole 3's first flag, top-left
	ngColDX                = 16 * zoom              // one flag to the next
)

// colourText is each colour's lightest game.pal ramp entry (Ramp(c, 3)).
var colourText = [game.NumColours]color.RGBA{{0, 170, 81, 255}, {158, 81, 174, 255}, {255, 235, 97, 255},
	{239, 154, 174, 255}, {121, 105, 69, 255}, {36, 117, 207, 255}, {235, 121, 4, 255}}

// The preview's camera: the starmap's (Focal/Dist = 400 px a unit)
// scaled to the original's preview radius, ~130 original px (x 49-291,
// y 24-282) = 260 native: Focal = 260·Dist = 1040, Dist kept at 4 so the
// brightness buckets read as on the starmap. The ball's silhouette is
// Focal/√(Dist²−1) ≈ 268 px around (340,306), inside ngPreview.
const (
	ngCamX, ngCamY = 170 * zoom, 153 * zoom
	ngRadius       = 130 * zoom
)

func ngCamera() cam.Camera {
	c := mapCamera()
	c.Cx, c.Cy = ngCamX, ngCamY
	c.Focal = ngRadius * c.Dist
	return c
}

// ngFace is the species list's, sized to fill its box: 21 lines of
// ngPitch = 630 of the list's 656 px.
var (
	ngFace  = ui.Face(24)
	ngPitch = ngFace.Size * 1.25
)

// hoverColour is the hovered name's, the summary's and the labels'.
var hoverColour = color.RGBA{255, 255, 255, 255}

var countWords = [...]string{"Three", "Four", "Five", "Six", "Seven"}

// sizeWords name planet sizes MinSize..MaxSize (static.txt 41-45).
var sizeWords = [...]string{"Tiny", "Small", "Medium", "Large", "Enormous"}

// ngLabels are the bottom holes' labels, by hole (the fifth's is Begin).
var ngLabels = [...]string{"Star Density", "Species", "Atmosphere", "Player Color"}

type newGameScreen struct {
	seed    uint64
	density game.Density
	species game.Species
	empires int
	atmos   game.Atmosphere
	colour  int // the player's, game.Green..Orange
	diff    bot.Difficulty
	line    int  // the hovered list line, −1 none
	hole    int  // the hovered bottom hole, −1 none
	onDiff  bool // the Difficulty line hovered

	// The preview's cluster and the (seed, density) it was rolled for.
	cl        *game.Cluster
	clSeed    uint64
	clDensity game.Density
}

func newNewGameScreen() *newGameScreen {
	return &newGameScreen{seed: 1, density: game.Average, species: game.Snovemdomas, empires: 3, line: -1, hole: -1}
}

func (s *newGameScreen) Enter(*app) {}

// preview is the cluster Begin will roll, re-rolled on a new seed or
// density.
func (s *newGameScreen) preview() *game.Cluster {
	if s.cl == nil || s.clSeed != s.seed || s.clDensity != s.density {
		s.cl, s.clSeed, s.clDensity = game.NewCluster(s.seed, s.density), s.seed, s.density
	}
	return s.cl
}

// colourAt is the colour of hole 3's flag under (x,y), −1 for none.
func colourAt(x, y float64) int {
	for c := range game.NumColours {
		if (rect{ngColX0 + c*ngColDX, ngColY, ngColX0 + c*ngColDX + zoom*flagW, ngColY + zoom*flagH}).has(x, y) {
			return c
		}
	}
	return -1
}

// pick is the list line or bottom hole under (x,y), −1 for none.
func (s *newGameScreen) pick(x, y float64) (line, hole int) {
	line, hole = -1, -1
	if ngList.has(x, y) {
		if i := int((y - float64(ngList.Y0+padY)) / ngPitch); y >= float64(ngList.Y0+padY) && i < int(game.NumSpecies) {
			line = i
		}
	}
	for k, r := range ngHoles {
		if r.has(x, y) {
			hole = k
		}
	}
	return line, hole
}

func (s *newGameScreen) Update(a *app) error {
	x, y := a.mouse()
	s.line, s.hole = s.pick(x, y)
	s.onDiff = ngDiff.has(x, y)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		s.click(a, x, y)
	}
	return nil
}

// click acts on a press at (x,y).
func (s *newGameScreen) click(a *app, x, y float64) {
	if ngDiff.has(x, y) {
		s.diff = (s.diff + 1) % bot.NumDifficulties
		return
	}
	if ngPreview.has(x, y) {
		s.seed = 1 + rand.Uint64N(maxSeed)
		return
	}
	line, hole := s.pick(x, y)
	if line >= 0 {
		s.species = game.Species(line)
	}
	switch hole {
	case densityHole:
		s.density = (s.density + 1) % game.NumDensities
		s.empires = min(s.empires, s.density.Stars())
	case empiresHole:
		s.empires++
		if s.empires > maxEmpires || s.empires > s.density.Stars() {
			s.empires = minEmpires
		}
	case atmosHole: // Peaceful, Neutral, Hostile, round
		s.atmos = game.Peaceful + (s.atmos-game.Peaceful+1)%3
	case colourHole:
		if c := colourAt(x, y); c >= 0 {
			s.colour = c
		}
	case beginHole:
		s.begin(a)
	}
}

// begin makes the game of the choices: the player's species first, a bot
// for every other empire, a fresh starmap under the introduction.
func (s *newGameScreen) begin(a *app) {
	g := game.NewGame(s.seed, game.PickSpecies(s.seed, s.species, s.empires), s.density)
	g.SetColour(0, s.colour)
	g.Atmosphere = s.atmos
	a.setGame(g, 0)
	cfg := bot.AtLevel(a.botConfig(), s.diff)
	a.bots = bot.NewAll(g, cfg)
	a.bots[0] = nil
	a.log = nil
	a.restart()
	a.saveLast()
	a.push(&introScreen{}) // it says the choices: no toast over its frame
}

// summary is the -shot line: the choices.
func (s *newGameScreen) summary() string {
	return fmt.Sprintf("new game: %s, %s, %s, %d empires, %s, %s bots, seed %d", s.species, game.ColourName(s.colour), s.density, s.empires, s.atmos, s.diff, s.seed)
}

// summaryLines are the three lines under the preview.
func (s *newGameScreen) summaryLines() string {
	return fmt.Sprintf("%s Star Cluster\n%s Species\nseed %d", s.density, countWords[s.empires-minEmpires], s.seed)
}

// ngLabelY is the top of hole k's label, native px: above the picture.
func ngLabelY(k int) float64 { return float64(ngHoles[k].Y0 + ngLabelDY) }

// ngFacts are the right part's lines, cols wide, at most rows: the
// description, a blank line, the ability, the home world if room.
func ngFacts(in game.SpeciesInfo, cols, rows int) []string {
	lines := strings.Split(wrap("The "+in.Name+" "+in.Description, cols), "\n")
	lines = append(lines, "")
	lines = append(lines, strings.Split(wrap("Ability: "+in.AbilityText, cols), "\n")...)
	if len(lines) < rows {
		lines = append(lines, fmt.Sprintf("Home: %s %s", sizeWords[in.HomeSize-game.MinSize], in.HomeType))
	}
	return lines[:min(len(lines), rows)]
}

// ngFactsFit is ngFacts sized to the right part.
func ngFactsFit(in game.SpeciesInfo) []string {
	cols := int(float64(ngDescX1-ngDescX0) / ui.Width("m", smallFace))
	rows := int(float64(ngArea.Y1-ngDescY) / (smallFace.Size * 1.25))
	return ngFacts(in, cols, rows)
}

// Render paints the chrome, the preview and the pictures.
func (s *newGameScreen) Render(a *app, sc *ebiten.Image) {
	sc.Fill(color.Black)
	drawSprite(sc, a.spr(ngChrome, 0), 0, 0)
	fillRect(sc, ngArea)
	fillRect(sc, ngList)
	for _, r := range ngHoles {
		fillRect(sc, r)
	}

	// The preview: back to front, whole pixels, brightness by depth.
	cl := s.preview()
	c := ngCamera()
	ps := make([]cam.Point, len(cl.Stars))
	order := make([]int, len(cl.Stars))
	for i, st := range cl.Stars {
		ps[i], order[i] = c.Project(st.Pos), i
	}
	slices.SortStableFunc(order, func(i, j int) int { return -cmpFloat(ps[i].Depth, ps[j].Depth) })
	pv := clipped(sc, ngPreview)
	for _, i := range order {
		f := starFrame(cl.Stars[i].Type, brightness(ps[i].Depth, c.Dist))
		drawSprite(pv, a.spr(starBank, f), math.Round(ps[i].X), math.Round(ps[i].Y))
	}

	in := s.species.Info()
	drawSprite(sc, a.spr(ringBank, s.colour), ngPortX, ngPortY)
	drawSprite(sc, a.spr(portraitBank, int(s.species)), ngPortX, ngPortY)
	// Half the zoomed size: scale 1, the asset pixel for pixel.
	half := func(bank string, frame int, x, y float64) { a.spr(bank, frame).DrawAt(sc, x, y) }
	half("planets", int(in.HomeType)*game.MaxSize+in.HomeSize-1, ngGlobeX, ngGlobeY)
	drawSprite(sc, a.flag(s.species, s.colour), ngFlagX, ngFlagY)

	at := func(hole, frame int, dim float32) {
		var op ebiten.DrawImageOptions
		op.GeoM.Scale(zoom, zoom)
		op.GeoM.Translate(float64(ngHoles[hole].X0), float64(ngHoles[hole].Y0))
		op.ColorScale.Scale(dim, dim, dim, 1)
		a.spr(ngBank, frame).Draw(sc, &op)
	}
	at(densityHole, densityFrame0+int(s.density), 1)
	at(empiresHole, empiresFrame0+s.empires-minEmpires, 1)
	at(atmosHole, neutralFrame+int(s.atmos), 1)
	// The species' flag in each colour, the chosen one's 1 px edge white.
	for c := range game.NumColours {
		x := ngColX0 + c*ngColDX
		drawSprite(sc, a.flag(s.species, c), float64(x), ngColY)
		if c == s.colour {
			vector.StrokeRect(sc, float32(x)+zoom/2, ngColY+zoom/2, zoom*flagW-zoom, zoom*flagH-zoom, zoom, hoverColour, false)
		}
	}
	half(ringBank, s.colour, ngBeginX, ngBeginPortY)
	half(portraitBank, int(s.species), ngBeginX, ngBeginPortY)
}

// centred prints s (lines centred one by one) in r's width from native y.
func centred(frame *ebiten.Image, s string, face *text.GoTextFace, r rect, y float64, c color.Color) {
	centreAt(frame, s, face, float64(r.X0+r.X1)/2, y, c)
}

// centreAt is centred about native x.
func centreAt(frame *ebiten.Image, s string, face *text.GoTextFace, x, y float64, c color.Color) {
	for i, l := range strings.Split(s, "\n") {
		w := ui.Width(l, face)
		ui.Print(frame, l, face, x-w/2, y+float64(i)*face.Size*1.25, c)
	}
}

// Text writes the summary, the facts, the list and the labels.
func (s *newGameScreen) Text(a *app, frame *ebiten.Image) {
	centreAt(frame, s.summaryLines(), bigFace, ngSumX, ngSumY, hoverColour)
	dc := hoverColour
	if s.onDiff {
		dc = yellow
	}
	ui.Print(frame, "Bots: "+s.diff.String(), bigFace, float64(ngDiff.X0), float64(ngDiff.Y0), dc)

	in := s.species.Info()
	centreAt(frame, in.Name, bigFace, ngNameX, ngNameY, colourText[s.colour])
	ui.Print(frame, strings.Join(ngFactsFit(in), "\n"), smallFace, ngDescX0, ngDescY, colourText[s.colour])

	centred(frame, "Player Species", bigFace, ngTitle, float64(ngTitle.Y0+ngTitle.Y1)/2-bigFace.Size/2-zoom, textColour)
	for i := range int(game.NumSpecies) {
		c := textColour
		switch {
		case game.Species(i) == s.species:
			c = yellow
		case i == s.line:
			c = hoverColour
		}
		ui.Print(frame, game.Species(i).String(), ngFace, float64(ngList.X0+padX),
			float64(ngList.Y0+padY)+float64(i)*ngPitch, c)
	}

	for k, str := range ngLabels {
		c := hoverColour
		if s.hole == k {
			c = yellow
		}
		centred(frame, str, smallFace, ngHoles[k], ngLabelY(k), c)
	}
	c := hoverColour
	if s.hole == beginHole {
		c = yellow
	}
	centred(frame, "Begin New Game", bigFace, ngHoles[beginHole], ngBeginY, c)
}
