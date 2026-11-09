package main

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/help"
	"github.com/rewk/aiscendancy/ui"
)

// discApp is a two-empire game on the player's starmap, no bots.
func discApp(t *testing.T) *app {
	t.Helper()
	a := testApp(game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average))
	a.push(newStarmapScreen())
	return a
}

// studyDone makes a studiable tech the topic with its Progress at Cost:
// the next close's Advance discovers it.
func studyDone(t *testing.T, a *app) game.Tech {
	t.Helper()
	r := a.e.Research
	for tc := range game.NumTechs {
		if r.CanStudy(tc) {
			if err := a.e.Study(tc); err != nil {
				t.Fatal(err)
			}
			r.Progress[tc] = r.Cost(tc)
			return tc
		}
	}
	t.Fatal("nothing to study")
	return game.NoTech
}

// techs is the queue's techs in order.
func techs(a *app) []game.Tech {
	var out []game.Tech
	for _, d := range a.discoveries {
		out = append(out, d.tech)
	}
	return out
}

// A day that completes the topic queues it, "Our scientists", the
// popup up over the starmap (no log pops: the Discovered has no place,
// the climb ignores it); the next day queues nothing new.
func TestDiscoveryResearch(t *testing.T) {
	a := discApp(t)
	tc := studyDone(t, a)
	a.pressTurn(nextButton)
	if !slices.ContainsFunc(a.log, func(l logEntry) bool { return l.Ev.Kind == game.Discovered && l.Ev.Tech == tc }) {
		t.Fatalf("fixture: no Discovered %s in the log", tc)
	}
	if !slices.Equal(a.discoveries, []discovery{{tc, "Our scientists"}}) {
		t.Fatalf("queue %+v, want %s from our scientists", a.discoveries, tc)
	}
	top[*starmapScreen](t, a)
	if !a.discovering() {
		t.Error("the popup is not up over the starmap")
	}
	a.discoveries = nil
	a.endDay()
	if len(a.discoveries) != 0 {
		t.Errorf("a quiet day queued %+v", a.discoveries)
	}
}

// A completed Xeno dig queues its tech with the dig's line, naming the
// planet.
func TestDiscoveryDig(t *testing.T) {
	a := discApp(t)
	c := a.e.Colonies[0]
	r, p, dig := a.e.Research, c.Planet, game.XenoArcheologicalDig
	r.Grant(game.Tech(dig.Info().ResearchReq))
	c.Pop += 2
	at := game.Cell{I: -1}
	for _, x := range p.Cells() {
		if sq := p.Surface[x]; sq.Item == game.NoItem && p.Reachable(x) {
			sq.Ruins, at = true, x
			break
		}
	}
	if at.I < 0 {
		t.Fatal("fixture: no reachable empty square")
	}
	if err := c.Start(r, dig, at); err != nil {
		t.Fatal(err)
	}
	c.Project.Progress = c.Project.Cost()
	evs := a.endDay()
	i := slices.IndexFunc(evs, func(x game.Event) bool { return x.Kind == game.Discovered && x.Star >= 0 })
	if i < 0 {
		t.Fatalf("fixture: no dig discovery in %+v", evs)
	}
	want := discovery{evs[i].Tech, dig.String() + " on " + p.Name}
	if !slices.Contains(a.discoveries, want) {
		t.Errorf("queue %+v, want %+v", a.discoveries, want)
	}
}

// An exchange of theirs accepted in Talk queues what we learnt,
// "Learnt from the <species>".
func TestDiscoveryTrade(t *testing.T) {
	a, s := speciesApp(t)
	o := a.g.Empires[1]
	o.Research.Grant(0)
	tc := a.g.Gain(0, 1, game.TradeResearch).Tech
	if tc == game.NoTech {
		t.Fatal("fixture: nothing to learn from them")
	}
	if _, err := a.g.OfferTrade(1, 0, game.TradeResearch); err != nil {
		t.Fatal(err)
	}
	rowClick(a, s, 0)
	sayKey(t, a, top[*negotiateScreen](t, a), "ACCEPT_TradeResearch")
	if want := []discovery{{tc, "Learnt from the " + o.Species.String()}}; !slices.Equal(a.discoveries, want) {
		t.Errorf("queue %+v, want %+v", a.discoveries, want)
	}
}

// The Dubtaks' theft from the starmap's ability box queues every stolen
// tech in tech order, the ability's line as their source.
func TestDiscoveryTheft(t *testing.T) {
	a := testApp(game.NewGame(1, []game.Species{game.Dubtaks, game.Orfa, game.Mebes}, game.Average))
	charge(a.g, 0)
	m := newStarmapScreen()
	a.push(m)
	var want []game.Tech
	for _, tc := range []game.Tech{7, 3} {
		if a.e.Research.Known[tc] {
			t.Fatalf("fixture: %s known", tc)
		}
		a.g.Empires[1].Research.Grant(tc)
		a.g.Empires[2].Research.Grant(tc)
		want = append(want, tc)
	}
	slices.Sort(want)
	m.useAbility(a)
	i := slices.IndexFunc(a.log, func(l logEntry) bool { return l.Ev.Kind == game.AbilityUsed })
	if i < 0 {
		t.Fatalf("no AbilityUsed in the log: %+v", a.log)
	}
	if !slices.Equal(techs(a), want) {
		t.Fatalf("queue %+v, want %v", a.discoveries, want)
	}
	for _, d := range a.discoveries {
		if d.from != a.log[i].Ev.Text {
			t.Errorf("%s from %q, want %q", d.tech, d.from, a.log[i].Ev.Text)
		}
	}
}

// learnt queues in tech order, a source per tech from its own event;
// each close pops the head, the next shows, the batch count holds.
func TestDiscoveryQueue(t *testing.T) {
	a := discApp(t)
	before := a.e.Research.Known
	a.e.Research.Grant(9)
	a.e.Research.Grant(2)
	evs := []game.Event{
		{Kind: game.Discovered, Tech: 9, Star: -1, Planet: -1, Other: -1},
		{Kind: game.TradeMade, Tech: game.NoTech, Star: -1, Planet: -1, Other: 1},
	}
	a.learnt(before, evs)
	want := []discovery{{2, "Learnt from the " + a.g.Empires[1].Species.String()}, {9, "Our scientists"}}
	if !slices.Equal(a.discoveries, want) || a.discTotal != 2 {
		t.Fatalf("queue %+v total %d, want %+v", a.discoveries, a.discTotal, want)
	}
	l := discLayout(a, a.discoveries[0].tech)
	a.discoveryPress(false, true, float64(l.box.X0+discInset), float64(l.box.Y0+discInset)) // inside, off OK
	if len(a.discoveries) != 2 {
		t.Fatal("a click inside the box closed it")
	}
	a.discoveryPress(false, true, float64(l.ok.X0+l.ok.X1)/2, float64(l.ok.Y0+l.ok.Y1)/2)
	if !slices.Equal(a.discoveries, want[1:]) || a.discTotal != 2 {
		t.Fatalf("after OK: %+v total %d", a.discoveries, a.discTotal)
	}
	l = discLayout(a, a.discoveries[0].tech)
	a.discoveryPress(false, true, float64(l.box.X0-1), float64(l.box.Y0)) // outside
	if len(a.discoveries) != 0 || a.discovering() {
		t.Fatalf("after a click outside: %+v", a.discoveries)
	}
	// A new batch restarts the count.
	before = a.e.Research.Known
	a.e.Research.Grant(11)
	a.learnt(before, nil)
	if !slices.Equal(a.discoveries, []discovery{{11, "Learnt"}}) || a.discTotal != 1 {
		t.Errorf("next batch %+v total %d", a.discoveries, a.discTotal)
	}
}

// The popup waits under a system screen replaying combat, is up over
// the log, never once the game is over; a closing key pops the head and
// nothing else (no day, no screen popped).
func TestDiscoveryWhen(t *testing.T) {
	a := discApp(t)
	a.discoveries = []discovery{{1, "x"}, {2, "y"}}
	if !a.discovering() {
		t.Fatal("not up over the starmap")
	}
	v := newSystemScreen(a.e.Colonies[0].Star)
	a.push(v)
	v.replay = &replay{}
	if a.discovering() {
		t.Error("up over a replay")
	}
	v.replay = nil
	if !a.discovering() {
		t.Error("not up once the replay ended")
	}
	a.push(newLogScreen())
	if !a.discovering() {
		t.Error("not up over the log")
	}
	day, n := a.e.Day, len(a.stack)
	a.discoveryPress(true, false, 0, 0)
	if a.e.Day != day || a.g.Day != day || len(a.stack) != n || len(a.discoveries) != 1 {
		t.Errorf("a key: day %d -> %d, stack %d -> %d, queue %d", day, a.e.Day, n, len(a.stack), len(a.discoveries))
	}
	a.discoveries = append(a.discoveries, discovery{3, "z"})
	a.e.Lost = a.e.Day + 1 // defeated (game/defeat.go)
	if a.e.Alive() || a.discovering() {
		t.Errorf("alive %v: up once the game is over", a.e.Alive())
	}
}

// F9 (load's restart) and New game drop the queue.
func TestDiscoveryCleared(t *testing.T) {
	a := discApp(t)
	a.discoveries, a.discTotal = []discovery{{1, "x"}}, 1
	a.restart()
	if a.discoveries != nil || a.discTotal != 0 {
		t.Errorf("restart kept %+v", a.discoveries)
	}
	a.discoveries, a.discTotal = []discovery{{1, "x"}}, 1
	if err := a.answerEnd(true); err != nil {
		t.Fatal(err)
	}
	if a.discoveries != nil || a.discTotal != 0 || a.discovering() {
		t.Errorf("New game kept %+v", a.discoveries)
	}
}

// Every tech's rows are all its unlocks in order, each described by its
// own help paragraph (parsed apart from the app), a hull by its slots,
// whole: no tech's help is cut; the box is the prompt's, OK the bar's
// right half, the rows stacked inside the frame above the bar.
func TestDiscoveryRows(t *testing.T) {
	a, items, gizmos := helpApp(t)
	w := discTextW()
	c := discContent()
	for tc := range game.NumTechs {
		us := unlocks(a.e.Research, a.e.Species, tc)
		l := discLayout(a, tc)
		rows := l.rows
		if len(rows) != len(us) {
			t.Errorf("%s: %d rows, %d unlocks", tc, len(rows), len(us))
		}
		if l.box != dlgBox || l.ok != (rect{(dlgBar.X0 + dlgBar.X1) / 2, dlgBar.Y0, dlgBar.X1, dlgBar.Y1}) {
			t.Errorf("%s: box %+v, OK %+v", tc, l.box, l.ok)
		}
		y := c.Y0 + discPicW
		for i, r := range rows {
			if r.unlock != us[i] {
				t.Errorf("%s row %d: %+v, want %+v", tc, i, r.unlock, us[i])
			}
			var text string
			switch r.bank {
			case "planitem":
				text = help.Fill(items[r.frame], game.Item(r.frame).String())
			case gizmoBank:
				text = help.Fill(gizmos[r.frame], game.Gizmo(r.frame).String())
			default:
				text = fmt.Sprintf("%d slots", game.Hull(r.frame).Slots())
			}
			all := discLines(text, w)
			if r.bank == "planitem" && game.Item(r.frame) == game.ColonyBase {
				all = nil // its paragraph is dropped (itemHelp)
			}
			if !slices.Equal(r.lines, all) {
				t.Errorf("%s %s: %q, want %q (cut?)", tc, r.name, r.lines, all)
			}
			for _, l := range r.lines {
				if ui.Width(l, discTextFace) > w {
					t.Errorf("%s %s: %q wider than %.0f", tc, r.name, l, w)
				}
			}
			if r.y < y || r.y+r.h > c.Y1 || r.h < r.iconH || r.iconH > discIconW || r.w > discIconW {
				t.Errorf("%s %s: row [%d,%d) icon %dx%d, after %d, bottom %d", tc, r.name, r.y, r.y+r.h, r.w, r.iconH, y, c.Y1)
			}
			y = r.y + r.h
		}
	}
}

// Research closes the head and opens the research screen once (not over
// itself); its button sits left of OK on the box's bottom, apart; the
// next discovery shows over the research screen.
func TestDiscoveryGoResearch(t *testing.T) {
	a := discApp(t)
	a.discoveries = []discovery{{1, "x"}, {2, "y"}}
	a.discTotal = 2
	l := discLayout(a, 1)
	if l.research.X1 > l.ok.X0 || l.research.Y0 != l.ok.Y0 || l.research.Y1 != l.ok.Y1 || l.research.X0 < l.box.X0 ||
		float64(l.research.X1-l.research.X0) < ui.Width("Research (R)", bigFace) {
		t.Errorf("research %+v, OK %+v, box %+v", l.research, l.ok, l.box)
	}
	n := len(a.stack)
	cx, cy := float64(l.research.X0+l.research.X1)/2, float64(l.research.Y0+l.research.Y1)/2
	if !a.onResearchButton(cx, cy) || a.onResearchButton(float64(l.ok.X0+1), cy) {
		t.Error("the Research button's hit-test")
	}
	a.discoveryResearch()
	top[*researchScreen](t, a)
	if len(a.stack) != n+1 || !slices.Equal(a.discoveries, []discovery{{2, "y"}}) || !a.discovering() {
		t.Fatalf("stack %d (was %d), queue %+v", len(a.stack), n, a.discoveries)
	}
	a.discoveryResearch()
	if len(a.stack) != n+1 || len(a.discoveries) != 0 {
		t.Errorf("over the research screen: stack %d, queue %+v", len(a.stack), a.discoveries)
	}
}

// Two research exchanges in one batch (two bots accepting the player's
// offers the same day): each tech names the empire it came from, not
// the batch's first TradeMade.
func TestDiscoveryTwoTrades(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa, game.Mebes}, game.Average)
	a := testApp(g)
	a.push(newStarmapScreen())
	a.record(g.Contact(0, 1))
	a.record(g.Contact(0, 2))
	var want []discovery
	for _, j := range []int{1, 2} {
		for tc := range game.NumTechs {
			if !a.e.Research.Known[tc] && !slices.ContainsFunc(want, func(d discovery) bool { return d.tech == tc }) {
				g.Empires[j].Research.Grant(tc)
				break
			}
		}
		tc := g.Gain(0, j, game.TradeResearch).Tech
		if tc == game.NoTech {
			t.Fatalf("fixture: nothing to learn from %d", j)
		}
		want = append(want, discovery{tc, "Learnt from the " + g.Empires[j].Species.String()})
		if _, err := g.OfferTrade(0, j, game.TradeResearch); err != nil {
			t.Fatal(err)
		}
	}
	if want[0].tech == want[1].tech {
		t.Fatal("fixture: the same tech from both")
	}
	before := a.e.Research.Known
	var reps []game.Report
	for len(g.Offers) > 0 {
		r, err := g.Answer(g.Offers[0], true)
		if err != nil {
			t.Fatal(err)
		}
		reps = append(reps, r...)
	}
	a.learnt(before, a.record(reps))
	slices.SortFunc(want, func(x, y discovery) int { return int(x.tech) - int(y.tech) })
	if !slices.Equal(a.discoveries, want) {
		t.Errorf("queue %+v, want %+v", a.discoveries, want)
	}
}

// The singularity's story: set only by the batch taking the tree to
// Complete, told by that batch's last popup alone; a batch short of the
// tree, an empty one on a complete tree (a load) and the next queue
// leave it off.
func TestDiscoveryStory(t *testing.T) {
	a := discApp(t)
	r := a.e.Research
	last, prev := game.NumTechs-1, game.NumTechs-2
	for tc := range game.NumTechs - 2 {
		r.Grant(tc)
	}
	before := r.Known
	r.Grant(prev)
	a.learnt(before, nil)
	if a.discStory || a.storyShown() {
		t.Fatal("a batch short of the tree tells the story")
	}
	a.discoveries = nil
	before = r.Known
	r.Grant(last)
	a.learnt(before, nil)
	if !a.discStory || !a.storyShown() || len(a.discoveries) != 1 || a.discoveries[0].tech != last {
		t.Fatalf("the completing batch: story %v shown %v queue %+v", a.discStory, a.storyShown(), a.discoveries)
	}
	if l := discLayout(a, last); l.story == nil {
		t.Error("no story laid out")
	}
	a.discoveries = append([]discovery{{0, "x"}}, a.discoveries...) // not the last yet
	if a.storyShown() {
		t.Error("shown before the batch's last")
	}
	if l := discLayout(a, 0); l.story != nil {
		t.Error("story laid out before the batch's last")
	}

	b := discApp(t)
	for tc := range game.NumTechs {
		b.e.Research.Grant(tc)
	}
	b.learnt(b.e.Research.Known, nil) // a complete save: nothing learnt
	if b.discStory || len(b.discoveries) != 0 {
		t.Errorf("complete before: story %v queue %+v", b.discStory, b.discoveries)
	}
	b.discStory = true // a told story, its queue emptied
	b.learnt(b.e.Research.Known, nil)
	if b.discStory {
		t.Error("an empty queue kept the story")
	}
}

// The story fits uncut under every tech's rows: its lines are the
// paragraph's whole, none wider than the content, below the last row
// and above the button bar.
func TestDiscoveryStoryFits(t *testing.T) {
	a, _, _ := helpApp(t)
	for tc := range game.NumTechs {
		a.e.Research.Grant(tc)
	}
	a.discStory = true
	a.discoveries = []discovery{{0, "x"}}
	c := discContent()
	w := float64(c.X1 - c.X0)
	want := discLines(game.SingularityStory, w)
	for tc := range game.NumTechs {
		l := discLayout(a, tc)
		if !slices.Equal(l.story, want) || strings.Join(strings.Fields(strings.Join(l.story, " ")), " ") != game.SingularityStory {
			t.Fatalf("%s: story %q", tc, l.story)
		}
		for _, s := range l.story {
			if ui.Width(s, discTextFace) > w {
				t.Errorf("%q wider than %.0f", s, w)
			}
		}
		if bottom := l.storyY + int(float64(len(l.story))*discTextH); bottom > c.Y1 {
			t.Errorf("%s: story bottom %d past %d", tc, bottom, c.Y1)
		}
		for _, r := range l.rows {
			if r.y+r.h > l.storyY-discGap {
				t.Errorf("%s %s: row bottom %d over the story at %d", tc, r.name, r.y+r.h, l.storyY)
			}
		}
		if len(l.rows) == 0 && l.foot+int(discTextH) > l.storyY {
			t.Errorf("%s: foot %d over the story at %d", tc, l.foot, l.storyY)
		}
	}
}
