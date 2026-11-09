package main

import (
	"os"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/rewk/aiscendancy/cob"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// realHistory is the game's history.txt parsed; the test skips without
// the game.
func realHistory(t *testing.T) []speciesHistory {
	t.Helper()
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	g, err := cob.OpenGame(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	b, err := g.Read("history.txt")
	if err != nil {
		t.Fatal(err)
	}
	hs, err := parseHistory(string(b))
	if err != nil {
		t.Fatal(err)
	}
	return hs
}

// The grammar: specnum blocks, sections up to their end line, "@@"
// paragraphs, the "_" indent, soft line breaks, the file's two spaces
// kept and a trailing space before a break not making three; the
// neutral section runs on into the intro's last paragraph.
func TestParseHistoryGrammar(t *testing.T) {
	src := "// A\r\nspecnum 0\r\npower\r\nBig Ones \r\nendpower\r\n\r\nintro\r\n_____One.  Two \r\nthree.\r\n" +
		"@@____Four.\r\nendintro\r\npeace\r\n_P.\r\nendpeace\r\nneutral\r\n_Five\r\nsix.\r\nendneutral\r\n" +
		"hostile\r\nendhostile\r\ntext\r\nNo indent. \r\nendtext\r\n// B\r\nspecnum 1\r\npower\r\nX\r\nendpower\r\nEND\r\n\x1a"
	hs, err := parseHistory(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(hs) != 2 || hs[0].power != "Big Ones" || hs[1].power != "X" {
		t.Fatalf("%+v", hs)
	}
	want := []paragraph{{true, "One.  Two  three."}, {true, "Four.  Five six."}}
	if got := hs[0].introParas(game.Neutral); !slices.Equal(got, want) {
		t.Fatalf("intro %+v, want %+v", got, want)
	}
	if !slices.Equal(hs[0].text, []paragraph{{false, "No indent."}}) || len(hs[0].intro) != 2 {
		t.Fatalf("text %+v, intro %+v", hs[0].text, hs[0].intro)
	}
	// Peaceful runs the peace section on instead; Hostile's is empty here.
	if got := hs[0].introParas(game.Peaceful); !slices.Equal(got, []paragraph{{true, "One.  Two  three."}, {true, "Four.  P."}}) {
		t.Errorf("peaceful %+v", got)
	}
	if got := hs[0].introParas(game.Hostile); !slices.Equal(got, hs[0].intro) {
		t.Errorf("hostile %+v", got)
	}
	for _, bad := range []string{"specnum 1\r\n", "specnum 0\r\nintro\r\nx\r\n", "specnum 0\r\nwhat\r\n"} {
		if _, err := parseHistory(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

// The real file: 21 species in game.Species order (the titles of the
// captures image0044 and image0045), every one with an intro and a text;
// the Snovemdomas' intro ends on their neutral section, as image0045.
func TestParseHistory(t *testing.T) {
	hs := realHistory(t)
	if len(hs) != int(game.NumSpecies) {
		t.Fatalf("%d species", len(hs))
	}
	if hs[game.Orfa].power != "Black Square Dwellers" || hs[game.Snovemdomas].power != "Builders of Nigh-Indestructible Ships" {
		t.Fatalf("Orfa %q, Snovemdomas %q", hs[game.Orfa].power, hs[game.Snovemdomas].power)
	}
	if hs[game.Minions].power != "Supreme Planet Invaders" || hs[game.Species(game.NumSpecies-1)].power != "Expert Builders" {
		t.Fatalf("first %q, last %q", hs[0].power, hs[game.NumSpecies-1].power)
	}
	for sp, h := range hs {
		if len(h.intro) == 0 || len(h.text) == 0 || !h.intro[0].indent {
			t.Errorf("%s: %d intro, %d text paragraphs", game.Species(sp), len(h.intro), len(h.text))
		}
		for _, p := range append(h.introParas(game.Neutral), h.text...) {
			if strings.ContainsAny(p.text, "_@\r\n") || strings.Contains(p.text, "   ") {
				t.Errorf("%s: %q", game.Species(sp), p.text)
			}
		}
	}
	ps := hs[game.Snovemdomas].introParas(game.Neutral)
	if len(ps) != 3 || !strings.HasSuffix(ps[2].text, "among the stars.  Heed the Song of Mandenonda, which foretells the destruction of the unwary.") {
		t.Fatalf("Snovemdomas intro %+v", ps)
	}
}

// reflow keeps lines within cols, the indent first, the file's gaps
// inside a line, none at its ends, every word in order.
func TestReflow(t *testing.T) {
	p := paragraph{true, "The Plan.  The Rituals of Reproduction have gotten out of hand.  Go."}
	lines := reflow(p, 20)
	if !strings.HasPrefix(lines[0], paraIndent+"The") {
		t.Fatalf("first line %q", lines[0])
	}
	for _, l := range lines {
		if utf8.RuneCountInString(l) > 20 || strings.HasSuffix(l, " ") || (l != lines[0] && strings.HasPrefix(l, " ")) {
			t.Fatalf("line %q of %q", l, lines)
		}
	}
	if got := strings.Fields(strings.Join(lines, " ")); !slices.Equal(got, strings.Fields(p.text)) {
		t.Fatalf("words %q", got)
	}
	if !slices.Contains(lines, paraIndent+"The Plan.  The") {
		t.Fatalf("the two spaces lost: %q", lines)
	}
	if got := reflow(paragraph{false, "averyveryverylongword x"}, 5); !slices.Equal(got, []string{"averyveryverylongword", "x"}) {
		t.Fatalf("long word: %q", got)
	}
}

// Every species' intro and description fit their panels whole in one
// face, each line within the box's width; the face is logged, and none
// falls back below bigFace.
func TestIntroFits(t *testing.T) {
	hs := realHistory(t)
	boxes := []rect{inRightBox, inBottomBox}
	for sp, h := range hs {
		sets := [][]paragraph{h.introParas(game.Neutral), h.text}
		ls, face := fitText(sets, boxes)
		if face.Size < bigFace.Size {
			t.Errorf("%s: %.0f px, under bigFace", game.Species(sp), face.Size)
		}
		for i, lines := range ls {
			b := boxes[i]
			w, ht := b.X1-b.X0, b.Y1-b.Y0
			want, ok := layText(sets[i], w, ht, face)
			if !ok || !slices.Equal(lines, want) || float64(len(lines))*face.Size*1.25 > float64(ht) {
				t.Errorf("%s panel %d: %d lines in %d px at %.0f", game.Species(sp), i, len(lines), ht, face.Size)
			}
			for _, l := range lines {
				if ui.Width(l, face) > float64(w) {
					t.Errorf("%s panel %d: %q wider than %d", game.Species(sp), i, l, w)
				}
			}
		}
		t.Logf("%-12s %2d + %2d lines at %.0f px", game.Species(sp), len(ls[0]), len(ls[1]), face.Size)
	}
}

// Begin pushes the introduction over the starmap; its Enter plays the
// player's stinger (race02 for the Orfa) over the paused theme, the pick
// before it none; leaving (a key, a click, or Esc as app.back) uncovers
// the starmap and lets the stinger play out. Space and Tab stay the
// screen's (typing).
func TestIntroFlow(t *testing.T) {
	for _, esc := range []bool{false, true} {
		d := &fakeDeck{}
		a := &app{music: &jukebox{dev: d, on: true}}
		a.music.update()
		a.open([]screen{newNewGameScreen()})
		s := top[*newGameScreen](t, a)
		s.click(a, float64(ngList.X0+ngList.X1)/2, ngLineY(int(game.Orfa)))
		if len(d.started) != 1 {
			t.Fatalf("the pick started %v", d.names())
		}
		s.click(a, float64(ngHoles[beginHole].X0+ngHoles[beginHole].X1)/2, float64(ngHoles[beginHole].Y0+ngHoles[beginHole].Y1)/2)
		in := top[*introScreen](t, a)
		if len(a.stack) != 2 || in.species != game.Orfa || !in.typing() {
			t.Fatalf("%d screens, introduction of %s", len(a.stack), in.species)
		}
		if !slices.Equal(d.names(), []string{"theme00", "race02"}) || d.started[0].playing || !d.last().playing {
			t.Fatalf("started %v, theme playing %v", d.names(), d.started[0].playing)
		}
		if esc {
			if err := a.back(); err != nil || a.quitting {
				t.Fatalf("Esc: %v, quitting %v", err, a.quitting)
			}
		} else {
			in.leave(a)
		}
		top[*starmapScreen](t, a)
		a.music.update()
		if len(a.stack) != 1 || !d.last().playing || d.started[0].playing || len(d.started) != 2 {
			t.Fatalf("after leaving: %d screens, started %v", len(a.stack), d.names())
		}
	}
}
