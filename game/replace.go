package game

import "fmt"

// OutputWith is Output as if surface square at held it (NoItem: empty),
// the rest as is: the square's Item swapped, Output read, swapped back.
func (c *Colony) OutputWith(at Cell, it Item) (ind, res, pros int) {
	sq, ok := c.Planet.Surface[at]
	if !ok {
		return c.Output()
	}
	old := sq.Item
	sq.Item = it
	defer func() { sq.Item = old }()
	return c.Output()
}

// CanReplace reports why the structure at at cannot be replaced by the
// surface structure it, nil when it can: CanAbandon's refusals, the same
// item, a Running project (an ongoing one gives way, as to any start),
// an item not Surface, then CanStart's verdict on the square emptied (the
// old structure's worker counts as free). The emptying is undone.
func (c *Colony) CanReplace(r *Research, at Cell, it Item) error {
	if err := c.CanAbandon(at); err != nil {
		return err
	}
	sq := c.Planet.Surface[at]
	switch {
	case sq.Item == it:
		return fmt.Errorf("%v holds %s already", at, it)
	case c.Running():
		return fmt.Errorf("%w: %s", ErrProjectRunning, c.Project.what())
	case !it.Surface():
		return fmt.Errorf("%s is not a surface structure", it)
	}
	old, auto := sq.Item, sq.Automated
	sq.Item, sq.Automated = NoItem, false
	defer func() { sq.Item, sq.Automated = old, auto }()
	return c.CanStart(r, it, at)
}

// Replace abandons the structure at at and starts it there, journalled
// as both ("demolish" then "start"); refused as CanReplace says, nothing
// changed.
func (c *Colony) Replace(r *Research, at Cell, it Item) error {
	if err := c.CanReplace(r, at, it); err != nil {
		return err
	}
	sq := c.Planet.Surface[at]
	old, auto := sq.Item, sq.Automated
	if err := c.Abandon(at); err != nil {
		return err
	}
	if err := c.Start(r, it, at); err != nil { // CanReplace checked it: unreachable
		sq.Item, sq.Automated = old, auto
		return err
	}
	return nil
}
