// Package save writes and reads a game in progress: our own JSON (never
// the original's format), gzipped (compress/gzip, default level: a 2 MB
// game to about 90 KB), readable with `zcat x.json.gz | jq .`. Play draws from no PRNG
// (only the cluster's generation does), so the state is plain data: the
// whole Game (the cluster once, every empire), the bots' progress and the
// window's log.
//
// JSON drops two pointers, Empire.Cluster (the Game's) and Colony.Planet
// (the planet is owned by its star's system); Read relinks them
// (Game.Relink) and each bot's Founded map is rebuilt by bot.Resume from
// State.Founded.
package save

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/game"
)

// Version is the format Write writes and Read accepts.
const Version = 62

// File is a saved game: the whole Game (its cluster included, so a save
// is self-contained whatever the generator does later), the bots'
// progress and the window's log.
type File struct {
	Version int // Version (1: a single Empire and Bot)
	// Summary is what a list of saves shows, filled by Write. It follows
	// Version (JSON keeps the fields' order), so Peek reads both off the
	// file's head.
	Summary *Summary
	Seed    uint64 // for the record: the cluster is saved, not regenerated
	Game    *game.Game
	// Bots holds one entry per empire, in Game.Empires order, nil for an
	// empire no bot plays; or none at all.
	Bots []*bot.State
	Log  []Entry
	View View
}

// View is the window's settings that travel with the game: the planets
// list's order (cmd/aiscendancy's sort key index, clamped there).
type View struct {
	PlanetsIdleFirst     bool
	PlanetsProjectsFirst bool
	PlanetsSort          int
}

// LogKeep is how many log entries a game keeps (the window, cmd/mksave):
// the oldest are dropped past it.
const LogKeep = 500

// Entry is a logged event and the day it happened.
type Entry struct {
	Day   int
	Event game.Event
	Idle  bool `json:",omitempty"` // the news left its colony idle
}

// Write saves f to path (Version set, Summary filled from the Game,
// Saved now), its directory created: the JSON goes to path+".tmp", then
// is renamed over path, so a crash never leaves a half-written save.
func Write(path string, f *File) error {
	if f.Game == nil {
		return errors.New("save: no game")
	}
	if len(f.Bots) != 0 && len(f.Bots) != len(f.Game.Empires) {
		return fmt.Errorf("save: %d bots for %d empires", len(f.Bots), len(f.Game.Empires))
	}
	f.Version = Version
	f.Summary = summarise(f.Game, now())
	b, err := json.Marshal(f)
	if err != nil {
		return fmt.Errorf("save: %w", err)
	}
	var z bytes.Buffer
	zw := gzip.NewWriter(&z)
	if _, err := zw.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, z.Bytes(), 0o644); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// The limits a file must keep (Read, Peek): maxFile on disk, maxJSON
// once gunzipped (a day-1451 game is 2.1 MB of JSON), so a gzip bomb
// costs at most maxJSON read and decoded.
const (
	maxFile = 16 << 20
	maxJSON = 64 << 20
)

// errTooLarge is a file or its JSON past maxFile or maxJSON.
var errTooLarge = errors.New("too large")

// errPanic is a panic Read recovered: a check missing (the tests look
// for it).
var errPanic = errors.New("broken save")

// open opens path's single gzip member, a regular file of at most maxFile
// bytes: the stream yields at most maxJSON bytes, errTooLarge past them
// (capped). br is under the gzip reader, which stops at the member's end
// (Multistream false): a byte left in br is data after it.
func open(path string) (fh *os.File, br *bufio.Reader, zr *gzip.Reader, r io.Reader, err error) {
	fh, err = os.Open(path)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	st, err := fh.Stat()
	switch {
	case err != nil:
	case !st.Mode().IsRegular():
		err = fmt.Errorf("%s: not a file", path)
	case st.Size() > maxFile:
		err = fmt.Errorf("%s: %w (%d bytes, at most %d)", path, errTooLarge, st.Size(), maxFile)
	}
	if err != nil {
		fh.Close()
		return nil, nil, nil, nil, err
	}
	br = bufio.NewReader(fh)
	zr, err = gzip.NewReader(br)
	if err != nil {
		fh.Close()
		return nil, nil, nil, nil, fmt.Errorf("%s: not a save: %w", path, err)
	}
	zr.Multistream(false)
	return fh, br, zr, &capped{r: zr, left: maxJSON}, nil
}

// capped reads r up to left bytes; then one byte more is errTooLarge, r's
// end (io.EOF, or its error: gzip's checksum) passes through. Unlike
// io.LimitReader, a stream cut at the limit is told from one past it.
type capped struct {
	r    io.Reader
	left int64
}

func (c *capped) Read(p []byte) (int, error) {
	if c.left <= 0 {
		var b [1]byte
		for {
			n, err := c.r.Read(b[:])
			if n > 0 {
				return 0, fmt.Errorf("JSON %w (over %d bytes)", errTooLarge, int64(maxJSON))
			}
			if err != nil {
				return 0, err
			}
		}
	}
	if int64(len(p)) > c.left {
		p = p[:c.left]
	}
	n, err := c.r.Read(p)
	c.left -= int64(n)
	return n, err
}

// Read loads a save written by Write, checks it and relinks it
// (Game.Validate, Game.Relink): any file, tampered or cut, is an error,
// never a panic nor a Game that panics in play. The object must end the
// stream (the decoder's next token io.EOF: gzip then checks its CRC and
// length), the gzip member the file. Bot i is resumed by the caller with
// bot.Resume(f.Game.Empires[i], f.Bots[i]) when f.Bots[i] is not nil:
// every State is checked against the Game (bot.State.Validate), every log
// entry too (Game.ValidateEvent).
func Read(path string) (_ *File, err error) {
	fh, br, zr, r, err := open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	defer zr.Close()
	// Belt and braces: the checks below are meant to leave nothing that
	// panics; a panic left is an error all the same.
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("%s: %w: %v", path, errPanic, p)
		}
	}()
	var f File
	d := json.NewDecoder(r)
	if err := d.Decode(&f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if _, err := d.Token(); err != io.EOF {
		if err == nil {
			err = errors.New("data after the save")
		}
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if _, err := br.ReadByte(); err != io.EOF {
		return nil, fmt.Errorf("%s: data after the gzip stream", path)
	}
	switch {
	case f.Version != Version:
		return nil, fmt.Errorf("%s: save version %d, want %d", path, f.Version, Version)
	case f.Game == nil || f.Game.Cluster == nil || len(f.Game.Empires) == 0:
		return nil, fmt.Errorf("%s: no game", path)
	case len(f.Bots) != 0 && len(f.Bots) != len(f.Game.Empires):
		return nil, fmt.Errorf("%s: %d bots for %d empires", path, len(f.Bots), len(f.Game.Empires))
	}
	for i, e := range f.Game.Empires {
		if e == nil || e.Research == nil {
			return nil, fmt.Errorf("%s: empire %d is empty", path, i)
		}
	}
	for i, st := range f.Game.Cluster.Stars {
		if st.System == nil {
			return nil, fmt.Errorf("%s: star %d has no system", path, i)
		}
	}
	// Validate before Relink: Relink walks the colonies unchecked (a nil
	// one would panic).
	if err := f.Game.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := f.Game.Relink(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	for i, st := range f.Bots {
		if st == nil {
			continue
		}
		if err := st.Validate(f.Game); err != nil {
			return nil, fmt.Errorf("%s: bot %d: %w", path, i, err)
		}
	}
	for i, en := range f.Log {
		if err := f.Game.ValidateEvent(en.Event); err != nil {
			return nil, fmt.Errorf("%s: log entry %d: %w", path, i, err)
		}
	}
	if f.Summary != nil {
		if err := f.Summary.check(); err != nil {
			return nil, fmt.Errorf("%s: summary: %w", path, err)
		}
	}
	return &f, nil
}

// DefaultPath is dataDir()/aiscendancy/quick.json.gz, else quick.json.gz here.
// dataDir is per OS, picked at build time by the file name's suffix:
// path_windows.go (%LOCALAPPDATA%), path_other.go (XDG, ~/.local/share).
func DefaultPath() string {
	dir := dataDir()
	if dir == "" {
		return "quick.json.gz"
	}
	return filepath.Join(dir, "aiscendancy", "quick.json.gz")
}

// Summary is a save as a list shows it, all about empire 0 (the window's
// player): computed from the Game by Write, never read back into it.
type Summary struct {
	Species  game.Species
	Colour   int // Empire.Colour
	Day      int
	Saved    time.Time // local time at Write
	Ships    int       // the starships
	Colonies int
	Techs    int   // the techs known
	Met      []Met // the empires in contact, in Game.Empires order
}

// Met is another empire the player has met: its stance with it (Peace,
// War or Alliance), Dead once out of the game.
type Met struct {
	Species game.Species
	Colour  int
	Stance  game.Stance
	Dead    bool
}

// now is Summary.Saved's clock, a seam: the tests pin it, so two saves of
// one game are the same bytes.
var now = time.Now

// summarise is g's Summary at now.
func summarise(g *game.Game, now time.Time) *Summary {
	e := g.Empires[0]
	s := &Summary{Species: e.Species, Colour: e.Colour, Day: e.Day, Saved: now,
		Ships: len(e.Ships), Colonies: len(e.Colonies)}
	for _, k := range e.Research.Known {
		if k {
			s.Techs++
		}
	}
	for j, o := range g.Empires {
		if st := e.StanceWith(j); j != 0 && st != game.Unknown {
			s.Met = append(s.Met, Met{Species: o.Species, Colour: o.Colour, Stance: st, Dead: !o.Alive()})
		}
	}
	return s
}

// check refuses a Summary a list cannot show: a species or colour off
// its table, a stance off the enum.
func (s *Summary) check() error {
	if s.Species >= game.NumSpecies || s.Colour < 0 || s.Colour >= game.NumColours {
		return fmt.Errorf("species %d, colour %d", s.Species, s.Colour)
	}
	for i, m := range s.Met {
		if m.Species >= game.NumSpecies || m.Colour < 0 || m.Colour >= game.NumColours || m.Stance > game.Alliance {
			return fmt.Errorf("met %d: species %d, colour %d, stance %d", i, m.Species, m.Colour, m.Stance)
		}
	}
	return nil
}

// Peek reads path's Version and Summary off the file's head, decoding
// nothing else: it walks the top object's keys and stops at the first
// other one (Write puts both first), so a list of saves costs no full
// Read. A file of another version still gives its version, and its
// summary when that decodes and checks (Summary.check); err is for a
// file with no version (broken, not a save) or, at Version, no readable
// summary. The limits are Read's (open), but Peek never reaches the
// stream's end: gzip's CRC is not checked, the data after the head not
// seen.
func Peek(path string) (version int, s *Summary, err error) {
	fh, _, zr, r, err := open(path)
	if err != nil {
		return 0, nil, err
	}
	defer fh.Close()
	defer zr.Close()
	d := json.NewDecoder(r)
	if t, err := d.Token(); err != nil || t != json.Delim('{') {
		return 0, nil, fmt.Errorf("%s: not a save", path)
	}
	haveV, haveS := false, false
	var serr error
	for !(haveV && haveS) {
		t, err := d.Token()
		if err != nil {
			break
		}
		switch t {
		case "Version":
			if err := d.Decode(&version); err != nil {
				return 0, nil, fmt.Errorf("%s: %w", path, err)
			}
			haveV = true
			continue
		case "Summary":
			var sum Summary
			if serr = d.Decode(&sum); serr == nil {
				s = &sum
			}
			haveS = true
			if serr == nil {
				continue
			}
		}
		break
	}
	if s != nil {
		if serr = s.check(); serr != nil {
			s = nil // another version's is dropped, ours an error below
		}
	}
	switch {
	case !haveV:
		return 0, nil, fmt.Errorf("%s: no version", path)
	case version != Version:
		return version, s, nil
	case s == nil && serr != nil:
		return version, nil, fmt.Errorf("%s: %w", path, serr)
	case s == nil:
		return version, nil, fmt.Errorf("%s: no summary", path)
	}
	return version, s, nil
}

// The slots: numbered saves beside the quick save, slotNN.json.gz (two
// digits at least) in its directory.
var slotName = regexp.MustCompile(`^slot(\d+)\.json\.gz$`)

// SlotPath is slot n's file beside the quick save quick.
func SlotPath(quick string, n int) string {
	return filepath.Join(filepath.Dir(quick), fmt.Sprintf("slot%02d.json.gz", n))
}

// Slots lists the numbers of the slot files beside quick, ascending (none
// when the directory is missing): SlotPath(quick, n) is each one's file.
func Slots(quick string) ([]int, error) {
	ents, err := os.ReadDir(filepath.Dir(quick))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ns []int
	for _, de := range ents {
		m := slotName.FindStringSubmatch(de.Name())
		if m == nil || de.IsDir() {
			continue
		}
		// Only SlotPath's spelling: slot1.json.gz would name another file.
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 && filepath.Base(SlotPath(quick, n)) == de.Name() {
			ns = append(ns, n)
		}
	}
	slices.Sort(ns)
	return ns, nil
}

// NextSlot is the number a new slot takes: one past the highest of ns
// (Slots' list), 1 for none, so new saves go to the list's end.
func NextSlot(ns []int) int {
	if len(ns) == 0 {
		return 1
	}
	return slices.Max(ns) + 1
}
