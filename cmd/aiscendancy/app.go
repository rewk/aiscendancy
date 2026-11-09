package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/cam"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/journal"
	"github.com/rewk/aiscendancy/save"
	"github.com/rewk/aiscendancy/ui"
)

// The logical screen, native px; Ebiten scales it to the window.
const screenW, screenH = 1280, 960

// zoom is the original's pixel in native px (its 640x480 at x2); orig
// writes a rect measured in original px.
const zoom = 2

func orig(x0, y0, x1, y1 int) rect { return rect{zoom * x0, zoom * y0, zoom * x1, zoom * y1} }

// drawSprite draws sp at zoom (nearest) with its origin at native (x,y).
func drawSprite(dst *ebiten.Image, sp *assets.Sprite, x, y float64) {
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(zoom, zoom)
	op.GeoM.Translate(x, y)
	sp.Draw(dst, &op)
}

// rect is half-open, [X0,X1) x [Y0,Y1).
type rect struct{ X0, Y0, X1, Y1 int }

func (r rect) has(x, y float64) bool {
	return x >= float64(r.X0) && x < float64(r.X1) && y >= float64(r.Y0) && y < float64(r.Y1)
}

// Text faces, at native resolution, shared by the screens.
var (
	bigFace    = ui.Face(16)
	smallFace  = ui.Face(12)
	textColour = color.RGBA{200, 220, 230, 255}
)

// app is the window: the game, the empire it plays, the screen stack
// (only the top one updated and drawn into frame). -shot reads the frame
// back after the first Draw, then the next Update quits.
type app struct {
	a      *assets.Assets
	g      *game.Game
	player int           // the window's empire, its index in g.Empires
	e      *game.Empire  // g.Empires[player]
	cl     *game.Cluster // g.Cluster
	// The planets list's order, saved with the game (save.View): idle
	// colonies first, the sort key (planets.go's sortFounded...).
	planetsIdleFirst bool
	// planetsProjectsFirst puts the ongoing projects before the idle.
	planetsProjectsFirst bool
	planetsSort          int
	// bots[i] plays g.Empires[i]; the player's is nil unless a save holds one.
	bots      []*bot.Bot
	log       []logEntry // the last save.LogKeep events, oldest first
	frame     *ebiten.Image
	stack     []screen
	gunTicks  map[game.Gizmo]int // each weapon's gbNN length in ticks (gunTicks, preload): a replayed shot's flight
	sndTicks  map[string]int     // the impacts' and blasts' sounds' lengths in ticks (fxSoundTicks, preload)
	shot      string
	shotTaken bool
	snapDue   bool           // a screenshot asked: taken at the next Draw
	quitting  bool           // the quit prompt is up
	keysUp    bool           // the F1 key help is up, over everything (keys.go)
	setUp     *settingsPanel // the F2 settings are up (settings.go), nil none
	endTxt    []string       // the end prompt's lines, read once
	winTxt    []string       // the victory popup's lines, read for winFor (victoryPrompt)
	winFor    game.Victory

	discoveries []discovery    // the discovery popups waiting, the head shown
	discTotal   int            // the queue's batch size, for "k of n"
	discStory   bool           // the batch completed the tree: its last popup tells the singularity (storyShown)
	explored    []exploration  // the exploration popups waiting, the head shown
	colReports  []colonyReport // the colony reports waiting, the head shown
	contacts    []contactNote  // the first-contact, war and break popups waiting, the head shown
	proposals   []game.Offer   // the proposal popups waiting (offers to us), the head shown
	charged     bool           // the ability popup waits (an AbilityReady of ours)
	victory     game.Victory   // the victory popup waits (a Won of ours), NoVictory none

	savePath   string          // where F5 saves and F9 loads
	lastPath   string          // saveLast's file, the opening's Resume; "" none (tests)
	configPath string          // where the settings' Done writes, "" nowhere (tests)
	set        settings        // the sound settings in force (settings.go)
	genome     *bot.Genome     // -genome: a fresh bot's (botConfig), nil the fixed policy
	reveal     bool            // -reveal: every game set explores the whole cluster
	journal    *journal.Writer // -journal, nil none
	tick       int             // Updates run
	toast      string          // drawn until toastUntil
	toastErr   bool
	toastUntil int

	music *jukebox // nil under -shot
	sfx   *sfx     // nil under -shot
	// desDetailed is the design screens' list view (designScreen.detailed),
	// kept while the game runs.
	desDetailed bool
	// orbitSlots is the orbit slot each ship was last drawn in, per
	// planet (orbitShipsAt keeps it while free): a ship leaving moves no
	// other. Kept while the window runs, not saved.
	orbitSlots map[*game.Starship]orbitSpot

	history []speciesHistory // history.txt by species, read by preload
	opening *openingArt      // the opening screen's, made by preload

	dips    *dipScripts           // the .dip scripts, read once
	helpTxt *helpTexts            // planhelp.txt and gizhelp.txt, read once
	greeted map[*game.Empire]bool // the empires greeted this session

	warn *dayWarning // the day's end warning is up (warning.go), nil none
}

// logEntry is an event and the day it happened.
type logEntry struct {
	Day  int
	Ev   game.Event
	Idle bool // the news left its colony with no project (idleAfter)
}

// botConfig is a fresh bot's: bot.Default with the -genome Genome.
func (a *app) botConfig() bot.Config {
	cfg := bot.Default()
	cfg.Genome = a.genome
	return cfg
}

// newApp makes the window playing empire player of g (nil: loadFrom
// sets it next). The bots are the caller's.
func newApp(a *assets.Assets, g *game.Game, player int, shot string) *app {
	ap := &app{a: a, shot: shot, frame: ebiten.NewImage(screenW, screenH)}
	if g != nil {
		ap.setGame(g, player)
	}
	return ap
}

// setGame points the window at empire player of g, with the window's
// journal. With reveal the empire Visits every star (saved) and scans
// every ship (ScanAll, not saved).
func (a *app) setGame(g *game.Game, player int) {
	a.warn = nil
	a.g, a.player, a.e, a.cl = g, player, g.Empires[player], g.Cluster
	g.SetJournal(a.journal)
	a.e.ScanAll = a.reveal
	if a.reveal {
		for st := range a.cl.Stars {
			a.e.Visit(st)
		}
	}
}

// file is the game as a save.File: the Game, a bot state per empire
// (nil where none plays), the log.
func (a *app) file() *save.File {
	f := &save.File{Seed: a.g.Seed, Game: a.g, Bots: make([]*bot.State, len(a.g.Empires)),
		View: save.View{PlanetsIdleFirst: a.planetsIdleFirst, PlanetsProjectsFirst: a.planetsProjectsFirst, PlanetsSort: a.planetsSort}}
	for i, b := range a.bots {
		if b != nil {
			f.Bots[i] = b.Save()
		}
	}
	for _, l := range a.log {
		f.Log = append(f.Log, save.Entry{Day: l.Day, Event: l.Ev, Idle: l.Idle})
	}
	return f
}

// loadFrom replaces the game and the log by the save at path, playing
// empire 0. Saved bots resume; an empire without one gets bot.Default,
// the player's nil. The caller opens fresh screens (the old ones point
// into the old game). On error nothing changes.
func (a *app) loadFrom(path string) error {
	f, err := save.Read(path)
	if err != nil {
		return err
	}
	a.setGame(f.Game, 0)
	a.bots, a.log = make([]*bot.Bot, len(f.Game.Empires)), nil
	for i, e := range f.Game.Empires {
		switch {
		case i < len(f.Bots) && f.Bots[i] != nil:
			a.bots[i] = bot.Resume(e, f.Bots[i])
		case i != a.player:
			a.bots[i] = bot.New(e, a.botConfig())
		}
		if a.bots[i] != nil {
			a.bots[i].G = f.Game // the bots fight
		}
	}
	for _, l := range f.Log {
		a.log = append(a.log, logEntry{Day: l.Day, Ev: l.Event, Idle: l.Idle})
	}
	a.planetsIdleFirst, a.planetsProjectsFirst = f.View.PlanetsIdleFirst, f.View.PlanetsProjectsFirst
	a.planetsSort = min(max(f.View.PlanetsSort, 0), sortKeys-1)
	return nil
}

// toastFor shows m at the frame's bottom-left for two seconds, in
// errColour when bad.
func (a *app) toastFor(m string, bad bool) {
	a.toast, a.toastErr, a.toastUntil = m, bad, a.tick+120
}

// save (F5) writes the game to savePath.
func (a *app) save() { a.saveTo(a.savePath) }

// saveTo writes the game to path, toasted either way: true when written.
func (a *app) saveTo(path string) bool {
	if err := save.Write(path, a.file()); err != nil {
		a.toastFor(err.Error(), true)
		return false
	}
	a.toastFor("saved to "+path, false)
	for _, sc := range a.stack { // F5 under the slots screen: its row is stale
		if sv, ok := sc.(*slotsScreen); ok {
			sv.saved(path)
		}
	}
	return true
}

// saveLast writes the game to lastPath at each day's start (after a
// day or a Skip, and Begin), apart from the quick save: the opening's
// Resume Last Game. Only a failure is toasted.
func (a *app) saveLast() {
	if a.lastPath == "" || !a.inGame() {
		return
	}
	if err := save.Write(a.lastPath, a.file()); err != nil {
		a.toastFor(err.Error(), true)
	}
}

// load (F9) replaces the game by savePath's on a fresh starmap.
func (a *app) load() bool { return a.loadPath(a.savePath) }

// loadPath replaces the game by path's on a fresh starmap (the whole
// stack dropped); a refusal is toasted and changes nothing.
func (a *app) loadPath(path string) bool {
	if err := a.loadFrom(path); err != nil {
		a.toastFor(err.Error(), true)
		return false
	}
	a.restart()
	a.toastFor(fmt.Sprintf("loaded %s, day %d", path, a.e.Day), false)
	return true
}

// restart drops the stack, the queued discoveries, explorations, colony
// reports, contacts and proposals and the ability popup, and opens a fresh starmap
// on the current game, its species' sprites preloaded. A movie dropped
// with the stack (F9 under it) never leaves: its hold on the jukebox is
// released here.
func (a *app) restart() {
	if a.music != nil && a.music.held {
		a.music.hold(false)
	}
	a.discoveries, a.discTotal, a.charged, a.victory = nil, 0, false, game.NoVictory
	a.explored, a.colReports, a.contacts, a.proposals = nil, nil, nil, nil
	clear(a.stack)
	a.stack = nil
	a.preloadGame()
	a.open([]screen{newStarmapScreen()})
}

// inGame is whether a game exists (not yet on the New Game screen).
func (a *app) inGame() bool { return a.g != nil }

// endDay runs a day (roundmode.go), no warning (askDay's): its events.
// The topmost system screen gets the day's rounds and close to replay.
// No day runs once the player is defeated.
func (a *app) endDay() []game.Event {
	if a.over() {
		return nil
	}
	return a.finishDay(a.beginDay())
}

// freed reports a Grew worth noting: its colony had no free people
// before the day (free), has some now and builds nothing (!Running).
func freed(e *game.Empire, free map[*game.Colony]int, x game.Event) bool {
	c := e.Colony(x.Star, x.Planet)
	if c == nil {
		return false
	}
	before, ok := free[c]
	return ok && before <= 0 && c.Free() > 0 && !c.Running()
}

// popNews is evs as the log keeps them: a Grew only when it frees a
// worker (freed), once per colony (two units a day, a Cloning Plant's
// doubled bank, made two identical lines), its text "Available pop on
// <planet>"; game's own "<planet> grows to N" stays for sim and mksave.
func popNews(e *game.Empire, free map[*game.Colony]int, evs []game.Event) []game.Event {
	var out []game.Event
	said := map[*game.Colony]bool{}
	for _, x := range evs {
		if x.Kind == game.Grew {
			c := e.Colony(x.Star, x.Planet)
			if !freed(e, free, x) || said[c] {
				continue
			}
			said[c] = true
			x.Text = "Available pop on " + c.Planet.Name
		}
		out = append(out, x)
	}
	return out
}

// record logs the player's events of reps, stamped a.e.Day, the last
// save.LogKeep kept, and returns them.
func (a *app) record(reps []game.Report) []game.Event { return a.recordOn(reps, a.e.Day) }

// recordOn is record stamped day.
func (a *app) recordOn(reps []game.Report, day int) []game.Event {
	var ev []game.Event
	for _, r := range reps {
		if r.Empire == a.player {
			ev = append(ev, r.Event)
			l := logEntry{Day: day, Ev: r.Event, Idle: idleAfter(a.e, r.Event)}
			if noPopAfter(a.e, r.Event) {
				l.Ev.Text += ", no free pop"
			}
			a.log = append(a.log, l)
		}
	}
	if n := len(a.log); n > save.LogKeep {
		a.log = slices.Delete(a.log, 0, n-save.LogKeep)
	}
	return ev
}

// report is the day's news to the view: the climb (navigate), no popup.
func (a *app) report(evs []game.Event) {
	if len(evs) == 0 {
		return
	}
	a.navigate(evs)
}

// toggleLog pops the log when on top, else pushes it (push keeps one log
// in the stack).
func (a *app) toggleLog() error {
	if _, ok := a.top().(*logScreen); ok {
		return a.pop()
	}
	a.push(newLogScreen())
	return nil
}

// maxSkip bounds runUntilEvent: a quiet game would run on and on.
const maxSkip = 50

// runUntilEvent ends days until an event, maxSkip days or the player's
// defeat; it returns the days run and the last day's events. Its first
// day is the caller's to warn of (askDay); before any later one where
// the warning would apply (restless) it stops, toasted (the toast wins
// over a day's events: both stop it).
func (a *app) runUntilEvent() (int, []game.Event) {
	for n := 1; ; n++ {
		ev := a.endDay()
		if st := a.restless(); len(st) > 0 && !a.over() {
			a.toastFor("combat waiting at "+a.cl.Stars[st[0]].Name, false)
			return n, ev
		}
		if len(ev) > 0 || n >= maxSkip || a.over() {
			return n, ev
		}
	}
}

func (a *app) top() screen { return a.stack[len(a.stack)-1] }

// push enters s and puts it on top (stackOn).
func (a *app) push(s screen) { a.stackOn(s) }

// stackOn enters s and puts it on top. A located screen whose place is in
// the stack replaces it, everything above dropped.
func (a *app) stackOn(s screen) {
	if p, ok := s.(located); ok {
		for i, o := range a.stack {
			if q, ok := o.(located); ok && q.place() == p.place() {
				for j := i; j < len(a.stack); j++ {
					a.stack[j] = nil
				}
				a.stack = a.stack[:i]
				break
			}
		}
	}
	s.Enter(a)
	a.stack = append(a.stack, s)
}

// open pushes the initial screens in order.
func (a *app) open(ss []screen) {
	for _, s := range ss {
		a.push(s)
	}
}

// pop drops the top screen, the one below untouched; on the last one it
// returns ebiten.Termination.
func (a *app) pop() error {
	if len(a.stack) <= 1 {
		return ebiten.Termination
	}
	a.stack[len(a.stack)-1] = nil
	a.stack = a.stack[:len(a.stack)-1]
	return nil
}

// mouse is the cursor in native px.
func (a *app) mouse() (float64, float64) {
	mx, my := ebiten.CursorPosition()
	return float64(mx), float64(my)
}

// spr fetches a sprite; preload loaded them all, so a failure is fatal.
func (a *app) spr(bank string, frame int) *assets.Sprite {
	s, err := a.a.Sprite(bank, frame)
	if err != nil {
		log.Fatal(err)
	}
	return s
}

// frames is bank's frame count (preload has decoded it).
func (a *app) frames(bank string) int {
	b, err := a.a.Bank(bank)
	if err != nil {
		log.Fatal(err)
	}
	return len(b.Frames)
}

// flag is species sp's flag (raceflag.shp) in colour c.
func (a *app) flag(sp game.Species, c int) *assets.Sprite {
	s, err := a.a.SpriteIn(flagBank, int(sp), c)
	if err != nil {
		log.Fatal(err)
	}
	return s
}

// preload decodes every sprite any screen draws, at start: a broken
// install fails here. The species' banks are preloadGame's.
func (a *app) preload() {
	graftHabitat(a.a)
	a.spr(ngChrome, 0)
	a.spr(introChrome, 0)
	op, err := loadOpening(a.a)
	if err != nil {
		log.Fatal(err)
	}
	a.opening = op
	a.gunTicks = gunTicks(a.a.Game)
	a.sndTicks = fxSoundTicks(a.a.Game)
	b, err := a.a.Game.Read("history.txt")
	if err == nil {
		a.history, err = parseHistory(string(b))
	}
	if err == nil && len(a.history) != int(game.NumSpecies) {
		err = fmt.Errorf("history.txt: %d species, want %d", len(a.history), game.NumSpecies)
	}
	if err != nil {
		log.Fatal(err)
	}
	for f := densityFrame0; f <= 12; f++ {
		a.spr(ngBank, f)
	}
	for c := range game.NumColours {
		a.spr(ringBank, c)
	}
	for sp := range game.NumSpecies {
		a.spr(portraitBank, int(sp))
		for c := range game.NumColours {
			a.flag(sp, c)
		}
	}
	for c := range game.NumColours { // the slots screen's
		for _, f := range []int{intelFleet, intelColony, intelTech} {
			a.intel(f, c)
		}
	}
	a.spr(intelBank, intelWar)
	a.spr(intelBank, intelAlly)

	a.spr("1starmap.tmp", 0)
	for f := digit0; f <= dayBox; f++ {
		a.spr(mapBank, f)
	}
	for f := range starFrame(game.NumStarTypes, 0) {
		a.spr(starBank, f)
	}
	a.spr(starBank, hoverRing)

	a.spr("16battle.tmp", 0)
	for _, f := range []int{panelFrame, endTurnFrame, repeatFrame, autoFrame} {
		a.spr(battleBank, f)
	}
	for t := range game.NumStarTypes {
		a.spr("suns", t)
	}
	for bank, n := range fxBanks() {
		if got := a.frames(bank); n > 0 && got != n {
			log.Fatalf("%s: %d frames, the replay's timing assumes %d", bank, got, n)
		}
		for f := range a.frames(bank) {
			a.spr(bank, f)
		}
	}
	a.spr("lanes", 0)
	a.spr("lanes", 1)
	a.spr(shipIconBank, shieldIcon)
	for f := range int(game.NumPlanetTypes) * game.MaxSize {
		a.spr("planets", f)
	}
	for c := range game.NumColours {
		for f := range levels {
			a.pennant(f, c)
		}
	}

	for f := 0; f <= 4; f++ {
		a.spr("17np", f)
	}
	for f := 0; f < int(game.FirstProject); f++ {
		a.spr("planitem", f)
	}
	a.spr("planitem", ruinsFrame)
	for _, b := range []string{"planres", "planind", "planpro"} {
		for f := 0; f <= 20; f++ {
			a.spr(b, f)
		}
	}
	for t := range game.NumPlanetTypes {
		for size := game.MinSize; size <= game.MaxSize; size++ {
			a.spr(globeBank(t), size-1)
		}
	}

	a.spr("20res.tmp", 0)
	a.spr(mapBank, resFrame)
	for t := range game.NumTechs {
		a.spr(resBank, int(t))
	}

	a.spr(mapBank, shipsFrame)
	a.spr(mapBank, abilityFrame)
	a.spr(mapBank, planetsFrame)
	a.spr("8shipdes.tmp", 0)
	for f := 0; f <= 8; f++ {
		a.spr(desBank, f)
	}
	for g := range game.NumGizmos {
		a.spr(gizmoBank, int(g))
	}
	if a.inGame() {
		a.preloadGame()
	}
}

// graftHabitat makes gizmos.shp's frame game.SpaceHabitat (ours, past
// the bank's 76): planitem's Habitat (frame 3, 68x52, origin (34,18) its
// diamond's top vertex) on a gizmo tile's canvas (70x53, origin (34,36)),
// its diamond on the floor plate (the plates' left vertex is at x 1 on
// the same rows 34-35: the top vertex at (35,18)), its green ground in
// the plates' greys by luminance (game.pal 2-7). In memory only.
func graftHabitat(as *assets.Assets) {
	grey := map[uint8]uint8{181: 7, 187: 6, 191: 5, 189: 4, 192: 3, 199: 2}
	if err := as.Graft(gizmoBank, int(game.SpaceHabitat), "planitem", 3, 70, 53, image.Pt(34, 36), image.Pt(35, 18), grey); err != nil {
		log.Fatal(err)
	}
}

// preloadGame decodes the game's species' sprites: every empire's flag
// and ships, the player's design hulls and picture. No-op without assets.
func (a *app) preloadGame() {
	if a.a == nil {
		return
	}
	for _, e := range a.g.Empires {
		a.flag(e.Species, e.Colour)
		for h := range game.NumHulls {
			a.spr(shipBank(e.Species), int(h))
		}
	}
	for h := range game.NumHulls {
		a.spr(dkBank(a.e.Species), int(h))
	}
	a.spr(pictureBank(a.e.Species), 0)
}

// Update routes the app-wide keys before the top screen's: Esc back,
// Space a day (in combat mode the next round), Tab the log, F5/F9
// save/load (only Esc and F9 before a game), F3 the music even while
// typing, F12 a snap, F1 the key help. The key help, then the quit
// prompt, the day's end warning, the end prompt, the discovery popup,
// the exploration popup, the colony report, the ability popup take
// every other input while up (Space included: no day). A shot runs no
// Update.
func (a *app) Update() error {
	if a.shotTaken {
		return ebiten.Termination
	}
	if a.shot != "" {
		return nil
	}
	a.tick++
	a.music.update()
	if keyJustPressed(ebiten.KeyF3) && a.music != nil {
		m := "music off"
		if a.music.toggle() {
			m = "music on"
		}
		a.set.music = a.music.on // a dead jukebox toggles nothing
		a.toastFor(m, false)
	}
	if keyJustPressed(ebiten.KeyF12) {
		a.snapDue = true
	}
	click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	mx, my := a.mouse()
	if a.keysInput(keyJustPressed(ebiten.KeyF1), keyJustPressed(ebiten.KeyEscape) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButton3) || click, click && keysButton.has(mx, my)) {
		return nil
	}
	if a.setUp != nil || keyJustPressed(ebiten.KeyF2) {
		a.settingsUpdate()
		return nil
	}
	if a.quitting {
		return a.quitUpdate()
	}
	if a.warn != nil {
		return a.warnUpdate()
	}
	if a.ending() {
		return a.endUpdate()
	}
	if a.discovering() || a.exploring() || a.reporting() || a.contacting() || a.proposing() || a.charging() || a.winning() { // F5, F9 and Tab pass under them (F9 drops them)
		if keyJustPressed(ebiten.KeyF5) {
			a.save()
		}
		if keyJustPressed(ebiten.KeyF9) {
			a.load()
			return nil
		}
		if keyJustPressed(ebiten.KeyTab) {
			return a.toggleLog()
		}
		switch {
		case a.discovering():
			return a.discoveryUpdate()
		case a.exploring():
			return a.exploreUpdate()
		case a.reporting():
			return a.reportUpdate()
		case a.contacting():
			return a.contactUpdate()
		case a.proposing():
			return a.proposalUpdate()
		case a.charging():
			return a.chargedUpdate()
		}
		return a.victoryUpdate()
	}
	// Esc or the mouse's back button (MouseButton3, X11 button 8).
	if keyJustPressed(ebiten.KeyEscape) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButton3) {
		return a.back()
	}
	// A typing screen keeps Space and Tab; the forward button still ends
	// the day.
	forward := inpututil.IsMouseButtonJustPressed(ebiten.MouseButton4)
	if t, ok := a.top().(interface{ typing() bool }); ok && t.typing() && !forward {
		return a.top().Update(a)
	}
	if !a.inGame() {
		if keyJustPressed(ebiten.KeyF9) {
			a.load()
		}
		return a.top().Update(a)
	}
	// Space or the mouse's forward button (MouseButton4, X11 button 9).
	if keyJustPressed(ebiten.KeySpace) || forward {
		a.space()
	}
	if keyJustPressed(ebiten.KeyTab) {
		if err := a.toggleLog(); err != nil {
			return err
		}
	}
	if keyJustPressed(ebiten.KeyH) {
		if err := a.toggleStats(); err != nil {
			return err
		}
	}
	if keyJustPressed(ebiten.KeyF5) {
		a.save()
	}
	if keyJustPressed(ebiten.KeyF9) {
		a.load()
	}
	return a.top().Update(a)
}

// space skips the top system screen's replay when one runs, else plays
// the next round there in combat mode, else ends the day (askDay: the
// warning first).
func (a *app) space() {
	v, ok := a.top().(*systemScreen)
	switch {
	case ok && v.skipReplay(a):
	case ok && v.inCombat(a):
		a.stepRound(v)
	default:
		a.askDay(a.nextDay)
	}
}

func (a *app) Draw(screen *ebiten.Image) {
	s := a.top()
	// Cleared: the previous Draw's text must not survive.
	a.frame.Fill(color.Black)
	s.Render(a, a.frame)
	s.Text(a, a.frame)
	snap := a.snapDue // the screen as played: no quit prompt, no toast
	if a.tick < a.toastUntil && !snap {
		const pad = 6
		h := smallFace.Size * 1.25
		w := ui.Width(a.toast, smallFace)
		y := float64(screenH) - h - pad
		a.frame.SubImage(image.Rect(0, int(y)-pad, int(w)+2*pad, screenH)).(*ebiten.Image).Fill(color.Black)
		c := textColour
		if a.toastErr {
			c = errColour
		}
		ui.Print(a.frame, a.toast, smallFace, pad, y, c)
	}
	if !snap { // under the popups and the help, which cover it
		mx, my := a.mouse()
		drawButton(a.frame, keysButton, "?", keysButton.has(mx, my))
	}
	quitting := a.quitting && !snap
	if quitting {
		a.drawQuit(a.frame)
	}
	if a.warn != nil && !quitting {
		a.drawWarn(a.frame)
	}
	if a.ending() && !quitting { // the quit prompt asks over it
		a.endPrompt().draw(a, a.frame)
	}
	if a.discovering() && !quitting {
		a.drawDiscovery(a.frame)
	}
	if a.exploring() && !quitting {
		a.drawExplore(a.frame)
	}
	if a.reporting() && !quitting {
		a.drawReport(a.frame)
	}
	if a.contacting() && !quitting {
		a.drawContact(a.frame)
	}
	if a.proposing() && !quitting {
		a.drawProposal(a.frame)
	}
	if a.charging() && !quitting {
		a.chargedPrompt().draw(a, a.frame)
	}
	if a.winning() && !quitting {
		a.victoryPrompt().draw(a, a.frame)
	}
	if a.setUp != nil {
		a.drawSettings(a.frame)
	}
	if a.keysUp { // last, over the popups; a snap keeps it
		a.drawKeys(a.frame)
	}
	if snap {
		a.snapDue = false
		a.snap()
	}
	screen.DrawImage(a.frame, nil)
	if a.shot != "" && !a.shotTaken {
		img := image.NewRGBA(a.frame.Bounds())
		a.frame.ReadPixels(img.Pix) // premultiplied, but the frame is opaque
		writePNG(a.shot, img)
		a.shotTaken = true
	}
}

// Layout is the fixed logical screen: Ebiten scales it to the window.
func (a *app) Layout(int, int) (int, int) { return screenW, screenH }

// clickSlop is how far (native px) a click may stray before it is a drag.
const clickSlop = 3 * zoom

// drag follows the left button over a 3D view: a press inside the area
// starts it, the release is a click when it never strayed clickSlop.
type drag struct {
	on           bool
	x0, y0       float64 // where the press was
	lastX, lastY float64 // the cursor at the previous move
	far          float64 // the farthest the cursor went from (x0,y0)
}

func (d *drag) press(x, y float64, inside bool) {
	*d = drag{on: inside, x0: x, y0: y, lastX: x, lastY: y}
}

// move is the cursor's step since the previous move, 0 off a drag.
func (d *drag) move(x, y float64) (dx, dy float64) {
	if !d.on {
		return 0, 0
	}
	dx, dy = x-d.lastX, y-d.lastY
	d.lastX, d.lastY = x, y
	d.far = max(d.far, math.Hypot(x-d.x0, y-d.y0))
	return dx, dy
}

// release ends the drag and reports whether it was a click.
func (d *drag) release(x, y float64) bool {
	d.far = max(d.far, math.Hypot(x-d.x0, y-d.y0))
	click := d.on && d.far < clickSlop
	d.on = false
	return click
}

// steer is the 3D views' camera input: arrows pan, a left drag started
// in area rotates, the wheel zooms (anywhere). It returns the cursor and
// whether a click was released; the caller clamps the camera.
func steer(a *app, c *cam.Camera, d *drag, area rect) (x, y float64, click bool) {
	const pan = 4 * zoom // px a tick
	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		c.Cx -= pan
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
		c.Cx += pan
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
		c.Cy -= pan
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
		c.Cy += pan
	}
	x, y = a.mouse()
	switch {
	case inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft):
		d.press(x, y, area.has(x, y))
	case inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft):
		click = d.release(x, y)
	case ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft):
		// The near side follows the cursor.
		dx, dy := d.move(x, y)
		c.Yaw -= dx * 0.005 // radians per native px: 0.01 per original px
		c.Pitch += dy * 0.005
	}
	if _, wy := ebiten.Wheel(); wy != 0 {
		c.Focal *= math.Pow(1.1, wy)
	}
	return x, y, click
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func writePNG(path string, img image.Image) {
	out, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(out, img); err != nil {
		log.Fatal(err)
	}
	if err := out.Close(); err != nil {
		log.Fatal(err)
	}
}
