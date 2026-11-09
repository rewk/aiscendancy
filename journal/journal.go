// Package journal writes the action journal: one JSON object per line
// (JSON Lines) for every decision a player or a bot made and the model
// accepted (game.Game.SetJournal hooks the entry points). It is private
// and optional: not the public log (game.Event), not in the save. A
// refusal writes nothing; the day's automatic resolution writes nothing.
//
// An Entry is flat. Day, Empire and Act are always written; the strings
// and the bools only when set (omitempty); the indices Star, Planet,
// Slot, Part and Other are always written, −1 when they do not apply
// (0 is a real index). NewEntry starts one with every index at −1.
package journal

import (
	"encoding/json"
	"io"
	"log"
	"sync"
)

// Entry is one accepted action (docs/formats.md, "The action journal").
type Entry struct {
	Day    int    `json:"day"`
	Empire int    `json:"empire"` // who acted, its index in Game.Empires
	Act    string `json:"act"`    // "order", "build", "start", "war"...
	Colony string `json:"colony,omitempty"`
	Ship   string `json:"ship,omitempty"`
	Item   string `json:"item,omitempty"`
	Tech   string `json:"tech,omitempty"`
	Design string `json:"design,omitempty"`
	Kind   string `json:"kind,omitempty"` // an order's kind, an offer's kind
	Cell   string `json:"cell,omitempty"` // "(i,j)"
	Target string `json:"target,omitempty"`
	Star   int    `json:"star"`
	Planet int    `json:"planet"`
	Slot   int    `json:"slot"`
	Part   int    `json:"part"`
	Other  int    `json:"other"` // the other empire
	Accept bool   `json:"accept,omitempty"`
	On     bool   `json:"on,omitempty"`
}

// NewEntry is the entry for empire's act on day, every index −1.
func NewEntry(day, empire int, act string) Entry {
	return Entry{Day: day, Empire: empire, Act: act, Star: -1, Planet: -1, Slot: -1, Part: -1, Other: -1}
}

// Writer appends entries to an io.Writer, one line each, under a mutex
// (one Write per entry). A nil *Writer records nothing, so the callers
// never test for one.
type Writer struct {
	mu     sync.Mutex
	w      io.Writer
	failed bool // an error was logged: the later ones are not
}

// New is a Writer over w.
func New(w io.Writer) *Writer { return &Writer{w: w} }

// Record writes e as one JSON line. An error (encoding or writing) is
// logged once, then every error is ignored: the journal never stops a
// game.
func (w *Writer) Record(e Entry) {
	if w == nil {
		return
	}
	b, err := json.Marshal(e)
	w.mu.Lock()
	defer w.mu.Unlock()
	if err == nil {
		_, err = w.w.Write(append(b, '\n'))
	}
	if err != nil && !w.failed {
		w.failed = true
		log.Printf("journal: %v (further errors ignored)", err)
	}
}
