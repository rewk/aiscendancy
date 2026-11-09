package save

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/rewk/aiscendancy/game"
)

// TestMain pins Write's clock: the byte comparisons of two saves (one
// game played through, one loaded) would differ by Saved alone.
func TestMain(m *testing.M) {
	saved := time.Date(2026, 10, 22, 18, 34, 0, 0, time.Local)
	now = func() time.Time { return saved }
	os.Exit(m.Run())
}

// peekGame is four empires: the player at war with 1, allied with 2 (then
// defeated), never met by 3; fixture ships, a tech granted.
func peekGame(t *testing.T) *game.Game {
	t.Helper()
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa, game.Mebes, game.Minions}, game.Average)
	if err := game.SpawnFixtures(g, 2, 1, 0); err != nil {
		t.Fatal(err)
	}
	g.Contact(0, 1)
	if _, err := g.DeclareWar(0, 1); err != nil {
		t.Fatal(err)
	}
	g.Contact(0, 2)
	if _, err := g.OfferAlliance(0, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Answer(g.PendingOffers(2)[0], true); err != nil {
		t.Fatal(err)
	}
	g.Empires[2].Lost = g.Day + 1 // Lost 0 is the living
	g.Empires[0].Research.Grant(0)
	g.Empires[0].Research.Grant(game.NumTechs - 1)
	if g.Empires[2].Alive() {
		t.Fatal("fixture: empire 2 still alive")
	}
	return g
}

// Write's Summary, read back by Peek, says what the game says of empire
// 0: species, colour, day, ships, colonies, techs, and the met empires in
// Empires order with their stance and death; Saved is the clock's time.
func TestPeekRoundTrip(t *testing.T) {
	g := peekGame(t)
	e := g.Empires[0]
	path := filepath.Join(t.TempDir(), "a.json")
	if err := Write(path, &File{Game: g}); err != nil {
		t.Fatal(err)
	}
	v, s, err := Peek(path)
	if err != nil || v != Version || s == nil {
		t.Fatalf("Peek: version %d, summary %v, %v", v, s, err)
	}
	techs := 0
	for t := range game.NumTechs {
		if e.Research.Known[t] {
			techs++
		}
	}
	if s.Species != e.Species || s.Colour != e.Colour || s.Day != e.Day || s.Ships != len(e.Ships) ||
		s.Colonies != len(e.Colonies) || s.Techs != techs {
		t.Errorf("summary %+v; empire: %v colour %d day %d ships %d colonies %d techs %d",
			*s, e.Species, e.Colour, e.Day, len(e.Ships), len(e.Colonies), techs)
	}
	if s.Ships == 0 || s.Techs == 0 {
		t.Errorf("fixture: %d ships, %d techs: the counts prove nothing", s.Ships, s.Techs)
	}
	var want []Met
	for j, o := range g.Empires {
		if st := e.StanceWith(j); j != 0 && st != game.Unknown {
			want = append(want, Met{o.Species, o.Colour, st, !o.Alive()})
		}
	}
	if !reflect.DeepEqual(s.Met, want) {
		t.Errorf("met %+v, want %+v", s.Met, want)
	}
	if len(s.Met) != 2 || s.Met[0].Stance != game.War || s.Met[0].Dead ||
		s.Met[1].Stance != game.Alliance || !s.Met[1].Dead {
		t.Errorf("met %+v: want Orfa at war, Mebes allied and dead, Minions unmet", s.Met)
	}
	if !s.Saved.Equal(now()) {
		t.Errorf("saved at %v, the clock says %v", s.Saved, now())
	}
	f, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.Summary.Met, s.Met) || f.Summary.Day != s.Day {
		t.Errorf("Read's summary %+v, Peek's %+v", *f.Summary, *s)
	}
}

// Peek reads the head alone: a file cut right after the Summary (no
// Game, no closing brace) still peeks; Read refuses it.
func TestPeekStopsEarly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.json")
	if err := Write(path, &File{Game: peekGame(t)}); err != nil {
		t.Fatal(err)
	}
	b := plain(t, path)
	_, full, err := Peek(path)
	if err != nil {
		t.Fatal(err)
	}
	i := bytes.Index(b, []byte(`"Seed"`))
	if i < 0 || bytes.Index(b, []byte(`"Summary"`)) > i || bytes.Index(b, []byte(`"Game"`)) < i {
		t.Fatalf("not Version, Summary, Seed, Game in that order: Seed at %d", i)
	}
	cut := filepath.Join(dir, "cut.json")
	writePlain(t, cut, b[:i])
	v, s, err := Peek(cut)
	if err != nil || v != Version || !reflect.DeepEqual(s, full) {
		t.Errorf("cut: version %d, %+v, %v; want %+v", v, s, err, full)
	}
	if _, err := Read(cut); err == nil {
		t.Error("Read took the cut file")
	}
}

// A file of another version gives its version (and its summary); a
// broken one, a file with no version, or a current one with no summary
// is an error.
func TestPeekRefused(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.json")
	if err := Write(path, &File{Game: peekGame(t)}); err != nil {
		t.Fatal(err)
	}
	b := plain(t, path)
	cur := fmt.Appendf(nil, `"Version":%d,`, Version)
	old := filepath.Join(dir, "old.json")
	writePlain(t, old, bytes.Replace(b, cur, fmt.Appendf(nil, `"Version":%d,`, Version-1), 1))
	if v, s, err := Peek(old); err != nil || v != Version-1 || s == nil {
		t.Errorf("old: version %d, summary %v, %v", v, s, err)
	}
	if _, err := Read(old); err == nil {
		t.Error("Read took the old file")
	}
	noSum := filepath.Join(dir, "nosum.json")
	writePlain(t, noSum, fmt.Appendf(nil, `{"Version": %d, "Seed": 1}`, Version))
	for _, p := range []string{noSum, filepath.Join(dir, "missing.json")} {
		if _, _, err := Peek(p); err == nil {
			t.Errorf("%s: no error", filepath.Base(p))
		}
	}
	ungz := filepath.Join(dir, "ungz.json") // the JSON alone: not a save
	if err := os.WriteFile(ungz, b, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Peek(ungz); err == nil {
		t.Error("an ungzipped save peeked")
	}
	if _, err := Read(ungz); err == nil {
		t.Error("an ungzipped save read")
	}
	for name, body := range map[string]string{"broken": "{\"Vers", "array": "[1]", "noversion": `{"Seed": 1}`} {
		p := filepath.Join(dir, name+".json")
		writePlain(t, p, []byte(body))
		if v, _, err := Peek(p); err == nil || v != 0 {
			t.Errorf("%s: version %d, %v", name, v, err)
		}
	}
}

// The slots: slotNN.json.gz beside the quick save, listed ascending, other
// names and spellings ignored; the next number is one past the highest.
func TestSlots(t *testing.T) {
	dir := t.TempDir()
	quick := filepath.Join(dir, "quick.json")
	if ns, err := Slots(filepath.Join(dir, "none", "quick.json")); err != nil || ns != nil {
		t.Errorf("missing dir: %v, %v", ns, err)
	}
	if ns, _ := Slots(quick); NextSlot(ns) != 1 {
		t.Errorf("empty: next %d", NextSlot(ns))
	}
	for _, n := range []int{3, 1, 12, 100} {
		if err := os.WriteFile(SlotPath(quick, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"slot2.json.gz", "slot00.json.gz", "slot05.json.gz.tmp", "slot04.json", "quick.json.gz", "slotxx.json.gz"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ns, err := Slots(quick)
	if err != nil || !slices.Equal(ns, []int{1, 3, 12, 100}) {
		t.Errorf("slots %v, %v", ns, err)
	}
	if NextSlot(ns) != 101 {
		t.Errorf("next %d", NextSlot(ns))
	}
	if got := filepath.Base(SlotPath(quick, 7)); got != "slot07.json.gz" || filepath.Dir(SlotPath(quick, 7)) != dir {
		t.Errorf("slot 7: %s", SlotPath(quick, 7))
	}
}
