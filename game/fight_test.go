package game

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
)

// Combat mechanics alone (docs/rules.md "Combat", "Combat rounds"): no
// bot, no diplomacy beyond the fixture's war. A script drives the ships
// through the game API, rounds played as bot.Day plays them (Round until
// one does nothing, RoundCap at most, then EndDay).

// fightRoundCap is bot.RoundCap (bot is not importable from game).
const fightRoundCap = 30

// fightPulser is an energy-rich Electromagnetic Pulser (5 shots a day)
// behind an Ion Wrap: the arenas' only weapon with Uses > 1.
var fightPulser = Design{Name: "Pulser", Hull: Small, Parts: []Gizmo{Nanotwirler, TonklinMotor, ElectromagneticPulser, IonWrap}}

// arenaGame is SpawnArena on the duel's two empires (Snovemdomas, Orfa).
func arenaGame(t testing.TB, a, b []Design) *Game {
	t.Helper()
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	if err := SpawnArena(g, a, b); err != nil {
		t.Fatal(err)
	}
	return g
}

// fightStar is the arena's star (side A's home).
func fightStar(g *Game) int { return g.Empires[0].Colonies[0].Star }

// shipsAt is every ship out of the lanes at star, in Round's order
// (empire, then Ships).
func shipsAt(g *Game, star int) []*Starship {
	var out []*Starship
	for _, e := range g.Empires {
		for _, s := range e.Ships {
			if s.Star == star && s.Lane < 0 {
				out = append(out, s)
			}
		}
	}
	return out
}

// bestWeapons is d's weapon parts, highest Level first (part order on ties).
func bestWeapons(d Design) []int {
	var ps []int
	for p, gz := range d.Parts {
		if Gizmos[gz].Cat == Weapon {
			ps = append(ps, p)
		}
	}
	slices.SortStableFunc(ps, func(a, b int) int { return Gizmos[d.Parts[b]].Level - Gizmos[d.Parts[a]].Level })
	return ps
}

// book is the ledger's copy of one ship, kept from the rule text only.
type book struct {
	energy, hull int
	shots        []int
	dead         bool
	down         bool // knocked down by a hit short of the draw (takeShot)
}

// TestScriptedFight: Lancers against Lancers (Fourier Missiles 2 vs Ion
// Wrap 1: both sides hurt), every ship arming at the nearest foe in
// space (its weapons, highest Level first) and closing in to 3/4 of its
// best range, shields on. The test's ledger, from the rules (energy:
// PowerPerDay at the refill less the shields' reservation, nothing at a
// round's start, MoveCost per move, a fired shot's Power; hull: max(0,
// Level − Absorb); Uses a day; Generators a round, one taken by a move),
// must match the engine after every Arm, Round and EndDay, and the fight
// must end.
func TestScriptedFight(t *testing.T) {
	first, last := scriptedFight(t)
	again, _ := scriptedFight(t)
	if first != again {
		t.Errorf("first destruction on day %d, then %d: not deterministic", first, again)
	}
	t.Logf("first destruction day %d, last side wiped out day %d", first, last)
}

// scriptedFight plays the fight and returns the days of its first
// destruction and of the last ship of a side.
func scriptedFight(t *testing.T) (firstKill, end int) {
	a, b, err := ParseArena("lancer,lancer:lancer,lancer")
	if err != nil {
		t.Fatal(err)
	}
	g := arenaGame(t, a, b)
	star := fightStar(g)
	led := map[*Starship]*book{}
	for _, s := range shipsAt(g, star) {
		if !s.Shields || s.Power != s.Design.PowerPerDay()-s.Design.ShieldDraw() {
			t.Fatalf("%s spawned with shields %v, %d energy, PowerPerDay %d less the draw %d", s.Name, s.Shields, s.Power, s.Design.PowerPerDay(), s.Design.ShieldDraw())
		}
		led[s] = &book{energy: s.Design.PowerPerDay() - s.Design.ShieldDraw(), hull: s.MaxHull(), shots: make([]int, len(s.Design.Parts))}
	}
	check := func(when string) {
		t.Helper()
		for s, k := range led {
			if k.dead {
				if g.OwnerOf(s) != nil {
					t.Fatalf("%s: %s destroyed but still in its empire", when, s.Name)
				}
				continue
			}
			if g.OwnerOf(s) == nil {
				t.Fatalf("%s: %s gone, ledger hull %d", when, s.Name, k.hull)
			}
			if s.Power != k.energy || s.Hull != k.hull || s.ShieldsDown != k.down || !s.Shields || !slices.Equal(s.Shots, k.shots) {
				t.Fatalf("%s: %s energy %d hull %d dark %v shields %v shots %v, ledger %d %d %v",
					when, s.Name, s.Power, s.Hull, s.ShieldsDown, s.Shields, s.Shots, k.energy, k.hull, k.shots)
			}
		}
	}
	firstKill = -1
	for day := 0; day < 60; day++ {
		if len(g.CombatStars()) == 0 {
			break
		}
		for r := range fightRoundCap {
			when := fmt.Sprintf("day %d round %d", g.Day, r)
			here := shipsAt(g, star)
			armed := map[*Starship][]Shot{}
			ordered := map[*Starship]bool{}
			for _, s := range here {
				e, k := g.OwnerOf(s), led[s]
				if err := e.SetShields(s, true); err != nil { // on already: nothing taken
					t.Fatalf("%s: %s shields on: %v", when, s.Name, err)
				}
				var foe *Starship
				for _, f := range g.EnemiesAt(e, star) {
					if f.InSpace() && (foe == nil || dist(s.Pos, f.Pos) < dist(s.Pos, foe.Pos)) {
						foe = f
					}
				}
				if foe == nil { // every foe still in orbit: head for the nearest
					for _, f := range g.EnemiesAt(e, star) {
						if foe == nil || dist(s.Pos, f.Pos) < dist(s.Pos, foe.Pos) {
							foe = f
						}
					}
				}
				if foe == nil {
					continue
				}
				ws := bestWeapons(s.Design)
				if s.InSpace() && foe.InSpace() {
					sg, ok := g.SightingOf(e, foe)
					if !ok {
						t.Fatalf("%s: %s does not see %s", when, s.Name, foe.Name)
					}
					for _, p := range ws {
						w := &Gizmos[s.Design.Parts[p]]
						armedE := 0
						for _, x := range armed[s] {
							armedE += Gizmos[s.Design.Parts[x.Part]].Power
						}
						want := (w.Uses == 0 || k.shots[p] < w.Uses) && len(armed[s]) < s.Design.Generators() &&
							k.energy >= armedE+w.Power
						err := e.Arm(s, p, sg)
						if (err == nil) != want {
							t.Fatalf("%s: %s arms %s: %v, ledger accepts %v (energy %d, shots %v)", when, s.Name, w.Name, err, want, k.energy, k.shots)
						}
						if err == nil {
							armed[s] = append(armed[s], Shot{Part: p, Target: sg})
						}
					}
				}
				var o Order
				if len(ws) > 0 && dist(s.Pos, foe.Pos) > 0.75*s.WeaponRange(&Gizmos[s.Design.Parts[ws[0]]]) {
					o = Order{Kind: GoToPoint, Pos: foe.Pos}
				}
				if err := e.Order(s, o); err != nil {
					t.Fatalf("%s: %s order %v: %v", when, s.Name, o.Kind, err)
				}
				ordered[s] = o.Kind != NoOrder
			}
			// The ledger's round draws nothing: the shields' reservation
			// is off the energy already.
			absorb := map[*Starship]int{}
			for _, s := range here {
				if s.Shields && !led[s].down {
					absorb[s] = s.Design.ShieldScore()
				}
			}
			// The round's starting positions: the shots' (shots before moves).
			start, inSpace := map[*Starship]Vec3{}, map[*Starship]bool{}
			for _, s := range here {
				start[s], inSpace[s] = s.Pos, s.InSpace()
			}
			acted, _, lg := g.Round(star, nil)
			// Step 1: the shots, on the starting positions, part order.
			var want []Fired
			pending := map[*Starship]int{}
			perShip := map[*Starship]int{}
			for _, s := range here {
				k := led[s]
				for _, a := range armed[s] {
					tg := a.Target.ship
					w := &Gizmos[s.Design.Parts[a.Part]]
					if led[tg].dead || !inSpace[tg] || !inSpace[s] || dist(start[s], start[tg]) > float64(w.Range)*RangeUnit+1e-9 ||
						w.Uses > 0 && k.shots[a.Part] >= w.Uses || k.energy < w.Power {
						continue
					}
					k.shots[a.Part]++
					k.energy -= w.Power
					perShip[s]++
					want = append(want, Fired{Shooter: s, Target: tg, Damage: w.Level})
				}
				if perShip[s] > s.Design.Generators() {
					t.Fatalf("%s: %s fired %d shots, %d generators", when, s.Name, perShip[s], s.Design.Generators())
				}
			}
			// Every shot paid, the hits land in order: raised shields pay
			// their draw per hit off the energy left, the hit short of it
			// knocks them down (no damage), then the full Level.
			for i := range want {
				tg, lvl := want[i].Target, want[i].Damage
				d := lvl
				switch k := led[tg]; {
				case absorb[tg] > 0 && k.energy < tg.Design.ShieldDraw():
					absorb[tg], k.down, d = 0, true, 0
				case absorb[tg] > 0:
					k.energy -= tg.Design.ShieldDraw()
					d = max(0, lvl-absorb[tg])
				}
				pending[tg] += d
				want[i].Damage, want[i].Absorbed = d, lvl-d
			}
			if len(want) != len(lg.Shots) {
				t.Fatalf("%s: %d shots fired, ledger %d", when, len(lg.Shots), len(want))
			}
			for i, f := range lg.Shots {
				w := want[i]
				if f.Shooter != w.Shooter || f.Target != w.Target || f.Damage != w.Damage || f.Absorbed != w.Absorbed {
					t.Fatalf("%s: shot %d %s -> %s damage %d absorbed %d, ledger %s -> %s %d %d", when, i,
						f.Shooter.Name, f.Target.Name, f.Damage, f.Absorbed, w.Shooter.Name, w.Target.Name, w.Damage, w.Absorbed)
				}
			}
			var dead []*Starship
			for _, s := range here {
				k := led[s]
				k.hull -= pending[s]
				if k.hull <= 0 {
					k.dead = true
					dead = append(dead, s)
				}
			}
			if !slices.Equal(dead, lg.Destroyed) {
				t.Fatalf("%s: destroyed %d ships, ledger %d", when, len(lg.Destroyed), len(dead))
			}
			// Step 2: the survivors' moves must be allowed and paid, out of
			// what the shots and the shields' draws left.
			moved := map[*Starship]bool{}
			for _, m := range lg.Moves {
				s, k := m.Ship, led[m.Ship]
				if moved[s] || !ordered[s] || k.dead || s.Design.Generators()-perShip[s] < 1 || k.energy < s.Design.MoveCost() {
					t.Fatalf("%s: %s moved (twice %v, ordered %v, dead %v, %d fired, energy %d)", when, s.Name, moved[s], ordered[s], k.dead, perShip[s], k.energy)
				}
				moved[s] = true
				k.energy -= s.Design.MoveCost()
			}
			if len(dead) > 0 && firstKill < 0 {
				firstKill = g.Day
			}
			if acted != (len(lg.Moves) > 0 || len(want) > 0) {
				t.Fatalf("%s: acted %v with %d moves, %d shots", when, acted, len(lg.Moves), len(want))
			}
			check(when)
			if !acted {
				break
			}
		}
		g.EndDay()
		for s, k := range led {
			if !k.dead {
				k.energy, k.down = s.Design.PowerPerDay()-s.Design.ShieldDraw(), false
				clear(k.shots)
			}
		}
		check(fmt.Sprintf("refill of day %d", g.Day))
		if g.Empires[0].StanceWith(1) != War || g.Empires[1].StanceWith(0) != War {
			t.Fatalf("day %d: the war ended", g.Day)
		}
	}
	if firstKill < 0 {
		t.Fatalf("no ship destroyed by day %d", g.Day)
	}
	if len(g.CombatStars()) != 0 {
		t.Fatalf("first destruction day %d, still fighting on day %d", firstKill, g.Day)
	}
	return firstKill, g.Day
}

// fightArenas are TestFightInvariants' sides: the catalogue's pairs and
// the round fixtures (no shields, two guns, one generator, a Pulser).
var fightArenas = [][2][]Design{
	{{FixtureLancer, FixtureLancer}, {FixtureRaider, FixtureRaider}},
	{{FixtureLancer}, {FixtureLancer}},
	{{FixtureRaider, FixtureLancer}, {FixtureLancer, FixtureColonyShip}},
	{{twoGens, oneGen}, {FixtureLancer, fightPulser}},
	{{fightPulser, hulk}, {twoGens, fightPulser}},
	{{fightWrapped, FixtureRaider}, {twoGens, FixtureLancer}},
}

// fightWrapped is two Ion Wraps (Absorb 2) on a Mass Barrage Gun: a
// shot of Level 1 or 2 on it does nothing.
var fightWrapped = Design{Name: "Wrapped", Hull: Small, Parts: []Gizmo{Nanotwirler, TonklinMotor, MassBarrageGun, IonWrap, IonWrap}}

// TestFightInvariants: random scripts (a PCG of the test's own, seeds
// 1..50) arm random weapons at random foes, give random moves and flip
// shields on the fightArenas; every Round and every close must keep the
// rules: a switch on takes ShieldDraw (refused short of it), off gives it
// back; energy ≥ 0 and spent in a round exactly as MoveCost per move +
// the fired shots' Power, plus ShieldDraw per hit its raised shields
// took; the refill PowerPerDay less the reservation of shields kept on,
// never dark; hull never rising; a hit's damage takeShot's (every shot
// paid, then in order: max(0, Level − Absorb) while the energy left pays
// the draw, 0 and dark for the hit short of it, the full Level once
// dark or off), only from an
// armed shot, within WeaponRange of the round's starting positions (shots
// before moves); Uses a day, Generators a round less one for a move, a
// move only by a survivor out of the energy the shots and draws left;
// the destroyed gone from their empire and silent after their round; the
// war kept.
func TestFightInvariants(t *testing.T) {
	var n fightTally
	for seed := uint64(1); seed <= 50; seed++ {
		ar := fightArenas[int(seed)%len(fightArenas)]
		fightInvariants(t, seed, ar[0], ar[1], &n)
	}
	t.Logf("%+v", n)
	if n.moves == 0 || n.hits == 0 || n.absorbed == 0 || n.knocked == 0 || n.refunds == 0 || n.refused == 0 || n.kills == 0 || n.multi == 0 || n.uses == 0 {
		t.Errorf("a rule never walked: %+v", n)
	}
}

// fightTally counts what the random scripts walked: moves, hits doing
// damage, shots fully absorbed (knocked-down ones too), shields knocked
// down, switched off with a draw refunded, switches on refused short of
// the draw, kills, rounds with two shots from one ship, a weapon used
// more than once a day.
type fightTally struct{ moves, hits, absorbed, knocked, refunds, refused, kills, multi, uses int }

// shipState is one ship's state before a round.
type shipState struct {
	power, hull int
	shields     bool
	down        bool
	shots       []int
	armed       []Shot
	pos         Vec3
	inSpace     bool
}

func fightInvariants(t *testing.T, seed uint64, a, b []Design, n *fightTally) {
	rng := rand.New(rand.NewPCG(seed, 0xf16))
	g := arenaGame(t, a, b)
	star := fightStar(g)
	maxHull := map[*Starship]int{}
	for _, s := range shipsAt(g, star) {
		maxHull[s] = s.Hull
	}
	gone := map[*Starship]bool{}
	for day := 0; day < 15 && len(g.CombatStars()) > 0; day++ {
		for r := range fightRoundCap {
			when := fmt.Sprintf("seed %d day %d round %d", seed, g.Day, r)
			here := shipsAt(g, star)
			for _, s := range here {
				e := g.OwnerOf(s)
				if rng.IntN(5) == 0 {
					on, power, d := s.Shields, s.Power, s.Design.ShieldDraw()
					err := e.SetShields(s, !on)
					switch {
					case on && (err != nil || s.Shields || s.Power != power+d):
						t.Fatalf("%s: %s shields off: %v, shields %v, energy %d -> %d, draw %d", when, s.Name, err, s.Shields, power, s.Power, d)
					case !on && power < d && (err == nil || s.Shields || s.Power != power):
						t.Fatalf("%s: %s shields on short: %v, shields %v, energy %d -> %d, draw %d", when, s.Name, err, s.Shields, power, s.Power, d)
					case !on && power >= d && (err != nil || !s.Shields || s.Power != power-d):
						t.Fatalf("%s: %s shields on: %v, shields %v, energy %d -> %d, draw %d", when, s.Name, err, s.Shields, power, s.Power, d)
					case on && d > 0:
						n.refunds++
					case !on && power < d:
						n.refused++
					}
				}
				foes := g.EnemiesAt(e, star)
				for _, p := range bestWeapons(s.Design) {
					if len(foes) == 0 || rng.IntN(10) < 3 {
						continue
					}
					tg := foes[rng.IntN(len(foes))]
					sg, ok := g.SightingOf(e, tg)
					if !ok {
						continue
					}
					// Arm's rule: both in space, a Use and a generator left,
					// the energy for the armed shots (the reservation off it).
					w := &Gizmos[s.Design.Parts[p]]
					armedE := 0
					for _, x := range s.Armed {
						armedE += Gizmos[s.Design.Parts[x.Part]].Power
					}
					want := s.InSpace() && tg.InSpace() && (w.Uses == 0 || s.Shots[p] < w.Uses) &&
						len(s.Armed) < s.Design.Generators() && s.Power >= armedE+w.Power
					if err := e.Arm(s, p, sg); (err == nil) != want {
						t.Fatalf("%s: %s arms %s at %s: %v, the rule accepts %v", when, s.Name, w.Name, tg.Name, err, want)
					}
				}
				var o Order
				switch k := rng.IntN(10); {
				case k < 5 && len(foes) > 0:
					o = Order{Kind: GoToPoint, Pos: foes[rng.IntN(len(foes))].Pos}
				case k < 8:
					o = Order{Kind: GoToPoint, Pos: Vec3{X: s.Pos.X + rng.Float64() - 0.5, Y: s.Pos.Y + rng.Float64() - 0.5, Z: s.Pos.Z}}
				}
				_ = e.Order(s, o)
			}
			pre := map[*Starship]shipState{}
			for _, s := range here {
				pre[s] = shipState{s.Power, s.Hull, s.Shields, s.ShieldsDown, slices.Clone(s.Shots), slices.Clone(s.Armed), s.Pos, s.InSpace()}
			}
			// The rules' Absorb at the round's start.
			absorb := map[*Starship]int{}
			for s, p := range pre {
				if p.shields && !p.down {
					absorb[s] = s.Design.ShieldScore()
				}
			}
			acted, _, lg := g.Round(star, nil)
			moved := map[*Starship]bool{}
			for _, m := range lg.Moves {
				if moved[m.Ship] {
					t.Fatalf("%s: %s moved twice", when, m.Ship.Name)
				}
				moved[m.Ship] = true
				n.moves++
			}
			// Which armed shots fire: simultaneous (a target dying earlier
			// in the round is still shot), in range at the round's start
			// (before the moves), a Use and the energy left, part order.
			var want []Fired
			for _, s := range here {
				p := pre[s]
				left := p.power
				used := slices.Clone(p.shots)
				for _, a := range p.armed {
					tg, w := a.Target.ship, &Gizmos[s.Design.Parts[a.Part]]
					if q, ok := pre[tg]; !ok || !q.inSpace || !p.inSpace || dist(p.pos, q.pos) > s.WeaponRange(w)+1e-9 ||
						w.Uses > 0 && used[a.Part] >= w.Uses || left < w.Power {
						continue
					}
					used[a.Part]++
					left -= w.Power
					want = append(want, Fired{Shooter: s, Target: tg, Gizmo: s.Design.Parts[a.Part]})
				}
			}
			if len(want) != len(lg.Shots) {
				t.Fatalf("%s: %d shots fired, the rule fires %d", when, len(lg.Shots), len(want))
			}
			for i, f := range lg.Shots {
				if w := want[i]; f.Shooter != w.Shooter || f.Target != w.Target || f.Gizmo != w.Gizmo {
					t.Fatalf("%s: shot %d %s -> %s, the rule's %s -> %s", when, i, f.Shooter.Name, f.Target.Name, w.Shooter.Name, w.Target.Name)
				}
			}
			spent := map[*Starship]int{}
			fired := map[*Starship]int{}
			for i, f := range lg.Shots {
				s, tg := f.Shooter, f.Target
				if f.FromPlanet || f.AtPlanet || s == nil || tg == nil {
					t.Fatalf("%s: shot %d is not ship to ship", when, i)
				}
				if gone[s] {
					t.Fatalf("%s: %s fired after its destruction", when, s.Name)
				}
				w := &Gizmos[f.Gizmo]
				if f.From != pre[s].pos || f.To != pre[tg].pos || dist(f.From, f.To) > s.WeaponRange(w)+1e-9 {
					t.Fatalf("%s: %s -> %s at %.3f, range %.3f", when, s.Name, tg.Name, dist(f.From, f.To), s.WeaponRange(w))
				}
				if !slices.ContainsFunc(pre[s].armed, func(x Shot) bool { return x.Target.ship == tg && s.Design.Parts[x.Part] == f.Gizmo }) {
					t.Fatalf("%s: %s fired %s at %s unarmed", when, s.Name, w.Name, tg.Name)
				}
				spent[s] += w.Power
				fired[s]++
			}
			// takeShot's rule: every shot paid first, then the hits in
			// order, raised shields paying ShieldDraw off the energy left
			// per hit, the hit short of it knocking them down (no damage).
			left, knocked := map[*Starship]int{}, map[*Starship]bool{}
			for _, s := range here {
				left[s] = pre[s].power - spent[s]
			}
			for _, f := range lg.Shots {
				tg, w := f.Target, &Gizmos[f.Gizmo]
				d, down := w.Level, false
				switch {
				case absorb[tg] > 0 && left[tg] < tg.Design.ShieldDraw():
					absorb[tg], knocked[tg], d, down = 0, true, 0, true
					n.knocked++
				case absorb[tg] > 0:
					left[tg] -= tg.Design.ShieldDraw()
					d = max(0, w.Level-absorb[tg])
				}
				if f.ShieldsDown != down {
					t.Fatalf("%s: %s -> %s %s logged knock-down %v, the rule %v", when, f.Shooter.Name, tg.Name, w.Name, f.ShieldsDown, down)
				}
				if f.Damage != d || f.Absorbed != w.Level-f.Damage {
					t.Fatalf("%s: %s -> %s %s damage %d absorbed %d, the rule %d (absorb %d, energy %d)", when, f.Shooter.Name, tg.Name, w.Name, f.Damage, f.Absorbed, d, absorb[tg], left[tg])
				}
				if f.Damage > 0 {
					n.hits++
				} else {
					n.absorbed++
				}
			}
			destroyed := map[*Starship]bool{}
			for _, s := range lg.Destroyed {
				destroyed[s] = true
			}
			// Then the moves: survivors only, a generator and MoveCost left
			// after the shots and the draws.
			for s := range moved {
				if destroyed[s] || s.Design.Generators()-fired[s] < 1 || left[s] < s.Design.MoveCost() {
					t.Fatalf("%s: %s moved (destroyed %v, %d fired, energy %d)", when, s.Name, destroyed[s], fired[s], left[s])
				}
				left[s] -= s.Design.MoveCost()
			}
			for _, s := range here {
				p := pre[s]
				if s.ShieldsDown != (p.down || knocked[s]) || s.Shields != p.shields {
					t.Fatalf("%s: %s dark %v -> %v (knocked %v), shields %v -> %v", when, s.Name, p.down, s.ShieldsDown, knocked[s], p.shields, s.Shields)
				}
				if fired[s] > 1 {
					n.multi++
				}
				if s.Power < 0 || s.Power != left[s] {
					t.Fatalf("%s: %s energy %d -> %d, the rule %d (moved %v, shots %d)", when, s.Name, p.power, s.Power, left[s], moved[s], spent[s])
				}
				free := s.Design.Generators()
				if moved[s] {
					free--
				}
				if fired[s] > free {
					t.Fatalf("%s: %s fired %d shots, moved %v, %d generators", when, s.Name, fired[s], moved[s], s.Design.Generators())
				}
				for q, k := range s.Shots {
					if u := Gizmos[s.Design.Parts[q]].Uses; u > 0 && k > u || k < p.shots[q] {
						t.Fatalf("%s: %s part %d shots %d -> %d, Uses %d", when, s.Name, q, p.shots[q], k, u)
					}
					if k > 1 && p.shots[q] < k {
						n.uses++
					}
				}
				if s.Hull > p.hull {
					t.Fatalf("%s: %s hull rose %d -> %d", when, s.Name, p.hull, s.Hull)
				}
				if (g.OwnerOf(s) == nil) != destroyed[s] || destroyed[s] != (s.Hull <= 0) {
					t.Fatalf("%s: %s hull %d, owned %v, logged destroyed %v", when, s.Name, s.Hull, g.OwnerOf(s) != nil, destroyed[s])
				}
				if len(s.Armed) != 0 {
					t.Fatalf("%s: %s still armed after the round", when, s.Name)
				}
			}
			for s := range destroyed {
				gone[s] = true
				n.kills++
			}
			if g.Empires[0].StanceWith(1) != War || g.Empires[1].StanceWith(0) != War {
				t.Fatalf("%s: the war ended", when)
			}
			if !acted {
				break
			}
		}
		hull := map[*Starship]int{}
		on := map[*Starship]bool{}
		for _, s := range shipsAt(g, star) {
			hull[s], on[s] = s.Hull, s.Shields
		}
		goesOn := slices.Contains(g.CombatStars(), star) // else they Step, a fight ended
		g.EndDay()
		for _, s := range shipsAt(g, star) {
			if !goesOn {
				break
			}
			keep := on[s] && s.Design.ShieldDraw() <= s.Design.PowerPerDay()
			want := s.Design.PowerPerDay()
			if keep {
				want -= s.Design.ShieldDraw()
			}
			if s.Shields != keep {
				t.Fatalf("seed %d refill of day %d: %s shields %v -> %v", seed, g.Day, s.Name, on[s], s.Shields)
			}
			if s.Power != want || s.ShieldsDown || slices.ContainsFunc(s.Shots, func(n int) bool { return n != 0 }) || s.Hull != hull[s] || s.Hull > maxHull[s] {
				t.Fatalf("seed %d refill of day %d: %s energy %d dark %v shots %v hull %d (was %d)", seed, g.Day, s.Name, s.Power, s.ShieldsDown, s.Shots, s.Hull, hull[s])
			}
		}
		if g.Empires[0].StanceWith(1) != War || g.Empires[1].StanceWith(0) != War {
			t.Fatalf("seed %d day %d: the war ended at the close", seed, g.Day)
		}
	}
}
