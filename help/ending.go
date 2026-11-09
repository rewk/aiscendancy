package help

import (
	"fmt"
	"strings"

	"github.com/rewk/aiscendancy/cob"
)

// The end texts, end-<name>.txt at ascend00.cob's root (ctrl, capt, dest,
// unit, xtnc: docs/rules.md "Winning"): // comments, then blocks opened
// by a keyword line and closed by endtext (enddesc, the short
// description; congrat, the long one) or by the next blank line (ranks),
// "end" last; CRLF and a trailing DOS ^Z. A block's first line starts
// with underscores or carets (the original's indent: end-unit's are ^).

// ParseEnding is the enddesc block of an end text: its lines between
// "enddesc" and "endtext", trimmed, blank ones dropped, the leading
// underscores of the first stripped. It fails without the block or its
// endtext.
func ParseEnding(b []byte) ([]string, error) {
	var out []string
	in := false
	for _, l := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(strings.Trim(l, "\r\x1a"))
		switch {
		case !in && t == "enddesc":
			in = true
		case in && t == "endtext":
			return out, nil
		case in && t != "":
			if len(out) == 0 {
				t = strings.TrimLeft(t, "_^")
			}
			out = append(out, t)
		}
	}
	if in {
		return nil, fmt.Errorf("help: enddesc without endtext")
	}
	return nil, fmt.Errorf("help: no enddesc")
}

// ParseRanks is the ranks block of an end text: its lines from "ranks"
// to the next blank line, trimmed, lowest first (ten in the game's).
func ParseRanks(b []byte) ([]string, error) {
	var out []string
	in := false
	for _, l := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(strings.Trim(l, "\r\x1a"))
		switch {
		case !in && t == "ranks":
			in = true
		case in && t == "":
			return out, nil
		case in:
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("help: no ranks")
	}
	return out, nil
}

// Ending is ParseEnding of end-<name>.txt in the game at cob.DefaultDir.
func Ending(name string) ([]string, error) {
	g, err := cob.OpenGame(cob.DefaultDir())
	if err != nil {
		return nil, err
	}
	defer g.Close()
	b, err := g.Read("end-" + name + ".txt")
	if err != nil {
		return nil, err
	}
	return ParseEnding(b)
}
