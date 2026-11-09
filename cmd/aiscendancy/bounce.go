package main

import (
	"math"
	"slices"

	"github.com/rewk/aiscendancy/game"
)

// The bounce off a barred orbit (the original's): a
// GoToPlanet hop that Hop stopped PlanetClear short of a guarded planet
// is drawn flying in to bounceTouch of the planet, then back out to its
// end, bounceSound at the touch. The model is Hop's; only the path drawn
// changes.
const (
	bounceSound = "sg49bull"
	bounceTouch = game.PlanetClear / 2
	bounceIn    = 2.0 / 3 // the path's share flown in, the rest back out
)

// bounceAt is the touch point of sh's move ending at to, pl its planet;
// ok false unless sh, still ordered GoToPlanet out of the lanes and out
// of that orbit, stands within PlanetClear+ShipGap of a planet its owner
// is Barred from.
func bounceAt(g *game.Game, sh *game.Starship, to game.Vec3) (touch game.Vec3, pl int, ok bool) {
	if g == nil || sh.Order.Kind != game.GoToPlanet || sh.Lane >= 0 || sh.Orbit == sh.Order.Planet {
		return game.Vec3{}, -1, false
	}
	o := g.OwnerOf(sh)
	pl = sh.Order.Planet
	if o == nil || !o.Barred(sh.Star, pl) {
		return game.Vec3{}, -1, false
	}
	p := g.Cluster.Stars[sh.Star].System.Planets[pl].Pos
	d := math.Sqrt((to.X-p.X)*(to.X-p.X) + (to.Y-p.Y)*(to.Y-p.Y) + (to.Z-p.Z)*(to.Z-p.Z))
	if d < 1e-9 || d >= game.PlanetClear+game.ShipGap {
		return game.Vec3{}, -1, false
	}
	return lerp(p, to, bounceTouch/d), pl, true
}

// bouncePos is the bounce's point fraction f of the way: from to touch
// over bounceIn, then touch to to.
func bouncePos(from, touch, to game.Vec3, f float64) game.Vec3 {
	if f < bounceIn {
		return lerp(from, touch, f/bounceIn)
	}
	return lerp(touch, to, (f-bounceIn)/(1-bounceIn))
}

// bounceLine is the status line of the player's bounce off planet pl:
// "<planet>'s orbit is shielded" when e sees a shield in it, else
// "guarded".
func bounceLine(e *game.Empire, star, pl int) string {
	p := e.Planet(star, pl)
	what := "guarded"
	if e.SeesOrbit(p) && slices.ContainsFunc(p.Orbit[:], func(it game.Item) bool {
		return it == game.OrbitalShield || it == game.OrbitalMegaShield
	}) {
		what = "shielded"
	}
	name := p.Name + "'s"
	if p.Name == "" {
		name = "The planet's"
	}
	return name + " orbit is " + what
}
