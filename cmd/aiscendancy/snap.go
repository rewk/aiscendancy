package main

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Screenshots: F12 or the quit prompt's button sets snapDue; the next
// Draw paints the screen as played (popups, not the quit prompt nor the
// toast) and snap writes it to shotsDir.

// shotsDir is where the screenshots go: shots/ beside the quick save.
func (a *app) shotsDir() string { return filepath.Join(filepath.Dir(a.savePath), "shots") }

// snap writes aiscendancy-<date>-<time>[-day N].png, toasting the path
// or the error.
func (a *app) snap() {
	name := "aiscendancy-" + time.Now().Format("20060102-150405")
	if a.inGame() {
		name += fmt.Sprintf("-day%d", a.e.Day)
	}
	path := filepath.Join(a.shotsDir(), name+".png")
	img := image.NewRGBA(a.frame.Bounds())
	a.frame.ReadPixels(img.Pix) // premultiplied, but the frame is opaque
	path, err := writeSnap(path, img)
	if err != nil {
		a.toastFor("screenshot: "+err.Error(), true)
		return
	}
	a.toastFor("screenshot "+path, false)
}

// writeSnap encodes img to path (its directory made) and returns the
// path written: an existing file is kept, the new one taking "-2"...
func writeSnap(path string, img image.Image) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for n := 1; ; n++ {
		p := path
		if n > 1 {
			p = fmt.Sprintf("%s-%d%s", base, n, ext)
		}
		out, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if err := png.Encode(out, img); err != nil {
			out.Close()
			return "", err
		}
		return p, out.Close()
	}
}
