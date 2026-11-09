package game

import (
	"errors"
	"fmt"
)

// Routes (GoToSystem): a ship sent to any star flies lane after lane
// along Route, recomputed at every close it stands out of a lane, so a
// lane learnt or blocked meanwhile changes the way.

// Route is the stars s passes after its current star, dest last: the
// fewest lane days (LaneDays at s's LaneStrength and Traits, as the lane
// move counts them) over the lanes e knows (KnowsLane) between stars it
// knows (Knows), dest excepted (a scout goes to an unexplored star down
// a known lane), a lane LaneBlocker closes today skipped. Dijkstra,
// settling the nearest star first, the lower index on a tie; a star
// reached as cheaply from two stars keeps the lower one, then the lower
// lane. A ship in a lane starts from the lane's far end, its first stop.
// Refused: no lane drive, dest out of the cluster, dest the star s
// stands at out of a lane, no known way.
func (e *Empire) Route(s *Starship, dest int) ([]int, error) {
	c := e.Cluster
	strength := s.Design.LaneStrength()
	switch {
	case !(strength > 0):
		return nil, errors.New("no star lane drive")
	case dest < 0 || dest >= len(c.Stars):
		return nil, fmt.Errorf("no star %d", dest)
	case s.Lane < 0 && s.Star == dest:
		return nil, fmt.Errorf("%s is at %s already", s.Name, c.Stars[dest].Name)
	}
	from := s.Star
	var head []int
	if s.Lane >= 0 {
		from = c.Other(s.Lane, s.Star)
		head = []int{from}
	}
	tr := s.Species.Traits()
	n := len(c.Stars)
	days, prev, via := make([]int, n), make([]int, n), make([]int, n)
	done := make([]bool, n)
	for i := range days {
		days[i], prev[i], via[i] = -1, -1, -1
	}
	days[from] = 0
	for {
		u := -1
		for i, d := range days {
			if !done[i] && d >= 0 && (u < 0 || d < days[u]) {
				u = i
			}
		}
		if u < 0 || u == dest {
			break
		}
		done[u] = true
		for _, li := range c.Stars[u].Lanes {
			v := c.Other(li, u)
			if done[v] || !e.KnowsLane(li) || !e.Knows(v) && v != dest || e.LaneBlocker(u, li, e.Day) != nil {
				continue
			}
			d := days[u] + LaneDays(c, li, strength, tr)
			if days[v] < 0 || d < days[v] || d == days[v] && (u < prev[v] || u == prev[v] && li < via[v]) {
				days[v], prev[v], via[v] = d, u, li
			}
		}
	}
	if days[dest] < 0 {
		return nil, fmt.Errorf("no known route to %s", c.Stars[dest].Name)
	}
	var tail []int
	for v := dest; v != from; v = prev[v] {
		tail = append(tail, v)
	}
	for i, j := 0, len(tail)-1; i < j; i, j = i+1, j-1 {
		tail[i], tail[j] = tail[j], tail[i]
	}
	return append(head, tail...), nil
}

// RouteLanes is route's cluster lanes from s's star (the lane s is in
// first, when in one) and their LaneDays summed at s's strength, each
// lane counted whole; it stops at a pair of stars not adjacent.
func (e *Empire) RouteLanes(s *Starship, route []int) (lanes []int, days int) {
	strength := s.Design.LaneStrength()
	if !(strength > 0) {
		return nil, 0
	}
	at := s.Star
	for _, to := range route {
		li, _, ok := laneEnd(e.Cluster, at, to)
		if !ok {
			break
		}
		lanes = append(lanes, li)
		days += LaneDays(e.Cluster, li, strength, s.Species.Traits())
		at = to
	}
	return lanes, days
}

// CanNavigate reports why e cannot send s to another system (GoToSystem)
// now: command's refusals, then Check's (a lane, no lane drive, no
// engine).
func (e *Empire) CanNavigate(s *Starship) error {
	if err := e.command(s); err != nil {
		return err
	}
	return s.Check(e.Cluster, Order{Kind: GoToSystem, Star: s.Star})
}

// steer swaps s's GoToSystem order, as holdAtBlock swaps (the caller
// moves s, then unsteer), for the GoToStar it flies today: in a lane, to
// the lane's far star; out of one, to Route's first star. At the
// destination the order is cleared (no swap); with no route left too,
// and the NoRoute event is returned (placed at s's star).
func (e *Empire) steer(s *Starship) (o, hop Order, swapped bool, lost *Event) {
	if s.Order.Kind != GoToSystem {
		return Order{}, Order{}, false, nil
	}
	o = s.Order
	if s.Lane >= 0 {
		hop = Order{Kind: GoToStar, Star: e.Cluster.Other(s.Lane, s.Star)}
		s.Order = hop
		return o, hop, true, nil
	}
	if s.Star == o.Star {
		s.Order = Order{}
		return o, Order{}, false, nil
	}
	r, err := e.Route(s, o.Star)
	if err != nil {
		s.Order = Order{}
		return o, Order{}, false, &Event{Kind: NoRoute, Item: NoItem, Tech: NoTech, Star: s.Star, Planet: -1, Other: -1,
			Text: fmt.Sprintf("%s has no known route to %s", s.Name, e.StarLabel(o.Star))}
	}
	hop = Order{Kind: GoToStar, Star: r[0]}
	s.Order = hop
	return o, hop, true, nil
}

// unsteer puts o back after s moved on steer's hop: while the hop stands
// (flying to the lane, or in it) or a lane was crossed (Step clears the
// order at the exit; from is the star s stood at), not when the move
// dropped it (Hop's Check). Out of a lane at o's star the order is done.
func unsteer(s *Starship, o, hop Order, from int) {
	if s.Order == hop || s.Order == (Order{}) && s.Star != from {
		s.Order = o
	}
	if s.Order == o && s.Lane < 0 && s.Star == o.Star {
		s.Order = Order{}
	}
}
