package bot

import (
	"math"
	"slices"

	"github.com/rewk/aiscendancy/game"
)

// The bot's round tactics (docs/rules.md "The bot's combat policy"): what
// each of its ships at a combat star does in one round, decided from what
// its empire sees at the round's start, never from Act's estimates (ships
// die between rounds).

// Round arms the bot's shots and sets its moves for one round at star:
// first the specials (an Accutron before any shot, the hostiles read
// again after each use), then my colonies' Tractor Beams (tractor), then
// the sides (theirSide, my ships there, P their pressure), then my
// colonies' orbital weapons (defend), then each ship of mine out of the
// lanes there, in Ships order, its shields set first (shieldSwitch,
// fighting only in the last case):
//   - being refitted: nothing (every move and shot is refused);
//   - unarmed: it flees when the hostiles deal damage;
//   - armed: it flees when P > FleeRatio and canEscape (one that cannot
//     would run in vain and never fire); else it holds when it cannot
//     shoot today or has nothing to shoot at (a spent gun is no reason to
//     leave); else it fights.
//
// Nothing without G, for a defeated empire, or with neither a hostile
// seen nor a planet to hit.
func (b *Bot) Round(star int) {
	if b.G == nil || !b.E.Alive() {
		return
	}
	hs := b.G.HostilesAll(b.E, star)
	pls := b.planetTargets(star)
	if len(hs) == 0 && len(pls) == 0 {
		return
	}
	cg := b.genes()
	hs = b.specials(star, hs, cg)
	hs = b.tractor(star, hs, cg)
	their := b.theirSide(star, hs, cg)
	b.defend(star, hs, cg)
	p := pressure(their, b.mine(star))
	for _, s := range b.E.Ships {
		if s.Star != star || s.Lane >= 0 || b.E.Refitting(s) != nil {
			continue
		}
		switch {
		case !armed(s.Design):
			b.shieldSwitch(s, cg, false)
			if their.dmg > 0 {
				b.roundFlee(s, hs, cg)
			}
		case p > cg.FleeRatio && b.canEscape(s, hs, cg):
			b.shieldSwitch(s, cg, false)
			b.roundFlee(s, hs, cg)
		case longestRange(s) <= 0 || !slices.ContainsFunc(hs, inSpace) && len(pls) == 0:
			b.shieldSwitch(s, cg, false)
			roundHold(b.E, s)
		default:
			b.shieldSwitch(s, cg, true)
			b.roundFight(s, hs, pls, cg)
		}
	}
}

// inSpace reports a sighting out of every orbit: a ship Arm may target.
func inSpace(h game.Sighting) bool { return h.Orbit < 0 }

// planetTargets is the planets of star my ships may fire at
// (Game.CanHitPlanet), in planet order.
func (b *Bot) planetTargets(star int) []int {
	var out []int
	for pl := range b.E.Cluster.Stars[star].System.Planets {
		if b.G.CanHitPlanet(b.E, star, pl) == nil {
			out = append(out, pl)
		}
	}
	return out
}

// defend arms the orbital weapons of my colonies at star, in Colonies then
// slot order: each weapon with a shot left today at the weakest hostile
// in space within its range of the planet, else at the nearest planet it
// may hit (planetAim).
func (b *Bot) defend(star int, hs []game.Sighting, cg Combat) {
	for _, col := range b.E.Colonies {
		if col.Star != star {
			continue
		}
		p := col.Planet
		for slot, it := range p.Orbit {
			d, ok := it.Defence()
			if !ok || !d.Weapon() || col.OrbitShots[slot] >= d.Shots {
				continue
			}
			if t := b.weakest(p.Pos, hs, cg, float64(d.Range)*game.RangeUnit); t >= 0 {
				b.E.ArmPlanet(col, slot, hs[t]) // a refusal: no shot
			} else if pl := b.planetAim(col, slot); pl >= 0 {
				b.E.ArmPlanetAt(col, slot, pl)
			}
		}
	}
}

// planetAim is the planet of col's star nearest col's that its weapon in
// slot may be armed at (CanArmPlanetAt: the check alone, no refusal
// journaled), ties to the lower index; −1 none.
func (b *Bot) planetAim(col *game.Colony, slot int) int {
	t, tD := -1, 0.0
	for pl, p := range b.E.Cluster.Stars[col.Star].System.Planets {
		if b.E.CanArmPlanetAt(col, slot, pl) != nil {
			continue
		}
		if d := dist(col.Planet.Pos, p.Pos); t < 0 || d < tD {
			t, tD = pl, d
		}
	}
	return t
}

// roundFlee disarms s and takes it to safety; a landing ship stays. A
// lane is safe: with a lane drive a GoToStar order is kept, else it goes
// toRefuge. Without one, or refused, it goes to safestPoint (a GoToPoint
// cleared when that is where it stands).
func (b *Bot) roundFlee(s *game.Starship, hs []game.Sighting, cg Combat) {
	b.E.Disarm(s)
	if landing(s) {
		return
	}
	if s.Design.LaneDrives() > 0 {
		if s.Order.Kind == game.GoToStar || b.toRefuge(s) {
			return
		}
	}
	pt, _ := safestPoint(s.Pos, hs, cg)
	if pt == s.Pos {
		stopPoint(s)
		return
	}
	b.E.Order(s, game.Order{Kind: game.GoToPoint, Pos: pt}) // a refusal: no move
}

// canEscape reports s able to get away from hs this round: a lane drive
// with a GoToStar order or a refuge, or a safestPoint out of every
// hostile's reach. Never while being refitted (every move is refused).
func (b *Bot) canEscape(s *game.Starship, hs []game.Sighting, cg Combat) bool {
	if b.E.Refitting(s) != nil {
		return false
	}
	if s.Design.LaneDrives() > 0 && (s.Order.Kind == game.GoToStar || b.refuge(s) >= 0) {
		return true
	}
	_, sc := safestPoint(s.Pos, hs, cg)
	return sc > 0
}

// roundHold keeps s where it is for the round: it arms nothing, and any
// order but a GoToStar (a lane departure already decided) is cleared.
func roundHold(e *game.Empire, s *game.Starship) {
	e.Disarm(s)
	if s.Order.Kind != game.GoToStar {
		s.Order = game.Order{}
	}
}

// safestPoint is the candidate point farthest out of the hostiles' reach:
// from, then the 26 directions on the system's rim, each scored min over
// hostiles of (distance − threatReach); the highest wins, ties the first.
// The rim sits 1e-9 inside SystemReach, so Check never refuses it.
func safestPoint(from game.Vec3, hs []game.Sighting, cg Combat) (pt game.Vec3, score float64) {
	scoreOf := func(p game.Vec3) float64 {
		m := math.Inf(1)
		for _, h := range hs {
			m = min(m, dist(p, h.Pos)-threatReach(h, cg))
		}
		return m
	}
	best, bestS := from, scoreOf(from)
	r := game.SystemReach - 1e-9
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			for dz := -1; dz <= 1; dz++ {
				if dx == 0 && dy == 0 && dz == 0 {
					continue
				}
				n := math.Sqrt(float64(dx*dx + dy*dy + dz*dz))
				p := game.Vec3{X: float64(dx) / n * r, Y: float64(dy) / n * r, Z: float64(dz) / n * r}
				if sc := scoreOf(p); sc > bestS {
					best, bestS = p, sc
				}
			}
		}
	}
	return best, bestS
}

// threatReach is how far a hostile may hit within a round: its parts
// scanned, one move (when its energy covers MoveCost) plus its longest
// weapon range; else HullReach of its hull.
func threatReach(h game.Sighting, cg Combat) float64 {
	sc := h.Scan
	if sc == nil || !sc.Parts {
		return cg.HullReach[h.Hull]
	}
	r := 0.0
	if sc.Energy >= sc.Design.MoveCost() {
		r = sc.Design.Reach()
	}
	far := 0
	for _, gz := range sc.Design.Parts {
		if w := &game.Gizmos[gz]; w.Cat == game.Weapon {
			far = max(far, w.Range)
		}
	}
	return r + float64(far)*game.RangeUnit
}

// shieldSwitch sets s's shields by cg's ShieldFirst. At ShieldFirst ≥
// 0.5 the bot never touches the switch: the shields stay as they are (on
// from Spawn, their ShieldDraw reserved, the shots getting the rest).
// Below, a ship fighting this round turns them off when the energy left
// with them on (Power, or Power − ShieldDraw while off) is short of the
// shots it wants (wantedEnergy), refunding the draw; on otherwise (dark
// too: a dark shield still holds its reservation). Any other ship, a
// fleeing one too, and every ship at Act turns them back on, left off
// when the energy cannot pay the draw.
func (b *Bot) shieldSwitch(s *game.Starship, cg Combat, fighting bool) {
	if cg.ShieldFirst >= 0.5 {
		return
	}
	on := true
	if fighting {
		on = s.Power+reserved(s)-s.Design.ShieldDraw() >= wantedEnergy(s)
	}
	_ = b.E.SetShields(s, on)
}

// wantedEnergy is the energy of the shots roundFight arms in range with
// the shields aside: the weapons that canShoot, higher Level first (part
// order among equals), up to Generators; 0 in orbit (it arms nothing).
func wantedEnergy(s *game.Starship) int {
	if s.Orbit >= 0 {
		return 0
	}
	n, energy := 0, 0
	for _, p := range weaponOrder(s) {
		if n == s.Design.Generators() {
			break
		}
		if canShoot(s, p) {
			n++
			energy += game.Gizmos[s.Design.Parts[p]].Power
		}
	}
	return energy
}

// weaponOrder is s's weapon parts, higher Level first, part order kept
// among equals (a stable insertion sort).
func weaponOrder(s *game.Starship) []int {
	ws := make([]int, 0, len(s.Design.Parts))
	for p, gz := range s.Design.Parts {
		if game.Gizmos[gz].Cat == game.Weapon {
			ws = append(ws, p)
		}
	}
	for i := 1; i < len(ws); i++ {
		for j := i; j > 0 && game.Gizmos[s.Design.Parts[ws[j]]].Level > game.Gizmos[s.Design.Parts[ws[j-1]]].Level; j-- {
			ws[j], ws[j-1] = ws[j-1], ws[j]
		}
	}
	return ws
}

// canShoot reports weapon part p of s able to shoot today: a shot left
// and its Power within the energy left with the shields' reservation
// counted back (switching them off frees it: shieldSwitch's choice).
func canShoot(s *game.Starship, p int) bool {
	w := &game.Gizmos[s.Design.Parts[p]]
	return w.Cat == game.Weapon && shotLeft(s, p, w) && w.Power <= s.Power+reserved(s)
}

// reserved is the energy s's shields hold off Power: ShieldDraw while on.
func reserved(s *game.Starship) int {
	if s.Shields && !s.Sapped {
		return s.Design.ShieldDraw()
	}
	return 0
}

// shotLeft reports part p of s, gizmo w, with a shot left today (Uses 0:
// no limit).
func shotLeft(s *game.Starship, p int, w *game.GizmoInfo) bool {
	return w.Uses == 0 || p >= len(s.Shots) || s.Shots[p] < w.Uses
}

// longestRange is the longest WeaponRange of s's weapons that canShoot, 0
// when none can.
func longestRange(s *game.Starship) float64 {
	far := 0.0
	for p := range s.Design.Parts {
		if canShoot(s, p) {
			far = max(far, s.WeaponRange(&game.Gizmos[s.Design.Parts[p]]))
		}
	}
	return far
}

// weakest is the index in hs of the hostile in space within rng of pos of
// least estimated hull points, ties the nearest, then hs order; -1 none.
func (b *Bot) weakest(pos game.Vec3, hs []game.Sighting, cg Combat, rng float64) int {
	t, tHP, tD := -1, 0.0, 0.0
	for i, h := range hs {
		d := dist(pos, h.Pos)
		if !inSpace(h) || d > rng+1e-9 {
			continue
		}
		if hp := sightingSide(b.G, cg, h).hp; t < 0 || hp < tHP || hp == tHP && d < tD {
			t, tHP, tD = i, hp, d
		}
	}
	return t
}

// roundFight aims s at the weakest hostile in space, else at the nearest
// of pls (ArmAtPlanet), and heads for the point at Standoff ×
// longestRange from the target on its own side (+X when on it), inside
// the rim. Out of range, the move comes first (GoToPoint, its dest kept
// for attack), then the shots with one generator and MoveCost kept for
// it. In range, the shots come first, with every generator, then the
// move only with a generator and MoveCost left (else a ship with energy
// for one action chased its point and never fired). With a shot armed it
// never moves away from the target: kept from when the moves resolved
// before the shots (two ships each backing off to their stand ended out
// of range), no longer needed since Round fires first, on the round's
// starting positions (an out-of-range armed shot is dropped at no cost
// and takes no generator from the move). The
// shields' draw is already off the energy (the reservation). Shots go
// higher Level first, then part order (weaponOrder). In orbit it arms
// nothing (Arm refuses): the move takes it out. Round calls it with a
// target in hs or pls.
func (b *Bot) roundFight(s *game.Starship, hs []game.Sighting, pls []int, cg Combat) {
	t := b.weakest(s.Pos, hs, cg, math.Inf(1))
	var at game.Vec3
	var fire func(part int) error
	if t >= 0 {
		tg := hs[t]
		at, fire = tg.Pos, func(part int) error { return b.E.Arm(s, part, tg) }
	} else {
		planets := b.E.Cluster.Stars[s.Star].System.Planets
		pl, pd := -1, 0.0
		for _, k := range pls {
			if d := dist(s.Pos, planets[k].Pos); pl < 0 || d < pd {
				pl, pd = k, d
			}
		}
		at, fire = planets[pl].Pos, func(part int) error { return b.E.ArmAtPlanet(s, part, pl) }
	}
	u := game.Vec3{X: 1}
	if d := dist(s.Pos, at); d > 0 {
		u = game.Vec3{X: (s.Pos.X - at.X) / d, Y: (s.Pos.Y - at.Y) / d, Z: (s.Pos.Z - at.Z) / d}
	}
	sd := longestRange(s) * cg.Standoff
	pt := game.Vec3{X: at.X + u.X*sd, Y: at.Y + u.Y*sd, Z: at.Z + u.Z*sd}
	if n, r := dist(pt, game.Vec3{}), game.SystemReach-1e-9; n > r {
		pt = game.Vec3{X: pt.X * r / n, Y: pt.Y * r / n, Z: pt.Z * r / n}
	}
	ws := weaponOrder(s)
	mc, energy := s.Design.MoveCost(), 0
	arm := func(moving bool) {
		if s.Orbit >= 0 {
			return
		}
		free := s.Design.Generators()
		if moving {
			free--
		}
		for _, p := range ws {
			if len(s.Armed) >= free {
				break
			}
			w := game.Gizmos[s.Design.Parts[p]].Power
			if moving && s.Power-energy-w < mc {
				continue
			}
			if fire(p) == nil {
				energy += w
			}
		}
	}
	wantMove := dist(s.Pos, pt) > 1e-9 && s.Design.Reach() > 0
	if dist(s.Pos, at) <= longestRange(s)+1e-9 {
		arm(false)
		wantMove = wantMove && s.Design.Generators()-len(s.Armed) >= 1 && s.Power-energy >= mc &&
			(len(s.Armed) == 0 || dist(pt, at) < dist(s.Pos, at))
		if !wantMove || b.E.Order(s, game.Order{Kind: game.GoToPoint, Pos: pt}) != nil {
			stopPoint(s)
		}
		return
	}
	moving := false
	if wantMove {
		moving = b.E.Order(s, game.Order{Kind: game.GoToPoint, Pos: pt}) == nil
	} else {
		stopPoint(s)
	}
	arm(moving)
}
