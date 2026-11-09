package bot

import (
	"math"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// ShieldFirst: 1 for Neutral and a nil genome, clamped to [0, 1].
func TestShieldFirstGene(t *testing.T) {
	if Neutral().ShieldFirst != 1 || NeutralCombat().ShieldFirst != 1 {
		t.Errorf("neutral %v, combat %v", Neutral().ShieldFirst, NeutralCombat().ShieldFirst)
	}
	for _, tc := range []struct{ in, want float64 }{{2, 1}, {-1, 0}, {math.NaN(), 0}, {0.3, 0.3}} {
		g := Neutral()
		g.ShieldFirst = tc.in
		if g.Clamp(); g.ShieldFirst != tc.want {
			t.Errorf("Clamp(%v): %v, want %v", tc.in, g.ShieldFirst, tc.want)
		}
	}
}

// A Raider in range of a scout, its shields' draw reserved, the energy
// left short of its shot by one: ShieldFirst 1 keeps the shields and
// arms nothing; 0 turns them off (the draw refunded) and fires, then
// cannot switch them on again this day (the shot spent the refund), and
// does at Act after the refill. With the energy for the shot, 0 keeps
// them on; a dark shield (ShieldsDown) still holds its reservation, so 0
// turns it off to fire. Shields a fixture turned off: 0 switches them on
// when the energy pays the draw and the shot, else leaves them off and
// fires; 1 never touches the switch. Inside its stand, no move is
// ordered.
func TestShieldFirst(t *testing.T) {
	d, mbg := game.FixtureRaider, game.Gizmos[game.MassBarrageGun]
	draw := d.ShieldDraw()
	if draw < 1 || d.Generators() < 2 || draw+mbg.Power > d.PowerPerDay() {
		t.Fatalf("fixture: draw %d, %d generators, supply %d", draw, d.Generators(), d.PowerPerDay())
	}
	for _, tc := range []struct {
		first   float64
		on      bool // the switch before the round
		energy  int  // Power before the round (the draw off it while on)
		down    bool // ShieldsDown before the round
		shields bool // after the bot's Round
		shots   int
	}{
		{1, true, mbg.Power - 1, false, true, 0},
		{0, true, mbg.Power - 1, false, false, 1},
		{0.49, true, mbg.Power - 1, false, false, 1},
		{0.5, true, mbg.Power - 1, false, true, 0},
		{0, true, mbg.Power, false, true, 1},
		{0, true, mbg.Power - 1, true, false, 1},
		{0, false, draw + mbg.Power, false, true, 1},
		{0, false, draw + mbg.Power - 1, false, false, 1},
		{1, false, draw + mbg.Power, false, false, 1},
	} {
		g, b, home := roundFixture(t, genomeWith(func(g *Genome) { g.FleeRatio, g.ShieldFirst = 1e9, tc.first }))
		foe := put(g.Empires[1], unarmedScout, home, game.Vec3{X: -0.05})
		me := put(b.E, d, home, game.Vec3{X: 0.05})
		if !tc.on {
			_ = me.SetShields(false)
		}
		me.Power, me.ShieldsDown = tc.energy, tc.down
		b.Round(home)
		if me.Shields != tc.shields || len(me.Armed) != tc.shots || me.Order.Kind != game.NoOrder {
			t.Errorf("ShieldFirst %v on %v energy %d down %v: shields %v, armed %d, order %+v; want %v, %d, none",
				tc.first, tc.on, tc.energy, tc.down, me.Shields, len(me.Armed), me.Order, tc.shields, tc.shots)
			continue
		}
		switched := tc.energy
		switch {
		case tc.on && !tc.shields:
			switched += draw
		case !tc.on && tc.shields:
			switched -= draw
		}
		if me.Power != switched {
			t.Errorf("ShieldFirst %v on %v energy %d: power %d after the switch, want %d", tc.first, tc.on, tc.energy, me.Power, switched)
		}
		hull := foe.Hull
		_, _, log := g.Round(home, nil)
		if len(log.Shots) != tc.shots || foe.Hull != hull-tc.shots*mbg.Level || me.Power != switched-tc.shots*mbg.Power {
			t.Errorf("ShieldFirst %v on %v energy %d: shots %d, foe hull %d of %d, power %d", tc.first, tc.on, tc.energy, len(log.Shots), foe.Hull, hull, me.Power)
		}
		pre, was := me.Power, me.Shields
		b.Round(home) // the gun spent: nothing wanted
		if want := was || tc.first < 0.5 && pre >= draw; me.Shields != want {
			t.Errorf("ShieldFirst %v on %v energy %d: shields %v at the next round (power %d), want %v", tc.first, tc.on, tc.energy, me.Shields, pre, want)
		}
		_ = me.SetShields(false)
		me.Refill()
		b.Act()
		if me.Shields != (tc.first < 0.5) {
			t.Errorf("ShieldFirst %v: Act left shields %v", tc.first, me.Shields)
		}
	}
}

// A fleeing ship keeps its shields on whatever its energy: ShieldFirst 0
// drops them only to fire.
func TestShieldFirstFlee(t *testing.T) {
	g, b, home, _ := fleeFixture(t, genomeWith(func(g *Genome) { g.HullThreat[game.Small], g.ShieldFirst = 50, 0 }))
	d := game.Design{Name: "Lone", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.StarLaneDrive, game.MassBarrageGun, game.IonWrap}}
	me := b.E.Spawn(d, home, 0)
	me.Orbit = -1 // in space: in orbit it wants no shot
	me.Power = d.ShieldDraw() + game.Gizmos[game.MassBarrageGun].Power - 1
	g.Empires[1].Spawn(unarmedScout, home, 0)
	b.assess()
	if p := pressure(b.their(home), b.mine(home)); p <= b.cg.FleeRatio || !b.canEscape(me, g.Hostiles(b.E, home), b.cg) {
		t.Fatalf("fixture: pressure %v, escape %v", p, b.canEscape(me, g.Hostiles(b.E, home), b.cg))
	}
	if wantedEnergy(me) == 0 {
		t.Fatal("fixture: no shot wanted")
	}
	b.Round(home)
	if !me.Shields || me.Order.Kind != game.GoToStar {
		t.Errorf("fleeing: shields %v, order %+v", me.Shields, me.Order)
	}
}
