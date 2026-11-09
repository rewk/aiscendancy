package save

import "testing"

func TestDefaultPath(t *testing.T) {
	t.Setenv("LOCALAPPDATA", `C:\L`)
	if p := DefaultPath(); p != `C:\L\aiscendancy\quick.json.gz` {
		t.Errorf("LOCALAPPDATA: %s", p)
	}
	t.Setenv("LOCALAPPDATA", "")
	if p := DefaultPath(); p != "quick.json.gz" {
		t.Errorf("unset: %s", p)
	}
}
