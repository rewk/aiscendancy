// Package cob reads Ascendancy's COB archives (ascend00.cob, ascend01.cob,
// ascend02.cob).
//
// The format is a flat table of contents followed by the raw files:
//
//	u32      count
//	count ×  50-byte NUL-padded path (DOS style, "data\foo.shp")
//	count ×  u32 offset from the start of the archive
//	files    each one runs to the next offset, the last to the end
//
// There is no size field and no compression: a file's size is the gap to the
// next offset. Names are matched case-insensitively with either slash.
package cob

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const nameLen = 50

// Entry is one file of an archive.
type Entry struct {
	Name   string // normalised: lower case, forward slashes
	Offset int64
	Size   int64
}

// Archive is one open .cob file.
type Archive struct {
	Path    string
	f       *os.File
	entries []Entry
	index   map[string]int
}

// Open reads the table of contents of one archive and keeps the file open.
func Open(path string) (*Archive, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	a, err := parse(f, path)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return a, nil
}

func parse(f *os.File, path string) (*Archive, error) {
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	var hdr [4]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return nil, err
	}
	count := int(binary.LittleEndian.Uint32(hdr[:]))
	if count <= 0 || int64(count)*(nameLen+4) > st.Size() {
		return nil, fmt.Errorf("implausible file count %d", count)
	}
	names := make([]byte, count*nameLen)
	if _, err := io.ReadFull(f, names); err != nil {
		return nil, err
	}
	offs := make([]byte, count*4)
	if _, err := io.ReadFull(f, offs); err != nil {
		return nil, err
	}
	a := &Archive{Path: path, f: f, index: make(map[string]int, count)}
	for i := 0; i < count; i++ {
		raw := names[i*nameLen : (i+1)*nameLen]
		if k := strings.IndexByte(string(raw), 0); k >= 0 {
			raw = raw[:k]
		}
		e := Entry{Name: Normalise(string(raw)), Offset: int64(binary.LittleEndian.Uint32(offs[i*4:]))}
		a.entries = append(a.entries, e)
	}
	for i := range a.entries {
		end := st.Size()
		if i+1 < count {
			end = a.entries[i+1].Offset
		}
		if end < a.entries[i].Offset || a.entries[i].Offset > st.Size() {
			return nil, fmt.Errorf("entry %q: offsets out of order", a.entries[i].Name)
		}
		a.entries[i].Size = end - a.entries[i].Offset
		a.index[a.entries[i].Name] = i
	}
	return a, nil
}

// Normalise turns a DOS path into the key used for lookups.
func Normalise(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, `\`, "/"))
}

// Close releases the file.
func (a *Archive) Close() error { return a.f.Close() }

// Entries lists the files in archive order.
func (a *Archive) Entries() []Entry { return a.entries }

// Lookup finds an entry by name.
func (a *Archive) Lookup(name string) (Entry, bool) {
	i, ok := a.index[Normalise(name)]
	if !ok {
		return Entry{}, false
	}
	return a.entries[i], true
}

// Read returns the whole content of one file.
func (a *Archive) Read(name string) ([]byte, error) {
	e, ok := a.Lookup(name)
	if !ok {
		return nil, fmt.Errorf("%s: no %q in archive", a.Path, name)
	}
	b := make([]byte, e.Size)
	if _, err := a.f.ReadAt(b, e.Offset); err != nil {
		return nil, fmt.Errorf("%s: %q: %w", a.Path, name, err)
	}
	return b, nil
}

// Game is the set of archives of one installed game, searched in order.
type Game struct {
	Dir      string
	Archives []*Archive
}

// DefaultDir is where the game is looked for when nothing else is given:
// $ASCEND_DIR, else ~/dosbox/Ascend.
func DefaultDir() string {
	if d := os.Getenv("ASCEND_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "dosbox", "Ascend")
}

// OpenGame opens every ascend*.cob of a game directory.
func OpenGame(dir string) (*Game, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "ascend*.cob"))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		// DOS installs may carry upper-case names.
		paths, _ = filepath.Glob(filepath.Join(dir, "ASCEND*.COB"))
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no ascend*.cob in %s", dir)
	}
	sort.Strings(paths)
	g := &Game{Dir: dir}
	for _, p := range paths {
		a, err := Open(p)
		if err != nil {
			g.Close()
			return nil, err
		}
		g.Archives = append(g.Archives, a)
	}
	return g, nil
}

// Close releases every archive.
func (g *Game) Close() {
	for _, a := range g.Archives {
		a.Close()
	}
}

// Read returns one file, from the first archive that holds it.
func (g *Game) Read(name string) ([]byte, error) {
	for _, a := range g.Archives {
		if _, ok := a.Lookup(name); ok {
			return a.Read(name)
		}
	}
	return nil, fmt.Errorf("no %q in %s", name, g.Dir)
}

// Names lists every file of every archive, in archive order, with the
// archive's path alongside. A suffix (".shp") filters the list; "" keeps all.
func (g *Game) Names(suffix string) []string {
	var out []string
	for _, a := range g.Archives {
		for _, e := range a.entries {
			if strings.HasSuffix(e.Name, suffix) {
				out = append(out, e.Name)
			}
		}
	}
	return out
}
