package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// mid is r's middle, a press's point.
func mid(r rect) (float64, float64) { return float64(r.X0+r.X1) / 2, float64(r.Y0+r.Y1) / 2 }

// The settings panel: F2 opens it; a press toggles the music (the
// jukebox follows), − and + step the gap within 0..gapMax, a drag on a
// slider sets its volume snapped to volStep; Cancel restores the opening
// values and writes nothing; Done writes the four keys into the config
// file, keeping its others; Esc keeps too, nothing written when unchanged.
func TestSettings(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "aiscendancy.json")
	if err := os.WriteFile(cfg, []byte(`{"game": "/g", "volume": 0.6}`), 0o644); err != nil {
		t.Fatal(err)
	}
	deck := &fakeDeck{}
	a := &app{configPath: cfg, music: &jukebox{dev: deck, on: true}}
	a.set = settings{music: true, volume: 0.6, sfx: 0.5, gap: time.Minute}
	if a.settingsInput(false, false, false, false, 0, 0) || a.setUp != nil {
		t.Fatal("up without F2")
	}
	if !a.settingsInput(true, false, false, false, 0, 0) || a.setUp == nil {
		t.Fatal("F2 does not open it")
	}
	press := func(r rect) {
		x, y := mid(r)
		a.settingsInput(false, false, true, true, x, y)
		a.settingsInput(false, false, false, false, x, y)
	}
	press(setCtrl(setMusic))
	if a.set.music || a.music.on {
		t.Errorf("music %v, jukebox %v after the toggle", a.set.music, a.music.on)
	}
	minus, plus := setGapButtons()
	for range 3 {
		press(minus)
	}
	if a.set.gap != 0 {
		t.Errorf("gap %v after three −", a.set.gap)
	}
	for range 30 {
		press(plus)
	}
	if a.set.gap != gapMax || a.music.gap != int(gapMax.Seconds()*60) {
		t.Errorf("gap %v, jukebox %d frames", a.set.gap, a.music.gap)
	}
	r := setCtrl(setVolume)
	_, y := mid(r)
	a.settingsInput(false, false, true, true, float64(r.X0)+1, y)
	a.settingsInput(false, false, false, true, float64(r.X0)+0.26*float64(r.X1-r.X0), y-100) // dragged off the row
	if a.set.volume != 0.25 {
		t.Errorf("volume %v, want 0.25", a.set.volume)
	}
	a.settingsInput(false, false, false, false, float64(r.X1)+50, y) // let go: no more drag
	if a.set.volume != 0.25 || a.setUp.drag != setNone {
		t.Errorf("volume %v, drag %d after the release", a.set.volume, a.setUp.drag)
	}

	press(setCancel)
	if a.setUp != nil || a.set != (settings{music: true, volume: 0.6, sfx: 0.5, gap: time.Minute}) || !a.music.on {
		t.Fatalf("Cancel: up %v, %+v, music %v", a.setUp != nil, a.set, a.music.on)
	}
	if b, _ := os.ReadFile(cfg); string(b) != `{"game": "/g", "volume": 0.6}` {
		t.Errorf("Cancel wrote %s", b)
	}

	a.settingsInput(true, false, false, false, 0, 0)
	a.settingsInput(false, true, false, false, 0, 0) // Esc, unchanged
	if b, _ := os.ReadFile(cfg); a.setUp != nil || string(b) != `{"game": "/g", "volume": 0.6}` {
		t.Errorf("Esc unchanged: up %v, wrote %s", a.setUp != nil, b)
	}

	a.settingsInput(true, false, false, false, 0, 0)
	press(setCtrl(setMusic))
	press(plus)
	press(setDone)
	if a.setUp != nil {
		t.Fatal("Done left it up")
	}
	b, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"game": "/g", "music": false, "volume": 0.6, "sfx": 0.5, "music-gap": "1m30s"}
	if len(m) != len(want) {
		t.Errorf("config %s", b)
	}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("%s = %v, want %v (%s)", k, m[k], v, b)
		}
	}
}

// writeConfig makes an absent file and refuses to overwrite one that
// does not parse.
func TestWriteConfig(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "new.json")
	if err := writeConfig(p, map[string]any{"sfx": 0.3}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "{\n  \"sfx\": 0.3\n}\n" {
		t.Errorf("made %q", b)
	}
	bad := filepath.Join(dir, "bad.json")
	os.WriteFile(bad, []byte("{oops"), 0o644)
	if err := writeConfig(bad, map[string]any{"sfx": 0.3}); err == nil || !strings.Contains(err.Error(), bad) {
		t.Errorf("bad file: %v", err)
	}
	if b, _ := os.ReadFile(bad); string(b) != "{oops" {
		t.Errorf("bad file overwritten: %q", b)
	}
}

// fmtGap writes minutes and seconds, the zero ones dropped.
func TestFmtGap(t *testing.T) {
	for d, want := range map[time.Duration]string{0: "0s", 30 * time.Second: "30s", 3 * time.Minute: "3m", 150 * time.Second: "2m30s"} {
		if got := fmtGap(d); got != want {
			t.Errorf("%v: %q, want %q", d, got, want)
		}
	}
}

// F3 keeps the settings' music what the jukebox plays: a dead one
// (no device) toggles nothing, so the panel and Done's file stay put.
func TestSettingsF3Dead(t *testing.T) {
	a := &app{lastPath: filepath.Join(t.TempDir(), "none.json.gz"), music: &jukebox{dev: &fakeDeck{}, on: true, dead: true}}
	a.set.music = true
	a.open([]screen{newOpeningScreen()})
	pressing(t, ebiten.KeyF3)
	for range 3 {
		if err := a.Update(); err != nil {
			t.Fatal(err)
		}
	}
	if a.set.music != a.music.on {
		t.Errorf("settings' music %v, the jukebox's %v", a.set.music, a.music.on)
	}
}
