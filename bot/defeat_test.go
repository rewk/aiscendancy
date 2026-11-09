package bot

import (
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// A defeated empire's bot does nothing: no topic studied, no round
// played, every offer refused; the day runs on without a panic.
func TestDefeatedBotIdle(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	a, b := g.Empires[0], g.Empires[1]
	home := a.Colonies[0]
	// A's home captured by the Orfa, as an invasion leaves it.
	home.Planet.Owner = b.Index
	a.Colonies = nil
	b.Colonies = append(b.Colonies, home)
	bots := NewAll(g, Default())
	Day(g, bots)
	if a.Alive() {
		t.Fatal("fixture: A not defeated")
	}
	a.Research.Topic = game.NoTech
	for range 3 {
		Day(g, bots)
	}
	if a.Research.Topic != game.NoTech || len(a.Ships) != 0 || len(a.Colonies) != 0 {
		t.Errorf("the defeated bot acted: topic %v, %d ships, %d colonies", a.Research.Topic, len(a.Ships), len(a.Colonies))
	}
	bots[0].Round(home.Star)
	if bots[0].Decide(game.Offer{From: 1, To: 0, Kind: game.PeaceOffer, Third: -1}) {
		t.Error("a defeated bot accepts an offer")
	}
}

// A defeated enemy is never the subject of a war request: its War stance
// stays, so askWar (first enemy in index order) asked about it every
// AskZeal days, refused, the day's offer to the ally spent on it.
func TestAskWarSkipsDefeated(t *testing.T) {
	g := reqGame3(t, game.Dubtaks, game.Orfa)
	atWar(t, g, 0, 2)
	g.Empires[2].Lost = 1
	b := NewAll(g, Config{NoResearch: true})[0]
	b.Act()
	if b.lastAsk[1] != -1 || len(requests(t, g, 1)) != 0 {
		t.Errorf("asked about a defeated enemy: lastAsk %d", b.lastAsk[1])
	}
}
