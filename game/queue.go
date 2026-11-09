package game

import (
	"errors"
	"fmt"
	"slices"
)

// The construction queue: a colony's constructions
// waiting for its project, each tied to a square or an orbit slot: the
// surface structures, Terraforming, Automation, the dig, the orbital
// structures but the Ship. Never a ship, a refit nor a planet-wide
// project. An entry is judged at enqueue against the planet as it will
// be (Planned): the running project and every entry before it applied.
// Free workers are not checked then. Next starts the head at the close
// (Colony.EndDay) or after the player's cancel; an entry that cannot
// start any more (its Terraforming dequeued, a structure abandoned...) is
// dropped then with an Unqueued event, one short of workers waits.

// Queued is one queue entry, its target encoded as a Project's.
type Queued struct {
	Item    Item
	Cell    Cell // surface target, when !Orbital
	Slot    int  // orbit target, when Orbital
	Orbital bool
}

// Planned is a copy of the colony's planet as it will be once the running
// project and the whole queue are done, in order: a structure placed,
// Terraforming whitening its square, the dig clearing its ruins, an
// Automation marking its square Automated. A ship, a refit, an ongoing or
// planet-wide project place nothing. The colony's planet is untouched.
func (c *Colony) Planned() *Planet { return c.planned(len(c.Queue)) }

// planned is Planned with the first n entries only.
func (c *Colony) planned(n int) *Planet {
	p := c.Planet.clone()
	if pj := c.Project; c.Running() && pj.Refit == "" {
		p.plan(pj.Item, pj.Cell, pj.Slot, pj.Orbital)
	}
	for _, q := range c.Queue[:n] {
		p.plan(q.Item, q.Cell, q.Slot, q.Orbital)
	}
	return p
}

// clone is a copy of p whose squares are its own.
func (p *Planet) clone() *Planet {
	cp := *p
	cp.Surface = make(map[Cell]*Square, len(p.Surface))
	for c, sq := range p.Surface {
		s := *sq
		cp.Surface[c] = &s
	}
	return &cp
}

// plan applies a completed it at its target to p, unchecked (Planned).
func (p *Planet) plan(it Item, at Cell, slot int, orbital bool) {
	sq, ok := p.Surface[at]
	switch {
	case it == Ship || it.PlanetWide():
	case orbital:
		if slot >= 0 && slot < OrbitSlots {
			p.Orbit[slot] = it
		}
	case !ok:
	case it == Terraforming:
		p.whiten(at)
	case it == XenoArcheologicalDig:
		sq.Ruins = false
	case it == Automation:
		sq.Automated = true
	default:
		sq.Item = it
	}
}

// queueable is what any entry needs: not a Colony Base, the research.
func queueable(r *Research, it Item) error {
	if it == ColonyBase {
		return fmt.Errorf("%w: a Colony Base comes with the colony", ErrNotBuildable)
	}
	return allowed(r, it)
}

// CanEnqueue reports why the OnSquare item it, or an Automation, cannot
// be queued at cell at, nil when it can: CanStart's refusals but the
// running project and the workers, its target judged on Planned. A target
// a queued entry or the project takes already is refused by the plan
// itself (the square holds it, is white, is dug, is Automated).
func (c *Colony) CanEnqueue(r *Research, it Item, at Cell) error {
	if !it.OnSquare() && it != Automation {
		return fmt.Errorf("%s is not a surface structure", it)
	}
	if err := queueable(r, it); err != nil {
		return err
	}
	return c.squareErr(c.Planned(), it, at)
}

// CanEnqueueOrbital reports why the orbital structure it cannot be queued
// in slot, nil when it can; a Ship never is.
func (c *Colony) CanEnqueueOrbital(r *Research, it Item, slot int) error {
	switch {
	case it == Ship:
		return fmt.Errorf("%s cannot be queued: build it from a Shipyard", it)
	case !it.Orbital():
		return fmt.Errorf("%s is not an orbital structure", it)
	}
	if err := queueable(r, it); err != nil {
		return err
	}
	return orbitErr(c.Planned(), it, slot)
}

// Enqueue appends it at cell at, refused as CanEnqueue says.
func (c *Colony) Enqueue(r *Research, it Item, at Cell) error {
	if err := c.CanEnqueue(r, it, at); err != nil {
		return err
	}
	c.Queue = append(c.Queue, Queued{Item: it, Cell: at, Slot: -1})
	c.recordQueued("enqueue", c.Queue[len(c.Queue)-1])
	return nil
}

// EnqueueOrbital appends it in slot, refused as CanEnqueueOrbital says.
func (c *Colony) EnqueueOrbital(r *Research, it Item, slot int) error {
	if err := c.CanEnqueueOrbital(r, it, slot); err != nil {
		return err
	}
	c.Queue = append(c.Queue, Queued{Item: it, Slot: slot, Orbital: true})
	c.recordQueued("enqueue", c.Queue[len(c.Queue)-1])
	return nil
}

// errNoEntry refuses an index outside the queue.
var errNoEntry = errors.New("no such queue entry")

// Dequeue removes entry i. The later entries stay, even one it made
// possible: Next drops it when its turn comes.
func (c *Colony) Dequeue(i int) error {
	if i < 0 || i >= len(c.Queue) {
		return errNoEntry
	}
	q := c.Queue[i]
	c.Queue = slices.Delete(c.Queue, i, i+1)
	c.recordQueued("dequeue", q)
	return nil
}

// QueueUp swaps entry i with the one before it (i ≥ 1); the entry moved
// is recorded.
func (c *Colony) QueueUp(i int) error {
	if i < 1 || i >= len(c.Queue) {
		return errNoEntry
	}
	c.Queue[i-1], c.Queue[i] = c.Queue[i], c.Queue[i-1]
	c.recordQueued("queue-up", c.Queue[i-1])
	return nil
}

// recordQueued journals act on entry q: colony, item, cell or slot.
func (c *Colony) recordQueued(act string, q Queued) {
	if c.journal == nil {
		return
	}
	en := c.entry(act)
	en.Item = q.Item.String()
	if q.Orbital {
		en.Slot = q.Slot
	} else {
		en.Cell = q.Cell.String()
	}
	c.journal.Record(en)
}

// Next starts the queue's head while the colony runs no project (none,
// or an ongoing one, which gives way): Start or StartOrbital (journalled
// "start"). Short of workers (ErrNoFreePop) the head waits; any other
// refusal drops it with an Unqueued event and the next is tried. Called
// by Colony.EndDay last, and by the window after the player's Cancel or
// Abandon.
func (c *Colony) Next(r *Research) []Event {
	var ev []Event
	for !c.Running() && len(c.Queue) > 0 {
		q := c.Queue[0]
		var err error
		if q.Orbital {
			err = c.StartOrbital(r, q.Item, q.Slot)
		} else {
			err = c.Start(r, q.Item, q.Cell)
		}
		if errors.Is(err, ErrNoFreePop) {
			break
		}
		c.Queue = slices.Delete(c.Queue, 0, 1)
		if err != nil {
			ev = append(ev, Event{Kind: Unqueued, Item: q.Item, Tech: NoTech, Star: c.Star, Planet: c.PlanetIdx, Other: -1,
				Text: fmt.Sprintf("%s dropped from the queue on %s: %v", q.Item, c.Planet.Name, err)})
		}
	}
	return ev
}
