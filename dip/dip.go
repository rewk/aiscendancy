// Package dip reads the original's diplomacy scripts, player.dip and
// race00.dip..race20.dip (ascend00.cob, in game.Species order): the
// lines a species says in the negotiation.
//
// A file is a comment header, a DIPVALUES table (ignored here: the
// thresholds are typed into bot.Personalities), then an ACTIONS line
// followed by DA_<Key> lines, a RESPONSES line followed by DR_<Key>
// lines, and END DIPLOMACY. Each key line is followed by its text, one
// or more lines up to the next blank line (whitespace only counts as
// blank), joined by a space. A few keys carry a trailing comma
// (DA_BreakAlliance,), stripped. Lines are CRLF or LF. The texts hold
// placeholders: %y you (the one spoken to), %m me (the speaker), %t
// the third party, %s a star system.
package dip

import (
	"fmt"
	"strings"
)

// Script is one file's texts by bare key: Actions from the DA_ lines
// ("Discover", "DeclareWar", "NoDipAction"), Responses from the DR_
// ones ("RESPONSE_Discover", "ACCEPT_TradeStars", "REJECT_OfferPeace",
// "NoDipResponse").
type Script struct {
	Actions, Responses map[string]string
}

// Parse reads one .dip file. It fails without an ACTIONS line, or on a
// text line outside a key between ACTIONS and END DIPLOMACY.
func Parse(b []byte) (*Script, error) {
	s := &Script{Actions: map[string]string{}, Responses: map[string]string{}}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r", ""), "\n")
	body := false // past ACTIONS
	var into map[string]string
	key := ""
	var text []string
	flush := func() {
		if key != "" {
			into[key] = strings.Join(text, " ")
		}
		key, text = "", nil
	}
	for n, l := range lines {
		t := strings.TrimSpace(l)
		switch {
		case t == "ACTIONS":
			flush()
			body = true
		case !body:
			// the header and DIPVALUES
		case t == "RESPONSES":
			flush()
		case t == "END DIPLOMACY":
			flush()
			return s, nil
		case t == "":
			flush()
		case strings.HasPrefix(t, "DA_"), strings.HasPrefix(t, "DR_"):
			flush()
			into = s.Actions
			if t[1] == 'R' {
				into = s.Responses
			}
			key = strings.TrimSuffix(t[3:], ",")
		case key == "":
			return nil, fmt.Errorf("line %d: text outside a key: %q", n+1, t)
		default:
			text = append(text, t)
		}
	}
	if !body {
		return nil, fmt.Errorf("no ACTIONS")
	}
	flush()
	return s, nil
}

// fill substitutes the placeholders.
func fill(t, you, me, third, system string) string {
	return strings.NewReplacer("%y", you, "%m", me, "%t", third, "%s", system).Replace(t)
}

// Say is the action key's text with the placeholders filled, false when
// s is nil, lacks the key or its text is empty.
func (s *Script) Say(key, you, me, third, system string) (string, bool) {
	if s == nil {
		return "", false
	}
	t := s.Actions[key]
	return fill(t, you, me, third, system), t != ""
}

// Reply is Say over the Responses.
func (s *Script) Reply(key, you, me, third, system string) (string, bool) {
	if s == nil {
		return "", false
	}
	t := s.Responses[key]
	return fill(t, you, me, third, system), t != ""
}
