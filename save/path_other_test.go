//go:build !windows

package save

import "testing"

func TestDefaultPath(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/x/data")
	if p := DefaultPath(); p != "/x/data/aiscendancy/quick.json.gz" {
		t.Errorf("XDG: %s", p)
	}
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "/h")
	if p := DefaultPath(); p != "/h/.local/share/aiscendancy/quick.json.gz" {
		t.Errorf("HOME: %s", p)
	}
}
