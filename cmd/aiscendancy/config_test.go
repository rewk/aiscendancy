package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// configFlags are a command line (game, config) and the file-only sound
// set (volume, music-gap, music), as main's.
func configFlags() (cl, file *flag.FlagSet, game *string, v *float64, g *time.Duration, m *bool) {
	cl = flag.NewFlagSet("t", flag.ContinueOnError)
	game = cl.String("game", "/default", "")
	cl.String("config", configDefault, "")
	file = flag.NewFlagSet("settings", flag.ContinueOnError)
	v = file.Float64("volume", 0.6, "")
	g = file.Duration("music-gap", time.Minute, "")
	m = file.Bool("music", true, "")
	return
}

// The file sets the flags the command line left alone and the file-only
// ones, bare numbers and bools as strings alike; the command line wins;
// a file-only key is no command-line flag.
func TestConfigApplies(t *testing.T) {
	cl, file, game, v, g, m := configFlags()
	if err := cl.Parse([]string{"-game", "/cli"}); err != nil {
		t.Fatal(err)
	}
	in := `{"game": "/file", "volume": 0.9, "music-gap": "90s", "music": false}`
	if err := parseConfig(cl, file, "c", strings.NewReader(in)); err != nil {
		t.Fatal(err)
	}
	if *game != "/cli" || *v != 0.9 || *g != 90*time.Second || *m {
		t.Errorf("game %s (command line /cli), volume %v, gap %v, music %v", *game, *v, *g, *m)
	}
	cl, file, _, v, _, m = configFlags()
	if err := parseConfig(cl, file, "c", strings.NewReader(`{"volume": "0.3", "music": "false"}`)); err != nil || *v != 0.3 || *m {
		t.Errorf("quoted values: %v, volume %v, music %v", err, *v, *m)
	}
	cl, _, _, _, _, _ = configFlags()
	cl.SetOutput(new(strings.Builder))
	if err := cl.Parse([]string{"-volume", "0.2"}); err == nil {
		t.Error("-volume on the command line accepted")
	}
}

// Unknown keys, -config itself, bad values and a malformed file are
// refused, the source named.
func TestConfigRefuses(t *testing.T) {
	for _, in := range []string{`{"loudness": 3}`, `{"config": "x.json"}`, `{"volume": "loud"}`, `{"volume": [1]}`, `volume = 0.4`, `{"music": "yes"}`} {
		cl, file, _, _, _, _ := configFlags()
		if err := parseConfig(cl, file, "c", strings.NewReader(in)); err == nil || !strings.HasPrefix(err.Error(), "c:") {
			t.Errorf("%q: %v", in, err)
		}
	}
}

// The default file may be absent; a named one must exist.
func TestConfigMissing(t *testing.T) {
	cl, file, _, _, _, _ := configFlags()
	p := filepath.Join(t.TempDir(), "none.json")
	if err := applyConfig(cl, file, p, false); err != nil {
		t.Errorf("default absent: %v", err)
	}
	if err := applyConfig(cl, file, p, true); err == nil {
		t.Error("named absent: no error")
	}
	if err := os.WriteFile(p, []byte(`{"volume": 0.1}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyConfig(cl, file, p, true); err != nil || file.Lookup("volume").Value.String() != "0.1" {
		t.Errorf("named present: %v, volume %s", err, file.Lookup("volume").Value)
	}
}
