package help

import (
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/cob"
)

// Both shapes: planhelp's two blocks (the last wins), gizhelp's one with
// %s; the stray # line between entries, a blank line inside a block, an
// entry without text, a gap in the indices, CRLF, help end and the ^Z.
func TestParse(t *testing.T) {
	src := strings.Join([]string{
		"// header", "//", "#Factory", "#1  0  0", "help plitem00", "", "text", "%s", "endtext", "pos 150 60",
		"text", "The Factory is  common.", "  More here.  ", "", "endtext", "button", "title OK", "msg 0",
		"#X^Ray", "help gizmo01", "#Passive. always in effect, takes no power.", "text", "The %s sees.", "endtext",
		"help item02", "pos 1 1",
		"help gizmo04", "text", "Four.", "endtext",
		"help end", "text", "after the end", "endtext", "\x1a",
	}, "\r\n")
	got, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"The Factory is  common. More here.", "The %s sees.", "", "", "Four."}
	if len(got) != len(want) {
		t.Fatalf("%d entries %q, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d: %q, want %q", i, got[i], want[i])
		}
	}
	if f := Fill(got[1], "Mass Barrage Gun"); f != "The Mass Barrage Gun sees." {
		t.Errorf("Fill: %q", f)
	}
	// Not two digits: no entry (help end, help x7, help x123).
	if got, err := Parse([]byte("help x7\ntext\nA\nendtext\nhelp x123\ntext\nB\nendtext\n")); err != nil || len(got) != 0 {
		t.Errorf("odd suffixes: %q %v", got, err)
	}
	if _, err := Parse([]byte("help plitem00\ntext\nno end\n")); err == nil {
		t.Error("text without endtext parsed")
	}
}

// The game's files: one paragraph per item (39) and per part (76).
func TestGameFiles(t *testing.T) {
	g, err := cob.OpenGame(cob.DefaultDir())
	if err != nil {
		t.Skipf("no game: %v", err)
	}
	defer g.Close()
	for _, tc := range []struct {
		name string
		n    int
	}{{"planhelp.txt", 39}, {"gizhelp.txt", 76}} {
		b, err := g.Read(tc.name)
		if err != nil {
			t.Fatal(err)
		}
		ps, err := Parse(b)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if len(ps) != tc.n {
			t.Errorf("%s: %d entries, want %d", tc.name, len(ps), tc.n)
		}
		for i, p := range ps {
			if p == "" || p == "%s" || strings.ContainsAny(p, "\r\n\x1a") {
				t.Errorf("%s entry %d: %q", tc.name, i, p)
			}
		}
		switch tc.name {
		case "planhelp.txt":
			if !strings.HasPrefix(ps[0], "The Factory is") {
				t.Errorf("plitem00: %q", ps[0])
			}
		case "gizhelp.txt":
			if f := Fill(ps[0], "Mass Barrage Gun"); !strings.Contains(f, "Mass Barrage Gun") || strings.Contains(f, "%s") {
				t.Errorf("gizmo00: %q", f)
			}
		}
	}
}

// The enddesc block alone, its first line's underscores off; the other
// blocks ignored; a missing block or endtext fails.
func TestParseEnding(t *testing.T) {
	src := strings.Join([]string{
		"// Text file", "//", "enddesc", "_____First line.", "  Second _line.", "", "endtext", "",
		"ranks", "Pushover.", "", "congrat", "_____Long.", "endtext", "end", "\x1a",
	}, "\r\n")
	got, err := ParseEnding([]byte(src))
	if want := []string{"First line.", "Second _line."}; err != nil || strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("%q %v, want %q", got, err, want)
	}
	for _, bad := range []string{"ranks\r\nx\r\n", "enddesc\r\nx\r\n"} {
		if _, err := ParseEnding([]byte(bad)); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
	if got, err := ParseEnding([]byte("enddesc\r\n^^^^^Unified.\r\nendtext\r\n")); err != nil || len(got) != 1 || got[0] != "Unified." {
		t.Errorf("carets: %q %v", got, err)
	}
	if got, err := ParseRanks([]byte(src)); err != nil || len(got) != 1 || got[0] != "Pushover." {
		t.Errorf("ranks %q %v", got, err)
	}
	if _, err := ParseRanks([]byte("enddesc\r\nx\r\nendtext\r\n")); err == nil {
		t.Error("ranks parsed without the block")
	}
}

// The game's extinction ending: two lines, the first the annihilation.
func TestEndingGame(t *testing.T) {
	g, err := cob.OpenGame(cob.DefaultDir())
	if err != nil {
		t.Skipf("no game: %v", err)
	}
	g.Close()
	ls, err := Ending("xtnc")
	if err != nil {
		t.Fatal(err)
	}
	if len(ls) != 2 || !strings.HasPrefix(ls[0], "The hostile forces") || strings.ContainsAny(strings.Join(ls, ""), "_\r\x1a") {
		t.Errorf("end-xtnc: %q", ls)
	}
}
