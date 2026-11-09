package game

import "testing"

// archetypes are the warship archetypes, smallest hull first.
var archetypes = []Design{FixtureCorvette, FixtureFrigate, FixtureCruiser, FixtureBattleship}

// TestArchetypes: each archetype is Valid on the whole tree, fills its
// hull, meets the composition rule from Gizmos (supply ≥ two rounds'
// shield draw + every weapon's Power + MoveCost; Generators ≥ Weapons +
// 1), carries a scanner and a lane drive (not the corvette: five slots),
// is found by ArenaDesign under its name, and outclasses the next smaller
// one: more hull, more ShieldScore, more DesignDamageOn the smaller
// than the smaller does on it and than the smaller does on its own kind.
func TestArchetypes(t *testing.T) {
	r := allKnown()
	for i, d := range archetypes {
		if err := d.Valid(r); err != nil {
			t.Errorf("%s: %v", d.Name, err)
		}
		if len(d.Parts) != d.Hull.Slots() {
			t.Errorf("%s: %d parts on %d slots", d.Name, len(d.Parts), d.Hull.Slots())
		}
		if d.Hull != Hull(i) {
			t.Errorf("%s: hull %s, archetype %d", d.Name, d.Hull, i)
		}
		weaponsP, scanners, specials := 0, 0, 0
		for _, g := range d.Parts {
			switch Gizmos[g].Cat {
			case Weapon:
				weaponsP += Gizmos[g].Power
			case Scanner:
				scanners++
			case Special:
				specials++
			}
		}
		need := 2*d.ShieldDraw() + weaponsP + d.MoveCost()
		if d.PowerPerDay() < need {
			t.Errorf("%s: supply %d < two draws %d + weapons %d + move %d", d.Name, d.PowerPerDay(), d.ShieldDraw(), weaponsP, d.MoveCost())
		}
		if d.Generators() < d.Weapons()+1 {
			t.Errorf("%s: %d generators for %d weapons", d.Name, d.Generators(), d.Weapons())
		}
		if d.Weapons() == 0 || d.ShieldScore() == 0 {
			t.Errorf("%s: %d weapons, shield %d", d.Name, d.Weapons(), d.ShieldScore())
		}
		if lane := d.LaneDrives() > 0; specials != btoi(lane) || (scanners == 0 || !lane) != (d.Hull == Small) {
			t.Errorf("%s: %d scanners, %d lane drives, %d specials", d.Name, scanners, d.LaneDrives(), specials)
		}
		got, err := ArenaDesign(d.Name)
		if err != nil || got.Name != d.Name || got.String() != d.String() {
			t.Errorf("ArenaDesign(%q) = %v, %v", d.Name, got, err)
		}
		t.Logf("%s: supply %d gens %d draw %d round %d hull %d shield %d damage vs each %v", d.Name,
			d.PowerPerDay(), d.Generators(), d.ShieldDraw(), d.ShieldDraw()+weaponsP+d.MoveCost(),
			d.Hull.Strength(), d.ShieldScore(), damageVs(d))
		if i == 0 {
			continue
		}
		s := archetypes[i-1]
		big, small := DesignDamageOn(d, s), DesignDamageOn(s, d)
		own := DesignDamageOn(s, s)
		if d.Hull.Strength() <= s.Hull.Strength() || d.ShieldScore() <= s.ShieldScore() || big <= small || big <= own {
			t.Errorf("%s vs %s: hull %d/%d shield %d/%d damage %d/%d (the smaller vs its kind %d)", d.Name, s.Name,
				d.Hull.Strength(), s.Hull.Strength(), d.ShieldScore(), s.ShieldScore(), big, small, own)
		}
	}
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// damageVs is d's DesignDamageOn each archetype (its shields' energy
// counted: the hit short of their draw knocks them down).
func damageVs(d Design) []int {
	var out []int
	for _, a := range archetypes {
		out = append(out, DesignDamageOn(d, a))
	}
	return out
}

// TestArchetypeFights: each archetype against the next smaller one, 1 v 1,
// on two seeds and both spawn sides (Orfa and Mebes: hull factor 1 both):
// the bigger wins, unhurt or not, within fightDays. Then two of the
// smaller against one bigger, the winner logged only.
func TestArchetypeFights(t *testing.T) {
	const fightDays = 20
	for i := 1; i < len(archetypes); i++ {
		big, small := archetypes[i], archetypes[i-1]
		for seed := uint64(1); seed <= 2; seed++ {
			for _, swap := range []bool{false, true} {
				a, b := []Design{big}, []Design{small}
				if swap {
					a, b = b, a
				}
				w, days, left := archetypeFight(t, seed, a, b, fightDays)
				winner := a[0].Name
				if w == 1 {
					winner = b[0].Name
				}
				if w < 0 || winner != big.Name {
					t.Errorf("seed %d %s:%s: winner %d (%s) after %d days, hull left %v", seed, a[0].Name, b[0].Name, w, winner, days, left)
				}
			}
		}
		w, days, left := archetypeFight(t, 1, []Design{small, small}, []Design{big}, fightDays)
		t.Logf("2 %s vs 1 %s: winner side %d after %d days, hull left %v", small.Name, big.Name, w, days, left)
	}
}

// archetypeFight is SpawnArena's fight between Orfa (a) and Mebes (b)
// at seed, scripted as TestScriptedFight's (shields on, every weapon
// armed at the nearest foe, highest Level first, closing to 3/4 of the
// best range; Round until one does nothing, then EndDay), no bot. winner
// is the side left at the star (0 a, 1 b), −1 when both or neither are
// after days; left is the survivors' hull.
func archetypeFight(t *testing.T, seed uint64, a, b []Design, days int) (winner, day int, left []int) {
	t.Helper()
	g := NewGame(seed, []Species{Orfa, Mebes}, Average)
	if err := SpawnArena(g, a, b); err != nil {
		t.Fatal(err)
	}
	star := fightStar(g)
	for day = 0; day < days && len(g.CombatStars()) > 0; day++ {
		for range fightRoundCap {
			for _, s := range shipsAt(g, star) {
				e := g.OwnerOf(s)
				e.SetShields(s, true)
				var foe *Starship
				for _, f := range g.EnemiesAt(e, star) {
					if foe == nil || f.InSpace() && !foe.InSpace() ||
						f.InSpace() == foe.InSpace() && dist(s.Pos, f.Pos) < dist(s.Pos, foe.Pos) {
						foe = f
					}
				}
				if foe == nil {
					continue
				}
				ws := bestWeapons(s.Design)
				if sg, ok := g.SightingOf(e, foe); ok && s.InSpace() && foe.InSpace() {
					for _, p := range ws {
						_ = e.Arm(s, p, sg) // refusals (no shot, no energy) are the rules'
					}
				}
				var o Order
				if dist(s.Pos, foe.Pos) > 0.75*s.WeaponRange(&Gizmos[s.Design.Parts[ws[0]]]) {
					o = Order{Kind: GoToPoint, Pos: foe.Pos}
				}
				if err := e.Order(s, o); err != nil {
					t.Fatalf("seed %d day %d: %s order: %v", seed, g.Day, s.Name, err)
				}
			}
			if acted, _, _ := g.Round(star, nil); !acted {
				break
			}
		}
		g.EndDay()
	}
	winner = -1
	sides := [2]int{}
	for _, s := range shipsAt(g, star) {
		i := 0
		if g.OwnerOf(s) == g.Empires[1] {
			i = 1
		}
		sides[i]++
		left = append(left, s.Hull)
	}
	switch {
	case sides[0] > 0 && sides[1] == 0:
		winner = 0
	case sides[1] > 0 && sides[0] == 0:
		winner = 1
	}
	return winner, day, left
}
