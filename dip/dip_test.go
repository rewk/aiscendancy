package dip

import (
	"fmt"
	"testing"

	"github.com/rewk/aiscendancy/cob"
)

// sample is a minimal file in the original's shape: CRLF, a header
// naming ACTIONS in prose, DIPVALUES, a key with a trailing comma, a
// two-line text, a whitespace-only separator, END DIPLOMACY then junk.
const sample = "Diplomacy text for the test.\r\n" +
	"Everything above the ACTIONS line is comments.\r\n" +
	"\r\n" +
	"DIPVALUES\r\n" +
	"DECLARE_WAR        70\r\n" +
	"\r\n" +
	"ACTIONS\r\n" +
	"\r\n" +
	"DA_Discover\r\n" +
	"The %m greet the %y.\r\n" +
	"\r\n" +
	"DA_BreakAlliance,\r\n" +
	"Our alliance\r\n" +
	"is over.\r\n" +
	"  \r\n" +
	"DA_OfferPeaceToOtherRace\r\n" +
	"Offer peace to the %t, near %s.\r\n" +
	"\r\n" +
	"RESPONSES\r\n" +
	"DR_ACCEPT_OfferPeace,\r\n" +
	"Yes, %y.\n" + // an LF among the CRLFs
	"\r\n" +
	"DR_NoDipResponse\r\n" +
	"Hm.\r\n" +
	"END DIPLOMACY\r\n" +
	"stray text\r\n"

func TestParseSample(t *testing.T) {
	s, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Actions) != 3 || len(s.Responses) != 2 {
		t.Fatalf("actions %q, responses %q", s.Actions, s.Responses)
	}
	for _, w := range []struct {
		resp     bool
		key, out string
	}{
		{false, "Discover", "The Orfa greet the Mebes."},
		{false, "BreakAlliance", "Our alliance is over."},
		{false, "OfferPeaceToOtherRace", "Offer peace to the Govorom, near Sol."},
		{true, "ACCEPT_OfferPeace", "Yes, Mebes."},
		{true, "NoDipResponse", "Hm."},
	} {
		say := s.Say
		if w.resp {
			say = s.Reply
		}
		if got, ok := say(w.key, "Mebes", "Orfa", "Govorom", "Sol"); !ok || got != w.out {
			t.Errorf("%s: %q %v, want %q", w.key, got, ok, w.out)
		}
	}
	if _, ok := s.Say("ACCEPT_OfferPeace", "", "", "", ""); ok {
		t.Error("a response found among the actions")
	}
	if _, ok := s.Reply("Missing", "", "", "", ""); ok {
		t.Error("a missing key found")
	}
	var none *Script
	if _, ok := none.Say("Discover", "", "", "", ""); ok {
		t.Error("a nil script says something")
	}
}

func TestParseErrors(t *testing.T) {
	if _, err := Parse([]byte("only a header\nDIPVALUES\n")); err == nil {
		t.Error("no ACTIONS accepted")
	}
	if _, err := Parse([]byte("ACTIONS\n\norphan text\n")); err == nil {
		t.Error("text outside a key accepted")
	}
	// No END DIPLOMACY: the last text is kept.
	s, err := Parse([]byte("ACTIONS\nDA_DeclareWar\nWar."))
	if err != nil || s.Actions["DeclareWar"] != "War." {
		t.Errorf("%v %q", err, s.Actions)
	}
}

// The game's files: player.dip and the 21 race files parse and hold the
// keys the negotiation uses.
func TestGameScripts(t *testing.T) {
	g, err := cob.OpenGame(cob.DefaultDir())
	if err != nil {
		t.Skipf("no game: %v", err)
	}
	defer g.Close()
	actions := []string{"Discover", "DeclareWar", "OfferPeace", "ProposeAlliance", "BreakAlliance",
		"TradeStarlanes", "TradeStars", "TradeResearch", "OfferPeaceToOtherRace", "DeclareWarOnOtherRace",
		"BreakAllianceWithOtherRace", "ProposeAllianceToOtherRace", "NoDipAction"}
	var responses []string
	for _, k := range []string{"Alliance", "TradeStarlanes", "TradeStars", "TradeResearch", "OfferPeace",
		"OfferPeaceToOtherRace", "DeclareWarOnOtherRace", "BreakAllianceWithOtherRace", "ProposeAllianceToOtherRace"} {
		responses = append(responses, "ACCEPT_"+k, "REJECT_"+k)
	}
	responses = append(responses, "RESPONSE_Discover", "RESPONSE_DeclareWar", "RESPONSE_BreakAlliance", "NoDipResponse")
	names := []string{"player.dip"}
	for i := range 21 {
		names = append(names, fmt.Sprintf("race%02d.dip", i))
	}
	for _, n := range names {
		b, err := g.Read(n)
		if err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		s, err := Parse(b)
		if err != nil {
			t.Errorf("%s: %v", n, err)
			continue
		}
		for _, k := range actions {
			if _, ok := s.Say(k, "y", "m", "t", "s"); !ok {
				t.Errorf("%s: no DA_%s", n, k)
			}
		}
		for _, k := range responses {
			if _, ok := s.Reply(k, "y", "m", "t", "s"); !ok {
				t.Errorf("%s: no DR_%s", n, k)
			}
		}
	}
}
