package game

import "testing"

// FreeAtCompletion under Terraforming: the project places nothing, so its
// worker comes back whole (not a structure's Staff: the
// forecast would be one short).
func TestFreeAtCompletionTerraform(t *testing.T) {
	e := NewEmpire(1, Snovemdomas, Average)
	c := e.Colonies[0]
	c.Pop += 2
	r := e.Research
	r.Grant(Tech(Terraforming.Info().ResearchReq))
	p := c.Planet
	var black Cell
	found := false
	for _, at := range p.Cells() {
		if sq := p.Surface[at]; sq.Item == NoItem && sq.Kind == White && p.Reachable(at) {
			black, found = at, true
			sq.Kind = Black
			break
		}
	}
	if !found {
		t.Fatal("no square")
	}
	if err := c.Start(r, Terraforming, black); err != nil {
		t.Fatal(err)
	}
	pop0, want := c.Pop, c.FreeAtCompletion()
	for d := 0; c.Project != nil; d++ {
		if d > 1000 {
			t.Fatal("never completes")
		}
		c.EndDay(r, 0)
	}
	if got := c.Free() - (c.Pop - pop0); got != want {
		t.Errorf("free %d after the Terraforming (growth aside), forecast %d", got, want)
	}
}
