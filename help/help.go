// Package help reads the original's help paragraphs for the planetary
// items, planhelp.txt, and for the ship parts, gizhelp.txt (both at
// ascend00.cob's root), and the end texts' descriptions (ending.go).
//
// A file is a // comment header, then one entry per thing: a #Name line
// (spaces as ^), a # stats line, a "help <word>NN" line, NN two digits
// (planhelp's "help plitemNN" is game.Item NN, gizhelp's "help gizmoNN"
// game.Gizmo NN), then text ... endtext blocks among pos, shape, button,
// title and msg lines (ignored here). planhelp has two blocks, the first
// just %s (the name, the original's dialog title), the second the
// paragraph; gizhelp one, the paragraph with %s where the name goes. An
// entry's paragraph is its LAST block: its lines trimmed, the blank ones
// dropped, joined by one space (a double space inside a line is kept).
// Entries are keyed on the help line alone, never by counting # lines:
// gizhelp holds a stray "#Passive. always in effect, takes no power."
// between entries. "help end" closes the file, which is CRLF with a
// trailing DOS ^Z.
package help

import (
	"fmt"
	"strconv"
	"strings"
)

// Parse reads one help file: the paragraphs by NN, the slice as long as
// the highest NN+1, "" for an entry without text. A help line whose word
// does not end in exactly two digits ("help end") ends the entry before
// it and starts none. It fails on a text block without its endtext.
func Parse(b []byte) ([]string, error) {
	var out []string
	cur := -1   // the entry the blocks belong to, -1 none
	start := -1 // the open text block's line number, -1 none
	var blk []string
	for n, l := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(strings.Trim(l, "\r\x1a"))
		switch {
		case start >= 0 && t == "endtext":
			if cur >= 0 {
				out[cur] = strings.Join(blk, " ")
			}
			start, blk = -1, nil
		case start >= 0:
			if t != "" {
				blk = append(blk, t)
			}
		case t == "text":
			start = n + 1
		case strings.HasPrefix(t, "help ") || t == "help":
			cur = index(strings.TrimSpace(strings.TrimPrefix(t, "help")))
			for cur >= len(out) {
				out = append(out, "")
			}
		}
	}
	if start >= 0 {
		return nil, fmt.Errorf("help: line %d: text without endtext", start)
	}
	return out, nil
}

// index is NN of a help line's word ("plitem07" 7), -1 unless the word
// ends in exactly two digits after a non-digit.
func index(w string) int {
	if len(w) < 3 {
		return -1
	}
	d, before := w[len(w)-2:], w[len(w)-3]
	if before >= '0' && before <= '9' {
		return -1
	}
	n, err := strconv.Atoi(d)
	if err != nil || d[0] < '0' || d[0] > '9' {
		return -1
	}
	return n
}

// Fill is paragraph p with every %s replaced by name.
func Fill(p, name string) string { return strings.ReplaceAll(p, "%s", name) }
