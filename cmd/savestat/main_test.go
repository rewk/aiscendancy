package main

import (
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/save"
)

// TestTable: a row per empire, the player starred, today's numbers the
// home colony's.
func TestTable(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Orfa, game.Mebes}, game.Average)
	f := &save.File{Game: g, Bots: []*bot.State{nil, {}}}
	var b strings.Builder
	table(&b, f)
	lines := strings.Split(strings.TrimSpace(b.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("%d lines:\n%s", len(lines), b.String())
	}
	if !strings.HasPrefix(lines[1], "0 Orfa*") || !strings.HasPrefix(lines[2], "1 Mebes ") {
		t.Errorf("rows:\n%s", b.String())
	}
	if r := today(g.Empires[0]); r.Colonies != 1 || r.Pop != g.Empires[0].Colonies[0].Pop || r.Systems != 1 {
		t.Errorf("today %+v", r)
	}
	b.Reset()
	detail(&b, f, g.Empires[1], 160)
	if !strings.Contains(b.String(), "stances: none met") {
		t.Errorf("detail:\n%s", b.String())
	}
}
