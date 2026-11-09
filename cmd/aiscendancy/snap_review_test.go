package main

import (
	"image"
	"os"
	"path/filepath"
	"testing"
)

// Two screenshots in the same second (one name) both stay: the second
// takes a suffix instead of replacing the first.
func TestSnapNoOverwrite(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "shots")
	path := filepath.Join(dir, "aiscendancy-x.png")
	for i := range 2 {
		if _, err := writeSnap(path, image.NewRGBA(image.Rect(0, 0, 1+i, 1))); err != nil {
			t.Fatal(err)
		}
	}
	if fs, err := os.ReadDir(dir); err != nil || len(fs) != 2 {
		t.Errorf("%d files in shots/ after two snaps, want 2 (%v)", len(fs), err)
	}
}
