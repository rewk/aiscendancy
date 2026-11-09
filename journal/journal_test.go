package journal

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// Entries round-trip through the lines; each line ends with "\n".
func TestRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	w := New(&buf)
	a := NewEntry(3, 1, "start")
	a.Colony, a.Item, a.Cell = "Orfa Prime", "Factory", "(2,3)"
	b := NewEntry(4, 0, "answer")
	b.Other, b.Kind, b.Accept = 2, "peace", true
	w.Record(a)
	w.Record(b)
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Fatalf("no trailing newline: %q", buf.String())
	}
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("%d lines, want 2: %q", len(lines), buf.String())
	}
	for i, want := range []Entry{a, b} {
		var got Entry
		if err := json.Unmarshal([]byte(lines[i]), &got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("line %d: %+v, want %+v", i, got, want)
		}
	}
}

// Empty strings and false bools are omitted; the indices are always
// written, −1 included.
func TestOmitted(t *testing.T) {
	var buf bytes.Buffer
	New(&buf).Record(NewEntry(0, 0, "study"))
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"colony", "ship", "item", "tech", "design", "kind", "cell", "target", "accept", "on"} {
		if _, ok := m[k]; ok {
			t.Errorf("%q written: %s", k, buf.String())
		}
	}
	for _, k := range []string{"day", "empire", "act"} {
		if _, ok := m[k]; !ok {
			t.Errorf("%q missing: %s", k, buf.String())
		}
	}
	for _, k := range []string{"star", "planet", "slot", "part", "other"} {
		if v, ok := m[k]; !ok || v != -1.0 {
			t.Errorf("%q = %v, want -1: %s", k, v, buf.String())
		}
	}
}

// A nil Writer records nothing and does not panic.
func TestNil(t *testing.T) {
	var w *Writer
	w.Record(NewEntry(1, 0, "war"))
}

type failing struct{ n int }

func (f *failing) Write(p []byte) (int, error) { f.n++; return 0, errors.New("disk full") }

// A failing writer never panics; Record keeps trying (the error logged
// once).
func TestFailing(t *testing.T) {
	f := &failing{}
	w := New(f)
	w.Record(NewEntry(1, 0, "war"))
	w.Record(NewEntry(2, 0, "war"))
	if f.n != 2 || !w.failed {
		t.Errorf("writes %d failed %v, want 2 true", f.n, w.failed)
	}
}
