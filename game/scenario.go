package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
)

// A scenario (cmd/mksave -scenario) stages one system of a new game for
// the combat screens: colonies founded and equipped for several empires,
// ships of the arena designs placed in orbit, by a lane's endpoint or by
// a planet, the stances set. Setup, not play: nothing is journalled, no
// Report kept, no cost, cap or research consulted.
//
//	{"star": "home",
//	 "stances": [{"a": 0, "b": 1, "stance": "war"}],
//	 "planets": [{"planet": 2, "empire": 1, "pop": 6, "orbit": ["Orbital Shield"], "surface": ["Planetary Barrier Shield"]}],
//	 "ships": [{"empire": 0, "design": "cruiser", "count": 2, "at": "orbit 0"}]}
type Scenario struct {
	// Star is the system: "home" (or omitted) empire 0's first colony's
	// star, else a star index (a JSON number).
	Star StarRef `json:"star,omitempty"`
	// Stances are set after the planets and ships. nil (absent) puts
	// every pair of the empires named by Planets (empire ≥ 0) and Ships
	// at War; an empty list sets none.
	Stances []ScenarioStance `json:"stances,omitempty"`
	Planets []ScenarioPlanet `json:"planets,omitempty"`
	Ships   []ScenarioShips  `json:"ships,omitempty"`
}

// StarRef is a Scenario's star: "" or "home", else a star index in
// decimal. Its JSON is the string "home" or a number.
type StarRef string

// UnmarshalJSON takes "home", "" or a non-negative integer.
func (r *StarRef) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		if s != "" && s != "home" {
			return fmt.Errorf("star %q: \"home\" or a star index", s)
		}
		*r = StarRef(s)
		return nil
	}
	var n int
	if err := json.Unmarshal(b, &n); err != nil || n < 0 {
		return fmt.Errorf("star %s: \"home\" or a star index", b)
	}
	*r = StarRef(strconv.Itoa(n))
	return nil
}

// ScenarioStance sets A↔B to Stance, a Stance's String: "unknown",
// "peace", "war", "alliance".
type ScenarioStance struct {
	A      int    `json:"a"`
	B      int    `json:"b"`
	Stance string `json:"stance"`
}

// ScenarioPlanet is planet Planet of the system. Empire −1: it must be
// unowned and stays so (nothing else allowed). Empire ≥ 0: kept when
// already its colony, else founded (it must be unowned); then Orbit's
// items in the first free orbit slots (BuildOrbital), Surface's on the
// first squares in Cells order BuildWith takes under the empire's
// traits, then Pop (0: kept) at most the Capacity and at least the
// structures' Staff (Colony.Busy: a kept Pop rises to it).
type ScenarioPlanet struct {
	Planet  int      `json:"planet"`
	Empire  int      `json:"empire"`
	Pop     int      `json:"pop,omitempty"`
	Orbit   []string `json:"orbit,omitempty"`
	Surface []string `json:"surface,omitempty"`
}

// ScenarioShips is Count (0: 1) ships of empire Empire, of the
// ArenaDesign named Design, placed At: "orbit P" Spawned in orbit of
// planet P; "lane K" out of orbit at the FreeSpot of the endpoint of the
// star's K-th lane (Star.Lanes), as a lane arrival; "near P" out of orbit
// at the FreeSpot of planet P's LeavePoint, PlanetClear off it.
type ScenarioShips struct {
	Empire int    `json:"empire"`
	Design string `json:"design"`
	Count  int    `json:"count,omitempty"`
	At     string `json:"at"`
}

// ParseScenario reads a Scenario's JSON, strictly (an unknown field or
// trailing data is an error), and checks what needs no game: stance,
// design and item names, the "at" forms, the signs.
func ParseScenario(r io.Reader) (*Scenario, error) {
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	var s Scenario
	if err := d.Decode(&s); err != nil {
		return nil, fmt.Errorf("scenario: %w", err)
	}
	if err := d.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("scenario: trailing data after the object")
	}
	if err := s.check(); err != nil {
		return nil, err
	}
	return &s, nil
}

// ParseStance is the Stance whose String is s, any case.
func ParseStance(s string) (Stance, error) {
	for st := Unknown; st <= Alliance; st++ {
		if strings.EqualFold(strings.TrimSpace(s), st.String()) {
			return st, nil
		}
	}
	return Unknown, fmt.Errorf("no stance %q (unknown, peace, war, alliance)", s)
}

// itemNamed is the item whose Items name is s, any case.
func itemNamed(s string) (Item, error) {
	for it := range NumItems {
		if strings.EqualFold(strings.TrimSpace(s), Items[it].Name) {
			return it, nil
		}
	}
	return NoItem, fmt.Errorf("no item %q", s)
}

// parseAt is an "at" spec's kind ("orbit", "lane", "near") and index.
func parseAt(at string) (string, int, error) {
	f := strings.Fields(at)
	if len(f) == 2 {
		if n, err := strconv.Atoi(f[1]); err == nil && n >= 0 {
			switch k := strings.ToLower(f[0]); k {
			case "orbit", "lane", "near":
				return k, n, nil
			}
		}
	}
	return "", 0, fmt.Errorf("at %q: \"orbit P\", \"lane K\" or \"near P\"", at)
}

// check is ParseScenario's game-free validation.
func (s *Scenario) check() error {
	for _, st := range s.Stances {
		if _, err := ParseStance(st.Stance); err != nil {
			return fmt.Errorf("scenario stance %d-%d: %w", st.A, st.B, err)
		}
		if st.A == st.B {
			return fmt.Errorf("scenario stance %d-%d: an empire with itself", st.A, st.B)
		}
	}
	for _, p := range s.Planets {
		switch {
		case p.Pop < 0:
			return fmt.Errorf("scenario planet %d: pop %d", p.Planet, p.Pop)
		case p.Empire < 0 && (p.Pop > 0 || len(p.Orbit) > 0 || len(p.Surface) > 0):
			return fmt.Errorf("scenario planet %d: empire %d is no colony: no pop, orbit or surface", p.Planet, p.Empire)
		}
		for _, n := range p.Orbit {
			it, err := itemNamed(n)
			switch {
			case err != nil:
				return fmt.Errorf("scenario planet %d orbit: %w", p.Planet, err)
			case !it.Orbital() || it == Ship:
				return fmt.Errorf("scenario planet %d orbit: %s is no orbital structure", p.Planet, it)
			}
		}
		for _, n := range p.Surface {
			it, err := itemNamed(n)
			switch {
			case err != nil:
				return fmt.Errorf("scenario planet %d surface: %w", p.Planet, err)
			case !it.Surface():
				return fmt.Errorf("scenario planet %d surface: %s is no surface structure", p.Planet, it)
			}
		}
	}
	for _, sh := range s.Ships {
		if _, err := ArenaDesign(sh.Design); err != nil {
			return fmt.Errorf("scenario ships: %w", err)
		}
		if sh.Count < 0 {
			return fmt.Errorf("scenario ships %s: count %d", sh.Design, sh.Count)
		}
		if _, _, err := parseAt(sh.At); err != nil {
			return fmt.Errorf("scenario ships %s: %w", sh.Design, err)
		}
	}
	return nil
}

// ApplyScenario stages s on g. Validate-then-apply by a dry run: s is
// applied first to a copy of g (its JSON, Relinked), and only when that
// succeeds to g itself; an error leaves g untouched. In order: empire 0
// Visits the star; the planets in order (founded, equipped, peopled); the
// ships in order; the stances (Stances, or War between every pair named),
// Contact first, so a colony founded here tallies no encroachment between
// empires that have not met.
func ApplyScenario(g *Game, s *Scenario) error {
	if err := s.check(); err != nil {
		return err
	}
	b, err := json.Marshal(g)
	if err != nil {
		return fmt.Errorf("scenario: copying the game: %w", err)
	}
	var dry Game
	if err := json.Unmarshal(b, &dry); err != nil {
		return fmt.Errorf("scenario: copying the game: %w", err)
	}
	if err := dry.Relink(); err != nil {
		return fmt.Errorf("scenario: copying the game: %w", err)
	}
	if err := applyScenario(&dry, s); err != nil {
		return err
	}
	if err := applyScenario(g, s); err != nil {
		panic(fmt.Sprintf("scenario: the dry run passed, the game refused: %v", err))
	}
	return nil
}

// applyScenario is ApplyScenario on g directly, stopping at the first
// error (g then half staged).
func applyScenario(g *Game, s *Scenario) error {
	n := len(g.Empires)
	empire := func(i int, what string) error {
		if i < 0 || i >= n {
			return fmt.Errorf("scenario %s: empire %d, the game has %d", what, i, n)
		}
		return nil
	}
	c := g.Cluster
	star := -1
	switch s.Star {
	case "", "home":
		if len(g.Empires[0].Colonies) == 0 {
			return errors.New("scenario: empire 0 has no home star")
		}
		star = g.Empires[0].Colonies[0].Star
	default:
		star, _ = strconv.Atoi(string(s.Star))
		if star < 0 || star >= len(c.Stars) {
			return fmt.Errorf("scenario: star %d, the cluster has %d", star, len(c.Stars))
		}
	}
	sys := c.Stars[star].System
	planet := func(i int, what string) error {
		if i < 0 || i >= len(sys.Planets) {
			return fmt.Errorf("scenario %s: planet %d, %s has %d", what, i, c.Stars[star].Name, len(sys.Planets))
		}
		return nil
	}
	g.Empires[0].Visit(star)
	named := map[int]bool{}

	for _, sp := range s.Planets {
		what := "planet " + strconv.Itoa(sp.Planet)
		if err := planet(sp.Planet, what); err != nil {
			return err
		}
		p := sys.Planets[sp.Planet]
		if sp.Empire < 0 {
			if p.Owner >= 0 {
				return fmt.Errorf("scenario %s: %s is empire %d's", what, p.Name, p.Owner)
			}
			continue
		}
		if err := empire(sp.Empire, what); err != nil {
			return err
		}
		e := g.Empires[sp.Empire]
		named[sp.Empire] = true
		col := e.Colony(star, sp.Planet)
		switch {
		case col != nil:
		case p.Owner >= 0:
			return fmt.Errorf("scenario %s: %s is empire %d's", what, p.Name, p.Owner)
		default:
			at, ok := p.BestCell(ColonyBase, e.Species.Traits())
			if !ok {
				return fmt.Errorf("scenario %s: %s has no square for a Colony Base", what, p.Name)
			}
			col = e.settle(star, sp.Planet, at)
			e.Visit(star)
		}
		for _, name := range sp.Orbit {
			it, _ := itemNamed(name)
			slot := slices.Index(p.Orbit[:], NoItem)
			if slot < 0 {
				return fmt.Errorf("scenario %s: no free orbit slot for %s", what, it)
			}
			if err := p.BuildOrbital(slot, it); err != nil {
				return fmt.Errorf("scenario %s: %w", what, err)
			}
		}
		for _, name := range sp.Surface {
			it, _ := itemNamed(name)
			tr := e.Species.Traits()
			i := slices.IndexFunc(p.Cells(), func(at Cell) bool { return p.CanBuild(at, it, tr) })
			if i < 0 {
				return fmt.Errorf("scenario %s: no square for %s", what, it)
			}
			if err := p.BuildWith(p.Cells()[i], it, tr); err != nil {
				return fmt.Errorf("scenario %s: %w", what, err)
			}
		}
		if sp.Pop > 0 {
			if capa := col.Capacity(); sp.Pop > capa {
				return fmt.Errorf("scenario %s: pop %d over the capacity %d", what, sp.Pop, capa)
			}
			col.Pop = sp.Pop
		}
		// Staffed as a played colony is (Free ≥ 0): a kept Pop rises to
		// the structures' Staff, a given one under it is refused.
		if busy := col.Busy(); col.Pop < busy {
			switch capa := col.Capacity(); {
			case sp.Pop > 0:
				return fmt.Errorf("scenario %s: pop %d under the staff %d", what, sp.Pop, busy)
			case busy > capa:
				return fmt.Errorf("scenario %s: staff %d over the capacity %d", what, busy, capa)
			}
			col.Pop = busy
		}
	}

	for _, sh := range s.Ships {
		what := "ships " + sh.Design
		if err := empire(sh.Empire, what); err != nil {
			return err
		}
		e := g.Empires[sh.Empire]
		named[sh.Empire] = true
		d, _ := ArenaDesign(sh.Design)
		kind, k, _ := parseAt(sh.At)
		var pl int
		var to *Vec3 // out of orbit near here; nil in orbit
		switch kind {
		case "orbit", "near":
			if err := planet(k, what); err != nil {
				return err
			}
			pl = k
			if kind == "near" { // where Leave Orbit goes, off the orbit ring
				p := LeavePoint(c, star, k)
				to = &p
			}
		case "lane":
			if err := planet(0, what); err != nil { // Spawn starts at a planet
				return err
			}
			lanes := c.Stars[star].Lanes
			if k >= len(lanes) {
				return fmt.Errorf("scenario %s: lane %d, %s has %d", what, k, c.Stars[star].Name, len(lanes))
			}
			_, end, ok := laneEnd(c, star, c.Other(lanes[k], star))
			if !ok {
				panic("a star's own lane is not adjacent")
			}
			to = &sys.Lanes[end].Pos
		}
		for range max(sh.Count, 1) {
			ship := e.Spawn(d, star, pl)
			if to != nil {
				ship.Orbit = -1
				ship.Pos = FreeSpot(*to, spaceAt(g.Empires, star, ship))
			}
		}
	}

	stances := s.Stances
	if stances == nil {
		var es []int
		for i := range n {
			if named[i] {
				es = append(es, i)
			}
		}
		for x, a := range es {
			for _, b := range es[x+1:] {
				stances = append(stances, ScenarioStance{A: a, B: b, Stance: War.String()})
			}
		}
	}
	for _, st := range stances {
		what := fmt.Sprintf("stance %d-%d", st.A, st.B)
		if err := empire(st.A, what); err != nil {
			return err
		}
		if err := empire(st.B, what); err != nil {
			return err
		}
		to, _ := ParseStance(st.Stance)
		if err := g.scenarioStance(st.A, st.B, to); err != nil {
			return fmt.Errorf("scenario %s: %w", what, err)
		}
	}
	return nil
}

// scenarioStance sets a↔b to st, unjournalled, no Report: Unknown
// unmeets them (Met −1); otherwise they meet (Contact), then War is
// declareWar's from Peace (an alliance broken first, unrecorded), Peace
// and Alliance set; the Offers between them dropped.
func (g *Game) scenarioStance(a, b int, st Stance) error {
	g.dropOffers(a, b)
	if st == Unknown {
		g.setStance(a, b, Unknown)
		g.Empires[a].Relations[b].Met, g.Empires[b].Relations[a].Met = -1, -1
		return nil
	}
	g.Contact(a, b)
	switch {
	case st == War && g.Empires[a].StanceWith(b) == War:
	case st == War:
		g.setStance(a, b, Peace)
		if _, err := g.declareWar(a, b); err != nil {
			return err
		}
	default:
		g.setStance(a, b, st)
	}
	return nil
}
