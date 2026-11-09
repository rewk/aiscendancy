package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The species screen: a row per other empire met, in empire order: their
// portrait, their flag, their public facts on line 1, their pending offer
// to us in words on line 2, our ledger about them under it; a click on a row opens Talk (negotiate.go),
// where every act and answer lives. Nothing the player could not know is
// shown: no attitude, no exchange's gains. The selection (framed: a log
// link's or a contact's empire) is an empire index, rows read from the
// game at each use: a contact made under the screen shifts nothing kept.
type speciesScreen struct {
	off      int // rows scrolled: the top row shows met[off]
	hover    int // row under the cursor, −1 none
	sel      int // the framed empire's index, −1 none (then the first row's)
	tick     int
	msg      string // the last refusal, shown until msgUntil
	msgUntil int
	news     newsView
}

// newSpeciesScreen selects empire sel (−1: the first row).
func newSpeciesScreen(sel int) *speciesScreen {
	return &speciesScreen{hover: -1, sel: sel, news: newNewsView()}
}

func (s *speciesScreen) Enter(*app) {}

const speciesHelp = "click: talk to a species\nEsc: back"

// statsBox is the Stats button (the Stats screen, as key H): stacked box
// 2, as the fleet screen's Designs.
var statsBox = mapStack[2]

// onSpeciesKey is the S key: the species screen pushed, true (the
// caller's Update stops).
func onSpeciesKey(a *app, down bool) bool {
	if !down || !a.inGame() {
		return false
	}
	a.push(newSpeciesScreen(-1))
	return true
}

// speciesCols are the columns' widths in bigFace characters, right of the
// portrait; the first holds the flag then the name (the flag in the
// empire's colour says the colour, no column for its name). Our
// friendship toward them (game.Relation.Friendship) is green while it
// bars our war on them (barredDays). The last, their pending
// offer, is no column: line 2, under the name.
var speciesCols = [...]int{17, 9, 8, 11, 10, 10, 0}

var speciesHeads = [len(speciesCols)]string{"species", "stance", "systems", "ships seen", "met", "friendship", "their offer"}

// speciesFriendCol is the friendship's column, speciesOfferCol the
// pending offer's.
const (
	speciesFriendCol = len(speciesCols) - 2
	speciesOfferCol  = len(speciesCols) - 1
)

// defeatedAlpha greys a defeated empire's row: its flag's opacity, its
// text's brightness.
const defeatedAlpha = 0.4

// A row is speciesRowPad, the portrait (smraces, speciesPortrait px
// square, drawn 1:1: at zoom it would not fit six rows), speciesRowPad.
// Right of it the flag (raceflag, flagH original px) at the top, and
// bigFace lines (16 px a 1.25 line) from speciesTextDY: line 1, the
// offer's line 2, the ledger's at most speciesLedgerRows lines; they
// start speciesNameDX right of the flag's left. Native px.
const (
	speciesPortrait   = 100
	speciesPortDX     = speciesPortrait + 6*zoom
	speciesRowPad     = 8 * zoom
	speciesTextDY     = speciesRowPad
	speciesLedgerRows = 3
	speciesRowH       = speciesPortrait + 2*speciesRowPad
	speciesNameDX     = flagW*zoom + 4*zoom
)

// speciesColX is column k's native x.
func speciesColX(k int) float64 {
	n := 0
	for _, w := range speciesCols[:k] {
		n += w + 1
	}
	return float64(mapArea.X0+padX+speciesPortDX) + float64(n)*ui.Width("0", bigFace)
}

// speciesColW is column k's native width, the offer's (line 2) from the
// name to the right margin.
func speciesColW(k int) float64 {
	if k == speciesOfferCol {
		return float64(mapArea.X1-padX) - speciesColX(0) - speciesNameDX
	}
	return float64(speciesCols[k]) * ui.Width("0", bigFace)
}

// speciesMaxRows is the rows shown at most: every other empire of the
// largest game (maxEmpires), never scrolled in practice.
const speciesMaxRows = maxEmpires - 1

// met is the other empires the player has met, in empire order.
func met(a *app) []int {
	var out []int
	for j := range a.g.Empires {
		if j != a.player && a.e.StanceWith(j) != game.Unknown {
			out = append(out, j)
		}
	}
	return out
}

// speciesRowsBox is the rows' box: speciesMaxRows rows under the header.
func speciesRowsBox() rect {
	return rect{mapArea.X0, int(speciesTop()), mapArea.X1, int(speciesTop()) + speciesMaxRows*speciesRowH}
}

// shownRows is how many rows show now.
func (s *speciesScreen) shownRows(a *app) int { return min(len(met(a))-s.off, speciesMaxRows) }

// row is the row under native row y, −1 outside the rows.
func (s *speciesScreen) row(y float64) int {
	r := y - speciesTop()
	if r < 0 || y >= float64(mapArea.Y1) {
		return -1
	}
	if k := int(r / speciesRowH); k < speciesMaxRows {
		return k
	}
	return -1
}

// empire is the empire index row r shows, −1 when none.
func (s *speciesScreen) empire(a *app, r int) int {
	m := met(a)
	if r < 0 || r >= speciesMaxRows || s.off+r >= len(m) {
		return -1
	}
	return m[s.off+r]
}

// scroll moves by d rows, clamped.
func (s *speciesScreen) scroll(a *app, d int) {
	s.off = min(max(s.off+d, 0), max(0, len(met(a))-speciesMaxRows))
}

// selected is the framed empire: sel when met, else the first row's, −1.
func (s *speciesScreen) selected(a *app) int {
	m := met(a)
	for _, j := range m {
		if j == s.sel {
			return j
		}
	}
	if len(m) > 0 {
		return m[0]
	}
	return -1
}

// botOf is empire j's bot, nil none.
func botOf(a *app, j int) *bot.Bot {
	if j < 0 || j >= len(a.bots) {
		return nil
	}
	return a.bots[j]
}

// knownSystems counts the known stars holding a colony of empire j's.
func knownSystems(a *app, j int) int {
	seen := map[int]bool{}
	for _, c := range a.g.Empires[j].Colonies {
		if a.e.Knows(c.Star) {
			seen[c.Star] = true
		}
	}
	return len(seen)
}

// shipsSeen counts empire j's ships the player sees (Game.ShipsSeen).
func shipsSeen(a *app, j int) int {
	n := 0
	for st := range a.cl.Stars {
		for _, sg := range a.g.ShipsSeen(a.e, st) {
			if sg.Empire == j {
				n++
			}
		}
	}
	return n
}

// speciesLine is empire j's line 1: its columns, our friendship rounded,
// the last their oldest pending offer to the player in words ("" none).
func speciesLine(a *app, j int) [len(speciesCols)]string {
	o := a.g.Empires[j]
	r := a.e.Rel(j)
	stance := r.Stance.String()
	if !o.Alive() {
		stance = "defeated"
	}
	offer := ""
	if of, ok := incoming(a, j); ok {
		offer = theirOfferWords(a, of)
	}
	return [len(speciesCols)]string{
		o.Species.String(), stance,
		fmt.Sprint(knownSystems(a, j)), fmt.Sprint(shipsSeen(a, j)), fmt.Sprintf("day %d", r.Met),
		fmt.Sprintf("%.0f", r.Friendship), offer,
	}
}

// offerWords is an offer of kind k in words ("an alliance").
func offerWords(k game.OfferKind) string {
	switch {
	case k == game.AllianceOffer:
		return "an alliance"
	case k.Trade():
		return "an exchange of " + k.String()
	}
	return k.String()
}

// requestWords is request o's act in words ("war on the Mebes").
func requestWords(a *app, o game.Offer) string {
	return [...]string{"peace with the ", "war on the ", "a break with the ", "an alliance with the "}[o.Kind-game.AskPeaceWith] +
		a.g.Empires[o.Third].Species.String()
}

// theirOfferWords is offer o of theirs as a verb phrase ("offers peace",
// "asks war on the Mebes").
func theirOfferWords(a *app, o game.Offer) string {
	if o.Kind.Request() {
		return "asks " + requestWords(a, o)
	}
	return "offers " + offerWords(o.Kind)
}

// barredDays is game.WarBarred for the player on j, 0 at War with j or
// j defeated: no war left to bar.
func barredDays(a *app, j int) int {
	if a.e.StanceWith(j) == game.War || !a.g.Empires[j].Alive() {
		return 0
	}
	return a.g.WarBarred(a.player, j)
}

// ledgerFacts are the player's ledger about j, a short fact each; first,
// while allied with j, its wars and alliances (allyFacts, each fitting
// w native px when w > 0); the hospitality we gave read from their
// ledger; last, while our friendship bars our war on them (barredDays),
// the closes left.
func ledgerFacts(a *app, j int, w float64) []string {
	r := a.e.Rel(j)
	fs := append(allyFacts(a, j, w), fmt.Sprintf("war %d days", r.WarDays), fmt.Sprintf("wars on us %d", r.WarsOn),
		fmt.Sprintf("ships lost %d", r.ShipsLost), fmt.Sprintf("colonies lost %d", r.PlanetsLost),
		fmt.Sprintf("intrusions %d", r.Intrusions), fmt.Sprintf("refused %d", r.Refused),
		fmt.Sprintf("allied %d days", r.AllyDays), fmt.Sprintf("broke %d", r.Broken), fmt.Sprintf("exchanges %d", r.Trades),
		fmt.Sprintf("hospitality in %d / out %d", r.Hospitality, a.g.Empires[j].Rel(a.player).Hospitality),
		fmt.Sprintf("granted %d", r.Helped))
	if n := barredDays(a, j); n > 0 {
		fs = append(fs, fmt.Sprintf("no war for %d day%s", n, plural(n)))
	}
	return fs
}

// allyFacts are, while the player and j are allied (Game.AllyStances),
// j's wars then its alliances: "at war with the Mebes, 1 unmet",
// "allied with none". An empire the player never met is counted, not
// named. When w > 0, each fact names as many as fit in w native px
// (bigFace), the rest counted ("2 more"): ledgerLines never cuts one.
func allyFacts(a *app, j int, w float64) []string {
	wars, allies, ok := a.g.AllyStances(a.player, j)
	if !ok {
		return nil
	}
	return []string{fitList(a, "at war with ", wars, w), fitList(a, "allied with ", allies, w)}
}

// fitList is prefix and empireList of ks naming the most that fit in w
// native px (all when w <= 0).
func fitList(a *app, prefix string, ks []int, w float64) string {
	f := prefix + empireListMax(a, ks, len(ks))
	for n := len(ks) - 1; n >= 0 && w > 0 && ui.Width(f, bigFace) > w; n-- {
		f = prefix + empireListMax(a, ks, n)
	}
	return f
}

// empireList names the empires of ks the player met ("the Mebes and
// the Orfa"), then counts the others ("2 unmet"); "none" when empty.
func empireList(a *app, ks []int) string { return empireListMax(a, ks, len(ks)) }

// empireListMax is empireList naming at most n, the met ones past it
// counted ("3 more").
func empireListMax(a *app, ks []int, n int) string {
	var names []string
	unmet, more := 0, 0
	for _, k := range ks {
		switch {
		case a.e.StanceWith(k) == game.Unknown:
			unmet++
		case len(names) == n:
			more++
		default:
			names = append(names, "the "+a.g.Empires[k].Species.String())
		}
	}
	if more > 0 {
		names = append(names, fmt.Sprintf("%d more", more))
	}
	if unmet > 0 {
		names = append(names, fmt.Sprintf("%d unmet", unmet))
	}
	switch n := len(names); n {
	case 0:
		return "none"
	case 1:
		return names[0]
	default:
		return strings.Join(names[:n-1], ", ") + " and " + names[n-1]
	}
}

// ledgerSep joins the ledger's facts.
const ledgerSep = " · "

// ledgerLine is the ledger on one line.
func ledgerLine(a *app, j int) string { return strings.Join(ledgerFacts(a, j, 0), ledgerSep) }

// ledgerLines are the ledger's facts packed into lines w native px wide
// in bigFace, broken between facts only. Past speciesLedgerRows lines
// the facts left are dropped whole, the last line ending in ledgerMore
// (facts popped to make room): a fact cut mid-number would lie.
func ledgerLines(a *app, j int, w float64) []string {
	var ls, last []string // last: the last line's facts
	cut := false
	for _, f := range ledgerFacts(a, j, w) {
		switch n := len(ls); {
		case n > 0 && ui.Width(ls[n-1]+ledgerSep+f, bigFace) <= w:
			ls[n-1] += ledgerSep + f
			last = append(last, f)
		case n == speciesLedgerRows:
			cut = true
		default:
			ls, last = append(ls, f), []string{f}
		}
		if cut {
			break
		}
	}
	if cut {
		n := len(ls)
		for len(last) > 1 && ui.Width(strings.Join(last, ledgerSep)+ledgerMore, bigFace) > w {
			last = last[:len(last)-1]
		}
		ls[n-1] = strings.Join(last, ledgerSep) + ledgerMore
	}
	for i := range ls {
		ls[i] = fitWidth(ls[i], w, bigFace)
	}
	return ls
}

// ledgerMore ends a ledger cut short.
const ledgerMore = " …"

// speciesLedgerW is the ledger lines' width: right of the flag to the
// right margin.
func speciesLedgerW() float64 { return speciesColW(speciesOfferCol) }

// incoming is the oldest pending offer from j to the player (Game.Offers
// is in the order made).
func incoming(a *app, j int) (game.Offer, bool) {
	for _, o := range a.g.PendingOffers(a.player) {
		if o.From == j {
			return o, true
		}
	}
	return game.Offer{}, false
}

// ourOffer reports a pending offer from the player to j.
func ourOffer(a *app, j int) bool {
	for _, o := range a.g.PendingOffers(j) {
		if o.From == a.player {
			return true
		}
	}
	return false
}

// offerDim: any offer to j would be refused (one pending, or cooldown).
func offerDim(a *app, j int) bool {
	r := a.e.Rel(j)
	return ourOffer(a, j) || r != nil && r.LastOffer >= 0 && a.g.Day-r.LastOffer < game.OfferCooldown
}

// speciesTop is row 0's native y, under a smallFace header line's room.
func speciesTop() float64 { return logTop() + smallFace.Size*1.25 }

// speciesRowY is row r's top.
func speciesRowY(r int) float64 { return speciesTop() + float64(r*speciesRowH) }

// talk opens Talk with empire j; a defeated one (out of the game:
// nothing to say) flashes instead.
func (s *speciesScreen) talk(a *app, j int) {
	s.sel = j
	if o := a.g.Empires[j]; !o.Alive() {
		s.flash(fmt.Sprintf("the %s are defeated", o.Species))
		return
	}
	a.push(newNegotiateScreen(j))
}

// ability is the player's ability button, stacked box abilityStack:
// AbilityText (smallFace, wrapped) over its state (bigFace); dim unless
// ready.
func (s *speciesScreen) ability(a *app) (ls [2]string, r rect, dim, passive bool) {
	info := a.e.Species.Info()
	ready, days := a.g.AbilityState(a.player)
	ls[0] = info.AbilityText
	switch {
	case info.Recharge == 0:
		ls[1], passive = "passive", true
	case a.g.AbilityAsleep(a.player):
		ls[1] = "no system controlled"
	case ready:
		ls[1] = "ready"
	default:
		ls[1] = fmt.Sprintf("in %d days", days)
	}
	return ls, mapStack[abilityStack], passive || !ready, passive
}

// abilityStack is the ability button's stacked box: the last, over the
// news.
const abilityStack = 4

// abilityWords are the ability's text wrapped to its box in smallFace,
// abilityTextRows at most (the last cut).
func abilityWords(t string) []string {
	r := mapStack[abilityStack]
	w := float64(r.X1 - r.X0 - 2*padX)
	ls := strings.Split(wrap(t, int(w/ui.Width("0", smallFace))), "\n")
	if len(ls) > abilityTextRows {
		ls[abilityTextRows-1] = strings.Join(ls[abilityTextRows-1:], " ")
		ls = ls[:abilityTextRows]
	}
	for i := range ls {
		ls[i] = fitWidth(ls[i], w, smallFace)
	}
	return ls
}

const abilityTextRows = 2

// useAbility is the ability pressed (nothing when passive).
func (s *speciesScreen) useAbility(a *app) {
	if _, _, _, passive := s.ability(a); passive {
		return
	}
	before := a.e.Research.Known // a theft's discoveries (learnt)
	reps, err := a.g.UseAbility(a.player)
	a.learnt(before, a.record(reps))
	switch {
	case errors.Is(err, game.ErrNotModelled):
		s.flash(game.ErrNotModelled.Error())
	case err != nil:
		s.flash(err.Error())
	}
}

// flash shows m in red for two seconds (60 ticks a second), on the big
// area's last line.
func (s *speciesScreen) flash(m string) { s.msg, s.msgUntil = m, s.tick+120 }

// click is a left press at native (x,y).
func (s *speciesScreen) click(a *app, x, y float64) {
	if statsBox.has(x, y) {
		a.push(newStatsScreen())
		return
	}
	if _, r, _, _ := s.ability(a); r.has(x, y) {
		s.useAbility(a)
		return
	}
	if !mapArea.has(x, y) {
		return
	}
	if j := s.empire(a, s.row(y)); j >= 0 {
		s.talk(a, j)
	}
}

func (s *speciesScreen) Update(a *app) error {
	s.tick++
	x, y := a.mouse()
	_, wy := ebiten.Wheel()
	press := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	wheeled, pressed := s.news.input(a, x, y, wy, press)
	if pressed {
		return nil
	}
	if wy != 0 && !wheeled {
		s.scroll(a, notch(wy, 1))
	}
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowUp):
		s.scroll(a, -1)
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowDown):
		s.scroll(a, 1)
	}
	s.scroll(a, 0)
	s.hover = -1
	if mapArea.has(x, y) {
		s.hover = s.row(y)
	}
	if press {
		s.click(a, x, y)
		a.pressTurn(turnButton(x, y))
	}
	return nil
}

// Render paints the chrome, the news box, the scroll cues.
func (s *speciesScreen) Render(a *app, sc *ebiten.Image) {
	drawMapChrome(a, sc, len(mapStack))
	drawNewsBox(sc, newsBox)
	drawScrollCues(a, sc, speciesRowsBox(), s.off, speciesMaxRows, len(met(a)))
	for r := range speciesMaxRows {
		if j := s.empire(a, r); j >= 0 {
			o := a.g.Empires[j]
			var op ebiten.DrawImageOptions
			op.GeoM.Scale(zoom, zoom)
			op.GeoM.Translate(float64(mapArea.X0+padX+speciesPortDX), speciesRowY(r)+speciesRowPad)
			if !o.Alive() {
				op.ColorScale.ScaleAlpha(defeatedAlpha)
			}
			a.flag(o.Species, o.Colour).Draw(sc, &op)
			var pp ebiten.DrawImageOptions
			pp.GeoM.Translate(float64(mapArea.X0+padX+speciesPortrait/2), speciesRowY(r)+speciesRowPad+speciesPortrait/2) // its origin is its centre
			if !o.Alive() {
				pp.ColorScale.ScaleAlpha(defeatedAlpha)
			}
			a.spr(portraitBank, int(o.Species)).Draw(sc, &pp)
		}
	}
}

// Text writes the rows (bigFace), the flash on the big area's last
// line, the boxes.
func (s *speciesScreen) Text(a *app, frame *ebiten.Image) {
	for k, h := range speciesHeads[:speciesOfferCol] {
		ui.Print(frame, h, smallFace, speciesColX(k), logTop(), dim(textColour, 0.6))
	}
	m := met(a)
	if len(m) == 0 {
		ui.Print(frame, "No species met yet", bigFace, speciesColX(0), speciesTop(), textColour)
	}
	sel := s.selected(a)
	x0 := float64(mapArea.X0 + padX)
	w := float64(mapArea.X1-padX) - x0
	for r := range speciesMaxRows {
		j := s.empire(a, r)
		if j < 0 {
			break
		}
		c, out := textColour, !a.g.Empires[j].Alive()
		switch {
		case out:
			c = dim(textColour, defeatedAlpha)
		case r == s.hover:
			c = yellow
		}
		y := speciesRowY(r)
		if j == sel {
			strokeRect(frame, rect{int(x0) - zoom*2, int(y), int(x0+w) + zoom*2, int(y) + speciesRowH}, dim(textColour, 0.6))
		}
		ty := y + speciesTextDY
		tx := speciesColX(0) + speciesNameDX
		for k, str := range speciesLine(a, j) {
			x, cw, kc := speciesColX(k), speciesColW(k), c
			switch {
			case k == 0: // the name in its empire's colour, hovered or not, greyed out of the game
				x, cw, kc = tx, cw-speciesNameDX, empireColour(a.g.Empires[j].Colour)
				if out {
					kc = dim(kc, defeatedAlpha)
				}
			case k == speciesOfferCol: // line 2
				x, kc = tx, yellow
				if out {
					kc = c
				}
				ui.Print(frame, fitWidth(str, cw, bigFace), bigFace, x, ty+logLineH, kc)
				continue
			case k == speciesFriendCol && barredDays(a, j) > 0:
				kc = knownColour
			}
			ui.Print(frame, fitWidth(str, cw, bigFace), bigFace, x, ty, kc)
		}
		lc := dim(textColour, 0.6)
		if out {
			lc = dim(textColour, defeatedAlpha*0.6)
		}
		for i, l := range ledgerLines(a, j, speciesLedgerW()) {
			ui.Print(frame, l, bigFace, tx, ty+float64(2+i)*logLineH, lc)
		}
	}
	if s.tick < s.msgUntil {
		ui.Print(frame, fitWidth(s.msg, w, bigFace), bigFace, x0, float64(mapArea.Y1-padY)-logLineH, red)
	}
	ls, r, dimmed, passive := s.ability(a)
	mx, my := a.mouse()
	c := textColour
	switch {
	case r.has(mx, my) && !passive:
		c = yellow
	case dimmed:
		c = dim(textColour, 0.45)
	}
	tl := abilityWords(ls[0])
	ui.Print(frame, strings.Join(tl, "\n"), smallFace, float64(r.X0+padX), float64(r.Y0+padY), c)
	ui.Print(frame, fitWidth(ls[1], float64(r.X1-r.X0-2*padX), bigFace), bigFace, float64(r.X0+padX),
		float64(r.Y0+padY)+float64(len(tl))*smallFace.Size*1.25, c)
	putBox(frame, mapStack[0], speciesHelp, smallFace, textColour)
	putBox(frame, mapStack[1], fmt.Sprintf("%d species met\nof %d others\nhospitality %d", len(m), len(a.g.Empires)-1, a.e.HospitalityGiven), bigFace, textColour)
	c = textColour
	if statsBox.has(mx, my) {
		c = yellow
	}
	putBox(frame, statsBox, "Stats", bigFace, c)
	s.news.text(a, frame)
}
