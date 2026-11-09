package sound

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/rewk/aiscendancy/cob"
)

// Library is music.txt's tunes in an open game, by name ("theme00").
type Library struct {
	g     *cob.Game
	names []string          // music.txt's order
	paths map[string]string // name -> normalised path, "data/theme00.raw"
}

// OpenLibrary reads music.txt from g, which stays the caller's to close.
func OpenLibrary(g *cob.Game) (*Library, error) {
	b, err := g.Read("music.txt")
	if err != nil {
		return nil, err
	}
	paths, err := parseMusic(string(b))
	if err != nil {
		return nil, err
	}
	l := &Library{g: g, paths: map[string]string{}}
	for _, p := range paths {
		n := strings.TrimSuffix(path.Base(p), path.Ext(p))
		l.names = append(l.names, n)
		l.paths[n] = p
	}
	return l, nil
}

// parseMusic reads music.txt: a count line, then that many DOS paths,
// returned normalised. Lines are CRLF and may end on a ^Z; blank lines
// after the list are ignored.
func parseMusic(s string) ([]string, error) {
	var lines []string
	for l := range strings.SplitSeq(s, "\n") {
		if l = strings.Trim(l, "\r\x1a \t"); l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("music.txt: empty")
	}
	n, err := strconv.Atoi(lines[0])
	if err != nil {
		return nil, fmt.Errorf("music.txt: count %q: %w", lines[0], err)
	}
	if n != len(lines)-1 {
		return nil, fmt.Errorf("music.txt: count %d, %d paths", n, len(lines)-1)
	}
	out := make([]string, n)
	for i, l := range lines[1:] {
		out[i] = cob.Normalise(l)
	}
	return out, nil
}

// Names lists the tunes in music.txt's order.
func (l *Library) Names() []string { return l.names }

// Track reads tune name's raw bytes and indexes them (theme00: 8 MB,
// a few ms).
func (l *Library) Track(name string) (*Track, error) {
	p, ok := l.paths[name]
	if !ok {
		return nil, fmt.Errorf("music.txt lists no %q", name)
	}
	b, err := l.g.Read(p)
	if err != nil {
		return nil, err
	}
	return NewTrack(b), nil
}
