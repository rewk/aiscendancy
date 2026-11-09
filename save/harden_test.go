package save

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/game"
)

// hardGame is a small save holding a bit of everything Read checks: two
// empires on a Vacuous cluster, fixture ships, bots, 30 days played and
// logged; written to dir/h.json.gz, returned with its JSON.
func hardGame(t *testing.T, dir string) (string, []byte) {
	t.Helper()
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Vacuous)
	if err := game.SpawnFixtures(g, 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	bots := bot.NewAll(g, bot.Default())
	var log []Entry
	play(g, bots, 30, &log)
	if len(log) == 0 || len(g.Empires[0].Ships) == 0 {
		t.Fatalf("fixture: %d log entries, %d ships", len(log), len(g.Empires[0].Ships))
	}
	p := filepath.Join(dir, "h.json.gz")
	if err := Write(p, saveAll(g, bots, log)); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(p); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return p, plain(t, p)
}

// gz is b gzipped, one member.
func gz(t *testing.T, b []byte) []byte {
	t.Helper()
	var z bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&z, gzip.BestSpeed)
	if _, err := zw.Write(b); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return z.Bytes()
}

// A gzip bomb (a small file inflating past maxJSON) is "too large" to
// Read and to Peek, after maxJSON bytes read at most.
func TestReadBomb(t *testing.T) {
	head := fmt.Appendf(nil, `{"Version":%d,"Summary":`, Version)
	b := append(head, bytes.Repeat([]byte{' '}, maxJSON+(6<<20))...)
	b = append(b, `null}`...)
	p := filepath.Join(t.TempDir(), "bomb.json.gz")
	z := gz(t, b)
	if len(z) > 1<<20 {
		t.Fatalf("the bomb is %d bytes on disk", len(z))
	}
	if err := os.WriteFile(p, z, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(p); !errors.Is(err, errTooLarge) {
		t.Errorf("Read: %v, want too large", err)
	}
	if _, _, err := Peek(p); !errors.Is(err, errTooLarge) {
		t.Errorf("Peek: %v, want too large", err)
	}
}

// capped tells a stream ending at its limit (read whole) from one past it.
func TestCapped(t *testing.T) {
	for _, tc := range []struct {
		n    int
		want error
	}{{10, nil}, {11, errTooLarge}, {3, nil}} {
		c := &capped{r: bytes.NewReader(make([]byte, tc.n)), left: 10}
		b, err := readAll(c)
		if !errors.Is(err, tc.want) || err == nil && len(b) != tc.n {
			t.Errorf("%d bytes at limit 10: %d read, %v", tc.n, len(b), err)
		}
	}
}

// readAll is io.ReadAll that returns a non-EOF error as is.
func readAll(c *capped) ([]byte, error) {
	var out []byte
	buf := make([]byte, 4)
	for {
		n, err := c.Read(buf)
		out = append(out, buf[:n]...)
		if err != nil {
			if err == io.EOF {
				return out, nil
			}
			return out, err
		}
	}
}

// A file over maxFile is refused before any read, as is a directory.
func TestReadOverFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "big.json.gz")
	fh, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := fh.Truncate(maxFile + 1); err != nil { // sparse: no disk spent
		t.Fatal(err)
	}
	fh.Close()
	if _, err := Read(p); !errors.Is(err, errTooLarge) {
		t.Errorf("Read: %v, want too large", err)
	}
	if _, _, err := Peek(p); !errors.Is(err, errTooLarge) {
		t.Errorf("Peek: %v, want too large", err)
	}
	if _, err := Read(dir); err == nil {
		t.Error("a directory read")
	}
}

// The stream's end is checked: a CRC flipped in the gzip trailer (the
// JSON intact) is gzip.ErrChecksum to Read, Peek still reading the head;
// a cut file, junk after the object, a second object, a second gzip
// member, bytes after the member are all refused.
func TestReadStreamEnd(t *testing.T) {
	dir := t.TempDir()
	p, b := hardGame(t, dir)
	z, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	write := func(name string, data []byte) string {
		q := filepath.Join(dir, name)
		if err := os.WriteFile(q, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return q
	}
	crc := bytes.Clone(z)
	crc[len(crc)-8] ^= 0xff // the trailer: CRC-32 then ISIZE, little-endian
	q := write("crc.json.gz", crc)
	if _, err := Read(q); !errors.Is(err, gzip.ErrChecksum) {
		t.Errorf("CRC flipped: %v, want %v", err, gzip.ErrChecksum)
	}
	if v, s, err := Peek(q); err != nil || v != Version || s == nil {
		t.Errorf("CRC flipped, Peek: %d %v %v", v, s, err)
	}
	size := bytes.Clone(z)
	size[len(size)-1] ^= 0x01 // ISIZE
	for name, data := range map[string][]byte{
		"size":          size,
		"cut":           z[:len(z)/2],
		"trailer cut":   z[:len(z)-4],
		"junk":          gz(t, append(bytes.Clone(b), "xyz"...)),
		"second object": gz(t, append(bytes.Clone(b), "{}"...)),
		"brace":         gz(t, append(bytes.Clone(b), "}"...)),
		"second member": append(bytes.Clone(z), gz(t, []byte("{}"))...),
		"bytes after":   append(bytes.Clone(z), "junk"...),
		"a zero after":  append(bytes.Clone(z), 0),
	} {
		if _, err := Read(write("x.json.gz", data)); err == nil {
			t.Errorf("%s: read", name)
		}
	}
	// The intact file, rewritten through the same path, still reads.
	if _, err := Read(write("x.json.gz", z)); err != nil {
		t.Errorf("intact: %v", err)
	}
}

// tamper is b (a save's JSON) with f applied to its decoded tree.
func tamper(t *testing.T, b []byte, f func(m map[string]any)) []byte {
	t.Helper()
	var m map[string]any
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := d.Decode(&m); err != nil {
		t.Fatal(err)
	}
	f(m)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// at walks a decoded tree by keys and indices ("Game.Empires.0.Ships").
func at(m map[string]any, path string) any {
	var v any = m
	for _, k := range strings.Split(path, ".") {
		switch x := v.(type) {
		case map[string]any:
			v = x[k]
		case []any:
			var i int
			fmt.Sscan(k, &i)
			v = x[i]
		}
	}
	return v
}

// set sets path's last key or index to val.
func set(m map[string]any, path string, val any) {
	i := strings.LastIndex(path, ".")
	parent, k := at(m, path[:i]), path[i+1:]
	switch x := parent.(type) {
	case map[string]any:
		x[k] = val
	case []any:
		var n int
		fmt.Sscan(k, &n)
		x[n] = val
	}
}

// A real save with one field tampered is refused by Read, never a panic:
// a case per family Read checks (the Game's Validate through its JSON,
// the bots' State, the log, the summary).
func TestReadTampered(t *testing.T) {
	dir := t.TempDir()
	_, b := hardGame(t, dir)
	for _, tc := range []struct {
		path string
		val  any
		want string
	}{
		{"Game.Empires.0.Colonies.0", nil, "colony 0: missing"},  // Relink would dereference it
		{"Game.Empires.0.Colonies.0.Star", -1, "colony 0: star"}, // Relink's own check, now Validate's
		{"Game.Cluster.Stars.1.System.Planets.0.Size", 7, "size"},
		{"Game.Cluster.Stars.1.System.Planets.0.Type", 200, "type"},
		{"Game.Cluster.Lanes.0.B", 999, "lane 0"},
		{"Game.Cluster.Home", -3, "home star"},
		{"Game.Empires.1.Index", 5, "index"},
		{"Game.Empires.0.Research.Topic", 100, "topic"},
		{"Game.Empires.0.Relations.1.Stance", 9, "stance"},
		{"Game.Empires.0.Ships.0.Star", 99, "ship 0: star"},
		{"Game.Empires.0.Ships.0.Design.Parts", "yA==", "part 0: gizmo 200"}, // []Gizmo is a []byte: base64 of {200}
		{"Game.Empires.0.Ships.0.Shots", []any{}, "shots for"},
		{"Game.Empires.0.Ships.0.Order.Kind", 42, "order"},
		{"Game.Empires.0.Ships.0.Cargo", -5, "cargo"},
		{"Game.Offers", []any{map[string]any{"From": 0, "To": 7, "Kind": 0, "Day": 1, "Third": -1}}, "offer"},
		{"Bots.0.Dest", []any{999}, "bot 0: ship 0: destination"},
		{"Bots.0.Cfg.Script.0", 100, "bot 0: script 0"},
		{"Bots.0.Queue", []any{66}, "bot 0: queue 0"},
		{"Bots.1.Next", -1, "bot 1: script step"},
		{"Log.0.Event.Kind", 250, "log entry 0"},
		{"Summary.Species", 200, "summary"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			p := filepath.Join(dir, "t.json.gz")
			writePlain(t, p, tamper(t, b, func(m map[string]any) { set(m, tc.path, tc.val) }))
			_, err := Read(p)
			switch {
			case err == nil:
				t.Fatal("read")
			case errors.Is(err, errPanic):
				t.Fatalf("a panic: %v", err)
			case !strings.Contains(err.Error(), tc.want):
				t.Errorf("%v: want %q in it", err, tc.want)
			}
		})
	}
	// Peek checks the summary too: ours refused, another version's dropped.
	// Edited in the bytes: a re-marshalled map would sort Bots before
	// Version, past Peek's head.
	p := filepath.Join(dir, "s.json.gz")
	sum := regexp.MustCompile(`"Summary":\{"Species":\d+`)
	if !sum.Match(b) {
		t.Fatal("no summary species in the JSON")
	}
	writePlain(t, p, sum.ReplaceAll(b, []byte(`"Summary":{"Species":200`)))
	if _, _, err := Peek(p); err == nil {
		t.Error("Peek took a summary of species 200")
	}
	old := sum.ReplaceAll(b, []byte(`"Summary":{"Species":200`))
	old = bytes.Replace(old, fmt.Appendf(nil, `"Version":%d,`, Version), fmt.Appendf(nil, `"Version":%d,`, Version-1), 1)
	writePlain(t, p, old)
	if v, s, err := Peek(p); err != nil || v != Version-1 || s != nil {
		t.Errorf("an old version's bad summary: %d %v %v", v, s, err)
	}
}

// Random byte and digit changes to a small save's JSON never panic Read
// nor Peek (no error from Read's recover), and a file Read still takes
// plays on: its bots resumed for 3 days, no panic. Fixed seed: the same
// 400 mutants every run.
func TestReadMutants(t *testing.T) {
	dir := t.TempDir()
	_, b := hardGame(t, dir)
	r := rand.New(rand.NewPCG(1, 2))
	var digits []int
	for i, c := range b {
		if c >= '0' && c <= '9' {
			digits = append(digits, i)
		}
	}
	p := filepath.Join(dir, "m.json.gz")
	read := 0
	for k := range 400 {
		m := bytes.Clone(b)
		var what []string
		for range 1 + r.IntN(3) {
			if r.IntN(4) == 0 {
				i := r.IntN(len(m))
				m[i] = byte(r.IntN(256))
				what = append(what, fmt.Sprintf("byte %d=%#x", i, m[i]))
			} else { // a number changed: indices and counts out of range
				i := digits[r.IntN(len(digits))]
				m[i] = "0123456789-9"[r.IntN(12)]
				what = append(what, fmt.Sprintf("digit %d=%c", i, m[i]))
			}
		}
		writePlain(t, p, m)
		_, _, _ = Peek(p)
		f, err := Read(p)
		if errors.Is(err, errPanic) {
			t.Fatalf("mutant %d (%s): %v", k, strings.Join(what, ", "), err)
		}
		if err != nil {
			continue
		}
		read++
		func() {
			defer func() {
				if x := recover(); x != nil {
					t.Fatalf("mutant %d (%s) read, then panicked in play: %v", k, strings.Join(what, ", "), x)
				}
			}()
			bots := make([]*bot.Bot, len(f.Game.Empires)) // nil: no bot
			for i, e := range f.Game.Empires {
				if i < len(f.Bots) && f.Bots[i] != nil {
					bots[i] = bot.Resume(e, f.Bots[i])
					bots[i].G = f.Game
				}
			}
			for range 3 {
				bot.Day(f.Game, bots)
			}
		}()
	}
	if read == 0 || read == 400 {
		t.Errorf("%d of 400 mutants read: the mutation walks nothing", read)
	}
	t.Logf("%d of 400 mutants read and played on", read)
}
