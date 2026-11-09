package main

import (
	"os"
	"testing"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/cob"
)

// The icon is the sun at each size: square, opaque at the centre,
// transparent at the corners.
func TestWindowIcon(t *testing.T) {
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	as, err := assets.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer as.Close()
	icon := windowIcon(as)
	if len(icon) != len(iconSizes) {
		t.Fatalf("%d images, want %d", len(icon), len(iconSizes))
	}
	for k, img := range icon {
		n := iconSizes[k]
		if b := img.Bounds(); b.Dx() != n || b.Dy() != n {
			t.Errorf("size %d: bounds %v", n, b)
		}
		if _, _, _, a := img.At(n/2, n/2).RGBA(); a == 0 {
			t.Errorf("size %d: transparent centre", n)
		}
		if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
			t.Errorf("size %d: opaque corner", n)
		}
	}
}
