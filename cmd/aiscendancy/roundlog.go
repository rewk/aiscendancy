package main

import (
	"fmt"
	"image/color"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The round log (docs/screens.md "Combat replay"):
// an ephemeral overlay at the view's top right, in sync with the replay.
// layReplay adds each round's lines with the tick they show at: a
// "Round N" header with the round's first shot landing or blast, a line
// per shot at its landing (r.landsAt), one per ship destroyed or
// structure razed as the blasts start; none for moves. The last
// rlogMax shown, smallFace on badgeBack plates, right-aligned, each
// name led by its owner's flag (two a shot line: shooter and target).
// It stays while the system screen does (until the screen is left; a
// new one starts empty); a skip shows every line at once; a replay
// appended (a round stepped, the day's close) adds to it.
const rlogMax = 8

// logPart is a flag (empire −1: none) then a text, keep after it never
// cut (a shot's arrow).
type logPart struct {
	empire     int
	text, keep string
}

// logLine is a line of the round log: its parts, colour and the absolute
// tick it shows at.
type logLine struct {
	t     int
	parts []logPart
	c     color.RGBA
}

// roundLog is the overlay's lines in show order.
type roundLog struct {
	lines []logLine
}

// add puts ls in show order (stable: a use noted now during a replay
// goes before the lines still to come).
func (l *roundLog) add(ls ...logLine) {
	l.lines = append(l.lines, ls...)
	slices.SortStableFunc(l.lines, func(a, b logLine) int { return a.t - b.t })
}

// cut shows every line by tick at once (a skip). Nil-safe.
func (l *roundLog) cut(tick int) {
	if l == nil {
		return
	}
	for i := range l.lines {
		l.lines[i].t = min(l.lines[i].t, tick)
	}
}

// shown is the lines shown at tick: the last rlogMax of those due.
func (l *roundLog) shown(tick int) []logLine {
	if l == nil {
		return nil
	}
	n := 0
	for n < len(l.lines) && l.lines[n].t <= tick {
		n++
	}
	return l.lines[max(0, n-rlogMax):n]
}

var logWhite = color.RGBA{230, 230, 230, 255}

// sightingIn is sh's Sighting in bs, the player's view of it there: a
// replay cast's castSeen, else a live body's seen; nil none.
func sightingIn(bs []body, sh *game.Starship) *game.Sighting {
	i := slices.IndexFunc(bs, func(b body) bool { return b.kind == shipBody && b.ship == sh })
	switch {
	case i < 0:
		return nil
	case bs[i].castSeen != nil:
		return bs[i].castSeen
	}
	return bs[i].seen
}

// useColour is a special's line.
var useColour = color.RGBA{190, 120, 255, 255}

// useLine is special use u's line at tick t, false when the player does
// not see its user at s's star in bs (ours always; a foreign one with a
// Sighting there): "<user>  <gadget> → <target>  <outcome>", a planet
// target "→ <planet>  <outcome>", none (a lane's too) "<user>  <gadget>
// <outcome>"; every name as logName says (a foreign target in a veiled
// orbit as hiddenIn, no flag), in violet.
func (s *systemScreen) useLine(bs []body, u *game.SpecialUse, t int, veil []bool) (logLine, bool) {
	if u == nil || u.Star != s.star || u.UserEmpire != s.player && sightingIn(bs, u.User) == nil {
		return logLine{}, false
	}
	from := s.logName(bs, u.User, u.UserEmpire) + "  " + game.Gizmos[u.Gizmo].Name
	switch {
	case u.Kind == game.ShipTarget:
		te, to := u.TargetEmpire, s.logName(bs, u.Target, u.TargetEmpire)
		if u.Target != nil && te != s.player && u.Target.Lane < 0 && veiled(veil, u.Target.Orbit) {
			te, to = -1, s.hiddenIn(u.Target.Orbit)
		}
		return logLine{t, []logPart{{u.UserEmpire, from, " → "}, {te, to + "  " + u.Outcome, ""}}, useColour}, true
	case u.Kind == game.PlanetTarget && u.Index >= 0 && u.Index < len(s.sys.Planets):
		p := s.sys.Planets[u.Index]
		return logLine{t, []logPart{{u.UserEmpire, from, " → "}, {p.Owner, p.Name + "  " + u.Outcome, ""}}, useColour}, true
	}
	return logLine{t, []logPart{{u.UserEmpire, from + "  " + u.Outcome, ""}}, useColour}, true
}

// useLines is uses' lines seen (useLine) at tick t.
func (s *systemScreen) useLines(bs []body, uses []*game.SpecialUse, t int, veil []bool) []logLine {
	var out []logLine
	for _, u := range uses {
		if l, ok := s.useLine(bs, u, t, veil); ok {
			out = append(out, l)
		}
	}
	return out
}

// noteUse adds the player's own use to the round log at once and, out
// of a replay, plays its effects at once as a replay of its own (its
// kills' blasts, its moves' tween; layReplay), over the ships as drawn
// before it (s.bodies: afterUse calls it before its rebuild). During a
// replay: the line alone.
func (s *systemScreen) noteUse(u *game.SpecialUse) {
	if s.rlog == nil {
		s.rlog = &roundLog{}
	}
	if s.replay == nil && u != nil && len(u.Moves)+len(u.Destroyed) > 0 && knows(s.e, s.star) {
		var cast []body
		for _, b := range s.bodies {
			if b.kind == shipBody { // a foreign one's seen names it (sightingIn)
				cast = append(cast, b)
			}
		}
		s.layReplay(s.e, cast, []game.RoundLog{{Star: s.star}}, moveTicks, -1, [][]*game.SpecialUse{{u}}, nil)
		return
	}
	s.rlog.add(s.useLines(s.bodies, []*game.SpecialUse{u}, s.tick, s.orbitVeil(s.e, nil))...)
}

// specialUses is the SpecialUse records of reps, each once (the user's
// report and its victim's copy share one).
func specialUses(reps []game.Report) []*game.SpecialUse {
	var out []*game.SpecialUse
	for _, r := range reps {
		if u := r.Event.Use; u != nil && !slices.Contains(out, u) {
			out = append(out, u)
		}
	}
	return out
}

// roundLines is round r's lines, the round starting at absolute tick t0:
// each shot at its landing, the blasts (its header and the specials used
// before it: layReplay's); veil the orbits the player did not see then
// (orbitVeil): a foreign ship destroyed there is hiddenIn, its flag left
// out, a structure razed there "something razed in <planet>'s orbit".
func (s *systemScreen) roundLines(rp *replay, r *replayRound, t0 int, veil []bool) []logLine {
	var out []logLine
	for k, f := range r.log.Shots {
		out = append(out, s.shotLine(rp, r, f, t0+r.landsAt(k), veil))
	}
	bt := t0 + r.blastStart()
	for _, d := range r.log.Destroyed {
		o := -1
		if i := slices.IndexFunc(rp.cast, func(b body) bool { return b.ship == d }); i >= 0 {
			o = rp.cast[i].owner
		}
		name := s.logName(rp.cast, d, o)
		if o != s.player && d.Lane < 0 && veiled(veil, d.Orbit) { // a destroyed ship keeps its last Orbit
			o, name = -1, s.hiddenIn(d.Orbit)
		}
		out = append(out, logLine{bt, []logPart{{o, name + " destroyed", ""}}, logWhite})
	}
	for _, z := range r.log.Razed {
		what := z.Item.String() + " razed"
		if veiled(veil, z.Planet) {
			what = "something razed in " + s.sys.Planets[z.Planet].Name + "'s orbit"
		}
		out = append(out, logLine{bt, []logPart{{s.sys.Planets[z.Planet].Owner, what, ""}}, logWhite})
	}
	return out
}

// orbitVeil is, per planet of s's star, its orbit hidden from e at the
// start of the first of logs' rounds: e does not SeesOrbit it now, or an
// Orbital Cloaker razed there in one of logs (that round or a later one)
// still stood then. Nil (every orbit seen) without e or with ScanAll.
func (s *systemScreen) orbitVeil(e *game.Empire, logs []game.RoundLog) []bool {
	if e == nil || e.ScanAll || s.sys == nil {
		return nil
	}
	out := make([]bool, len(s.sys.Planets))
	for pl, p := range s.sys.Planets {
		out[pl] = !e.SeesOrbit(p)
		for _, l := range logs {
			if l.Star == s.star && p.Owner != e.Index && slices.ContainsFunc(l.Razed, func(z game.Razing) bool {
				return z.Planet == pl && z.Item == game.OrbitalCloaker
			}) {
				out[pl] = true
			}
		}
	}
	return out
}

// veiled reports planet pl's orbit in veil (orbitVeil), false for −1.
func veiled(veil []bool, pl int) bool { return pl >= 0 && pl < len(veil) && veil[pl] }

// hiddenIn is what the player reads for anything in planet pl's veiled
// orbit (no name, no hull, no structure).
func (s *systemScreen) hiddenIn(pl int) string {
	return "something in " + s.sys.Planets[pl].Name + "'s orbit"
}

// roundHeader is round n's header at tick t.
func roundHeader(n, t int) logLine {
	return logLine{t, []logPart{{-1, fmt.Sprintf("Round %d", n), ""}}, textColour}
}

// shotLine is shot f's line at tick t: "<shooter>  <weapon> → <target>
// <tail>", a planet's shooter "<planet> (orbital)" with its structure for
// weapon, a structure struck "<planet>  -N <structure>"; red on hull,
// light blue absorbed, yellow knocked down (floatColour). In an orbit
// veil hides (orbitVeil), the planet's shooter and a target that is not
// ours are hiddenIn, the structure unnamed.
func (s *systemScreen) shotLine(rp *replay, r *replayRound, f game.Fired, t int, veil []bool) logLine {
	var from string
	switch {
	case f.FromPlanet && veiled(veil, f.ShooterPlanet):
		from = s.hiddenIn(f.ShooterPlanet)
	case f.FromPlanet:
		from = s.sys.Planets[f.ShooterPlanet].Name + " (orbital)  " + f.ShooterItem.String()
	default:
		from = s.logName(rp.cast, f.Shooter, f.ShooterEmpire) + "  " + game.Gizmos[f.Gizmo].Name
	}
	te, to := f.TargetEmpire, s.logName(rp.cast, f.Target, f.TargetEmpire)
	structure := f.AtPlanet && f.Target == nil
	hidden := f.AtPlanet && f.TargetEmpire != s.player && veiled(veil, f.TargetPlanet)
	switch {
	case hidden: // a ship or a structure, its owner's flag left out
		te, to = -1, s.hiddenIn(f.TargetPlanet)
	case structure:
		to = s.sys.Planets[f.TargetPlanet].Name
	}
	k := kindOf(f)
	c := textColour
	tail := "no effect"
	switch k {
	case hullHit:
		tail = fmt.Sprintf("-%d", f.Damage)
		if structure && !hidden {
			tail += " " + f.HitItem.String()
		}
		if f.Absorbed > 0 { // the shields took part
			tail += fmt.Sprintf(" (%d absorbed)", f.Absorbed)
		}
	case absorbed:
		tail = "absorbed"
	case knockDown:
		tail = "shields down"
	}
	if k != noImpact {
		c = floatColour[k]
	}
	return logLine{t, []logPart{{f.ShooterEmpire, from, " → "}, {te, to + "  " + tail, ""}}, c}
}

// logName is ship sh of empire o as the round log names it: ours by its
// name, a foreign one never: "<species> <design>" when its Sighting in
// bs (a replay's cast: castSeen, else a body's seen) scanned it, else
// "<species> <hull>".
func (s *systemScreen) logName(bs []body, sh *game.Starship, o int) string {
	if sh == nil {
		return "?"
	}
	if o == s.player {
		return sh.Name
	}
	sp := "?"
	if o >= 0 && o < len(s.g.Empires) {
		sp = s.g.Empires[o].Species.String()
	}
	hull := sh.Design.Hull
	if sg := sightingIn(bs, sh); sg != nil {
		if sg.Scan != nil {
			return sp + " " + sg.Scan.Design.Name
		}
		hull = sg.Hull
	}
	return sp + " " + strings.ToLower(hull.String())
}

// placedLog is a shown line laid out: its plate, each part's flag (x,
// −1 none) and text x, the texts cut to the width.
type placedLog struct {
	plate rectF
	flagX []float64
	textX []float64
	texts []string
	line  logLine
	y     float64
}

// rlogLineH is a round log line's height: a flag's and a gap.
func rlogLineH(flagH float64) float64 { return max(flagH, smallFace.Size*1.25) + 2 }

// layLog lays the lines out right-aligned to the view's right edge less
// padX from its top plus padY, flags flagW x flagH, each line at most
// half the view wide: its texts cut from the first (the shooter's side),
// the target and the outcome kept.
func layLog(ls []logLine, flagW, flagH float64) []placedLog {
	right := float64(sysArea.X1 - padX)
	maxW := float64(sysArea.X1-sysArea.X0) / 2
	y := float64(sysArea.Y0 + padY)
	var out []placedLog
	for _, l := range ls {
		p := placedLog{line: l, y: y}
		w := 0.0 // the flags', then the texts' as cut
		for _, pt := range l.parts {
			if pt.empire >= 0 {
				w += flagW + 3
			}
		}
		for i, pt := range l.parts {
			rest := ui.Width(pt.keep, smallFace) // the later texts in full: the shooter's side is cut first
			for _, q := range l.parts[i+1:] {
				rest += ui.Width(q.text+q.keep, smallFace)
			}
			txt := fitWidth(pt.text, max(0, maxW-w-rest), smallFace) + pt.keep
			p.texts = append(p.texts, txt)
			w += ui.Width(txt, smallFace)
		}
		x := right - w
		p.plate = rectF{x - 3, y - 1, right + 3, y + rlogLineH(flagH) - 1}
		for i, pt := range l.parts {
			p.flagX = append(p.flagX, -1)
			if pt.empire >= 0 {
				p.flagX[i] = x
				x += flagW + 3
			}
			p.textX = append(p.textX, x)
			x += ui.Width(p.texts[i], smallFace)
		}
		out = append(out, p)
		y += rlogLineH(flagH)
	}
	return out
}

// logText draws the round log over the view (Text side: plates, flags
// and texts in one place).
func (s *systemScreen) logText(a *app, frame *ebiten.Image) {
	ls := s.rlog.shown(s.tick)
	if len(ls) == 0 {
		return
	}
	fw, fh := 0.0, 0.0
	if s.g != nil && len(s.g.Empires) > 0 {
		b := s.panelFlag(a, 0).Image.Bounds()
		fw, fh = float64(b.Dx()), float64(b.Dy())
	}
	dst := clipped(frame, sysArea)
	for _, p := range layLog(ls, fw, fh) {
		vector.FillRect(dst, float32(p.plate.x0), float32(p.plate.y0), float32(p.plate.w()), float32(p.plate.h()), badgeBack, false)
		ty := p.y + (rlogLineH(fh)-2-smallFace.Size*1.25)/2
		for i, pt := range p.line.parts {
			if p.flagX[i] >= 0 {
				fl := s.panelFlag(a, pt.empire)
				var o ebiten.DrawImageOptions
				o.GeoM.Translate(p.flagX[i]-float64(fl.Offset.X), p.y-float64(fl.Offset.Y))
				fl.Draw(dst, &o)
			}
			ui.Print(dst, p.texts[i], smallFace, p.textX[i], ty, p.line.c)
		}
	}
}
