package game

import "fmt"

// The four victories (docs/rules.md "Winning", the game's end-*.txt), judged
// at the close for every living empire: the first one an empire wins is
// kept for good (Empire.Won, WonDay) and told once, a Won event for it
// and for every other living empire. Nothing ends: the game goes on.
type Victory uint8

const (
	NoVictory      Victory = iota
	ControlVictory         // more than two-thirds of the stars controlled (end-ctrl)
	CaptureVictory         // a colony in every other species' home system (end-capt)
	DestroyVictory         // every other species defeated (end-dest)
	UniteVictory           // every other living species allied (end-unit)
)

// Ending is v's end text: end-<Ending>.txt in the archives.
func (v Victory) Ending() string { return [...]string{"", "ctrl", "capt", "dest", "unit"}[v] }

func (v Victory) String() string {
	return [...]string{"none", "control", "capture", "destruction", "unification"}[v]
}

// VictoryOf is e's victory as the game stands, the first that holds of
// destruction (every other empire defeated), capture (a colony of e at
// every other's HomeStar, defeated or not), unification (every other
// living empire allied with e) and control (ControlledSystems over two
// thirds of the stars); none for an empire out or alone.
func (g *Game) VictoryOf(e *Empire) Victory {
	if !e.Alive() || len(g.Empires) < 2 {
		return NoVictory
	}
	alive, allied, captured := 0, true, true
	for _, o := range g.Empires {
		if o == e {
			continue
		}
		if o.Alive() {
			alive++
			allied = allied && e.StanceWith(o.Index) == Alliance
		}
		captured = captured && e.holdsAt(o.HomeStar())
	}
	switch {
	case alive == 0:
		return DestroyVictory
	case captured:
		return CaptureVictory
	case allied:
		return UniteVictory
	case 3*e.ControlledSystems() > 2*len(g.Cluster.Stars):
		return ControlVictory
	}
	return NoVictory
}

// holdsAt reports a colony of e at star.
func (e *Empire) holdsAt(star int) bool {
	for _, c := range e.Colonies {
		if c.Star == star {
			return true
		}
	}
	return false
}

// victories is the close's victories: each living empire not yet a
// winner that wins now (VictoryOf) keeps it, a Won Report for it (Other
// −1, at its HomeStar) then for every other living empire (Other the
// winner, Star −1: its home is no news, the fog keeps it).
func (g *Game) victories() []Report {
	var reps []Report
	for _, e := range g.Empires {
		if !e.Alive() || e.Won != NoVictory {
			continue
		}
		v := g.VictoryOf(e)
		if v == NoVictory {
			continue
		}
		e.Won, e.WonDay = v, g.Day
		ev := Event{Kind: Won, Item: NoItem, Tech: NoTech, Star: e.HomeStar(), Planet: -1, Other: -1,
			Text: "Victory by " + v.String()}
		reps = append(reps, Report{e.Index, ev})
		for _, o := range g.Empires {
			if o != e && o.Alive() {
				ev.Other, ev.Star, ev.Text = e.Index, -1, fmt.Sprintf("%s victory by %s", e.Species, v)
				reps = append(reps, Report{o.Index, ev})
			}
		}
	}
	return reps
}
