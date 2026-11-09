package main

import (
	"io"
	"path/filepath"
	"testing"

	"github.com/rewk/aiscendancy/sav"
)

// Every player colony of the test saves records the output the model
// computes on its squares (docs/rules.md "Colony output"); the saves are
// sav/testdata's.
func TestColoniesSaves(t *testing.T) {
	fs, _ := filepath.Glob(filepath.Join("..", "..", "sav", "testdata", "*.[sS][aA][vV]"))
	if len(fs) != 18 {
		t.Fatalf("%d saves in sav/testdata, want 18", len(fs))
	}
	n := 0
	for _, f := range fs {
		g, err := sav.Open(f)
		if err != nil {
			t.Fatal(f, err)
		}
		for _, p := range g.Planets {
			if c := colony(p); c != nil && p.Owner == 0 {
				n++
				if got, want := shown(c), recorded(p); got != want {
					t.Errorf("%s %s: model %v, recorded %v", filepath.Base(f), p.Name, got, want)
				}
			}
		}
		if d := colonies(io.Discard, g); d != 0 {
			t.Errorf("%s: colonies reports %d differing", filepath.Base(f), d)
		}
	}
	if n < 800 {
		t.Errorf("%d player colonies, the test saves hold 870", n)
	}
}
