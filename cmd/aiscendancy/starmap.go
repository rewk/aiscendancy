package main

import (
	"fmt"
	"image/color"
	"math"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/rewk/aiscendancy/cam"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The galactic display: the cluster in 3D (package cam projects), stars
// as cos_star.shp frames 4t..4t+3 (type t, level by depth: the near side
// shines), frame 54 the hover ring, a raceflag under each known home.
// Hover a star: its facts in the right column; click: its system; drag
// rotates (pitch may go below the plane), wheel zooms; arrows pan, L
// lanes, R resets the camera.
//
// The chrome is 1starmap.tmp frame 0 (640x480), measured as transparent
// runs per row and column; every box has opaque borders. Original px:
//
//	[7,466)x[7,473)       the big area
//	[474,633)x[7,44)      top right: the original's day counter
//	[474,549)x[52,83), [557,633)x[52,83)   the two turn buttons
//	[474,633)x[130+39k, 161+39k), k = 0..4  Planets, Ships, Research,
//	                      Special Ability, Species: 159x31 each, 8 px apart
//	[479+39i, 510+39i)x[364,395) and x[403,434), i = 0..3  the round buttons
//
// The stacked boxes are filled black, each a button with its label and
// its 51x31 1starmap.shp picture at the right end: 0 Planets (frame 5),
// 1 Ships (4), 2 Research (3), 3 Special Ability (2, the shuttle), 4
// Species (our flag). 1starmap.shp frames, origin (0,0), in their holes:
// 67 (159x37) the counter's background, 55-64 the digits (31x37, five
// from x 476: 4 px left over, split 2/2), 65 (75x31) NEXT DAY, 66 (76x31)
// UNTIL AN EVENT.
var (
	mapArea  = orig(7, 7, 466, 473)
	mapTop   = orig(474, 7, 633, 44)
	mapNext  = orig(474, 52, 549, 83)
	mapSkip  = orig(557, 52, 633, 83)
	mapStack = func() (out [5]rect) {
		for k := range out {
			out[k] = orig(474, 130+39*k, 633, 161+39*k)
		}
		return out
	}()
)

// mapTrim is everything right of the big area, painted black before the
// chrome so the scene never shows through the button holes.
var mapTrim = rect{mapArea.X1, 0, screenW, screenH}

const (
	mapBank      = "1starmap"
	digit0       = 55 // digit d is frame digit0+d
	nextFrame    = 65
	skipFrame    = 66
	resFrame     = 3 // the Research button's picture
	shipsFrame   = 4 // the Ships button's picture
	abilityFrame = 2 // the Special Ability button's picture (the shuttle)
	planetsFrame = 5 // the Planets button's picture
	dayBox       = 67
	dayDigits    = 5
	digitW       = 31 // original px
)

// The buttons, as button reports them; unique across the app.
const (
	noButton = iota
	nextButton
	skipButton
	logButton      // the day counter: the log screen
	researchButton // stacked box 2
	shipsButton    // stacked box 1
	abilityButton  // stacked box 3: the species' ability used
	speciesButton  // stacked box 4
	planetsButton  // stacked box 0
	designsButton  // the fleet screen's stacked box 2
)

// resButton..planetButton are the stacked boxes of the original's five
// buttons.
const (
	resButton    = 2
	shipButton   = 1
	abilityBox   = 3
	speciesBox   = 4
	planetButton = 0
)

// newsBox is the news, one black box over the chrome's two rows of round
// holes (the original's rotation and display filters: 31 px wide at x
// 479, 518, 557, 596, rows from y 364 and 403) and the flag row under
// them (its species filter, a dark band [474,633)x[441,473)): 13
// smallFace lines.
var newsBox = orig(474, 364, 633, 473)

// The camera starts lower than the system display's (the cluster is a
// ball, not a disc) and the drag may tip it under the plane.
const (
	mapPitch           = 15 * math.Pi / 180
	minPitch, maxPitch = -80 * math.Pi / 180, 80 * math.Pi / 180
)

// mapCamera centres the cluster in the big area: Dist 4, Focal 1600 =
// the unit ball spans ±400 native px.
func mapCamera() cam.Camera {
	c := cam.Default(float64(mapArea.X0+mapArea.X1)/2, float64(mapArea.Y0+mapArea.Y1)/2)
	c.Pitch = mapPitch
	return c
}

// clampMapCamera holds Pitch within ±80° and Focal within cam's limits
// (not cam.Clamp: it keeps the eye above the plane).
func clampMapCamera(c *cam.Camera) {
	c.Pitch = min(max(c.Pitch, minPitch), maxPitch)
	c.Focal = min(max(c.Focal, cam.MinFocal), cam.MaxFocal)
}

// A stacked box (62 native px) holds two 16 px lines or three 12 px
// (linesFit). The lanes' colours:
var (
	blueLane = color.RGBA{120, 170, 255, 255}
	redLane  = color.RGBA{255, 90, 70, 255}
)

const padX, padY = 6 * zoom, 3 * zoom // native px inside a box

// Sprite frames of cos_star.shp.
const (
	starBank  = "cos_star"
	hoverRing = 54
	levels    = 4 // brightness levels per star type
)

// The flag under a home star: raceflag.shp frame = species, 15x25, its
// top 4 original px under the star, centred (starmap-start-dosbox).
const (
	flagBank       = "raceflag"
	flagDX, flagDY = -7 * zoom, 4 * zoom
)

// empireColours are the original's eight colours (racering.shp,
// sampled, brightened for 1 px marks), by Empire.Colour; the eighth,
// white, is no empire's.
var empireColours = [8]color.RGBA{{0, 150, 75, 255}, {140, 80, 150, 255}, {210, 160, 40, 255}, {220, 110, 135, 255},
	{130, 110, 70, 255}, {40, 110, 190, 255}, {190, 90, 10, 255}, {180, 180, 180, 255}}

func empireColour(i int) color.RGBA { return empireColours[i%len(empireColours)] }

// The star marks (starmap-screen, measured), native px: an explored star
// gets four 1-original-px dots in the lane colour markGap away, a
// colonised one a circle of markRadius in the colonist's colour; with
// several colonists (as the original) concentric circles markStep apart,
// the dominant's the outermost.
const (
	markGap    = 4 * zoom
	markRadius = 5 * zoom
	markStep   = 2 * zoom
)

// starMark is star i's mark for e: none (unknown), dots (no colonist)
// or circles (starColonists).
func starMark(e *game.Empire, g *game.Game, cl *game.Cluster, i int) (marked bool, colonists []int) {
	if !knows(e, i) {
		return false, nil
	}
	return true, starColonists(e, g, cl, i)
}

// starColonists is the owners of star's colonies, the dominant first
// (the original's rule unknown): the most colonies,
// then the most population (g's colonies there whose surface e
// SeesSurface: a foreign Surface Cloaker hides its people; none without
// a game), then the lower index.
func starColonists(e *game.Empire, g *game.Game, cl *game.Cluster, star int) []int {
	n, pop := map[int]int{}, map[int]int{}
	var out []int
	for _, p := range cl.Stars[star].System.Planets {
		if p.Owner < 0 {
			continue
		}
		if n[p.Owner] == 0 {
			out = append(out, p.Owner)
		}
		n[p.Owner]++
	}
	if g != nil {
		for _, o := range out {
			for _, c := range g.Empires[o].Colonies {
				if c.Star == star && e.SeesSurface(c.Planet) {
					pop[o] += c.Pop
				}
			}
		}
	}
	slices.SortFunc(out, func(a, b int) int {
		switch {
		case n[a] != n[b]:
			return n[b] - n[a]
		case pop[a] != pop[b]:
			return pop[b] - pop[a]
		}
		return a - b
	})
	return out
}

// homeFlag is a home star's flag, in its empire's colour.
type homeFlag struct {
	star   int
	sp     game.Species
	colour int
}

// colourOf is empire i's colour; i itself without a game (view tests).
func colourOf(g *game.Game, i int) int {
	if g == nil || i < 0 || i >= len(g.Empires) {
		return i
	}
	return g.Empires[i].Colour
}

// homeFlags lists the home stars e knows, in empire order. A "home" is
// the empire's first colony, not Empire.HomeStar (the world NewGame
// placed): after a capture the flag moves to its oldest colony left.
// None without a game.
func homeFlags(e *game.Empire, g *game.Game) []homeFlag {
	if g == nil {
		return nil
	}
	var out []homeFlag
	for _, o := range g.Empires {
		if len(o.Colonies) == 0 {
			continue
		}
		if home := o.Colonies[0].Star; knows(e, home) {
			out = append(out, homeFlag{home, o.Species, o.Colour})
		}
	}
	return out
}

func starFrame(t game.StarType, level int) int { return levels*int(t) + level }

// brightness is the depth cue: the unit ball's depth span [dist−1,
// dist+1] cut in four buckets, 3 nearest to 0 farthest, clamped; depth =
// dist falls in bucket 1.
func brightness(depth, dist float64) int {
	b := 3 - int(math.Floor((depth-(dist-1))/0.5))
	return min(max(b, 0), 3)
}

// fade is brightness for lines: 1 at the ball's near edge, fadeFloor at
// the far edge, clamped.
const fadeFloor = 0.3

func fade(depth, dist float64) float64 {
	t := (depth - (dist - 1)) / 2 // 0 near, 1 far
	return min(max(1-t*(1-fadeFloor), fadeFloor), 1)
}

// markFade is fade softened a little for the colony circles: markFadeK
// of the lanes' dimming, 1 near, about 0.55 far.
const markFadeK = 0.65

func markFade(depth, dist float64) float64 { return 1 - (1-fade(depth, dist))*markFadeK }

// shade scales a colour's channels by f, keeping it opaque.
func shade(c color.RGBA, f float64) color.RGBA {
	return color.RGBA{uint8(float64(c.R) * f), uint8(float64(c.G) * f), uint8(float64(c.B) * f), 255}
}

// pickStar is the point nearest (x,y) within 6 original px, or −1.
func pickStar(ps []cam.Point, x, y float64) int {
	found, best := -1, 6.0*zoom
	for i, p := range ps {
		if d := math.Hypot(x-p.X, y-p.Y); d <= best {
			found, best = i, d
		}
	}
	return found
}

// linesFit is how many face lines fit h native px, padY above and below.
func linesFit(h float64, face *text.GoTextFace) int {
	return int((h - 2*padY) / (face.Size * 1.25))
}

// clip keeps the first n lines of s.
func clip(s string, n int) string {
	ls := strings.Split(s, "\n")
	return strings.Join(ls[:min(len(ls), max(n, 0))], "\n")
}

type starmapScreen struct {
	cam      cam.Camera
	lanes    bool
	drag     drag
	hover    int // star index, −1 when nothing is hovered
	btn      int // the button under the cursor, noButton none
	news     newsView
	nav      *navPick // the pick mode (navigate.go), nil none
	metFlags []int    // the Species box's flags (speciesFlags), reused each frame
}

func newStarmapScreen() *starmapScreen {
	return &starmapScreen{cam: mapCamera(), lanes: true, hover: -1, news: newNewsView()}
}

func (s *starmapScreen) Enter(*app) {}

// project places every star for this frame, by star index.
func (s *starmapScreen) project(a *app) []cam.Point {
	ps := make([]cam.Point, len(a.cl.Stars))
	for i, st := range a.cl.Stars {
		ps[i] = s.cam.Project(st.Pos)
	}
	return ps
}

func (s *starmapScreen) Update(a *app) error {
	if s.nav != nil {
		if popped, err := s.navUpdate(a, inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)); popped {
			return err
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyL) {
		s.lanes = !s.lanes
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		s.cam = mapCamera()
	}
	if onSpeciesKey(a, inpututil.IsKeyJustPressed(ebiten.KeyS)) {
		return nil
	}
	focal := s.cam.Focal
	x, y, click := steer(a, &s.cam, &s.drag, mapArea)
	_, wy := ebiten.Wheel()
	press := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	wheeled, pressed := s.news.input(a, x, y, wy, press)
	if wheeled {
		s.cam.Focal = focal // steer zooms on the wheel anywhere: over the news it scrolls
	}
	clampMapCamera(&s.cam)
	s.hover = -1
	if mapArea.has(x, y) {
		s.hover = pickStar(s.project(a), x, y)
	}
	if s.nav != nil {
		s.nav.at(a.e, s.hover)
	}
	if click {
		if err := s.click(a, x, y); err != nil || s.nav != nil && a.top() != s {
			return err // the pick ended: this screen is gone
		}
	}
	// A press outside mapArea starts no drag (steer), so no click: the
	// buttons act on the press, unless the news took it.
	s.btn = s.button(x, y)
	if press && !pressed {
		s.press(a, s.btn)
	}
	return nil
}

func (s *starmapScreen) press(a *app, b int) {
	if a.pressTurn(b) {
		return
	}
	switch b {
	case researchButton:
		a.push(newResearchScreen())
	case shipsButton:
		a.push(newFleetScreen())
	case abilityButton:
		s.useAbility(a)
	case speciesButton:
		a.push(newSpeciesScreen(-1))
	case planetsButton:
		a.push(newPlanetsScreen())
	}
}

// useAbility is the Special Ability press: its Reports logged, the first
// one's text or the refusal as a toast.
func (s *starmapScreen) useAbility(a *app) {
	before := a.e.Research.Known // a theft's discoveries (learnt)
	reps, err := a.g.UseAbility(a.player)
	a.learnt(before, a.record(reps))
	if err != nil {
		a.toastFor(err.Error(), true)
	} else if len(reps) > 0 {
		a.toastFor(reps[0].Event.Text, false)
	}
}

// abilityLines are the Special Ability box's lines: AbilityText and
// "ready", "in N days" or "passive".
func abilityLines(a *app) [2]string {
	info := a.e.Species.Info()
	ready, days := a.g.AbilityState(a.player)
	switch {
	case info.Recharge == 0:
		return [2]string{info.AbilityText, "passive"}
	case a.g.AbilityAsleep(a.player):
		return [2]string{info.AbilityText, "no system controlled"}
	case ready:
		return [2]string{info.AbilityText, "ready"}
	}
	return [2]string{info.AbilityText, fmt.Sprintf("in %d days", days)}
}

// researchLines are the Research box's lines: the topic and its days at
// today's rate ("in N days", "never" at rate 0), "Singularity" once
// the tree is complete, or "None".
func researchLines(a *app) [2]string {
	r := a.e.Research
	if r.Complete() {
		return [2]string{"Singularity", "reached"}
	}
	if r.Topic == game.NoTech {
		return [2]string{"None", ""}
	}
	d := r.Days(r.Topic, a.e.ResearchRate())
	state := "never"
	if d >= 0 {
		state = fmt.Sprintf("in %d days", d)
	}
	return [2]string{game.Techs[r.Topic].Name, state}
}

// planetsLines are the Planets box's lines: "N planets in M systems".
func planetsLines(a *app) [2]string {
	n, m := len(a.e.Colonies), a.e.ControlledSystems()
	return [2]string{fmt.Sprintf("%d planet%s in %d system%s", n, plural(n), m, plural(m)), ""}
}

// shipsLines are the Ships box's lines: "N/cap" (Empire.Fleet: flying
// plus under construction), then "K building" when K > 0.
func shipsLines(a *app) [2]string {
	n, capa := a.e.Fleet()
	k := n - len(a.e.Ships)
	second := ""
	if k > 0 {
		second = fmt.Sprintf("%d building", k)
	}
	return [2]string{fmt.Sprintf("%d/%d", n, capa), second}
}

// button is the button under (x,y), noButton none.
func (s *starmapScreen) button(x, y float64) int {
	if b := turnButton(x, y); b != noButton {
		return b
	}
	switch {
	case mapStack[resButton].has(x, y):
		return researchButton
	case mapStack[shipButton].has(x, y):
		return shipsButton
	case mapStack[abilityBox].has(x, y):
		return abilityButton
	case mapStack[speciesBox].has(x, y):
		return speciesButton
	case mapStack[planetButton].has(x, y):
		return planetsButton
	}
	return noButton
}

// dayFrames are day's digit frames, zero-padded, capped at 99999.
func dayFrames(day int) [dayDigits]int {
	var out [dayDigits]int
	day = min(max(day, 0), 99999)
	for k := dayDigits - 1; k >= 0; k-- {
		out[k] = digit0 + day%10
		day /= 10
	}
	return out
}

// click opens the system of the star under (x,y) when known (an unknown
// star cannot be opened); in pick mode it is navClick's.
func (s *starmapScreen) click(a *app, x, y float64) error {
	if !mapArea.has(x, y) {
		return nil
	}
	i := pickStar(s.project(a), x, y)
	switch {
	case i < 0:
	case s.nav != nil:
		return s.navClick(a, i)
	case knows(a.e, i):
		a.push(newSystemScreen(i))
	}
	return nil
}

// The depth pass: every lane cut in laneSegs pieces, the lane ships'
// pennants and the stars (each with its mark, flag and pennants) drawn
// far to near, so a nearer one covers a farther one whichever kind. A
// piece sorts at its farther end's depth: one touching a star is never
// nearer than it, so a star covers its own lanes' ends; at equal depth
// pieces come first, then pennants, then stars.
const laneSegs = 8

// seamPx is how far a lane piece overlaps the next, native px.
const seamPx = 1

const (
	itemSeg = iota
	itemShip
	itemStar
)

// mapItem is one draw of the depth pass: lane i's piece seg, ships[i]'s
// pennant, or star i.
type mapItem struct{ kind, i, seg int }

// mapOrder is the depth pass's draws, the farthest first: lanes' pieces
// (none when lanes is nil), ships' pennants, stars.
func mapOrder(cl *game.Cluster, ps []cam.Point, lanes []int, ships []game.LaneSighting) []mapItem {
	type keyed struct {
		it    mapItem
		depth float64
	}
	var ks []keyed
	for _, li := range lanes {
		p, q := ps[cl.Lanes[li].A].Depth, ps[cl.Lanes[li].B].Depth
		for k := range laneSegs {
			d0, d1 := p+(q-p)*float64(k)/laneSegs, p+(q-p)*float64(k+1)/laneSegs
			ks = append(ks, keyed{mapItem{itemSeg, li, k}, max(d0, d1)})
		}
	}
	for i, ls := range ships {
		_, _, d := laneAt(cl, ps, ls)
		ks = append(ks, keyed{mapItem{itemShip, i, 0}, d})
	}
	for i := range ps {
		ks = append(ks, keyed{mapItem{itemStar, i, 0}, ps[i].Depth})
	}
	slices.SortStableFunc(ks, func(x, y keyed) int { return -cmpFloat(x.depth, y.depth) })
	out := make([]mapItem, len(ks))
	for k, x := range ks {
		out[k] = x.it
	}
	return out
}

// ringed is the star under the hover ring: the hovered one, else the
// hovered news line's (newsStar), not while Navigate… picks (a line's
// click is no pick); −1 none.
func (s *starmapScreen) ringed(a *app) int {
	if s.hover >= 0 || s.nav != nil {
		return s.hover
	}
	if i := s.news.hovered; i >= 0 && i < len(a.log) {
		return newsStar(a, a.log[i].Ev)
	}
	return -1
}

// newsStar is the star a news line points at: its own (its click opens
// that system), else for an offer or a request (from or to Other) the
// other empire's home star when the map shows it theirs (known, a planet
// of theirs there); −1 none.
func newsStar(a *app, ev game.Event) int {
	if ev.Star >= 0 {
		return ev.Star
	}
	switch ev.Kind {
	case game.PeaceOffered, game.AllianceOffered, game.TradeOffered, game.HelpAsked:
	default:
		return -1
	}
	if ev.Other < 0 || ev.Other >= len(a.g.Empires) {
		return -1
	}
	h := a.g.Empires[ev.Other].HomeStar()
	if !knows(a.e, h) || !slices.Contains(starColonists(a.e, a.g, a.cl, h), ev.Other) {
		return -1
	}
	return h
}

// Render paints the depth pass (mapOrder), the stars at whole pixels
// (1-5 px dots: a sub-pixel place blurs them) with their marks and a
// fight's red ring (fightStars), each lane shaded by its mean depth as
// the original; the hover ring, then the chrome over it
// (its border trims what spilled), the boxes and their pictures, the
// news box and the turn box.
func (s *starmapScreen) Render(a *app, sc *ebiten.Image) {
	sc.Fill(color.Black)
	ps := s.project(a)
	var lanes []int
	switch {
	case s.lanes:
		lanes = knownLanes(a.e, a.cl)
	case s.nav != nil && s.nav.star == s.hover:
		lanes = s.nav.lanes // the route shows with the lanes off
	}
	ships := laneShips(a.g, a.e)
	at := func(frame, star int) {
		drawSprite(sc, a.spr(starBank, frame), math.Round(ps[star].X), math.Round(ps[star].Y))
	}
	pennants := starPennants(a.g, a.e)
	fights := fightStars(a)
	flags := map[int]homeFlag{}
	for _, f := range homeFlags(a.e, a.g) {
		flags[f.star] = f
	}
	for _, it := range mapOrder(a.cl, ps, lanes, ships) {
		switch it.kind {
		case itemSeg:
			l := a.cl.Lanes[it.i]
			p, q := ps[l.A], ps[l.B]
			c := blueLane
			if l.Red {
				c = redLane
			}
			c = shade(c, fade((p.Depth+q.Depth)/2, s.cam.Dist))
			w := float32(zoom)
			if s.navLane(it.i) {
				c, w = yellow, 2*zoom
			}
			// Butt ends meeting leave an antialiased seam: inner ends
			// reach seamPx on, into the next piece's opaque core.
			f0, f1 := float64(it.seg)/laneSegs, float64(it.seg+1)/laneSegs
			if n := math.Hypot(q.X-p.X, q.Y-p.Y); n > 0 {
				if it.seg > 0 {
					f0 -= seamPx / n
				}
				if it.seg < laneSegs-1 {
					f1 += seamPx / n
				}
			}
			vector.StrokeLine(sc, float32(p.X+(q.X-p.X)*f0), float32(p.Y+(q.Y-p.Y)*f0),
				float32(p.X+(q.X-p.X)*f1), float32(p.Y+(q.Y-p.Y)*f1), w, c, true)
			continue
		case itemShip:
			x, y, depth := laneAt(a.cl, ps, ships[it.i])
			a.drawPennant(sc, brightness(depth, s.cam.Dist), colourOf(a.g, ships[it.i].Empire), x, y)
			continue
		}
		i := it.i
		at(starFrame(a.cl.Stars[i].Type, brightness(ps[i].Depth, s.cam.Dist)), i)
		marked, cols := starMark(a.e, a.g, a.cl, i)
		x, y := float32(math.Round(ps[i].X)), float32(math.Round(ps[i].Y))
		switch {
		case !marked:
		case len(cols) > 0:
			f := markFade(ps[i].Depth, s.cam.Dist)
			for k, o := range cols {
				r := float32(markRadius + (len(cols)-1-k)*markStep)
				vector.StrokeCircle(sc, x, y, r, zoom, shade(empireColour(colourOf(a.g, o)), f), true)
			}
		default:
			for _, d := range [4][2]float32{{-markGap, 0}, {markGap, 0}, {0, -markGap}, {0, markGap}} {
				vector.FillRect(sc, x+d[0], y+d[1], zoom, zoom, blueLane, false)
			}
		}
		if slices.Contains(fights, i) {
			// A fight open to the player: a red ring past the colony
			// rings, "R n" (the day's rounds there) left of it.
			r := float32(markRadius + len(cols)*markStep)
			vector.StrokeCircle(sc, x, y, r, zoom, red, true)
			l := fmt.Sprintf("R %d", a.g.RoundsAt(i))
			ui.Print(sc, l, smallFace, float64(x-r)-ui.Width(l, smallFace)-zoom, float64(y)-smallFace.Size*0.6, red)
		}
		if f, ok := flags[i]; ok {
			drawSprite(sc, a.flag(f.sp, f.colour), float64(x)+flagDX, float64(y)+flagDY)
		}
		for k, p := range pennants[i] {
			alpha := 1.0
			if p.ghost {
				alpha = ghostAlpha
			}
			a.drawPennantAlpha(sc, brightness(ps[i].Depth, s.cam.Dist), colourOf(a.g, p.empire), float64(x)+pennantDX, float64(y)+float64(k*pennantStep), alpha)
		}
	}
	if r := s.ringed(a); r >= 0 {
		at(hoverRing, r)
	}

	fillRect(sc, mapTrim)
	drawSprite(sc, a.spr("1starmap.tmp", 0), 0, 0)
	for _, r := range mapStack {
		fillRect(sc, r)
	}
	for _, b := range []struct{ box, frame int }{{resButton, resFrame}, {shipButton, shipsFrame}, {abilityBox, abilityFrame}, {planetButton, planetsFrame}} {
		r := mapStack[b.box]
		drawSprite(sc, a.spr(mapBank, b.frame), float64(r.X1-zoom*51), float64(r.Y0))
	}
	drawNewsBox(sc, newsBox)
	if a.e != nil { // the Species box's picture: our flag, as the original's, then the met's
		s.metFlags = speciesFlags(a, s.metFlags[:0])
		pic := speciesPic()
		for k, j := range s.metFlags {
			o := a.g.Empires[j]
			var lum float32 = 1
			if !o.Alive() {
				lum = 0.4
			}
			drawDim(sc, a.flag(o.Species, o.Colour), speciesFlagX(pic, k, len(s.metFlags)), float64(pic.Y0), lum)
		}
	}
	drawTurnBox(a, sc)
}

// speciesFlags appends to dst the empires the Species box flags: ours,
// then every one met (StanceWith not Unknown), in Game.Empires order.
func speciesFlags(a *app, dst []int) []int {
	dst = append(dst, a.player)
	for j := range a.g.Empires {
		if j != a.player && a.e.StanceWith(j) != game.Unknown {
			dst = append(dst, j)
		}
	}
	return dst
}

// speciesPic is the Species box's picture, native px: from 2 px past the
// "Species" label to a 6 px right margin, 3 px down (seven flags fit
// side by side).
func speciesPic() rect {
	r := mapStack[speciesBox]
	x0 := r.X0 + padX + int(math.Ceil(ui.Width("Species", bigFace))) + 2*zoom
	return rect{x0, r.Y0 + 3*zoom, r.X1 - zoom*6, r.Y0 + (3+flagH)*zoom}
}

// speciesFlagX is the left x of flag k of n in pic: flush right, a zoom
// px apart, or overlapping evenly when they do not fit side by side.
func speciesFlagX(pic rect, k, n int) float64 {
	w := flagW * zoom
	step := w + zoom
	if n > 1 {
		step = min(step, (pic.X1-pic.X0-w)/(n-1))
	}
	return float64(pic.X1 - w - (n-1-k)*step)
}

// fillRect paints r black, fillRectWith in c.
func fillRect(sc *ebiten.Image, r rect) { fillRectWith(sc, r, color.Black) }

func fillRectWith(sc *ebiten.Image, r rect, c color.Color) {
	vector.FillRect(sc, float32(r.X0), float32(r.Y0), float32(r.X1-r.X0), float32(r.Y1-r.Y0), c, false)
}

// Text writes the boxes' labels (yellow hovered), the lanes' traffic
// counts, the news, and a hovered star's line at the map's bottom.
func (s *starmapScreen) Text(a *app, frame *ebiten.Image) {
	hot := func(b int) color.RGBA {
		if s.btn == b {
			return yellow
		}
		return textColour
	}
	label := func(b int, r rect, str string) { putBox(frame, r, str, bigFace, hot(b)) }
	label(speciesButton, mapStack[speciesBox], "Species")
	s.news.text(a, frame)
	label(planetsButton, mapStack[planetButton], "Planets")
	label(shipsButton, mapStack[shipButton], "Ships")
	label(researchButton, mapStack[resButton], "Research")
	if a.e != nil { // two small lines under each label, left of the picture
		for _, b := range []struct {
			box, button int
			ls          [2]string
		}{{resButton, researchButton, researchLines(a)}, {planetButton, planetsButton, planetsLines(a)}, {shipButton, shipsButton, shipsLines(a)}} {
			r := mapStack[b.box]
			w := float64(r.X1 - r.X0 - zoom*51 - 2*padX)
			ui.Print(frame, fitWidth(b.ls[0], w, smallFace)+"\n"+b.ls[1], smallFace,
				float64(r.X0+padX), float64(r.Y0+padY)+bigFace.Size*1.25, hot(b.button))
		}
	}
	if a.g != nil {
		// Three small lines fit: the text wrapped over two, the state.
		ls := abilityLines(a)
		r := mapStack[abilityBox]
		cols := int(float64(r.X1-r.X0-zoom*51-2*padX) / ui.Width("M", smallFace))
		text := strings.SplitN(wrap(ls[0], cols), "\n", 3)[:min(2, strings.Count(wrap(ls[0], cols), "\n")+1)]
		putBox(frame, r, strings.Join(append(text, ls[1]), "\n"), smallFace, hot(abilityButton))
	}
	if s.nav != nil {
		s.navText(a, frame)
		return
	}
	if s.hover < 0 {
		return
	}
	// A hovered star: one white line at the map's bottom, the buttons kept.
	ui.Print(frame, starText(a.g, a.e, s.hover), bigFace, float64(mapArea.X0+padX),
		float64(mapArea.Y1-padY)-bigFace.Size*1.25, color.RGBA{255, 255, 255, 255})
}

// fitWidth cuts s, by runes, to at most w native px in face.
func fitWidth(s string, w float64, face *text.GoTextFace) string {
	r := []rune(s)
	for len(r) > 0 && ui.Width(string(r), face) > w {
		r = r[:len(r)-1]
	}
	return string(r)
}

// The ships' pennants, the original's: cos_ship's frame by depth
// (brightness: 4x3 far, 5x5 near, each in two shades) in the empire's
// colour, right of the point it marks, its tip on it. A star's empires
// with ships there stack down pennantStep apart, their tips pennantDX
// right of the star; a ship in a lane has its own at its place.
const (
	pennantDX   = 9 * zoom
	pennantStep = 6 * zoom
)

// drawPennant draws the level's pennant in colour c, its tip (the left
// end of its middle row) at (x, y), rounded to whole px.
func (a *app) drawPennant(sc *ebiten.Image, level, c int, x, y float64) {
	a.drawPennantAlpha(sc, level, c, x, y, 1)
}

// ghostAlpha is a ghost pennant's opacity: a ship being built or refitted.
const ghostAlpha = 0.45

// drawPennantAlpha is drawPennant at opacity alpha.
func (a *app) drawPennantAlpha(sc *ebiten.Image, level, c int, x, y, alpha float64) {
	tip := 1 // the 4x3 frames' middle row; the 5x5's is 2
	if level >= 2 {
		tip = 2
	}
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(zoom, zoom)
	op.GeoM.Translate(math.Round(x), math.Round(y)-float64(tip*zoom))
	op.ColorScale.ScaleAlpha(float32(alpha))
	a.pennant(level, c).Draw(sc, &op)
}

// pennant is a star's pennant: the empire's, a ghost when it stands
// only for e's ships being built or refitted there.
type pennant struct {
	empire int
	ghost  bool
}

// starPennants is, by star, the empires with ships there out of the
// lanes that e sees: e first (a ghost where its colonies build or
// refit a ship and no other of its ships stands), then ShipsSeen's in
// empire order; nil without a game or an empire.
func starPennants(g *game.Game, e *game.Empire) [][]pennant {
	if g == nil || e == nil {
		return nil
	}
	out := make([][]pennant, len(g.Cluster.Stars))
	for _, sh := range e.Ships {
		if sh.Lane < 0 && e.Refitting(sh) == nil && len(out[sh.Star]) == 0 {
			out[sh.Star] = append(out[sh.Star], pennant{empire: e.Index})
		}
	}
	for _, c := range e.Colonies {
		if pj := c.Project; pj != nil && (pj.Item == game.Ship || pj.Refit != "") && len(out[c.Star]) == 0 {
			out[c.Star] = append(out[c.Star], pennant{empire: e.Index, ghost: true})
		}
	}
	for star := range out {
		for _, sg := range g.ShipsSeen(e, star) {
			if !slices.ContainsFunc(out[star], func(p pennant) bool { return p.empire == sg.Empire }) {
				out[star] = append(out[star], pennant{empire: sg.Empire})
			}
		}
	}
	return out
}

func laneShips(g *game.Game, e *game.Empire) []game.LaneSighting {
	if g == nil || e == nil {
		return nil
	}
	return g.LaneShips(e)
}

// laneAt is where ls is drawn and its depth: Frac of the way from the
// star it left to the far one.
func laneAt(c *game.Cluster, ps []cam.Point, ls game.LaneSighting) (x, y, depth float64) {
	p, q := ps[ls.From], ps[c.Other(ls.Lane, ls.From)]
	f := ls.Frac
	return p.X + (q.X-p.X)*f, p.Y + (q.Y-p.Y)*f, p.Depth + (q.Depth-p.Depth)*f
}

// knows, knowsLane and starLabel are the fog (game/fog.go), nil-safe:
// without an empire (bare view tests) everything is known.
func knows(e *game.Empire, star int) bool { return e == nil || e.Knows(star) }

func knowsLane(e *game.Empire, li int) bool { return e == nil || e.KnowsLane(li) }

func starLabel(e *game.Empire, cl *game.Cluster, star int) string {
	if e == nil {
		return cl.Stars[star].Name
	}
	return e.StarLabel(star)
}

func knownLanes(e *game.Empire, cl *game.Cluster) []int {
	var out []int
	for li := range cl.Lanes {
		if knowsLane(e, li) {
			out = append(out, li)
		}
	}
	return out
}

// starLanes counts star's lanes e sees.
func starLanes(e *game.Empire, cl *game.Cluster, star int) int {
	n := 0
	for _, li := range cl.Stars[star].Lanes {
		if knowsLane(e, li) {
			n++
		}
	}
	return n
}

// starText is the hovered star's line: "<name>, <type>, N planets, M
// lanes", then ", owned by <species>" when settled (the ring's
// Colonist), or "<type>, unexplored, M lanes"; M counts the lanes e sees.
func starText(g *game.Game, e *game.Empire, star int) string {
	cl := g.Cluster
	st := cl.Stars[star]
	if !knows(e, star) {
		return fmt.Sprintf("%s, unexplored, %d lanes", st.Type, starLanes(e, cl, star))
	}
	txt := fmt.Sprintf("%s, %s, %d planets, %d lanes", st.Name, st.Type, len(st.System.Planets), starLanes(e, cl, star))
	if cols := starColonists(e, g, cl, star); len(cols) > 0 {
		txt += ", owned by " + g.Empires[cols[0]].Species.String()
	}
	return txt
}

// mapSummary is the -shot line's cluster: the density and the counts.
func mapSummary(cl *game.Cluster) string {
	red := 0
	for _, l := range cl.Lanes {
		if l.Red {
			red++
		}
	}
	return fmt.Sprintf("%s  %d stars\n%d lanes, %d red", cl.Density, len(cl.Stars), len(cl.Lanes), red)
}
