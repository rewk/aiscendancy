# The original's screens, and ours

What the real game shows, from web screenshots and DOSBox
captures (`reference/`, git-ignored; DOSBox Staging writes captures to
`~/.var/app/io.github.dosbox-staging/config/dosbox/capture/`, 1920x1440
= x3; a nearest-neighbour downscale by 3 recovers the exact 640x480
frame), and what our screen does instead. Chrome rectangles and banks
are in `formats.md`.

**Keys** (ours): one scheme on every screen. Space
(or the mouse's forward button) ends the day and nothing else, Esc (or
the back button) goes back, S is the species screen everywhere but the
ship design (where it saves a template), F1 the key help: a modal
overlay over any screen, popups included, dimming the window and listing
`cmd/aiscendancy/keys.go`'s `keyTable` (THE list of the bindings, the
current screen's section bright); F1, Esc, the back button or a click
close it, F3 and F12 pass, nothing else reaches the game. The mouse's
way in is a 24x24 "?" button in the frame's bottom-right corner
(`keysButton`), drawn by the app over every screen, under the popups and
the help.

## Opening (the main menu; ours, `opening.go`)

The original: `data/0opening.gif` (640x480, its own palette: the head,
"ASCENDANCY", seven blue slots 319x31 at (159, 200+40k)), filled by
`0opening.shp`'s seven labels drawn in `data/logo.pal`: Play
Introduction, Tutorial, New Game, Resume Last Game, Load Saved Game,
Save Game, Exit to DOS, their key letter in red. No pressed or hovered
frames. Ours, the binary's first screen without `-load`, never shown
again (a game's end goes to New Game): the picture at zoom, each label
redrawn (the original's font is ugly) in Go Mono 20
with the colours of a capture of the original's: white on navy
(12,32,49), the key letter in the frame's teal (Load's L
and Exit's X in the word, the Alt line dropped); New Game (N) replaces it by the New Game screen, Resume Last
Game (R) loads `last.json.gz` (live only when `save.Peek` finds this
version's): beside the quick save, written at each day's start (after a
day or a Skip, and at Begin, `app.saveLast`), apart from F5/F9's
quick save; Load Saved Game (L) opens the slots screen over it (Esc
back), Settings (S) in Save Game's slot opens the settings panel
(below), Exit to DOS (X) quits at once; Alt is ignored. Tutorial is
drawn dimmed and dead (kept, a use may come; the spec below,
"Tutorial").
Esc asks to quit, F9 loads, as before a game.

Play Introduction (I; `movie.go`) plays
`data/intro65a.flc` (package `flic`, formats.md) at zoom over black,
timed and scored by `subtitle.txt`: its frame rates (12, 10 from frame
474, 12 from 604), intro00 at frame 20 and intro01 at 988 as the
jukebox's stinger (the themes held meanwhile, `jukebox.hold`), its
eighteen subtitles in Go Mono, white, centred in TITLERECT
(0,379)-(639,419), each from its FRAME_START to the next's. Left out:
FADEMUSIC (intro00's fade before intro01 cuts it), DELAY, the
INTRO.FNT font. Any click or key but F3, Esc, or the end goes back to
the opening, a theme drawn.

## New game (newgame-dosbox captures, beginnew.txt, chrome 3cfgnew)

The original (measured on `reference/newgame*-dosbox.png`):
the big area's left part is a PREVIEW of the cluster to be played (stars
only, no lanes, in x 49-291, y 24-282) over a white three-line summary
centred at (170, 325-361): "<Density> Star Cluster / <Five> Species /
<Neutral> Atmosphere" (static.txt 168); its right part shows the chosen
species: its name in the player's colour centred at (411,20), its round
portrait (smraces.shp frame = species, 100x100, origin the centre) in a
ring of the player's colour (racering.shp) centred at (408,96), its home
world globe (~52 px, centre (408,216)) with the species flag at
(441,205), and its description in small text in that colour wrapped in x
335-482 from y 265. The "Player Species" list shows the portraits three
at a time with scroll arrows. The bottom holes carry a white label ABOVE
the picture (y 391-399: "Star Density", "Species", "Atmosphere", "Player
Color"; the picture rows 25-80 of the 116x92 frame), the fifth a small
galaxy and the portrait over "Begin New Game" (y 455-464). Density: "the
lower the density, the shorter the game"; species: three to seven,
"includes our species"; atmosphere: "the general aggressiveness of
species". Then the species introduction (below).

Ours (`newgame.go`): the opening's New Game opens it, in the original's
layout: the preview of the very
cluster Begin rolls (NewCluster(seed, density), stars only) over the
summary, whose third line is our seed ("seed N": a click in the preview
re-rolls it, but on the "Bots: Original" line at its top-left, which
cycles the bots' Difficulty, Original or Hard (the evolved
genome), docs/rules.md "Colonisation pace"); the chosen species' name,
portrait in the colour's ring,
home world with its flag, description and ability at the right; the
species list as names, all 21 at once (QoL); the labels above the bottom
pictures; the density and the empire count (3-7) cycle on a click; the
atmosphere too (Peaceful, Neutral, Hostile, frames 10-12; Begin sets
`Game.Atmosphere`, the bots' bias on their
thresholds, docs/rules.md "Diplomacy"). Player Color, as
the original: the species' flag in the seven colours, a row of 15x25
flags at (381+16k, 433), the chosen one's edge white; a click picks it.
The colour paints the name and description (its ramp's lightest entry),
the ring (racering.shp frame = colour, also around the fifth hole's
portrait) and the flags. The other empires' colours are drawn by the
seed (game.PickColours), the player's then swapped in (Game.SetColour),
and saved (Empire.Colour; a save from before holds every colour 0 and
loads with the empires' indices). The flags are drawn in game.pal's
green ramp (16-19); each colour is the next 4-entry ramp (purple 20,
yellow 24, pink 28, brown 32, blue 36, orange 40), moved in at draw time
(assets.SpriteIn). The `race[a-g]03.haz` tables are not this
recolouring. Begin builds the game (the player plays by hand: a bot for
the player comes from `mksave -bot`), F9 continues the quick save, Esc
asks to quit.

## Species introduction (captures image0044 Orfa, image0045 Snovemdomas; chrome 12start)

The original, after Begin New Game: a top bar with the species' flag at
the left and the title "<Name>: <power>" centred, the species' picture
below left, its intro at the right in white, its description in the
bottom panel; the title and the description in the player's colour. Its
stinger raceNN plays while it is up. Ours (`intro.go`): the
same layout on 12start.tmp, texts from history.txt (docs/formats.md)
(the intro run into the atmosphere's section: `peace`, `neutral` or
`hostile`) reflowed in Go Mono at the largest of 22/20/18/16
px that holds both panels whole (22 for 10 species, 20 for 8, 18 for the
Orfa and the Dubtaks; smallFace the last fallback), the title at 22, the
colour's text tone as the New Game screen's name (the brown, too dull on
the starfield, x1.5: `introTone`). Begin makes the game, opens the
starmap and pushes the introduction over it, with no toast (the screen
says the choices); any click or key (Esc included, F3 excepted: the
music) uncovers the starmap. Space, Tab, F5 and F9 do nothing else
there. `-load` never shows it. Unlike the captures, whose font has a 13
px pitch at 640x480 (26 native) whatever the species, ours varies with
the text (bigger, on purpose).

## Music (ours, `music.go`, package `sound/`)

The jukebox plays theme00-04 in turn, looped, from the first frame (the
opening, or the starmap after `-load`), whole, silences included
(formats.md "Music").
The species introduction (after Begin, as the original) stops it and
plays the player's stinger, raceNN with NN its `game.Species` index
(history.txt's specnum order); leaving the introduction does not cut
it: it plays out, then the jukebox starts afresh on a randomly drawn
theme (not resumed where it was). A stinger started during
another replaces it: the old player is paused before its Close, oto
v3.4's `Player.Close` being a no-op (the cause of a double stinger when
the species pick still played them). **F3** toggles the music on any
screen, even while typing (M is the system screen's repeat order). In
`aiscendancy.json` only, never on the command line (the settings
panel's keys): `music` false (start muted, the device opened only when
F3 turns it on), `volume` 0..1 (0.6) and `music-gap` (3m: the silence
between two tunes, a few minutes; after a stinger
too, the first tune at once) (cmd/aiscendancy/config.go: a JSON object
keyed by flag name, encoding/json, no hand parser).

**Settings** (`settings.go`): a modal panel over any screen,
opened by F2, the opening's Settings, the quit prompt's Settings
button. Music On/Off, the music's and the effects' volumes (sliders by
5 %), the silence between tunes (− / + by 30 s, 0 to 10 min), each
applied at once (the playing tune's volume too, an effect played as the
effects' slider is let go); the game directory, the save file and the
settings file shown, edited in the file only. Done (Esc, Enter, F2, the
back button) writes the four keys `music`, `volume`, `sfx`, `music-gap`
into the config file (`-config`, else `./aiscendancy.json`, made if
absent), keeping its other keys, sorted and indented, through a
temporary file (a file that does not parse is left alone, the failure
toasted); unchanged, nothing is written. Cancel puts the opening
values back. These four keys are no flags (what
the panel writes is what plays next).
`-shot` never opens audio. A device
failure (no ALSA device) or a missing tune is logged once and the game
carries on muted. victory is not played (no victory screen); intro00/01
are the introduction's (below, Opening).

**Sound effects** (`sfx.go`): of `soundfx.txt`'s 86 only
four play, on the system screen, at the config's `sfx` 0..1 (0.6, 0 for none; F3
leaves them on). Never `button`, the original's ting on every
click. `blueexit` with every lane flare, red and blue alike, entering or
coming out (`redexit` is `shield.voc`'s bytes, it does not fit);
`trust01` when a hand hop or a replayed move phase (a round's, the
close's) moves ships the player sees, `trust02` when a Large or Enormous
hull is among them; a replayed shot's weapon `gbNN` as it takes off,
its `ghNN` on a hit or `shield` (absorbed, knocked down) as it lands
(NN its Gizmo index, `batfx.txt`'s sfx1 sfx2; a planet's shot is its
`planetGun`'s); `sg49bull` as a ship bounces
off a barred orbit. Each is a `cue` at the screen's tick, so it sounds
with what it goes with (a flare after the replay); a skipped replay
drops its moves' cues and brings the flares' forward. One still playing
is not started again (an Auto run's hops make one engine noise). The
blasts and razings sound `destroy` as they start, lasting as long.

## Colony screen (colony-screen, colony-home/buildlist/cornucopia-dosbox)

The original: five panels on top, in the order **research, industry,
prosperity** pictures (level 0 research = dark nebula, level 0 industry
= house on red ground, prosperity = forest), then population (figures:
red = busy, orange = free; green circles = room), then the project text
("No Project", "(No Free Population)"). Clicking a picture panel pops
"Tensoric II Industry: 3 per day": the totals are hidden otherwise. Big
globe right. Empty squares are SMALL markers (white, dark = black,
red/green/blue = bonus); only the squares sharing an edge with a
structure are full 68x34 diamonds (white or dark): the buildable ones,
the colony grows outward from the Colony Base. A structure hides its
square's colour (ours outlines it). Clicking a reachable square opens
the project list: tile picture + name, Factory/Agriplot/Laboratory/
Outpost at the start, scroll arrows. Ten orbital slots at the far left
over the starfield, invisible until hovered, a FLAT 2x5 grid of 70x70
cells (not diamonds; origins in formats.md, measured on the DOSBox
capture of a full colony with five orbital structures,
`reference/colony-full-orbit-dosbox.png`); slot order within the grid
assumed row-major. The top-right panel shows the current project with
its picture and "N days until completion".

Ours: the population panel as the original (17np frames 1-3,
docs/formats.md) on 7 rows, with "POP n/capacity" and the
free people (another empire's colony: its species) in Go Mono lines
under them, then for the player's colony its growth: "growth in N days"
(`Colony.GrowthDays`), "full" at its
capacity, "no growth" without prosperity (another empire's: nothing,
its prosperity is not ours to see); past 70 icons they all shrink to
150%, 13 a row on the same area, filtered linear;
past those 117 the rest is "+N" on the second line. Another empire's
colony draws all its people as the orange figure (which ones work is
its business), an uninhabited planet only its rings (a colony's
capacity there). Each picture panel's daily output stays on screen as a
number at its bottom-right corner. The project panel
(`projPanel`): the name centred, the picture (a structure's
planitem frame at zoom rising from a night-blue silhouette as it
progresses, a ship or a refit its hull's sprite), the days left at
today's industry ("no industry"), an ongoing project's diverted share
instead ("ongoing: +N prosperity", "+N research" or, Alien
Hospitality's poured into the species met, "+N hospitality"), never
the cost; idle "No Project"; an ongoing project's picture whole (it
never completes). The three output pictures and their numbers are what the
colony makes, as antag.exe shows them (the raw
`Sums` before, now only the rule's input): `Colony.Shown`, `Output`'s plants and curve, the
prosperity net of the pop/4 upkeep (what the growth bank gets, 0 or
less: "no growth"). An ongoing project shows at once, its share added to
research or prosperity (the frame
following), the industry it comes from kept, curved as idle; and a yellow triangle in the gap from
industry's picture to the receiving one points the move (`shiftCue`;
none for Alien Hospitality, whose share leaves the planet: its planitem
icon at scale 1 in the industry picture's top-left corner instead,
`hospitable`); the planets list (the triangle at two thirds) and its sort
keys too. A small line on top names the planet and its system.
Ours draws the markers too (13x7, the original's outline colours, filled
grey on white and dark on black) and they ignore the mouse: no hover, no
click, no "not reachable" message; a square holding a structure or
reachable is a full diamond, its bonus colour kept; a bare planet is all
markers (nothing is reachable without a colony),
full only while a settler picks its landing square.

The item picker (`cmd/aiscendancy/picker.go`): a left click on an empty
square or orbit slot of the player's colony opens it when something can
start there, i.e. with no project running and free people to work (the
hover is yellow then, red otherwise and the click flashes why). It lists
every item research allows at that spot, all at once with no scrolling:
on a square the surface items but the Colony Base, 21 at most; in a slot
the orbital items, Ship included, 9 at most. The refused ones are faded
with their reason in the footer; a `Unique` bonus structure the planet
already holds (the plants, the Cloning Plant, the Observation
Installation) is drawn blue, the Lush bomb's tint, still a valid choice
(the convention for a unique bonus already built).
A grid of full-size tiles with the name under each: 6x4 over the whole
screen for a square, a 3x3 box right of the orbit column for a slot (a
detailed list view with the numbers was tried and dropped: the footer
gives the hovered item's outputs, room, cost and days at the colony's
industry, then up to four lines of its planhelp.txt paragraph, the rows
shortened to 200/172 px to make room). The header says the
free population. A click on a choice starts its project; Ship (pictured
by the player's species' Medium hull, smshipNN frame 1: planitem 23
repeats the Orbital Missile Base) opens a blank design screen building
at that colony (the ship goes to the first free slot), as does the
Shipyard's dialog (Build Ship, below); a click outside, a right click or
Esc closes it. Another empire's colony is read-only. Its cloakers
(`Empire.SeesOrbit`/`SeesSurface`): an Orbital Cloaker
leaves the orbit's slots empty (no structure, no foreign ship; the
player's own ships in orbit take any slot) and adds "orbit: cloaked" to
the POP panel; a Surface Cloaker draws every square as a bare marker
(no structure, no ruins, no square hover) and empties the POP panel to
"<species>", "surface cloaked" (no people, outputs, shift cue nor
project). Ship's verdict
counts the fleet cap (game.Empire.CanStartShip), so a full fleet is
refused in the picker, not two screens later.

The planet-wide projects are buttons (the original
lists them in the same picker): a column down the big area's right edge
from its top-right corner, clear of every grid size's squares (a row
along the top bit into one), one 72x56 button per project research
allows, none for the others, the planitem frame at scale 1; faded while
refused (the status line gives why under the cursor: all but the Lush
Growth Bomb are game.ErrNotModelled), framed yellow while it runs; a
click starts it (Colony.StartProject). Unlike the original, the Lush
Growth Bomb's button turns blue once the planet is bombed (Planet.Lush),
not faded: the planet's one-shot is visibly spent,
the status line saying "already bombed". The Targeted projects are
elsewhere: Terraforming is in a square's picker, offered where Transport
Tubes go on black and on black under tubes (the click on tubes opens
their dialog, below, whose Terraform button starts it; the tubes are
removed).

**The structure dialog** (the captures image0041-43, ours
`cmd/aiscendancy/structdlg.go`): a left click on a structure
of the player's colony, on a square or in orbit, even while a project
runs, opens a box over the planet, the original's help.shp frame 0 (a
stone frame round a nebula, the button bar its hole, labels unboxed) at
(152,120)-(488,376), the structure's picture with its planhelp.txt
paragraph under it, titled "<structure> on <Kind> Square"
as static.txt 84-85 spell it (kinds White, Black, Red, Green, Blue;
"Automated Factory on White Square"; an orbital's "on Orbital Square").
Its buttons along the bottom: Build Ship first on a Shipyard (a blank
design screen for the colony), Terraform first on tubes on black once
researched, Abandon (`Colony.Abandon` / `AbandonOrbital`: not on the
Colony Base, and refused on the last Shipyard or Orbital Docks the
running project needs, the refusal flashing in the status line, the box
staying; greyed, a press doing nothing, while the population needs the
structure's room, `Colony.CanAbandon`, as the original greys an
Outpost's), Automate on a surface structure not automated that employs
someone (not on an Outpost or Tubes: not offered) once research allows
it (its refusal, `Colony.CanStart`'s, flashes in the status line, the
box stays), Cancel. Terraform and Automate queue while the picker does
(below). Esc, Cancel or a click outside the box close it; it closes when
the colony changes hands or the structure goes. A click on the running
project's target (an Automation's or a Terraforming's square) asks to
cancel the project instead. The hover on a structure is green; the
status line says "Automated <item>" on an automated one; nothing marks
it on the surface (the original has no mark). **No right click
demolishes** any more (a leftover of the first prototype).
Another empire's ship drawn in a free orbit slot takes no click: the
slot's own click (the picker) goes through.

Xeno ruins (`Square.Ruins`) are drawn on their square,
reachable or not, as `planitem.shp` frame 39 (the original's ruins tile,
formats.md), keeping a bonus square's outline as a structure does; the
status line says "(i,j) <kind> - Xeno ruins", followed, on the player's
colony once research allows the dig (not on the square being dug), by
its verdict ("(the dig can start)", "(the dig can be queued)" or the
refusal). The Xeno Archeological Dig is in that square's picker only
(game.Planet.Offers); running, it rises in place as any build (frame 38,
the ruins tile hidden), named "Xeno Archeological Dig" in the info panel
and the cancel question; done, the ruins are gone, and both log lines
(Built, then the Discovered tech) open the planet.

The cancel question: a click on the running project's target (orange
hover), on a project button while it runs, or on the cross (✕) in the
project panel's top-left corner (shown while a project runs, the title
moved right of it: the only way to a refit's) asks whether to cancel it:
what and where, the days left at today's industry, the industry done;
no, Esc or the back button change nothing, yes drops it and its industry
is lost (game.Colony.Cancel), then the next Update starts the queue's
head (`Colony.Next`; its drops go to the log and flash); not after a yes
asked from a project button (the colony left idle for that project) nor
from a ship's Refit. The yes/no box is the quit prompt's, the structure
dialog's (help.shp frame 0, the answers in its button bar), the project's
picture under the lines rising from the
unbuilt tint with its progress, as the project panel's (a ship's hull
too, in both).

**The construction queue** (`queue.go`; the rules:
rules.md "Construction queue"). While a project runs, or the queue waits
for workers, a click on an empty square or slot opens the picker titled
"Queue at ...", its verdicts `CanEnqueue` / `CanEnqueueOrbital`'s on the
planet as planned (a Ship listed, refused); a pick appends to the queue,
and the structure dialog's Terraform and Automate queue too. The squares
are drawn and hovered on the planned planet: a square next to the
running or a queued structure opens. Each queued target is a ghost of
its item (the unbuilt tint, an Automation leaving its structure as
drawn) under a dim outline, with its queue number in small text ("1,2"
for a Terraforming then its Factory); hovered green, the status line
ends "(queue N: click removes it)", and a click removes its last entry
(toast "Factory removed from the queue"). Under the project buttons,
right-aligned with them (x 1202..1258, from y 532: below the five
planet-wide projects' buttons there can be, down to the status line;
over the globe is fine, never over a square), a stack of 44 px
rows holds the entries' picker icons, built from the bottom: the head
there (its row never moves), the later ones above, numbered; past eight
rows the top one says "+N". A hovered row lights its entry's target on
the grid (green) and says "Factory (queue 2): click removes it"; a click
removes it; a ▼ left of the hovered row (never the head's) moves it
forward one (`Colony.QueueUp`). Ongoing projects, the Lush Growth Bomb,
Build Ship and Refit keep their flows: never queued.

Population capacity of a bare planet (the green circles), measured by
classifying every marker on the captures (sample the four outline tips
of the 11x7 marker at each grid cell; outline colours 000000 black,
dbdbdb/cacaca white, be2d10 red, 2dc20c green, 0000ff blue):

    Husk       size 5  73 black                         -> 5
    Congenial  size 2  7 white, 3 black, 1 green        -> 7
    Cornucopia size 3  6 red, 8 green, 9 blue           -> 12
    Tycoon     size 5  20 white, 14 black, 12/14/13 rgb -> 27
    Cornucopia size 5  24 red, 27 green, 22 blue        -> 30
    Home (Primordial, size 5): ~27 white, ~41 black, ~4 bonus, 4 built
                                                        -> 18 incl. Colony Base MPOP 2

No linear fit of these captures; the rule was found in the saves
instead (docs/rules.md, the population capacity: 5 + ⌊non-black/3⌋).

**The orbit** ("the ship moves to the planet, it
appears in the orbit, you go to the planet screen and there you do
invade or colonize"; `orbitships.go`; the rule: rules.md "Orbit"). The
orbit exists only on this screen. The ships in orbit of the planet the
player sees, its own first (Ships order) then `Game.ShipsSeen`'s, fill
the orbit slots holding no structure (nor the colony's orbital project
rising there: a Ship project rises as its hull's sprite, as the project
panel shows it) in slot order, each its species' hull sprite
fitted to the slot: a view mapping, the model gives a ship no slot (not
decided as a rule); ships past the free slots are not drawn. A ship
keeps the slot it was drawn in while that stays free (`app.orbitSlots`,
for the session, not saved; after a Leave Orbit
the others moved up and the one sent away could not be told), a new one
takes the first slot left. Hovered, a
slot says "orbit k - <ship> (<design>) (click: its dialog)", a foreign
one "orbit k - <species> ship, <hull> hull".

A click on one of the player's opens its dialog (from a DOSBox capture:
the original's box over the planet; ours is the
structure dialog's, same frame, same hover freeze, `structdlg.go`; no
selection, no yellow frame: "le cadre jaune ne sert plus vraiment", the
dialog alone holds the ship): titled `Ship "<name>"`; on the left the
ship's sprite, fitted (while a part of the grid is hovered: that part's
name and gizhelp.txt paragraph instead), its order line
under it, wrapped (the system's `shipOrder`: "colonise", "invade
<planet>" (resolved at the day's close), the refit's line, else "in
orbit"; the one place this screen shows a ship's order); on the right a
5x5 grid of what is aboard (the content at a
glance; 25 = the Enormous hull's slots), the design list's tiles at
scale 1 at its 74x60 pitch, row by row: `Starship.Aboard`'s parts (the
spent Colonizers and Invasion Modules gone, as Show), then the hull's
free slots as the empty-slot diamond, nothing past its Slots; a part
hovered puts its name in the status line (a button's line wins).

Its buttons along the bar, each shown only where the rule accepts it now
(no button that would only refuse):
- **Leave Orbit** (unless refitted; now and free, as the original,
`Empire.LeaveOrbit` puts the ship at
  `game.LeavePoint`, PlanetClear from the planet towards the star, or
  its FreeSpot, no power spent; it is selected on the system screen
  under the planet's, there to be moved);
- then ONE middle button, the first that applies: **Refit** in orbit of
  the player's colony with Orbital Docks, not being refitted (`atDocks`;
  what changes is the refit screen's verdict, so Abandon is never
  offered at the Docks: accepted, the press opens
  the design screen's refit mode, below "Refit"; hover = `refitFoot`;
  Refit in place of Abandon or Colonize there); **Create
  Colony** (`CanFound`: a Colonizer left over an uninhabited planet of
  no ally's system; now, not at the day's close:
  the next click on a square, black ones too, founds the colony there
  with that ship, `Empire.FoundAt` / `Planet.CanSettle`; Esc, another
  ship's dialog or that ship leaving the orbit gives up; the screen is
  the colony's at once and may start a project the same day; rules.md
  "Ships", Orders); **Invade**
  (`CanInvade`: an Invasion Module left over a colony of an empire at
  War, not invincible today, unguarded; the Invade order, game/
  resolves it at the day's close, `rules.md` "Invasions"); **Abandon**
  (unless refitted: `Empire.Abandon`, the ship scrapped now, nothing
  back, after a Yes/No question, refused before
  the question, `Empire.CanAbandon`, in a lane, with colonists aboard
  and under a refit; the only way to get rid of a ship before refits); a ship under refit has no middle button;
- then **Show** (always: its parts read-only, the
  design screen on what is aboard, `Starship.Aboard`, titled with its
  name, "the
  ship's parts, read-only", its hull and energy where the verdict goes,
  the category lists reduced to the parts aboard; the category buttons
  browse them to read a part, nothing adds, removes, saves, builds or
  renames, Esc comes back) and **Cancel**.

Cancel, Esc or a click outside the box close it; every accepted press
closes it too; a refusal flashes in the status line ("X is ours", "X is
inhabited", "X is guarded by the Orfa"), the box kept. The hovered
button's line says what a press does without judging the outcome
("Invade: <ship> launches its N invasion modules at the day's close").
The box closes once its ship is no longer the player's in this orbit
(left, destroyed, abandoned), not when the colony changes. Ships never
touch the project panel (top right): it always shows the colony's
project. The fleet list and the system screen's lists, on a ship of the
player's in orbit, open this screen with its dialog up. A colony
changing hands under the screen (an invasion, either way) closes the
picker and the cancel question; the screen turns read-only on the new
owner's colony or becomes the player's.

## System display (system-chippendale/-algorond/-bindacia, help `batmode`)

The original: a black area, a blue SQUARE grid clipped to a disc in the
ecliptic plane seen from ~25° above (ellipse ~440x175 px centred near
(240,335)), bodies as sprites scaled by depth. EVERY body floats above
the plane with a yellow elevation line down to a white foot dot, the
star included (foot = disc centre, ~0.25 radius up); lane endpoints sit
just outside the disc, up to ~1.4 radii high. Toggles for grid and
elevation lines, mouse rotate/zoom, re-centre. The top-right panel: the
star's name and type, then three blue buttons (battle.txt): End Turn,
Repeat Move (the selected ship makes one more move of its engines' reach
on its last order) and Auto (every ship ordered to a planet or a lane
proceeds while its power lasts; with enemies present the original
attacks). The selected ship replaces the panel: rows of icons (weapons,
shields, engines, scanners, generators) and a power column. Free
movement (move1/2/3-dosbox): the original selects the ship, then a cross
moves in the horizontal plane AT THE SHIP'S HEIGHT with a white line
down to its foot on the grid; a click fixes the foot, the mouse then
slides the cross up the vertical, a second click gives the order, a
green ring marks the destination while the ship flies.

Ours: world unit = disc radius, `game.NewStarSystem` rolls positions
(planets radius .25-.9, height [0,.6); lanes radius 1-1.15, height
[0,1.2)), `cam.Camera` (yaw, pitch 25°, dist 4, focal 800) projects,
bodies painted by depth descending, the far ones dimmed; the camera drag
makes the scene follow the cursor. Ships: `smshipNN.shp` is the
species' bank (NN = species index, ASSUMPTION), 4 frames = the hulls,
158x112 with the origin at the centre; every empire's ships of the
system are bodies of radius 0.05; a ship in a lane is gone from the
system once its hop's anim ends at the lane's sphere (it was drawn at half
alpha there), and cannot be interacted
with: one selected when the day's close took it
into its lane is dropped (`selectable`, `rebuild`). The hops are
animated. A ship entering a lane flares its endpoint (`flare.go`):
`blueexit` / `redexit` replace the sphere at its scale,
their 10 frames played forward then back (no abrupt
stop), when a hop's anim ends there or when the day's close took a drawn
ship into a lane (after the combat replay, if one plays); a drawn ship
out of a lane at the close lights the endpoint it came out of
(`inbound`, `startExits`). Each flare sounds `blueexit`, and
the moves sound their engine ("Sound effects").

The bounce off a barred orbit (the original's;
`bounce.go`): a GoToPlanet hop that Hop stopped PlanetClear short of a
guarded planet is drawn flying in to half PlanetClear of the planet,
then back out to its stop, over 1.5 hops' ticks, `sg49bull` at the
turn; a hand hop's status line says "<planet>'s orbit is shielded"
(a shield the player sees) or "guarded". A replayed move (a round's,
the close's) bounces the same, the sound for a ship the player sees.

The orbit exists only on the planet screen: a ship
in orbit is NOT drawn here, nor picked, nor kept selected (it is a body
still, at its planet's Pos, `orbitHidden`, so the lists show it); a hop
into an orbit tweens to the planet and the ship vanishes as the anim
ends (it stays drawn and selected while it flies), one leaving an orbit
appears at the planet and flies out. The one sign of an orbit is the
original's marker (battle.txt info24: "a colored circle surrounding the
planet in the system area", `guardRing`): a circle just outside the
planet's sprite, under the hover ring, when an orbital shield stands or
any ship is in its orbit, in the guard's colour when one guards it
(`Game.Guard`, `rules.md` "Orbit"), else the first orbiting ship's
empire's (a foreign ship in a colony's orbit).

The original's panel sits at the top right (`sysbar.go`: 16battle.shp
frame 10): on its strip, as the original's, the
star's `suns.shp` picture shrunk at the left, the title beside it,
"<name>", "<type>", "Day N  M lanes", and over the picture's top-left
corner, as the planet rows', the flag of the empire controlling the
system (`StarSystem.Controller`, `rules.md` "Winning"; the original's
sits at the strip's right end), when the player knows the star; the
three buttons in their holes, the hovered one framed yellow; the news
moved to the bottom of the info box. An order makes ONE move (a hop of
the engines' reach) and, out of combat mode, drops
the ship (the orbit, lane, point and Navigate…
orders; X keeps it, the rounds keep it for their orders); the middle
button or M repeats it for the selected ship, else the last one ordered
while still here, Auto or A moves every ordered ship of the
system hop after hop until none can (no attack on Auto yet). End Turn
never closes the day (combat mode, below): at a fight it plays the next
round, elsewhere every ordered ship of the system hops once.

Keys and clicks: a click selects one of the player's (yellow ring), then
a lane endpoint is an order (a planet click selects the planet instead:
below; the move mode orders an orbit), X cancels, F fires the chosen
weapon at the hovered foreign ship, D toggles the shields (S is the
species screen, as everywhere), G and E toggle the
grid and the elevation lines, R re-centres the camera, Space ends the
day (as on every screen, with the mouse's forward thumb button; the back
one is Esc). Colonise, Invade, Refit and Leave
Orbit are the planet screen's (the ship in orbit is selected there; this
screen keeps no selection in orbit, so its U key went). The move
(two buttons, the left selects and acts, the right
cancels; at first the right click designated): with a ship
selected, a click on a planet or a lane endpoint orders the orbit or the
lane (hovered, the target wears two yellow rings and the view's bottom a
white line "orbit X, N days" / "lane to Y, refused: ...", as the
starmap's hovered star); before it, the foot, the vertical and the
cross follow the cursor wherever a click would plant them, off when a
body is under it (a free spot, not an object;
`ghostAt`), and only until the first click after the selection (an
order, a shot, a foot: `placing`); a click on nothing plants the foot there
(`cam.UnprojectPlane` at the ship's height), the cross then runs up the
vertical with the cursor (`cam.UnprojectVertical`, clamped to
`game.SystemReach`) and a second click orders the point, whatever is
under it. A right click cancels one layer (the height step, the aim, the
inspected foreign ship, the ship, the planet) and never leaves the
screen; Esc does the same, then goes back. A click on the selected ship
keeps it selected.

Fog of war (`rules.md`): an unexplored star's system shows the star, the
grid and the endpoints of the lanes known from the other end, no planet,
the title "<type>, unexplored"; lane labels name an unknown far star by
its type. Foreign ships: the partial fog (`rules.md`), the bots' own
view. Every foreign ship out of the lanes at a star the player knows is
drawn, present or not (`Game.ShipsSeen`: sprite by the Sighting's hull,
at its position; in orbit hidden as the player's are); none at an
unknown star, none in a lane (the starmap counts those). Hovered, it
says nothing (the hover text went); the ring only.

**The foreign panel** (`fsel`, `foreignText`): a
click on a foreign ship out of the lanes, in the view or on its row,
outside the fire mode (where it still fires), opens a read-only panel
laid out as the ship panel, our selected ship kept aside (`back`). It
reads the Sighting afresh each frame and masks what the scanners do
not: the owner's flag then the title "<species> <hull> ship" ("Baliflids
enormous ship"; the flag for the colour's name),
never its name; unscanned, no design line, where it is, then "not
scanned" (no "your hits: N" line: it confused);
scanned (`Scan`), the design line (`scanShieldState`): with the parts
our panel's shield row from `Scan.ShieldHits` and `Scan.Absorb`
(nothing for no shield part); without them one icon lit when
`Scan.Shields` and not `Scan.ShieldsDown`, else dimmed with " off" or
" down", no number (cost and absorb unknown: a ship with no shield part
shows its switch, the scan cannot tell); the hull bar H/M and the energy bar (scaled by
`Scan.PowerPerDay`, the number E alone), ", cargo N" after where, then
"parts unknown (X Ray Megaglasses)"; with X Ray Megaglasses at the star
(`Scan.Parts`) the energy E/P and the parts' tiles in our bands, "xN"
the parts aboard, the shields' on/off/down their only state, the hover's
facts in the foot without our Usable verdicts. No buttons, no keys, no
clicks. A right click or Esc closes it and selects our ship again if it
is still here out of a lane; the ship's leaving (a lane, another star,
destroyed) does the same; selecting one of ours or a planet drops both.
The view reads no foreign design.

**The hover line**: a planet, the star or a lane
hovered in the view leaves the info box as it is and is described on
one white line at the view's bottom (`hoverLine`: "<name>, <type> size
N[, Xeno ruins]"); with a ship selected the line is the move target's
instead; a hovered ship has no text.

**The lists** (ours, `syslists.go`; the system's
ships and planets in place of the key help; two tabs). With
no ship selected, out of the move mode, the info box shows one of two
tabs, the planets (system order, shown first) and the ships (the
player's out of the lanes, then the foreign Sightings empire by empire:
exactly the view's bodies, fog included; during a replay its cast, a
ship leaving the list at its blast). The tabs are the panel's two 70x31
filter holes (the original's ally/foe ship filters, never of use): on a
dark blue plate, the first five rows' sprites
as miniatures side by side and the count in the corner, the tab shown
framed, the hovered one yellow. A press shows its list, out of the help,
the move mode and the selection (the lists take the panel's place). A
row: the body's own sprite at scale 1, shrunk only to fit its cell, and
smallFace lines.
- Planet (75 px rows, a system's five fit): `planets.shp` at its size
  (size 5 down to 71 px), the owner's flag (`raceflag.shp` at the
  asset's 15x25 px, the species' emblem in the empire's colour,
  battle.txt's "colored species flag") over the cell's top-left corner
  (no ring, the circle is the view's orbit marker);
  its name, "<type> s<size>", the owner's species, "Xeno ruins" when it
  has some (the hover line of a planet ends with it too; a star the
  player does not know lists no planet); the player's colony ends its
  row on its project (`rowProject`, a 56 px column
  at the right): the item's `planitem` tile (a ship's hull) shrunk into
  44 px over the days left ("no industry", "ongoing"), or "No Project"
  in red / "No Pop." in yellow alone when idle, as the planets list.
- Ship (60 px rows): its `smshipNN` frame (species, hull: the
  Sighting's for a foreign one) in a 104x56 cell, its drawn box centred
  (the hull's art sits off the canvas middle; the tabs' miniatures the
  same, `fitSprite`); the player's say name and design, a foreign one
  its hull class ("scanned" when a scanner reads it) and species, the
  first line led by `cos_ship.shp` frame 3 in the owner's colour (the
  starmap's fleet pennant).

The box itself runs down to the big area's bottom (y 473) over the
chrome's six button holes this screen never used, a half-black rim one
original px wide hiding the cut (`drawInfoRim`):
the hint sits above the news box (the starmap's, Turn reports; its soft
border stands in for the rim while it shows), the
planets tab five rows, the ships tab six; the ship panel's tiles stay
where they were. "a-b/N" left on the hint line when a tab overflows; the
wheel over the box scrolls the tab shown, not the camera. Hovering a row
hovers its body (the view's green ring; F acts on it, the row framed
green) and the lists stay; a press on a row inspects, never orders (a
ship selects, a foreign one opens its panel, a planet selects then
opens), but a ship in orbit (hidden
from the view, its row's third line "in orbit of <planet>") opens its
planet's screen, the player's ship's dialog open. Hovering a body in the
view leaves the lists. The key help went to F1
(the hint "F1: keys" sits above the news). With a ship
selected the box is its panel (below): both cannot fit the box, so the
lists come back on deselecting.

**Combat replay** (ours, `replay.go`). A day's rounds
resolve in `bot.DayRun` (rules.md "Combat rounds"); the system screen
redraws those at its star from the `RoundLog`s alone (the topmost system
screen in the stack; kept only when the player Knows the star, the fog's
rule for foreign ships): the rounds played at the day's close,
each round stepped in combat mode as it is played. The cast is snapshotted
before the rounds: the ships out of the lanes there, where they stood
(in orbit: at the planet, hidden). Each round, its empty phases skipped,
in the rule's order (shots before moves): the shots
one by one ("few fights, we can be a bit flashy"),
in `shotOrder`'s order: the sides alternate, each side's and each
target's order as Round resolved them (an outcome hangs on the shots
before it at that target), the shot that destroyed a ship (its last
hull hit) last, so the last exchange is the fatal one (cosmetic: Round
lands every shot before any ship goes), from the round's starting positions: shot k's
bullet (`bulletNN` of the weapon, from the shooter's drawn sprite
centre to the target's) flies as long as its weapon's sound, `gbNN`'s
length read from the archives (`gunTicks`, 0.31 to 1.32 s; 15 ticks
without them; the bullets were too quick between
close ships), landing as the sound ends (`shotAt`, `landsAt`), as the
next takes off; its impact plays from its landing
over its target (`impactOf`: `explNN` when the shot took hull or hit a
structure, `shield` when the shields absorbed it all or it knocked them
down, `Fired.ShieldsDown`), as long as its landing sound (`ghNN` or
`shield`, its frames spread over it; 3 ticks a
frame without the archives), so an impact overlaps the next flight and
the next impact; the destroyed explode (`destroy`) once the last impact
has ended, as long as the `destroy` sound played with them (1.3 s),
then the survivors' moves tween From→To
(18 ticks; a move ending in orbit, `Move.Orbit`, ends at the planet, the
ship hidden from then on; one leaving an orbit appears there as it
moves), the engine's cue with them. Planets in combat: a planet's
shot (`Fired.FromPlanet`, no shooter ship) flies from the planet's drawn
sprite with `planetGun`'s bullet and explosion (the game has no
orbital-weapon bank: the Missile Base draws Fourier Missiles', the
Whoppers a Mass Barrage Gun's, ASSUMPTION); a shot at a planet's
structure (`AtPlanet`, Target nil) explodes over the planet's sprite
(sized on it, drawn right after it), one at a ship in orbit at the
planet (the ship hidden); a structure razed (`RoundLog.Razed`) blows up
with the destroyed, `destroy` over the planet sized on its orbit ring
(its widest frame spans the ring: `razeAt`), the guard circle following
the model once it is gone. A bullet is depth-sorted with the bodies, one
art pixel = 0.001 world units (a ship sprite's ratio: `fxPixel`). An
impact is drawn over its target: right after its body in the draw order
(`drawList`; several in log order), on its drawn sprite's box centre,
the bank's largest frame box spanning the ship's larger side
(`impactAt`). A blast replaces its ship: the sprite goes at the blast's
first tick, frame 0 (an oval) centred on the ship's drawn box and scaled
so its inscribed ellipse passes through the box's corners, k =
hypot(w/Dx, h/Dy) (`blastAt`: it hides at least 95% of every hull's
pixels, a plain box-in-box fit 82%); the bloom reaches ~4.6 frame 0's
width, some 6 times the ship's. `destroy2` is left out (formats.md).
Each shot cues its weapon's `gbNN` at its take-off and its `ghNN` on a
hit, or `shield`, at its landing ("Sound effects"). At each landing a
floating text rises 30 px over the target's drawn box and fades over
45 ticks (`floats`, Go Mono 16 with a black shadow, several at once):
"-N" red for hull or structure damage, "0" light blue all absorbed,
"shields down" yellow for a knock-down; the replay's end cuts it.

**The round log** (`roundlog.go`): an ephemeral
overlay at the view's top right (the "Combat – round N" banner stays
top left), smallFace lines on `badgeBack` plates, right-aligned to the
view's edge less padX from its top plus padY, each at most half the
view wide (cut from the shooter's side, the arrow, target and outcome
kept), the last 8 shown, older ones scrolling off the top. In sync with
the replay (`layReplay` adds each round's lines with their ticks): a
"Round N" header with the round's first line, N the day's round number
at the star (`RoundsAt` when the replay is laid less the star's logs in
it, plus the log's place among them: a stepped round's is the banner's
of the round just played); a line per shot at its landing (`landsAt`),
"<shooter>  <weapon> → <target>  -N" red (then " (A absorbed)" when the
shields took part of it, `Fired.Absorbed`), "absorbed" light blue,
"shields down" yellow (`floatColour`), "no effect" for a shot that did
nothing; a planet's shot "<planet> (orbital)  <structure item>"; a
structure struck "→ <planet>  -N <structure>"; at the blasts' start
"<ship> destroyed", "<structure> razed" (white). Every name is led by
its owner's flag (`raceflag`, pixel for pixel; a shot line carries two,
the shooter's and the target's; a planet without owner none). Ours by
the ship's name; a foreign ship never: "<species> <design>" when the
player's Sighting scanned it as the replay's cast was taken
(`body.castSeen`, no fog leak), else "<species> <hull>". No line for
moves; the close's moves add none. The specials used at the star
are violet lines from their reports' `Event.Use`:
"<user>  <gadget> → <target>  <outcome>", a planet aimed at "→
<planet>  <outcome>", none (or a lane) "<user>  <gadget>  <outcome>",
names as above; a foreign user only when the player saw it (its cast
Sighting). The player's own show at once (`afterUse`); the bots', used
before a round's shots (`bot.Step`'s and `DayRun.Round`'s reps, the
close now played round by round in `finishDay` to keep each round's),
at that round's start with its header, before its first landing (a
round with nothing to replay still shows its header and uses where it
would have started). Their effects play there too: each use the player
sees (its line's fog rule) is a
replayed round of its own before the round's shots, one after the
other, from `SpecialUse.Destroyed` and `.Moves` (game: the ships out of
the lanes at the star gone or moved by the effect, diffed around it): a
ship it destroyed blows up at once (destroy.shp, its sprite replaced,
silent as a shot's kill), a ship it moved tweens From→To over
`moveTicks` with the engine's cue; the round's shots then fly from where
they left the ships. A ship leaving the star (Recaller) is not a move:
it stays drawn until the replay ends. The player's own use out of a
replay plays the same at once as a replay of its own over the ships as
drawn before it (`noteUse`, not a fight: Space goes on to the day);
during a replay its line alone. It stays while the system screen
does (until the screen is left; it held 4 s and
faded at first), a new system screen starting empty; a skip (Space, End
Turn, a click) shows every line at once; every later round replayed
there (stepped, the day's close) adds to it; a replay dropped unended
(a new day) shows its lines at once before the next's.

The close's moves (every day, fight or not): after the
rounds, one move-only phase over 30 ticks (`closeTicks`) takes each ship
drawn there to where the day's close (Step) put it, a lane entry to its
endpoint sphere as a round's (`closeMoves`: the ships before the close
diffed with after): no ship jumps any more. The planets glide over the
same phase from where they stood before the close to where its orbits
took them (docs/rules.md "Orbits": their Pos snapped with
the close's cast, `planetsNow`, drawn there through the day's rounds,
then along the chord, `glidePlanets`), so the phase plays every day at
a known star, its ships still or not; a ship riding its planet's orbit
is no move (no thrust). Outside a replay the planets sit at their Pos. A
ship that arrived or was built that day appears at the replay's end. Rounds laid while a replay
runs go on after its end (`layReplay`), never cutting it; the lane
flares start at the replay's end. Space, End Turn or a click in the view
skips it, the press spent on a fight's replay or one the climb waits
for; the close's moves alone are cut and the press goes on (Space ends
the next day at once, `replay.fight`); a new day replaces it. The day's
climb (Turn reports) waits for it: the view climbs at its end or skip
(`navDue`; at once, the fight would be gone), and
Esc off the screen climbs from the one below; under another screen it
waits, then plays when the system screen is back on top; the camera
stays live.

**Combat mode** (ours, `roundmode.go`,
`warning.go`; it replaces round mode, which paused the day
at each fight and locked the other screens: hard to follow). The rounds
play during the day, star by star, one panel button stepping them; the
model is rules.md "Combat rounds" (`Game.RoundsToday`, `Game.Engaged`,
`Game.StepRound`, `bot.Step`).

- The day never pauses. `endDay` is `beginDay` (the snapshots, `bot.Begin`:
  the bots Act) then `finishDay`: each combat star played by
  `DayRun.Auto` up to what is left of its caps (RoundCap, each bot's
  Rounds; the player plays the close's first round on its standing
  orders and armed shots, then passes), then `DayRun.Finish` (the
  close). The topmost system screen replays the close's rounds at its
  star, then the close's moves, as before. No lock is left: every
  screen opens and closes, Esc leaves (the quit prompt only on the last
  screen), F5 and the slots save, the fight waits as it is.
- Combat mode is derived, never stored (`systemScreen.inCombat`): the
  player `Engaged` at the screen's star and its rounds today below
  RoundCap. Opening a fight's system screen, or a war declared with
  ships of both present while on it, is combat mode at once; a fight
  that reached RoundCap is over until the close.
- The banner at the view's top: "Combat – round N" (N = the day's rounds
  there + 1: the round the next step plays) and the hint "End Turn,
  Space: next round". There is no Resolve button and no C key.
- A step (`app.stepRound`, `bot.Step`): one round at the star, the bots
  below their caps arming by `Bot.Round` on their standing orders, the
  player's armed shots and orders played (each ordered ship hops once,
  paying its energy). It replays after any replay still running, its
  lanes entered flare, its news goes to the log at once stamped today
  (no climb mid-day), and a toast sums the round up ("Round N: dealt …
  hits (… hull, … shields), took …, destroyed …, lost …", N the round
  just played, every zero left out; "Round N: no hits"). A refused step is toasted. F and the fire mode arm for
  the next round.
- Left button (End Turn): never closes the day. It skips a running
  replay first; in combat mode it steps; elsewhere every ordered ship of
  the player's in this system, out of the lanes, makes its next hop
  (`hopAll`): once each, one anim after another (`pending`, the Auto
  chain without repeats); "nothing to move" when none can. Repeat Move
  and Auto are unchanged; where the player is engaged every hand hop
  (an order's, Repeat Move, Auto, End Turn's) is refused, "moves play
  in the rounds".
- Space and the mouse's forward button: the top system screen's replay
  skipped when one runs, else a step on a system screen in combat mode,
  else the day's end with the warning. The starmap's next-day button
  and Skip warn too.
- The day's end warning (`askDay`, `restless`): when ships of the
  player's out of the lanes, with `UsableEnergy` (enough for a hop or a
  shot of one of its weapons), stand at a fight open to it, an app-level
  prompt comes up instead of the day, as the quit prompt (it takes every
  input while up): "Ships with energy left in combat:", a line per star
  by name (ascending; past what fits above the answers, "and N more"),
  then "End the day" (Enter, Y) and "Cancel" (Esc, N, the back button).
  A click on a star's line (`prompt.lineAt`: rows cut above the
  answers, so no click is read twice) cancels and pushes that star's
  system screen (an open one comes up, as the starmap's click). Skip
  asks once, before its first day, and stops before a later day the
  warning would hold, toasted "combat waiting at <star>" (without ending
  that day).
- The starmap marks each fight open to the player (`fightStars`: the
  same rule as combat mode) with a red ring past the colony rings and
  "R n" left of it, n the rounds played there today; a click opens it
  as any star.
- A fight opened at the close is a log line too:
  `Engaged`, "Combat at <star>: <empires> ships", the hostile empires
  seen there, for each side engaged at the close and not at its start
  (`Game.BeginClose`, from `bot.Begin`, before the bots act: a war a
  bot declares then is news; `Game.opened`); a fight going on makes
  none, nor one opened during the day (the player's war declared). It is the day's news and opens the system.

Settled: the close warns instead of pausing;
Resolve and C go; the right button stays Auto; Space and the forward
button step the round in combat mode and end the day elsewhere, with the
warning.

## Galactic display (starmap-screen, starmap-start/-endgame/-cheat1-3-dosbox)

The original: black, stars as small coloured dots cued by depth, lanes
as 1 px lines, light blue near and dark far; the species flag under the
home star, a coloured ring on colonised stars, a white ring on the last
viewed one; all star positions visible from day 1, lanes only once
discovered. The whole galaxy (cheat): stars spread evenly, the BLUE
lanes alone a connected, sparse, nearly tree-like web with many dead
ends and a few triangles; the RED lanes are long chords laid on top,
some crossing the map. The day counter "00173" sits in the top-right
box. After a turn, one dialog per event ("Construction of Factory is
complete on Verithil V. Verithil V has 0 free population for additional
projects.", "Go to Planet" / "OK").

Ours: the species flag (raceflag.shp, 15x25, frame = species, its top 4
px under the star, centred: measured) under every home star the player
knows, the player's own from day 1 (the flag marks the empire's first
colony, not `Empire.HomeStar`, the home world NewGame placed that the
Recaller and the capture victory use: after an invasion takes its home
world the flag moves to its oldest colony left, and an empire with no
colony shows none), in its empire's colour. An explored star carries
the original's four 1 px dots in the lane colour 4 px from its point, a
colonised one a 5 px circle in its colonist's colour (the species
colours of racering.shp by Empire.Colour: green, purple, yellow, pink,
brown, blue, orange); several colonists, as the original, concentric
circles 2 px apart, the dominant's the outermost (`starColonists`, the
original's rule unknown: the most colonies, then
the most people, then the empire order; the hovered star's "owned by"
names it); the circles dim with depth, softer than the lanes (0.65 of
their fade, about 55% at the far edge). The day digits and the two turn
buttons (one day, or until an
event, 50 days at most) come from `1starmap.shp` (`turn.go`, shared with
the log; a click on the counter opens the log, or closes it from the
log).

The stacked boxes are the original's five (from its
screenshots): the planets list ("Planets", 1starmap.shp frame 5;
under the label "N planets in M systems", `planetsLines`),
the fleet ("Ships"; under the label "N/cap", the fleet as the cap counts
it, `Empire.Fleet`, and "K building" when ships are under construction,
`shipsLines`), research (the topic and its days,
`researchLines`), "Special Ability" (frame 2, the shuttle: our ability's
text wrapped over two small lines and its state, ready / in N days /
passive; a press uses it, `Game.UseAbility`, the use or the refusal as a
toast) and "Species" (our flag at its right end, then the flag of each
empire met in empire order, a defeated one's dimmed, flush right in the
other boxes' 51 px picture, overlapping evenly past three; the Species
screen). The chrome's two rows of four round holes under the stack (the
original's map rotation and display filters, unneeded here) and the flag
row under them (the original's species filter, a dark band at
[474,633)x[441,473)) are the news box (Turn reports): one black box
[474,633)x[364,473) over all three and the stone between, 13 small
lines, with a soft border outside it (`drawNewsBox`: one native px of
black at 66% alpha, then one at 33%, so the black fades into the stone).
The upper row held a "Designs" button until it
moved to the fleet screen. Esc on the starmap asks
before quitting (`quit.go`, "Turn reports").

Fog of war: only the lanes touching an explored star are drawn. A
hovered star is one white line at the map's bottom, the boxes untouched:
"<name>, <type>, N planets, M lanes", then ",
owned by <species>" when settled (the ring's `Colonist`), or for an
unexplored one "<type>, unexplored, M lanes", M
its known lanes; a click on an unexplored star opens nothing.

The pick mode (ours, `navigate.go`): the ship panel's
Navigate… pushes a starmap for the selected ship (place "navigate",
stacked over its system), a yellow hint on the map's top ("Navigate
<ship>: click a system (Esc: cancel)"); the hovered star's
`Empire.Route` (cached by star and day, never per frame) draws its lanes
yellow and twice as thick over the others, even with L off, and the
bottom line says "<star>: N lanes, ~D days" (D the lanes' LaneDays, the
in-system legs left out) or the route's refusal in red. A click on a
reachable star orders GoToSystem and pops back; on an unreachable one
the refusal is toasted, the mode kept; Esc, the back button or a right
click pop without an order; the ship gone, lost or in a lane meanwhile
(a day passed) pops too.

The ships are the original's pennants (from a
DOSBox capture): `cos_ship` frames 0-3 by the depth's `brightness` (4x3
far, 5x5 near, two shades each) in the empire's colour, drawn at the
asset's size x2, the tip (the middle row's left end) on the point
marked. A star with ships out of the lanes, the player's or seen
(`ShipsSeen`), carries one per empire, the player's first, tips 9 px
right of the star, stacked 6 px down, drawn with the star in the depth
pass. Each ship in a lane has its own at its place, from the star it
left toward the far one (`Game.LaneShips`, `Starship.LaneProgress`): the
player's on any lane, the others' on a lane the player knows, so their
progress shows; the lanes' hide key (L) leaves them. The depth pass
(`mapOrder`) draws the lanes cut in 8 pieces (each
sorted at its farther end, overlapping the next by 1 px against
antialiased seams), the lane pennants and the stars far to near
together, so a near lane crosses over a far star and a far one passes
under a near star. Painter's order, as the original; Ebiten exposes no
depth buffer, and one would not handle the antialiased edges anyway.

## Turn reports: the news, the climb, the log

The original pops one dialog per event at the turn's start. Ours
shows a day's events where the player is, and
moves the view only when that place does not show them.

The news (`news.go`): one mini-log, the same widget in the same box on
the starmap, the planets, fleet and species lists and the system screen
(the starmap's news box, above, soft border included; on the system
screen over the info box's foot, under the F1 hint, with nothing
selected: the ship's and the planet's panels reach into it). Each screen
makes the same three calls: `drawNewsBox` in Render, `input` in Update
(the wheel, the hover, the press, taken first), `text` in Text. The
log's entries newest first, those of the last 10 days but never fewer
than 20 while the log holds more, a line each cut to the box's width,
paler with age (today bright, the highlight; 1-5 days 70%, older 45%);
the wheel over the box scrolls it (3 lines a notch, the camera
untouched), a scroll cue when it overflows. The texts are terse
(the original's sentences did not fit the box),
made in game/: no article, no "Construction of", the planet's and ship's
names whole, the other empire by its species name: "Factory built on
<planet>", "<ship> launched at <planet>", "<ship> refitted at <planet>",
"<tech> discovered", "Colony founded on <planet>", "<ship> destroys an
Orfa ship at <star>" / "<ship> lost to a Govorom ship at <star>" (the
other side's ship by its species alone, the star to go there;
a planet's gun by the planet's name),
"<item> razed on <planet>" / "<item> lost on <planet>", "<planet>
captured", "War declared by Orfa", "Orfa offers peace", "Peace with
Orfa", "Met the Orfa"; game's Grew is "<planet> grows to N", which the
window's endDay rewrites "Available pop on <planet>" for the freeing
growth it keeps, red as a line that left its colony idle (coming out of
"No Project / No Pop." an order is due;
`idleAfter`). A typical line stays under 40 runes (`TestEventTextsShort`
guards 48 over a bot-played galaxy); a hovered line still too wide
slides left through its strip of the box (`newsSlide`: a 60-tick rest,
1 native px a tick to its end, a rest, again), the others cut to the
box's width. The log popup's box is wide: no slide there. A linked line (as the log's, below) turns yellow hovered;
clicked it opens its screens over the current one; a press in the box
is the news' alone. On the starmap a hovered line with a star puts the
hover ring on that star, the system its click opens
(`starmapScreen.ringed`; a hovered star wins, none while
Navigate… picks; an offer's or request's line, from or to another
species, rings that empire's home star once the map shows it theirs:
known, a planet of theirs there, `newsStar`); on the system
screen a line of that system rings its planet, else the star
(`systemScreen.newsBody`), another system's line nothing.

The climb (`scope.go`), after a day or a skip with events: the places
make a tree, the galaxy (the starmap) at the root, a system under it, a
planet under its system. An event's place is its planet, else its
system, else the root; the diplomatic events, `AbilityUsed`,
`AbilityReady` and `Defeated` are the root's whatever their star; a
discovery with no place is left to the discovery popup, and a Built or
Unqueued of a colony running a project again after the close (its
construction queue took over: `queuedOn`) stays a log line. A screen
covers an event when it shows it already: the starmap and the log every
one; a system screen those of its system (its planets included); a
planet screen those of that planet; the planets list the colony events
(built, a ship included; grew, razed, invaded, repelled, unqueued) and
anything on a planet of ours; the fleet list the ship events (destroyed,
refitted, special used, scrapped, arrived, a ship built); the species
and negotiation screens the diplomacy; the other screens (research,
designs...) nothing. The top screen covering every event, nothing moves.
Else the view goes to the nearest place up the tree covering them all:
from a planet screen its system when they all are in it, else (and from
any other screen) the starmap. A screen of that place in the stack is
popped down to, as it was (camera, selection); else the stack drops to
its starmap and a fresh system screen goes over it. A system screen
replaying the day's combat holds the climb until the replay's end or
skip.

The log (`cmd/aiscendancy/log.go`), the whole history: a popup of 15
lines, a title (the day, the count) and a help line, centred over the
screen it came on, that screen drawn under it and dimmed; the newest at
the top pushing the old ones down, paler with age as the news. It never
opens by itself: Tab or the day counter (the starmap's and the turn
boxes') open and close it, Esc closes it, so does a click outside the
box. A line with a place is a link to its system then planet, a
"discovered" line to the research screen (a Xeno dig's discovery has a
place: its planet; an invasion's Invaded and Repelled lines, both
sides', open the planet; a diplomatic line opens the species screen,
below; an `Arrived` line, "<ship> in orbit of <planet>" or "<ship>
arrived at <star>" out of a lane, made only by the day's close, never by
a move on the system display, opens the planet or
the system; an `Engaged` line opens the fight's system, combat mode
above); a click on one closes the log and opens its screens over
that screen. `game.Event` carries Star/Planet (−1 none); the app keeps
every entry (saved with the game). A growth is logged only when it gives
a colony with no free people a worker while it builds nothing, idle or
on an ongoing project; the skip to the next event
stops there too. The popup has no turn buttons of its own: those of the
screen under it are under the dimming, a click on them closes the log.

The popups below share the prompt box (help.shp's dialog, frame 0) and
one set of rules: queued in this order, discovery, exploration, colony
report, first contact, proposal, then the ability notice, the victory last; up
over any screen, the log included, but each waits for a system screen's
combat replay to end, the quit prompt and the end prompt first; it takes
every input but F3, F5, F9 and Tab; Space does nothing under it (the
day's key, it neither closes a popup nor ends the
day); Enter, Esc, the mouse's back button, OK or a click outside the box
close one (the ability notice: not a click outside; first contact: a
right click too) and the next shows; the queues are window state, never
saved, dropped by F9 and New game.

The discovery popup (`discovery.go`): one box per
tech the player came to know, over the day's screen: the tech's restree
picture, its name, its source ("Our scientists", "Xeno Archeological Dig
on <planet>", the ability's line for the Dubtaks' theft, "Learnt from
the <species>" for an exchange), "k of n" when the batch holds several,
then a row per thing it unlocks (the research box's `unlocks`: icon
scaled into 80 px, name, the item's or part's help paragraph in full, a
hull's slot count; at most 3 rows sharing the room, the help small
enough for the longest, Energy Redirection's), in the prompt's box
(the flat box clashed), Research and OK the button
bar's halves. The queue is a diff of `Research.Known` around the day
(and around the window's own grants: the ability button, an accepted
exchange). Research (R, or the bar's left half) closes it too and opens
the research screen, the rest of the queue over it.

The victory popup (`victory.go`): the day the player wins (a
`Won` of ours, docs/rules.md "Winning") it shows the victory's end-*.txt
description, "Victory by <kind>, day N" and a rank, Continue alone; the
game goes on.

The ability popup (checked in DOSBox, `discovery.go`): a day
whose events hold the player's `AbilityReady` brings up the original's
notice "Your species has amassed enough energy to use its special
ability." (`prompt` with one answer: OK on the whole bar). The event
is a news and log line "Special ability ready", placed at the root, a
click on it (as on an `AbilityUsed`) opening the species screen, but for
a use with a place: the Govorom's "<planet> made rich" opens that planet;
the day skip stops on it as on any event.

The exploration popup (checked in DOSBox: the original's text
(help.txt event20), our picture: the star and its planets instead of the
ship; `explore.go`): a day after which one of the player's ships sits,
out of a lane, at a star it had never visited brings up "<ship>: We have
discovered the <star> System.  It contains N planet(s) and M star
lane(s).  We're awaiting further orders." under the star's suns.shp
picture and its planets' globes in a row, each named. Ours: the other
empires holding planets there are named before
the orders, "The A, B and C have colonies here." (`exploreOwners`, in
empire order), each such planet's globe carrying its holder's raceflag
at its top right. The queue is a
diff of `Empire.Visited` around the day, in star order, the first such
ship naming each star; stars learnt otherwise (an ally's charts, an
ability) give none, nor does a later visit. Go to System (or G) closes
it and opens the star's system on the starmap. No log line (the Arrived
event already is one).

The colony report (checked in DOSBox: the original's colony
report as it is; `colonyreport.go`): each colony the player founds (the
planet screen's Create Colony, or a Colonise order resolved at the day's
close; not an invasion) brings up help.txt's `colonize` "<our species>
Colony Report: Our empire now contains N colon(y/ies).  The largest
known empire belongs to the <species>, who have K." or, when no empire
in contact has more colonies, `maxcolonize` "... It is the largest known
empire in the galaxy." (ASSUMPTION: a tie counts as ours), the counts as
at the founding. In the box: the planet's name at the original's
(50,45), its globe big at the right of the upper half, the text below,
OK the whole bar.

The first-contact popup (`contact.go`): each
`Contacted` among the player's events of the day (never two bots'
contact) queues one, in the events' order: "First contact", the met
species' portrait in its colour ring, its flag in its colour
then its name, "Stance: <stance>" and
newgame.txt's description as "The <species> <description>", nothing
more of the empire (no fog leak). Talk (or T) closes it and opens the
negotiation with that empire (nothing when it is
already on top), the rest of the queue showing over it, as the
discovery's Research. The same queue takes a war declared on the player
and an alliance broken with it (`WarDeclared`, `AllianceBroken` in the
day's events, which hold only the others' acts: the player's own are
recorded in Talk): titled "War declared" /
"Alliance broken", their race's DA_DeclareWar / DA_BreakAlliance line
as the text ("Prepare for death, Snovemdomas.  The Shevar declare
war."), else "The <species> declare war on us." / "break our alliance".

The close of control's events (`game/defeat.go`): a
`Scrapped` line per ship lost over the fleet cap ("Loss of control of a
system forces us to scrap <ship>.", opens its system), a `Defeated` line
when an empire is out, its last colony lost ("We are defeated", or
"<species> defeated", at its `HomeStar`). The player defeated, no day runs
any more (turn buttons, skip, Space) and the end prompt comes up at once
(the view climbed to the starmap: `Defeated` is the root's; held back
while a log the player opened is up) (`ending.go`, the yes/no box of the
quit prompt): end-xtnc.txt's description (`help.ParseEnding`), New game
(Enter: the New Game screen, no game) or Quit (Esc: the quit prompt asks
first, its No back to the end prompt); F9 loads the quick save.

The quit prompt (Esc on the last screen, `quit.go`, the structure
dialog's box, help.shp frame 0, centred): "Quit AIscendancy?", Yes
(Enter, Y) / No (Esc, N), and raised buttons in the box (the game playable
with the mouse alone): Quick save (F5; the
prompt stays, greyed before a game exists), Quick load (F9; the prompt
closes on the loaded game, stays on a refusal, toasted), Save… and
Load… (the save and load screen), and under them Screenshot (F12, on
every screen too, `snap.go`): the screen as played, its popups but not
the quit prompt nor the toast, a PNG in shots/ beside the quick save,
aiscendancy-<date>-<time>-day<N>.png, the path toasted.

The scroll cue (`scrollcue.go`): a list that
overflows its box (the log, the fleet, the planets list, the species
rows) shows a yellow triangle in the box's right margin, up at the top
when rows are hidden above, down at the bottom when rows are hidden
below, blinking half a second on, half off (`drawScrollCues`). The
margin holds no content: every list ends at X1 − padX. The system
screen's lists box keeps its "a-b/N" hint line instead.

## Ship design (shipdesign-dosbox, chrome 8shipdes)

The original: title `<Name> "<Hull size>" (<N> Days)`, a power bar at
the left, the species' hull picture, the fitted parts as 68x52 tiles on
a grid (Small = 5 slots), a category button row (weapons, hull, scanner,
shield, drive, generator, special) and the list of parts allowed by
research. N days = industry cost / the planet's industry. Chrome
8shipdes.tmp: title box [7,307)x[7,106), part box [315,477)x[7,105),
button holes [485,516), [524,594) (the hull, 70 wide), [602,633) x
[7,38) and four 31x31 at [485..633)x[46,77), the list
[485,633)x[85,473), the picture box [7,322)x[114,337) (dkshipNN.shp:
315x223, four hulls per species), an L-shaped slot hole right of and
under the picture. 8shipdes.shp: 0-1 the empty-slot diamond (plain,
bright), 2 weapons, 3 the hull, 4 shields, 5 drives, 6 scanners, 7
generators, 8 specials (help.txt's button order: weapon, hull, defense,
engine, scanner, energy, special); gizmos.shp frame g = the 70x53 tile
of gizmo g.

Ours (`design.go`): the slots where the original scatters them, per hull
(the lattice measured in `formats.md`); a click on a listed
part picks it (framed green, a second click drops it; the part box keeps
its full description, a hover changing nothing), a click on a slot then
places it there, replacing the part in it, as often as wanted (the
hovered slot shows the held part faded); right click a slot to empty it,
elsewhere or Esc to drop the held part. A part keeps the slot it was
placed in and an emptied slot stays a hole (`Template.Slots`,
`game/slots.go`), through the saved template, the
built ship and its refit; a smaller hull drops the parts past its slots;
the hull button cycles the hulls research allows, the name is typed in a
text mode (F2 or a click on the title). The part box gives the hovered
part's facts, then its gizhelp.txt paragraph, the lines that fit.
The screen edits a `game.Template`:
on the template screen (the fleet's Designs, no colony) every category
list starts with its "Best <category>" entry once research allows a
gizmo of it, a Best slot shows the realised tile with a "best" label.
Best is a template's: a screen building at a
colony or refitting lists none, and a template loaded there (Load, Use
template) comes in with today's gizmo in each Best part's slot
(`fixedNow`). Only discovered material is listed, and
`Empire.AddTemplate` saves only that, so what the screen holds always
realises whole. Two toggles on the list's top:
**Compact**, the original's grid of tiles, and **Detailed**, one entry a
row, the tile then its name and, dim, its power and industry
(`entryCost`; the help's first line was tried and dropped, it
said nothing), 12 rows shown, the wheel over the list scrolling them
(the part box's help scrolls only elsewhere); a category switch goes
back to the top. The view is kept for the next design screens while the
game runs (`app.desDetailed`, not saved).

No templates screen: templates are the player's
own and optional, an empire starts with none (the stock Colonizer and
Lane Colonizer are the bots' and the fuzzer's, `ColonyTemplate`). Build
Ship (the Shipyard's dialog, the picker's Ship) opens a blank design
screen building at that colony; the fleet screen's "Designs" box one
building nowhere (no Build button). The title box gives the time to
build in days at the colony's industry today (`Colony.DaysFor`; "cost N"
on a screen with no colony). Its right column holds five gauges
(the original's five icon rows there, one icon per
4 units: `designBars`): weapon (the damage summed), shield (strength),
drive and scan (Level), energy (the generators' supply), each sum
printed, on one fixed scale (`desGaugeFull` = 40, four of the best
generators), graduated every 4 (the original's icon: a 1 px line a
little lighter than the fill or the empty gauge it crosses); past it, as
the original's rows run past their box, the gauge breaks its right edge:
the outline open there, the fill running on 10 px fading to black
(`drawBar`); the text keeps the left column, the
verdict on two lines at most.

Its bottom row holds three raised buttons: "Load
template" lists the templates in the list's place (a click loads a
copy, Esc or the button again cancels); "Save as template" beside it, or
S, opens a modal naming box (the quit prompt's box: a title, a one-line
field on the design's name, "Save (Enter)" / "Cancel (Esc)"); Save
stores the template under the typed name (a same name overwrites it: a
yellow "*" by the field and a warning line say so, nothing blocked) and
the design takes the name; a refusal (empty name, undiscovered material)
stays in the box, in red. "Build", at the row's right end, or Enter,
opens the same box in its build mode ("Build (Enter)"): the ship's class
is the last loaded or saved template's name, else the hull's
("Small"...; a hull switch drops the template's name: another hull would
never refit to it), the design's name (a refit finds a ship's template
by it); the field proposes the ship's own name, the numeral in it
(`Empire.ShipNameFor`: "Small III"), and Build
starts the design at the colony, the ship to bear the typed name as it
is (`BuildShipNamed`: none added; a name another ship bears is refused),
then the screen closes; a refusal (the fleet cap, Valid, the yard busy)
stays in the box. Cancel, Esc or the back button close the box, nothing
changed. While the box is up it takes every input (Space and Tab are
characters there). Read-only on a ship (`newShipViewScreen`, the Show
button of a ship's dialog on the planet screen): the ship's parts as
fixed tiles, no edit. Refit mode on a ship at Orbital Docks
(`newRefitScreen`, the ship dialog's Refit): its parts aboard edited by
hand, the hull locked, "Refit" in Build's place and "Use template" after
Save (the system screen's "Refit" below says how).

## Planets list (capture image0040, ours)

The original: one big row per colony, three to a screen (the star, the
globe, "Bob IV (Large Primordial Planet)", "Building Ship - 13 days
until completion", the research / industry / prosperity pictures, the
population pips), under "Orfa (82 Planets Occupied, 29 Systems
Controlled)". Ours (`planets.go`, the starmap's "Planets" box, in its
chrome like the fleet): 102 px rows, nine to a page, Colonies order: the
orbit's ten slots in miniature (two columns of five
18 px cells as the planet screen lays them, a structure's tile or a
ship's hull fitted to the cell, an empty slot nothing), the globe fitted
to 100x100 (never enlarged, so the size shows) over the colony's star
(`drawRowStar`: suns.shp drawn 110 px wide centred on the globe box's
top-right corner, clipped to the box, behind the globe: the original's
big star shrunk to a corner that still reads big; chosen of three
protos, never in front), the name, "(<Size> <Type>
Planet)", the star, "Building <project>" with its days or "No Project"
in red with free people (the idle ones are what to find; the Idle first
button below), "No Project/No Pop." in yellow when nobody is free to
work (nothing to do there yet), an ongoing
project's line in light blue (a colony parked on it
until people are free), the three output pictures at x1 with their
values as on the planet screen, the POP icons (10x8) and "POP n/cap". A
click (either button) opens the system then the planet screen. Side
boxes: help, the planets and systems counts, the home and cluster facts
the starmap's box 0 used to show; the day counter and the two turn
buttons in their holes, working as on the log (the fleet's too).

Stacked boxes 3 and 4 are the sort buttons (`colonyOrder`, state on the
app, saved with the game: `save.View`). Box
3 holds two toggles, each a frame green with its label when on
(`toggleRect`): "Idle first" puts the colonies with no
project on top, the red ones (free people), then the yellow ones
(`idleOrder`); "Planetary projects first" puts the light blue ongoing
ones (Scientist Takeover, Endless Party) before them instead of after
(dim while Idle first is off). Box 4's "Sort: <key>" cycles founded
(Colonies order), pop, industry, research, prosperity, queue (a right
click goes back; all but founded descending, ties in Colonies order). A
colony with a construction queue ends its project line with "queue N"
(nothing when empty). Size words are static.txt's (Enormous, not Huge,
fixed on the new-game screen too), and so are the type words (Mineral,
Supermineral, Special: planitem.txt's MINING, MOFFET, RICH are
internal).

## Save and load (capture image0064, ours, `slots.go`)

The original's Load Game: a green title bar, a row per save (number,
flag, species, "Day N", date and time, the portrait in its colour ring,
"Fleet", "Colonies", "Discoveries" each with an icon per unit, 14intel
frames 5-7 in the player's colour, then the empires met: their flag,
flanked by red bursts at war (frame 8), in a laurel allied (frame 9),
dimmed once dead). Ours, one screen in two modes on the starmap's chrome
(the right column one black panel with the news box's soft edge over
the chrome's boxes: the title "Save Game" / "Load Game" in a green bar
on top, then the preview, below; no turn buttons): 170 px rows (the
capture's 85 at zoom), five to a page, the quick save (`-save`, F5/F9's
file) pinned at the top as row "Q", then the slots (`slotNN.json.gz` beside
it) scrolling, and in Save mode a last "New save" row (one past the
highest number). Left to right: the number, the flag at zoom, species /
day / date (bigFace), the ring and portrait at x1, three blocks (a
smallFace "Fleet N" over icons at x1, 8 px apart, ten a line), two
columns of met empires (flag at zoom). Every file is Peeked once when
the screen opens (`save.Peek`, the head alone); another version or a
broken file is a dim row with the reason, never loaded, overwritable.
Load: a click on a loadable row loads it as F9 does (a fresh starmap,
the stack dropped). Save: the quick row and New save write at once, an
occupied slot asks "Overwrite save N?" (the quit prompt's yes/no box); a
save closes the screen with the toast. Reached from the quit prompt's
"Save…" / "Load…" buttons (Save… off before a game: on the New Game
screen only Load… opens it); `-screen save|load` for a shot.

The preview: the row under the mouse fills the panel
(clicks unchanged). A loadable one is read in full on its first hover
(`save.Read`, cached by path while the screen is up, dropped at Enter)
and shows the cluster turning in a 294 px square (cam's projection, 30°
above the plane, a turn every 40 s), strictly as the save's player knew
it: only the stars it knows (`Empire.Knows`) and the lanes between two
of them (a lane to an unknown star would place it), a settled star a dot
in its colonist's colour, the others small and grey; under it eight
small graphs of the player's History (Colonies, Pop, Industry, Research,
Prosperity, Fleet, Techs, Stars; two a line, the label and last value
over each, the line in the player's colour, the Stats screen's
`drawChart`). A refused or empty row shows its reason, a failed read its
error; nothing hovered, the help and the save directory.

## Stats (ours, `stats.go`)

The empires' `History` (docs/rules.md "History") as curves, on the
starmap's chrome as the other lists: a row of eight buttons over the big
area picks the series (Colonies, Pop, Industry, Research, Prosperity,
Fleet, Techs, Stars; the shown one framed yellow), the chart under it: X
the days, Y from 0 (or the lowest value) to a 1-2-5 step multiple at or
over the highest, its gridlines and labels, the day ticks under. One
line per empire in its colour (`empireColour`, the starmap's rings and
pennants), the player's twice as thick: the player, then the empires
met that are allied with it or defeated (a rival's
curves were cheating), the others fogged out entirely. The
news box holds the legend: each empire's species and last value
("(gone)" once defeated, its History kept). Box 0 the help, box 1 the
day and the sampling period; the turn box works. Opened by H anywhere in
a game (again: closes), or the Species screen's Stats box (stacked box
2, as the fleet's Designs); `-screen stats` for a shot. Day 0 has no
sample yet: an empty frame.

## Fleet (ours)

The fleet list (`fleet.go`, the starmap's "Ships" box): a three-line row
per ship: the ship's
sprite at the left in a small scene showing where it is (`sceneOf`, a 124
px cell clipped to the row, the backdrop at
x1: a quarter of its star's suns.shp frame in the top-left corner in
space, half its planet's globe at the left in orbit, half its lane's
blueexit/redexit sphere (frame 0) at the right in a lane, the ship's
nose under it; a build's ship night-blue above its progress, as on the
planet screen, beside the Shipyard's planitem frame), one text column
(the name, where it is, the order with its ETA), the design screen's
five gauges shrunk (`designBars` on the same fixed scale, a letter each,
W S D R E, stacked over the three lines, no number, `drawGaugeMini`:
the stacked bars chosen of four layouts), the system panel's hull
and energy gauges at the right on the first two lines, the consumables
left on the third (`Design.Consumables` on the
ship's `Aboard`: Colonizers, Invasion Modules and every once special,
Hyperfuel, the bombs, the lane specials...; each its gizmos tile shrunk
to a line's height and its count); the fleet cap in the side box. A
click (either button) opens the ship's system with the ship selected,
or for a ship in orbit its system then its planet's screen with the
ship's dialog open. Stacked box 2 is the "Designs" button (once the
starmap's; its label yellow hovered): a blank
design screen building nowhere, where templates are made away from a
Shipyard. The news in the starmap's news box. The order column says
"invade <planet>" for an Invade order (cut short),
"arrival N days" for a ship in a lane ("-> <star>, N days" lost its days
off the column; the where column names
the star), "being refitted, N days" for a ship under refit. After the
ships, a row per ship being built (`fleetBuilds`,
Colonies order): its hull's sprite, the name given in the build box or
else the class, "yard <planet>", "being built, N days", its design's
mini gauges and consumables, no hull or energy; a click opens the
colony's planet over its system; the side box says "N ships, K building"
(the cap counts both).

## Species (ours, diplomacy lots 1 and 2, `species.go`)

The original (species.txt's tutorial; docs/rules.md "Diplomacy"): the
starmap's Species button shows the flags of the species met; it opens
the Species screen, one button per species met, "colored icons that
indicate war or alliance", a flag with none "peaceful coexistence"; a
click on a picture opens the Diplomacy screen, where one "converses"
(the `.dip` texts): the options (war, peace, alliance, knowledge
exchanges, help asked at a star picked on a miniature Galactic Display),
third parties known to both picked by their portraits (species at war
"have no diplomatic options but to offer peace").

Ours, in the fleet's chrome (the starmap's, the big area black, the help
and the count boxes, the day counter and the turn buttons working, the
news in the starmap's news box): one row per other empire met, empire
order, "No species met yet" with none. A row (thick,
seven empires at most leave room for six) is their portrait (smraces,
100 px drawn 1:1, as Talk's), then the species' flag in its empire's
colour (raceflag.shp, as under a home star; no colour column), then
bigFace lines. Line 1: its name in that colour, the
stance (peace, war, alliance; "defeated" for an empire out of the game,
its row greyed), the systems of theirs we know, their ships we see, the
contact day ("day N"), our friendship toward them rounded
(`Relation.Friendship`, green from `FriendBar`: war on them barred). Line
2: their oldest pending offer to us in
words, yellow ("offers peace", "offers an exchange of stars", "asks war
on the Mebes"), blank without. Lines 3 to 5 (dim): our ledger about
them, broken between facts, the facts past the third line dropped whole
for a " …" (no number cut short). An ally's opens on its
wars and alliances, told while allied (`Game.AllyStances`): "at war with
the Minions and 1 unmet · allied with none",
an empire we never met counted, not named, a defeated one left out.
Then ("war 7 days ·
wars on us 1 · ships lost 1 · colonies lost 2 · intrusions 46 · refused
5 · allied 0 days · broke 0 · exchanges 1 · hospitality in 171 / out
10032 · granted 0": days at war, wars declared on us, ships we lost to
them, colonies they invaded, their armed ship-days in our systems, our
offers refused, allied days, alliances they broke, exchanges,
hospitality received and, from their ledger, given; last, "no war for
N days" while our friendship bars our war, `Game.WarBarred`, docs/rules.md
"Diplomacy"; the count box adds
every Alien Hospitality point our colonies made,
`Empire.HospitalityGiven`). Our ability is the last
stacked box, over the news: its text (smallFace, two lines at most) over
its state ("ready", "in 40 days", "passive"), a press uses it; a refusal
flashes red on the big area's last line.
A hovered row is yellow; a click opens the negotiation (below) with that
empire, every act and every answer living there; a click on a defeated
row flashes "the X are defeated" instead. The row a log link or a first
contact names is framed (the first row by default).

The screen's stats were cheating. Gone: the
attitude column (the bot's own mood toward us, `bot.Attitude`, read
against its thresholds), what each exchange would teach both sides
(`Game.Gain`: it told what they know), the detail block under the rows
and its buttons (Declare war, Offer peace or alliance, the exchanges,
Accept/Refuse): Talk does all of it.

Keys: S opens it from the starmap, the log, the fleet and the planets
list; wheel/arrows scroll; Esc back. A log line of a diplomatic event
(contact, war, an offer made, refused, accepted, an alliance made or
broken, an exchange made) is a link to it with that empire selected.
Stacked box 2 is the Stats button (the Stats screen). The player's own
species ability is a two-line button under the rows at the right (a
refusal flashes in red at its left): the `AbilityText`,
then its state from `Game.AbilityState`, "ready", "in N days" (dimmed)
or "passive" (dimmed, a press does nothing); pressed it is
`Game.UseAbility`, its reports logged, a refusal flashed as the others
("ready in N days", "at war with nobody"; the fourteen timed
abilities are all modelled, `abilities` in game/ability.go).

## Negotiation (ours, diplomacy lot 3, `negotiate.go`)

The original's Diplomacy screen (13negot) converses with the `.dip`
texts; ours keeps the texts, not the chrome: the starmap's chrome as the
Species screen's, in the big area their round portrait (smraces, frame =
Species, x2) top-left, beside it their flag in their colour,
then their name, colour and stance (no
attitude: the bot's mood is cheating), then our
friendship toward them, green with "no war for N days" while it bars our
war; under it a speech box with our last line (dimmed) and their line
(wrapped to the box, six lines at most); then our lines as a menu, each
the player's `DA_` text from player.dip (%y their species, %m ours),
hovered yellow. The lines by stance: at war offer peace; at peace declare war, propose an alliance,
the three exchanges; allied break the alliance, the exchanges, and the
four third-party requests (offer peace to, declare war on, break with,
ally with a third), each opening a sub-menu of the species the player
has met ("the Mebes", a "back" line; whom they met stays unknown, one
they never met refused as a wrong stance), %t
filled once one is picked; their offer gone unanswered under the screen
(lapsed, or answered in a popup) leaves "Offer withdrawn." as their line;
`DA_NoDipAction` last returns to the Species screen (Esc and the mouse's
back button too). A line is dimmed when the game would refuse it for a
reason the view reads (an offer of ours pending or the cooldown, no
third species to name; never an exchange, what they hold is hidden) but still
said: the game's refusal becomes their line in red ("offered 2 days
ago"). The menu is rebuilt from the game every frame: a stance changed
by a line or by a day ended under it shows at once.

A line is the game's call (`DeclareWar`, `BreakAlliance`, `OfferPeace`,
`OfferAlliance`, `OfferTrade`, `Request`), the reports logged. An offer
to a bot-played empire is answered at once: the offer just made (found
among `PendingOffers` by From, To, Kind and Third), `Bot.Decide`'s
verdict applied by `Game.Answer` (logged), their line the race script's
`DR_ACCEPT_<Key>` or `DR_REJECT_<Key>` (Alliance, OfferPeace,
TradeStars, TradeStarlanes, TradeResearch, and the four `…OtherRace`),
%y our species, %m theirs. A unilateral act answers
`DR_RESPONSE_DeclareWar` / `DR_RESPONSE_BreakAlliance`. An empire with
no bot (another human: none today) keeps the offer pending: "They will
answer later." Opening, their line is `DR_RESPONSE_Discover` on the
first talk with that empire in the session (not saved), then
`DR_NoDipResponse`. The scripts are read once from the archives
(player.dip, raceNN.dip with NN the Species index, `app.scripts`);
without them (or a key) our plain lines and the game's report texts
stand in. Not done: the fleet requests (`DA_HelpControlSystem`,
`DA_DiscoverAskHelp`).

Their offers (the Species screen's Accept/Refuse moved
here): while an offer of theirs to us is pending (`PendingOffers`, From
the empire talked to), the oldest is their line, said with their race
script's `DA_` text as the original's proposals (`DA_OfferPeace`,
`DA_ProposeAlliance`, `DA_TradeStars`, ..., the `…OtherRace` requests
with %t the third; %y our species, %m theirs; without it "The Minions
propose peace.", "The Orfa ask for war on the Mebes."), on entering or
as soon as one shows (made under the screen, or next after an answer).
The menu is then two lines and the leave line (the stance's lines
hidden until it is answered), Accept and Refuse, our player.dip
`DR_ACCEPT_<Key>` / `DR_REJECT_<Key>` texts (the same keys as their
answers; plain "Accept" / "Refuse" without), `Game.Answer` called, its
reports logged (an exchange's discoveries queued); their line then the
next offer, else the reports' texts, and the menu back to its stance's
lines. An exchange stays blind both ways: no line says what each side
would learn, and ours is never dimmed (one with nothing either way is
refused once said: "nothing to exchange with the Mebes").

The proposal popup: an offer or request made to
us during a day (pending at the close, not at the day's start) brings up
a popup as the first contact's: the maker's portrait in its colour ring,
its name, the stance, their DA_ line as Talk says it, then "Unanswered,
it lapses as a refusal in N days." (OfferExpiry). Four buttons: Accept
(A) and Refuse (R) answer it at once (`Game.Answer`, logged), Talk (T)
opens the negotiation with them (their offer on top), Postpone (Esc,
Enter, the back or right button, a click outside) closes it, the offer
still pending. One per offer, in the order made; after the first-contact
popups, before the ability popup; a head answered elsewhere is dropped;
never saved (`proposal.go`).

## The selected ship's panel (ours, on the system screen)

The original's selected-ship panel (rows of icons, a power column) says
little, and a separate screen of ours was wrong: the firing must happen
on the system screen. So the system screen's info box shows
the selected ship: our flag then its name (as a foreign ship's panel,
`drawPanelFlag`); the design and the shield row
(`shieldrow.go`): "<design>  " then `shipicon.shp`
frame 8, the 13x13 shield sphere (frames 6-11 are the 13x13 part
icons, formats.md), pixel for pixel, one per hit the shields can still
take (`Starship.ShieldHits`: Power / ShieldDraw while they absorb) 2 px
apart, then " <Absorb>"; off, dark, or on with no energy for a hit's
draw: one icon dimmed and " off", " down" or " no power"; no shield
part: the design alone; more icons than the line holds: one and " x<n>
<absorb>" (" x∞" for shields that draw nothing, a Wave Scatterer:
`game.NoShieldFall`), the design cut so that row fits (`layShields`
lays it out for both the icons' Render and the text's Text); the hull and the energy as
gauges (`shipBars`: the hull's green over a half
left, yellow over a quarter, red under; the energy's blue, of the day's
supply; the numbers after each); where it is and its order with the ETA
(a GoToSystem order reads "-> <dest> (N lanes)", N the lanes left on
today's route, here and in the fleet list); then its parts as half-size
gizmos.shp tiles in four bands, weapons, shields, specials (the
clickable ones first, then the Colonizers with their cargo and Invasion
Modules), core (drives, scanners, generators), down to the hover line's
three rows (the bottom had stayed on the chrome's box, so a ship with all
four bands
lost its core); best effort past it, the last groups dropped and counted
after the last band's label ("core +2"); identical parts once
with an "xN" badge (a limited special's uses left today, a Colonizer's
cargo, else the parts aboard, a weapon's too: a
spent shot looked like a part gone; its shots on a line under the tile,
"used U/M", M = its parts × Uses, `usedText`, the weapons' rows
`usedH` taller), the shields' on/off at the foot ("down" under a Shield Blaster
or once a hit found too little energy for the draw; switching on short
of the draw is refused), "no power" in red on a weapon or active special
with a use left but less energy than its Power (the original's
"insufficient power"), "no gen" in red on a weapon with a part left to
arm once the armed shots take every generator, the weapons band's label
counting the round's actions left in combat mode ("actions 3/5": one per
generator, a shot or a move taking one), all repeated in the hover line at the box's
bottom. The chosen weapon is framed yellow, a switched-on shield light
blue (the original frames it; to check on shots), the hovered tile
white. An armed shot shows until its round: its
weapon's tile framed red inside (`tileArmed`, "armed N for the next
round" in the hover line), a red dashed line in the view from each of
the player's armed ships and colonies to the target (`drawArmed`); the
round clears Armed, so both go.

Click a weapon: it becomes the one to fire and the screen enters the
fire mode (the prompt in the foot): a white line to the hovered hostile
ship when the shot would go (grey when CanFire refuses), a click on it
arms the shot (Empire.Arm; it fires in the day close's first round,
Game.Round) and ends the mode, a right click or Esc cancels; F still
fires the ready weapon at the hovered ship. Click a shield: all shields
toggle. A Stop button (`stopRect`, right-aligned on its own row under
the where and order lines, which keep the full width) is key X for the
mouse: shown for the empire's own ship only, lit while it has an order,
armed shots or the auto mode runs, dim otherwise;
a click, as X, cancels the order and disarms the shots (and the fire
mode), hover says "stop: cancel the order and the armed shots (X)". Navigate…
(`navRect`, left of Stop, as tall) is key N: shown with Stop, lit when
`Empire.CanNavigate` accepts the ship (a lane drive, an engine, not
being refitted), dim otherwise; a click opens the starmap's pick mode
(Galactic display).

The Colonizer and Invasion Module tiles order nothing (their orders are
the planet screen's): the hover says "colonise from the planet screen" /
"invade from the planet screen" in orbit, Usable's refusal elsewhere
("not in orbit"); their badges are the live Cargo and Invaders. The
other specials act on a click when `Starship.Usable` says "use" (lot 1,
the self-targeted ones): the view calls `Empire.UseSpecial` and shows
the report line in the foot (normal colour; a refusal, red: the model's
words, "Accutron: no use left today", "hull is full"); a special with a
daily limit shows its uses left as xN like a weapon, the Accutron's tile
says "boosted" and the Invulnerablizer's "invulnerable" for the day; a
one-time device (Hyperfuel, Invulnerablizer) vanishes from the panel
once used (the tiles are laid again from the new Design); a Recaller
sends the ship to its home star and the screen deselects it when that is
another star. Passive specials say "X is always on".

A ship-, planet- or lane-targeted special (lots 2-3, read off its flags,
the ship flag first: the Myrmidonic Carbonizer, whose flags name lanes
and planets too, aims at a hostile ship as the model does; Sacrificial
Orb and Plasma Coupler are the ally-aimed ones, Tractor Beam and
Hyperswapper the any-ship ones, view-side lists; a tile grouping two
parts of one gizmo uses the first that Usable accepts, `groupPart`)
enters the fire mode instead (`aimSpecial`, the part): the foot says
"use <name>: click a hostile ship" / "click a ship of yours or an
ally's" / "click a planet" (Phase Bomb) / "click any ship" (Tractor
Beam, Hyperswapper: a hostile, an own or an ally's, all but the user) /
"click a star lane opening" (Lane Blocker, Destabilizer, Magnetron,
Endoscope: a lane body, its Index in `System.Lanes`, the range to the
endpoint's Pos); the line goes to a hovered candidate of that kind,
white within the special's Range × RangeUnit of the user (the model's
distance, never boosted), grey beyond. The click calls `UseSpecial` with
the body as a Target (another empire's ship as its Sighting, an own ship
as Mine, a planet or a lane opening by its index; the view filters
nothing more, a wrong target is the rule's refusal in red), the report
as for the self-targeted ones, the tiles laid again (the self-targeted
Gravimetric Catapult, Gravity Distorter and Condensor have no aim: the
tile uses them at once, the displaced ship stays selected, the bodies
rebuilt); the mode ends on the click, a right click, Esc, or when the
aimed part shifted (a part lost at a day's close). The news lines are
not shown while a ship is selected.

Refit (`rules.md`; since the orbit moved to the planet
screen, its ship dialog's Refit button does this, the system screen's U
key is gone; by hand, as antag.exe lets you
change the parts by hand): in orbit of one of the player's colonies with
Orbital Docks, the button's hover offers `refit to <class>: +<cost>
industry, <days> days` when the ship's template (`RefitTarget`, named as
its class) would be accepted, its `RefitCost`, the days at the colony's
industry (at least 1; "no industry" when it makes none), else `refit:
edit the parts at <planet>'s Orbital Docks`. The press opens the design
screen's refit mode on the ship (`newRefitScreen`); with a project
running at the colony it first puts up the planet screen's cancel
question, whose Yes cancels it and opens the refit
screen at the next Update (`refitAsk`/`refitDue`, as `pickDue`), No
leaves all as it was. The refit mode edits `Starship.Aboard`'s parts
(the spent modules gone), named as the ship's class; the hull button
refuses ("a refit keeps the hull"), so does Load on another hull's
template; "Use template", between Save and Refit, is shown when
`RefitTarget` accepts the class's template and loads that Template (its
Best parts stay Best); title lines 2 and 3 read `refit +N industry` then
`D days` (`RefitCost` against the parts aboard, `Colony.DaysFor`, "busy:
" while a project runs), the verdict is `CanRefit`'s refusal or "valid".
"Refit" in Build's place, or Enter, starts it (`Empire.Refit`, the
colony's project, the design under the title's name: the ship's new
class, so a later Use template finds a template saved under it) and pops
the screen; a refusal (nothing to refit, busy, the ship gone from the
orbit under the screen) stays on it. The order line then says `being
refitted: <days> at <planet>` (the days first: the panel cuts a long
line) and every order is refused by the rule. On the planet screen the
project reads "Refit of <ship>" (panel, cancel question), nothing rises
at a slot, the Docks are not lit as building; cancelling it frees the
ship.

## The selected planet's panel (ours, on the system screen)

The original (the capture image0039): selecting a planet frames it
white in the view and puts its picture and name in the top-right box,
its orbital weapons listed below with their pictures and names. Ours
(`sysplanet.go`): a first click on a planet, in the view or on its row,
SELECTS it (a white square around its sprite, outside the guard circle);
a click on the selected planet, or on its globe in the panel, opens its
planet screen. One selection at a time: a planet click drops the
selected ship from a list row (in the view, a ship selected, the click
orders the orbit), a ship click drops the planet; a click on nothing
with no ship selected, a right click or a filter press drops one; Esc
(or the back button) drops the planet or the ship first, after a running
move or fire mode, and only the next one leaves the screen. The info box
(the ship panel's place) shows its name over
its globe (`planalNN` at its size, fitted to a 128 px disc at the top
right), left of the globe its type and size, its owner ("our colony",
"<species> colony, <colour>", "no colony"), its guard ("guarded:
<colour>"), the ruins; then its orbital structures as planitem tiles at
scale 1, one per slot in slot order, in three bands: weapons, shields,
structures (docks, the Orbital Cloaker and the rest). On the player's colony a weapon wears
"xN", its shots left today (Shots − `Colony.OrbitShots`), and a weapon
or a shield its hit points left ("11/15": `Planet.DefenceLeft`, the
owner's only); another empire's planet shows the same structures bare
(its damage is hidden, as in the original). The hover line at the foot:
"<structure>: 7 dmg x1 a day, range 40, 1 shot left, 15/15 pts" (the
owner's facts only on its own). Another empire's Orbital Cloaker
(docs/rules.md "Fog of war", the cloakers) hides the orbit:
no tile, "orbit: cloaked" in the guard line's place, no guard ring, and
its ships in orbit are no bodies (`sighted` pairs each body with its own
sighting, `Sighting.Of`); its Surface Cloaker hides the ruins line.

Firing from a planet: a press on one of the player's weapon tiles enters
the planet's fire mode (framed yellow, the foot "fire <structure>: click
the target (right click cancels)"): a white line from the planet to the
hovered hostile ship (grey once the weapon has no shot left today; the
range is Round's: no game helper tells it before), a click on it arms
the shot (`Empire.ArmPlanet`, fired in the day close's rounds after the
ships'), its refusals ("already armed this round", "no shot left today",
"the target is in orbit of <planet>: fire at the planet") in the box. A hovered
planet in this mode is labelled as in a ship's (below) with
`Empire.CanArmPlanetAt`'s verdict ("fire at <planet>", or "out of
range", "cannot fire at itself", "not hostile"...), a click arms
`Empire.ArmPlanetAt`. Firing at a
planet: in a ship's fire mode, a hovered planet is labelled under it
with `Game.CanHitPlanet`'s verdict, "fire at <planet>" (yellow, a white
line) or its refusal (red, a grey line: "not hostile", "nothing to hit
in orbit"); a click arms `Empire.ArmAtPlanet` (the first of the tile's
parts that accepts), its refusal shown otherwise.

The Tractor Beam (docs/rules.md "Planets in combat"): on the
player's colony holding one (a surface structure), its planitem picture,
shrunk into a tile, comes last in the structures band, "xN" its pulls
left this round (one per beam, `Colony.Tractors`); the hover line gives
the pull, the range and the pulls left. A press enters the tractor mode
(framed yellow, the foot "Tractor Beam: click a ship in space (right
click cancels)"): a white line to any hovered ship, own or foreign (grey
when `CanTractor` refuses), a click pulls it at once
(`Empire.UseTractor`), the report in the box ("<planet>'s Tractor Beam:
<ship> is moved"), a refusal in red. A pull takes the planet's round
(the rule: its armed shots dropped, its weapons refusing "Tractor Beam
used this round") until the next round (a step in combat mode) or the
day's close. Esc, a right click, another tile's press or a new selection
end the mode, as the fire mode's.

## Research (research-dosbox, chrome 20res)

The original: "<Species> Knowledge" title, black area, techs as ~55 px
disks (known = picture, available = empty ring) joined by lines, nearer
ones bigger and brighter (a 3D cylinder), a big empty box at the right,
two buttons below. Chrome 20res.tmp: big area [7,477)x[46,473), right
box [485,633)x[7,434), button holes [524,555) and [563,594) x [442,473);
restree.shp = 66 pictures 50x50, one per tech in restree order.

Ours (the table view was dropped, the tree reads better):

- The tree (`restree.go`, the original's look, view only): the techs as
  bubbles on a vertical cylinder of radius 1, level ℓ at height
  −0.85·ℓ, seen from 15° above (Dist 8, Focal 2340: 292 px per unit at
  the axis, a 60 px ring 90 px there, about 3 levels in the area; farther,
  levels 0.85 apart, between 0.55 and 1.2). Level = 0 without
  prerequisites,
  else 1 + the prerequisites' highest (18 levels of 1 to 6). The three
  roots at −120°, 0°, +120° (0° the front); a deeper tech wants the
  circular mean of its prerequisites' angles, then 500 relaxation passes
  push a level's circular neighbours to at least min(2π/n, 50°) apart.
  Shown: known, studiable and the topic; a locked tech is hidden (as the
  original at the start shows the three roots alone), so a tech known out
  of order (a dig, a trade) links only to a prerequisite known or
  studiable. Lines (light blue, dimmed by depth) and bubbles paint back
  to front, a line at its middle's depth; bubbles scaled
  and dimmed by depth: resring.shp's ring (frame 0 orange-red the topic,
  1 blue, 2 light blue hovered, 3 pink selected; a known tech's frame 2
  greyed and tinted sea green, `resKnownTint`) over restree.shp's
  picture, whole when known, grown with progress/cost while unknown,
  the topic or not (a dropped topic keeps its points), a dot for the topic
  at 0, else none. Controls as the starmap's on a
  cylinder: drag turns (dx) and moves along the axis (dy); wheel zooms;
  arrows turn and move; R re-centres; a click on a bubble studies it.
  Entering centres on the topic (else the first studiable tech). The
  chrome's two holes are the original's turn and scroll buttons
  (20res.shp frames 0-3: the left hole's left/right halves, the right
  hole's top/bottom halves, origin top-left); ours covers both with one
  Select current button (a black box with the news box's soft rim): it
  drops the selection and turns the view back on the topic, as R.

A click on a studiable tech studies it, on any other selects it
(pink ring), on nothing drops the selection: the topic
is untouched (read a known tech's unlocks and their
help). The right box shows the hovered tech, else the selected, else the
topic: picture, its name centred under it (green once known, else
white), "N days remaining" at today's rate, centred as well (no points: the original is
vague on purpose) or "Researched" once known, then what it
unlocks, each with its icon at zoom (an item's planitem frame, a part's
gizmos frame) and its name beside it, wrapped, in bigFace; no
prerequisites (the tree shows them) nor "unlocks:" header; a hovered
unlock row puts its name and its
planhelp.txt or gizhelp.txt paragraph over the tree on a black band
rising from the area's bottom in a 24 px face, smallFace ×2 (at most 8
lines, 240 px); a hull: none. Reached by
the starmap's Research box and by a "discovered" line of the log.

## Tutorial (spec, not started)

Specified, parked ("un gros morceau"). The
original's seven tutorials (tutindex.txt) take the mouse and play a
script, pausing on text boxes ("Continue Tutorial"); any key or click
aborts. Their files: docs/formats.md (Prose). The `.bin` is raw input
at the original's 640x480 positions, timed against its own rules: on
our layout and our rules it clicks the wrong things and desyncs after a
day, so "fixing its moves" means rewriting it blind. The spec:

1. A semantic script of ours, one per tutorial: steps "box N", "move to
   star X", "click button Planets", "click square (3,4)", "end the day",
   "wait". Each target resolves at run time from the screens' own hit
   rects, so the script survives layout changes; the cursor
   (data/mouse.shp) glides there on a smooth curve.
2. Driven by input injection: the screens' mouse and key reads go
   through a swappable seam (as `keyJustPressed` for the tests); no
   screen knows the tutorial, what is shown is the real thing. A real
   key or click aborts.
3. A fixed start and deterministic bots, so a replay is identical; a
   test plays each script to the end and fails on a target not found
   (the guard when a screen changes).
4. Texts: the original's boxes read from the archive at run time (as the
   .dip and the help, nothing committed), overridden by ours where our
   screens differ (exit corners → Esc / back, shift help-click → hover).

Open, proposed: first the engine plus tutorials 1-2 (Introduction, New
Game); the start from mksave rather than the `.tsv` converted (checked
tutorial by tutorial); passive like the original, no "your turn" steps.
