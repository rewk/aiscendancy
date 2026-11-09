# The game's files

Established by reading the COB archives in `~/dosbox/Ascend`. The
mechanisms live in the package comments of `cob/` and `shp/`; this is the map
of what each file holds. Measured rectangles are in the original's 640x480
pixels; the views draw at 1280x960 and write them through `orig()` (x2).

## Archives and sprites

- The game runs in **640x480** (VESA), not 320x200. Screen chrome banks are
  numbered: 0opening, 1starmap, 2savegam, 3cfgnew, 4loadgam, 7racesel,
  8shipdes, 9endgame, 10status, 12start, 13negot, 14intel, 16battle, 17np
  (the planetary display), 20res. `.tmp` banks are SHP too.
- **SHP**: `1.10`, count, (offset, palette offset=0 always) pairs; frame
  = u16 h-1, u16 w-1, u16 **cy, cx** (y first, like the size), i32 box
  x0,y0,x1,y1 inclusive relative to the origin; rows of RLE (00 end, 01
  n skip, odd = literal b>>1, even = run b>>1 of next byte). Boxes may
  spill outside the canvas (ships, explosions). 218 banks, 1567 frames,
  26 empty slots (garbage box). Palettes `data/game.pal`,
  `data/logo.pal`, 6-bit. `.haz` = 256-byte colour remap tables (not the
  empires' colours: those are game.pal's 4-entry ramps from 16,
  docs/screens.md "New game").

## Rule tables (typed into `game/`, each with a test re-reading the file)

- **planitem.txt** (cob0): 39 items (22 surface, 9 orbital, 8 projects;
  `39 22 9` line), columns NAME IND RES PROS MPOP UPOP COST RESEARCH-REQ
  FLAGS; 11 planet types in this order: Husk, Primordial, Congenial, Eden,
  Mineral, Supermineral, Chapel, Cathedral, Special, Tycoon, Cornucopia
  (static.txt's words; planitem.txt says MINING, MOFFET, RICH), each with 4
  percentages (black, then red = industry, green = prosperity, blue =
  research, the rest white), and two 5x15 tables = for sizes 1-5 and grid
  rows j, the first i and the count of squares. That IS the iso layout: a
  disc, centre cell (5,5), sizes hold 7/11/23/45/73 squares. Interstellar
  Trucking has ResearchReq 254: not a tech, never allowed. The fan manual's
  own building table disagrees with the file (Agriplot 1 against 2): the
  file wins, and the game's displayed values confirm it.
- **restree.txt** (cob0): 66 techs, "Name flag cost prereq... 255", ^ for
  spaces, 0-based indices (the fan manual counts from 1); three
  prerequisites point FORWARD in the file (5→6, 19→21, 55→56) but no cycle;
  only 3 roots (Orbital Structures, Tonklin Diary, Xenobiology). An
  abandoned table in capitals follows: ignored.
- **gizmos.txt** (cob0): 76 ship parts (10 weapons, 6 shields, 5 drives, 6
  scanners, 5 generators, 44 specials) with a documented header: Cat Power
  Range Level NumUses Industry Special1 Special2 ResReq flags. The header
  admits "a hack in Ship::init_global_gizmos() that alters their cost as
  read in": the guide's Small colony ship costs 207 against our 155 + hull.
  Ours follow from `game.NumGameGizmos` (76): the Space Habitat (population
  transport, `Empire.Load`/`Unload`). Its tile, `gizmos.shp`
  frame 76 (the bank has 76), is grafted in memory at preload
  (`assets.Graft`, cmd/aiscendancy `graftHabitat`): planitem.shp frame 3
  (the Habitat) on the 70x53 gizmo canvas, origin (34,36), its diamond's
  top vertex at (35,18) on the floor plate, its green ground remapped to
  the plates' greys (game.pal 181,187,191,189,192,199 -> 7,6,5,4,3,2).
- **starstat.txt**: 13 star types, share of stars and mean lanes per
  star (2.24 overall); their names are static.txt strings 178-190 in
  suns.shp order: Blue/Pink-Red/Red/Orange Giant,
  Blue/White/Yellow/Light Yellow/ Orange/Pink-Red Medium, Blue/White/Red
  Dwarf (`StarType.String`). **names.txt**: 100 star names (two blank
  lines and a ^Z, not 102). **static.txt**: the five densities (Vacuous,
  Sparse, Average, Dense, Very Dense), hull names. **cosmos.txt**: STARS
  50, RACES 5. **windows.txt**: COSMOSWnd NUM_STARS 100, NUM_LANES 100.
  **raceNN.dip** (21): first line `Diplomacy text for the <Species> -
  SA_<Ability>.`; race02 lacks the SA_, race03 misspells Kambuchka,
  race18 is Chronomyst. Their grammar (package `dip`, `dip.Parse`;
  player.dip the same): the comment header, `DIPVALUES` and its table
  (typed into `bot.Personalities`, ignored by the parser), then an
  `ACTIONS` line and `DA_<Key>` lines, a `RESPONSES` line and `DR_<Key>`
  lines (`DR_ACCEPT_TradeStars`, `DR_REJECT_OfferPeace`,
  `DR_RESPONSE_Discover`, `DR_NoDipResponse`...), `END DIPLOMACY`. Each
  key line is followed by its text up to the next blank line
  (whitespace-only counts as blank; every text in the files is one line,
  up to 125 characters). CRLF; a few keys carry a stray trailing comma
  (`DA_BreakAlliance,` in 18 files, `DR_ACCEPT_OfferPeace,` in 19),
  stripped. Placeholders `%y` the one spoken to, `%m` the speaker, `%t`
  a third species, `%s` a star system. Every file of the 22 holds the
  same 15 DA_ and 30 DR_ keys. **newgame.txt**: the 21 species
  descriptions in the same order. **planhelp.txt** and **gizhelp.txt**
  (ascend00.cob root; package `help`, `help.Parse`): one help paragraph
  per planetary item and per ship part, keyed by the `help plitemNN` /
  `help gizmoNN` line, NN two digits = our `game.Item` (39) /
  `game.Gizmo` (76) index. An entry is `#<Name>` (spaces as `^`), a `#`
  stats line, the help line, then `text` ... `endtext` blocks among
  `pos`, `shape`, `button`, `title`, `msg` lines: planhelp has two, the
  first just `%s` (the dialog's title), the second the paragraph;
  gizhelp one, the paragraph with `%s` for the name. The paragraph is
  the entry's last block, its lines joined by a space (blank lines
  dropped, the original's double space after a full stop kept). Key on
  the help line, never count `#` lines: gizhelp has a stray `#Passive.
  always in effect, takes no power.` before gizmo41's text. `help end`
  closes; CRLF and a trailing ^Z. Three names differ from ours
  (Torroidal Blaster, Orbital Shields, Tractor Beam Element): the index
  rules, `%s` takes our name.
- Prose: help.txt (every screen's help, the rules in words), planet.txt,
  ship.txt, galactic.txt, species.txt, beginnew.txt (tutorials,
  tutindex.txt lists the seven: tutintro, beginnew, galactic, battle,
  planet, ship, species). In cob2 each tutorial's `<name>.tsv` is an
  original save (savedump reads galactic.tsv: 75 stars), the game it
  plays on, and `<name>.bin` its recorded input: 24-byte records
  (kind, x, y, 0, tick, arg) after a short header (tutintro.bin
  decoded): kind 1 a mouse move in 640x480 px, about one per tick;
  2 a button (y 1 down, 0 up); 0x840003 shows the next text box (one per
  `help infoNN`); 0xFFFF0005/0xFFFF0007 and 0x80006 unread (window
  events, arg an id). Raw positions on the original's layout: not
  replayable on ours (docs/screens.md "Tutorial").
- **The introduction** (`data/intro65a.flc`, cob2, 19 MB): Autodesk FLC,
  magic 0xAF12, 640x480x8, 1143 frames plus the ring frame, speed 71 ms
  (subtitle.txt overrides it: 12 fps); chunks COLOR_256 (4), BYTE_RUN
  (15), DELTA_FLC (7), PSTAMP (18, skipped), a 0xF100 prefix block.
  Package `flic` decodes it, checked frame by frame against ffmpeg's
  decoder. `subtitle.txt` (cob0) times it: "KEY value" blocks (FRAME_RATE,
  DIGITALMUSIC/TUNEID = music.txt's index, FADEMUSIC, SUBTITLE with TEXT
  in underscores and FRAME_START/END) acting at their FRAMENUMBER.

## Banks per screen

- **Planet**: `planitem.shp` frames 0-21 = the surface items, 68x52,
  origin (34,18) = the TOP VERTEX of a 68x34 diamond filling the bottom
  34 rows; frames 22-30 = the orbital items, 70x70 canvases with the
  origin at the CENTRE (35,35), drawn in a flat 2x5 grid of 70x70 cells
  at the left of the big area, cell origins (14+70c, 119+70r) (measured
  on a full colony); frames 31-37 = the projects' pictures,
  68x52 origin (34,18) like the surface items; 38 = the Xeno
  Archeological Dig, an iso tile on its square (the ruins under a green
  excavator); 39 = the Xeno ruins themselves (broken pillars on brown
  earth, no item: drawn on a ruins square; by eye). The
  system screen's planet panel draws frames 22-30 at scale 1 (70x70,
  origin the centre) as the orbital structures' tiles: the DOSBox capture
  image0039 lists the Orbital Missile Base and the Long Range Orbital
  Whopper with these very pictures (by eye); its globe is `planalNN`
  (below) fitted to 128 native px (planal size 1 and planets.shp size 5
  are 95 px drawn, the capture's globe ~93). Cell (i,j) diamond centre
  on screen = G + ((i-j)*34, (i+j-10)*17) with **G = (410,297)**
  (measured on DOSBox captures; the square markers sit on that grid);
  sprite origin = centre - (0,17). Paint all diamonds, then all sprites
  in Layout order (back to front). `17np.shp`: frame 0 chrome (panels
  (7,7)-(115,107), (123,7)-(231,107), (239,7)-(347,107) black;
  (355,7)-(455,107), (463,7)-(633,107) dark blue; big area
  (7,115)-(633,473) transparent), frames 1-3 = the POP panel's 10x10
  icons, origin top-left (1 the red busy worker, 9x9 drawn; 2 the orange
  free figure, 6x9; 3 the green room ring, 8x8; laid 10 a row at a 10 px
  pitch from the panel's corner, measured on colony-home-dosbox.png),
  6-8 = 3x3 markers, frame 4 = 626x358 starfield. `planal00-10.shp` =
  the globe per type, 5 frames = sizes, origin (200,200), drawn at zoom
  as they are (radius 48/75/104/141/190 px orig., Small to Huge).
  `planind/planres/planpro.shp` = 21 levels of panel art, 108x100: frame
  = output/2, capped at 20 (40 points, as remembered from playing antag).
- **System** (`16battle.tmp` frame 0: big area [7,466)x[7,473),
  borderless top-right corner [466,640)x[0,142), info box
  [474,633)x[142,395); its button holes are transparent: paint black
  right of the area): `suns.shp` 13 frames 150x150 = starstat's
  TYPE_0..12; `planets.shp` 55 frames 100x100 TYPE-MAJOR (frame = type*5
  + size-1); `lanes.shp` 150x150: 0 blue lane sphere, 1 red, 2-3 with a
  structure inside. `16battle.shp` (measured): 10 = the
  system panel, 174x142 at (466,0), an opaque strip [0,55) for the
  star's name and type, three 44x31 holes at x 13/65/117, y [55,86) for
  the turn buttons (frames 16 End Turn, 17 Repeat Move, 18 Auto) and two
  70x31 holes at x 13/91, y [103,134) for the list filters (frames 7-9);
  11 = the selected-ship panel (a 159x88 hole, four 31x31 category holes
  at y [103,134): frames 12-15 weapons, shields, specials, passive); 19
  = a plain 159x88 hole; 0-6 the round buttons (grid, elevation, scroll
  arrows). The system's lists (measured): `cos_ship.shp` 28
  frames, origin (0,0) at the top-left, the starmap's fleet pennants: 7
  groups of 4 (frames 0-1 a 4x3 pennant, 2-3 a 5x5 one, each in two
  shades), group 0 in the green ramp (palette 16-19), the others in
  fixed colours (yellow, pink, purple, blue, brown, grey; not the
  empires' ramp order): draw group 0 through `SpriteIn` for an empire's
  colour. `shipicon.shp` 12 frames, the six part categories (drive
  chevron, weapon burst, shield sphere, special, scanner arcs, generator
  bars) at 7x7 (0-5) and 13x13 (6-11). `listbox.shp` 4 frames, scroll
  arrows up/down at 25x25 (0-1) and 15x15 (2-3). `smhome.shp` 21 frames
  52x52, a species' home globe; `smraces.shp` 21 frames 100x100, round
  species portraits. No small planet bank: the smallest is `planets.shp`
  at size 1 (24x24 drawn box). The combat effects (the system screen's
  replay), each frame with its own origin, drawn centred on
  the projected point: `bulletNN.shp` 40x40 origin (20,20), 4 frames
  looped (bullet09: 8), the projectile, not directional; `explNN.shp`
  140x140 origin (70,70), 10 frames, the impact (frame 0 empty in
  03/04/07/08); NN = the weapon's Gizmo index 00..09 (Mass Barrage Gun
  00 ... Nanomanipulator 09): a HYPOTHESIS, checked by eye on 00 (a
  sparkle spray, the barrage) and 05 (a red plasma, Plasmatron) only.
  `shield.shp` 140x140 origin (70,70), 8 frames, the blue shimmer (a hit
  the shields absorbed); `destroy.shp` 200x200 origin (100,100), 15
  frames, a ship blowing up: white, debris flying, frame 0 a solid 41x27
  oval, the widest box 187x171 (frame 13), frame 14 empty.
  `destroy2.shp` (200x200, origin (100,100), 15 frames) is a fiery
  orange blast, not a bigger one (widest box 194x139): its frame 0 is a
  sparse 43x27 speck, frames 1-3 ragged (scaled as destroy's frame 0
  they leave 2-20% of a hull showing); unused (the replay blows every
  hull up with `destroy`). `blueexit.shp` / `redexit.shp`: 10 frames
  each, 150x150 origin (75,75) as `lanes.shp`, the lane sphere flaring
  up (boxes growing to 131x137 / 140x148 at frame 9): the lane-entry
  flare (`flare.go`).
- **Starmap** (`1starmap.tmp` frame 0: big area [7,466)x[7,473), top box
  [474,633)x[7,44), five stacked boxes [474,633)x[130+39k,161+39k), button
  holes transparent too): `cos_star.shp` dots (frames 4t..4t+3 = type t at
  four brightnesses, dim to bright; types 0-3 are 5x5, 4-9 3x3, 10-12
  1x1; 52 = "?", 53-54 = 17x17 green rings); `1starmap.shp` holds the day
  counter digits and the buttons.
- **New game**: chrome `3cfgnew.tmp` frame 0: the big area [7,494)x[7,373)
  (the species' picture `lgraceNN.shp`, 297x200, and its text), the
  species list [502,633)x[45,373) under an opaque strip for "Player
  Species", five 116x92 holes at y [381,473): x 7, 131, 255, 379 (the
  density, the species count, the atmosphere, the colour in the original)
  and [502,633) (Begin New Game). `3cfgnew.shp`: 13 frames 116x92, the
  picture in rows 25-80: 0-4 the galaxy per density, 5-9 three to seven
  flags, 10-12 Peaceful/Neutral/Hostile. static.txt 153-169 holds the
  labels. `7racesel.tmp` (a big hole, a stack of seven boxes, eight
  small buttons) is NOT the screen shown after Begin: that is 12start.
- **Species introduction** (after Begin New Game; checked on the
  DOSBox captures image0044/0045): chrome `12start.tmp` frame 0, a 7
  px ring and 8 px bars, the panels opaque (a starfield the text is
  written on) but the picture's: top bar [7,633)x[7,38) (the flag at
  (10,10), the title "<Name>: <power>" centred), picture hole
  [7,304)x[46,246) transparent = `lgraceNN.shp` (297x200, origin the
  top-left, NN = the species' index), right panel [312,633)x[46,246) (the
  intro, white), bottom [7,633)x[254,473) (the description, in the
  player's colour). Texts from **history.txt** (ascend00.cob, CRLF, `END`
  then ^Z at the end): per species `specnum N` (0-20, game.Species
  order) then sections `power`, `intro`, `peace`, `neutral`, `hostile`,
  `text`, each closed by `end<name>`; `//` lines are comments. In a
  section line breaks are soft, `@@` opens a paragraph, a run of `_` at a
  paragraph's start is its indent (`_____`, once `____`; the diplomatic
  sections open on one `_`), sentences end on two spaces. The captured
  Snovemdomas intro ends on their `neutral` section run into its last
  paragraph: the start atmosphere's (only they have one); ours shows
  `peace`, `neutral` or `hostile` by `Game.Atmosphere`.
- **Ship design**: chrome 8shipdes. The slot diamonds (frame 0, 70x34,
  dark blue like a planet's squares) are scattered per hull on an iso
  lattice, centre = (44 + 36u, 166 + 18v), u + v even (measured on
  the DOSBox captures `reference/shipdes-<hull>-dosbox.png`),
  sorted top to bottom then left to right:
  Small (10,0) (9,7) (3,13) (11,15) (0,16);
  Medium (10,0) (9,3) (11,7) (11,11) (9,13) (8,14) (3,15) (5,15) (7,15) (0,16);
  Large (10,0) (10,2) (10,4) (11,7) (11,9) (9,11) (2,12) (6,12) (8,12)
  (5,13) (0,14) (2,14) (5,15) (9,15) (0,16);
  Enormous (10,0) (11,1) (10,2) (9,3) (11,5) (10,6) (9,7) (11,9) (10,10)
  (9,11) (0,12) (4,12) (3,13) (7,13) (9,13) (11,13) (2,14) (6,14) (1,15)
  (5,15) (9,15) (11,15) (0,16) (4,16) (8,16). **Research**: 20res.

## Music (`music.txt`, `*.raw`): package `sound/`

`music.txt` (ascend00.cob): a count line `29` then that many DOS paths
(`data\theme00.raw`), CRLF, a blank line after: theme00-04, victory,
intro00-01 (ascend01.cob), race00-20 (ascend02.cob, one stinger per
species, 9-31 s). Every `.raw` is headerless PCM, **8-bit unsigned, mono,
22050 Hz** (an earlier extraction of the music, proved by
autocorrelation). Each theme ends on a fade then silence; theme00 (371.5
s) holds four pieces split by three silences of ~32 s and ends on 14.5 s
of it (|b−128| ≤ 2); intro00 is theme02's first 90.7 s. `sound.Track`
streams a tune as s16le stereo ((b−128)<<8 on both channels) straight
from the raw bytes, silences included (once squeezed to 3 s: the theme
sounds odd without them). The device is oto
(Ebiten's own audio backend) used directly: Ebiten's audio package
returns a device failure from RunGame, oto reports it through `Err`.

**Sound effects** (`sound/voc.go`): `soundfx.txt`
(ascend00.cob) is three `channel? value` lines (`2 10000`, `3 45000`,
`1 65000`, unread), a blank, the count `86`, then the names; the exe
reads `data\%s.voc` by index (no other sound name in its strings). Index
0 is `button`, the click's ting; then shield, laser01, ship01,
bang01-03, trust01-02, destroy, destroy2, shield, blueexit, redexit,
gb00-09 / gh00-09 (the ten guns' shot / hit), the specials' sgNNbull /
sgNNexpl, ssNNexpl, orbb00-02 / orbex00-02 (the orbital weapons'),
SG32BULL...SM38BULL. `batfx.txt`'s `shape1 shape2 sfx1 sfx2` per gizmo
gives each weapon its shot and hit sound (MassBarrage 14 24 = gb00
gh00; a special's sfx2 0 reads as none). shield, destroy, destroy2 and
redexit are one file byte for byte, as sg32bull, sg59bull, sg68bull and
sg61expl. Most are Creative Voice Files (one type 9 block, 22050 Hz,
8-bit unsigned mono); blueexit, gbNN, ghNN, the sg/ss/orb ones are
headerless raw like the music (rate assumed 22050: ~1 s each); `bullet`
is 11025 Hz stereo. In ascend01.cob but not in the list: click08,
button01-02, switch01-02, bullet, ship02.

## Save games (`*.SAV`, `RESUME.GAM`): package `sav/`, tool `cmd/savedump`

Decoded (the package comment has every offset). A header of 180
bytes (u16 at 0 = the game day; record sizes at 0x78: 96, 123,
494, 39, 414, 13), a tech section of
7538 bytes (u16, then 66 records of 75 bytes in restree
order: +0 the name, +60 u16 the cost in points, Cost × costScale, all 66
equal to `Research.Cost`; +62 the prerequisites, +67 the techs they open,
each ff-ended; +72 a mask, bit k = empire k knows it (bit 0 the player);
+73 the same mask less bit 0 in most saves; then 2586 bytes unread), then sections of `u32 count` + fixed records: STARS (96 bytes:
+0 type, +4 index, +8 three float32 position, +28 name, +44 up to 6 lane
pointers, +68 u16 lane count, +70 up to 5 planet pointers, +90 u16 planet
count), SQUARES (4 bytes each, one per surface and orbital square of every
planet, laid planet after planet: byte 0 the colour code, 0 black, 1
white, 2/3/4 the colours, 8-12 Xeno ruins on a square of code−8, at most
one per planet (`sav.Square.Ruins`; a dig in progress shows item 38 on
it, a dug ruin is gone); byte 1 the planitem index, ff none;
byte 2 0 under construction, 1 built, 3 built and automated
(confirmed in the game: 17 of Ribulon I's 21 Factories in the
second install's save 01, 59 people;
`sav.Square.Built`, `Automated`); byte 3 unknown), PLANETS (123 bytes:
+12 star, +14 number in the system, +16 pointer to its squares, +20
size−1, +22 type, +24 squares, +26 squares+10, +28/+30 free slots, +36
name, +66 u8 the population, +68 u16 the day's industry, +70 u16 its
research, +72 u16 its prosperity net of the upkeep, 0 at worst (the three
as shown, docs/rules.md "Colony output"), +76 u8 probably the busy
population, +84 u8 the project's planitem index (ff none), +87 u8 the
owner (0 the player; read with a population: `sav.Planet`, `savedump
-colonies`), +74 u8 the
population capacity (docs/rules.md)), SHIPS (358 bytes, partly read,
see below), PLAYERS (494), LANES (39 bytes:
+0 star A, +4 star B, +8 a vector along the lane, +35 = 1 for RED). Pointers
are memory addresses; a lane's number = (pointer − lowest)/39, and every
lane is pointed at by exactly its two stars. Not decoded: the ships
past what is below, players, techs, the lane flags +32/+33. Saves live in
`~/dosbox/Ascend` and a second DOSBox install's Ascend directory
(fresh games of each density: 10 Sparse, 07 Average, 11 Dense, 08 Very
Dense; the second install's 09.SAV, once the fresh Vacuous game, is now day 1196 of 02.SAV's
Panacea galaxy, 19 ships). `savedump -stats <files>` prints one line per distinct
galaxy. Census of 12 galaxies, 1915 planets: planet types uniform (8-11%
each) and independent of the star type, sizes uniform, 1-5 planets per
star uniform, star types at starstat.txt's shares.

Ship records (from 02.SAV day 1191 against 09.SAV day 1196, the
same game, and 03/04): **packed**, a float may sit on a 2-byte boundary.
+0 u32 the ship's id, the records sorted by it, a freed id reused (a ship
built between two saves lands mid-array and shifts the rest: compare by
id, not by record index); +56 the name, NUL ended, 30 bytes (the player's,
or the hull size's "Medium"…); +86 u16 ≤ the day, likely the day it was
built; +108 a heap pointer (0 for a ship never placed); +112/+116/+120 f32,
zero for most idle ships; +124/+128/+132 f32 a position, multiples of 32 when
in orbit, shared by the ships at the same place, not a star's position
scaled (system coordinates?); +144 and +156 u16 20/30/40 on Medium/Large/
Enormous hulls (+144 18 on one damaged ship: current and max?); +162 f32
(unaligned), first taken for the energy, which is the integer at +140
(measured later: docs/rules.md "Movement by energy"):
13.333, 53.333, 219.33… thirds, it grows between the two saves on idle
ships (26.67 → 43.33 in 5 days) and drops on the ships that moved (70 →
13.33); on some ships +162/+166/+170
hold a vector instead (a union, or a state: +136 u16 is 9 on the thirds
ships, 1 or 3 on the others); from about +175 7-byte entries carrying
the parts (one byte a gizmo index, ff empty). Unconfirmed: to be measured
on saves one action apart.

`savedump -diff a.sav b.sav` prints the bytes that differ, section by
section (`sav.Sections`): one line per changed run within a record,
`ships[id 3:Medium]+126 2  6844 → f043` then the run's start read as u8, u16,
i16 (aligned to 2), u32, i32, f32 (aligned to 4, `@off` when not the
start) and the f32 aligned to 2 when that differs. Bytes changed in one
4-byte word make one run. Ships are paired by their id, labelled
`ships[id 17:TEST]+162`, the ids in one file only listed as added or
removed; the other arrays by record index (counts differing: the common
records, then the extra ones listed); the header,
tech and the undecoded rest as blobs, over their common prefix when the
lengths differ. `-diff-section ships` keeps one section. A save reloaded
and saved again differs in most pointers (the heap moved): compare saves
made in one session. `savedump -ships [-fmax 5000] file` lists each ship
record's 4-aligned f32 and the one at +162 of magnitude in [1e-3, fmax],
the position and energy candidates.

## Our save files (`save/`)

JSON, our own format, never the original's, gzipped (version
59: compress/gzip at the default level, the JSON unindented; a
day-1451 game went from 2.1 MB to 89 KB, its write 19 ms to 20 ms; read
it with `zcat x.json.gz | jq .`; an ungzipped file is not a save).
A file from anywhere is distrusted: at most 16 MiB
compressed and 64 MiB of JSON (a gzip bomb is "too large"), one gzip
member read to its end (CRC and size checked, nothing after it), then
`game.Game.Validate` before the relink (every saved index, enum and
coupled length in range: stars, planets, lanes, empires, squares, parts,
orders, projects), `game.ValidateEvent` per log entry and
`bot.State.Validate` per bot; any panic left is an error. Peek reads the
head only, its CRC unchecked:
`save.File{Version,
Summary, Seed, Game, Bots, Log, View}` with the whole `game.Game` (its
cluster included, so a save is self-contained whatever the generator
does later; Colony.Planet is relinked from Star/PlanetIdx after
decoding, Empire.Cluster is not serialised and relinked), one
`bot.State` per empire (nil = no bot) and the window's log. About 450 KB of JSON
for 50 stars. Written to `<path>.tmp` then renamed. Default path
`$XDG_DATA_HOME/aiscendancy/quick.json.gz` (`last.json.gz` beside it, each day's start); F5 saves, F9 loads (the screen
stack resets to the starmap: screens hold pointers into the cluster),
`-save`/`-load` on the window, `-load` on `sim -galaxy`; `cmd/mksave`
writes one offline (a seed, species, empires, the fixture ships, N days
played by the bots, or more days on a save): the window's own flags for
those went away (the game binary stays lean, a save
is the way to set a situation up). No PRNG runs during play, so a loaded
game continues identically (tested: the events of 100 further days and
the next save are byte-equal, in combat too). No backward compatibility
(not yet): Read heals no missing field, and a
format change bumps Version (3: the empires' colours; 4: the bots' ship
destinations; 5: the bots' genomes; 6: the abstract combat flag; 7:
combat rounds (the abstract flag withdrawn, armed shots); 8: the round
genes (HullReach, Standoff, Rounds); 9: HullReach in Pos units, ranges
in RangeUnit; 13: the Xeno ruins, `Square.Ruins`; 15: the planets'
hidden `Planet.OrbitDamage` and the colonies' `Colony.OrbitShots`; 16:
the invasions, `Starship.Invaders`; 17: the bots' Founded and Captured;
18: the ongoing projects; 19: Automation, `Square.Automated`; 20: the
gene AutomateCostW; 21: diplomacy, `Empire.Relations`, `Game.Offers`,
`Event.Other`, the bots' Grudge and Seen; 22: the bots' Weary; 23:
Alliance, the lot 2 ledger (AllyDays, Broken, Trades, Hospitality),
`Empire.LanesKnown`, the offer kinds; 24: the bots' Goodwill and
LastTrade; 25: `Offer.Third`, the lot 3 ledger (Helped, Defamed),
`Empire.AbilityUsed`; 26: the bots' LastAsk; 27: `Planet.PopBonus`,
`Empire.Invincible` and `BlockedUntil`; 28: the ship specials,
`Starship.Boosted` and `Invulnerable`, `LaneDrives` summing the lane
drives' Levels; 29: the targeted specials' day flags
`Starship.TiedDown`, `Scrambled` and `ShieldsDown`; 30: the Lane
Blockers' `Empire.LaneBlocks`; 34: the bots' replacement genes
ReplaceDays and ReplaceFull; 35: `Empire.AbilityUsed` 0 at the start,
the ability starting empty; 36: the shields' draw per combat round, the
bots' ShieldFirst gene; 37: the damage ledger `Starship.Dealt`; 38: the
bots' FoundFloor and InFlight genes; 39: the bots' AbandonIdle gene; 40:
the shields' reservation, `Starship.Power` holding ShieldDraw off while
`Shields` is on; 41: `Starship.Sapped`, the Shevar's sap having taken
the reservation; 42: the `Summary`; 43: `Empire.History`, a
`game.Sample` every 10 days, docs/rules.md "History", and the bots'
`Idle`, the abandon count a loaded bot used to restart from 0; 44:
`View`, the planets list's Idle first and sort key; 45:
`Empire.ShipNumbers`, a counter per ship class instead of the one
`NextShip`; 46: the bots' economy genes `Upgrade`, `IndShare`,
`ResShare`; 47: `Colony.Queue`, the construction queue; 48:
`Entry.Idle`, the log line that left its colony idle, red; 49: the bots'
IndFloor gene and their home script Agriplot, Factory; 50:
`Empire.ShipNumbers` gone (the first free numeral),
`View.PlanetsProjectsFirst` and `Colony.Tractors`, the Tractor Beam
pulls of the round: a reload must not give a pull back); 51:
`Template.Slots` and `Design.Slots`, the parts placed by hand; 52:
`Relation.Encroached` and the bots' GrudgeEncroach gene; 53: the bots'
Rage gene, `Empire.Won`, `WonDay`, `HospitalityGiven`; 54: no
`Colony.Species`, a colony's Traits are its owner's; 55:
`Game.RoundsToday`, the rounds played today per star), any other
refused.

The `Summary` sits right after `Version` (JSON keeps the struct's
order), filled by Write from empire 0 (the window's player): Species,
Colour, Day, Saved (local time at the write; a `now` seam the save tests
pin, or two saves of one game differ), Ships, Colonies, Techs (known),
and `Met`, one per empire in contact in Empires order (Species, Colour,
the `game.Stance`, Dead when no longer Alive). `save.Peek` reads Version
and Summary off the file's head with a `json.Decoder`, stopping at the
next key through the gzip stream, so a list of saves never decodes a
Game (a full Read is ~20 ms a MB of JSON); a file of another version still gives its version (and summary
when it decodes). Slots: `slotNN.json.gz` (two digits at least) in the quick
save's directory, `save.Slots` lists the numbers (only SlotPath's
spelling), `save.NextSlot` is one past the highest.

## The action journal (`journal/`)

An optional, private record of every decision taken in a game, the
player's and the bots' alike, to compare a human's play with the bots'
and to feed the fuzzer later. Not the public log (`game.Event`), not in
the save (the Writer hangs off `Game`, `Empire` and `Colony` in
unexported fields: a save written with a journal is byte-equal to one
without). `-journal <path>` on `cmd/aiscendancy` (also a key of
`aiscendancy.json`), `cmd/sim -galaxy` and `cmd/mksave`: the file is
opened for append (0644), so successive runs pile up; `*.jsonl` is
git-ignored. Not on `cmd/fuzz`: its games run in parallel and would
interleave in one file. The model's behaviour does not depend on it:
`sim -galaxy -seed 1 -days 300 -empires 3` prints the same bytes with
and without.

JSON Lines: one `journal.Entry` object per line, flat. `day` (the
acting empire's Day, the days run so far: the bots' day-0 decisions are
day 0), `empire` (who acted, its index) and `act` are always written;
the strings (`colony` = a planet's name, `ship`, `item`, `tech`,
`design`, `kind`, `cell` = "(i,j)", `target`) and the bools (`accept`,
`on`) only when set; the indices `star`, `planet`, `slot`, `part`,
`other` (the other empire) are always written, −1 when they do not
apply.

| act | entry point | carries |
|---|---|---|
| `order` | `Empire.Order` | ship, kind (none, planet, star, colonise, point, invade, system); star = the GoToStar's or GoToSystem's destination; star+planet = the GoToPlanet's, or the orbit of a Colonise/Invade; star + target "(x,y,z)" for a GoToPoint |
| `build` | `Empire.BuildShip` | colony, design, slot |
| `found` | `Empire.FoundAt` (and `Found`) | ship, colony (the planet), cell, star, planet |
| `invade` | `Game.InvadeNow` | ship, star, planet (the orbit; resolved at once: the window's Invade) |
| `leave` | `Empire.LeaveOrbit` | ship, star, planet (the orbit left) |
| `abandon` | `Empire.Abandon` | ship, star |
| `load` | `Empire.Load` | ship, colony (the planet it orbits), star, planet |
| `unload` | `Empire.Unload` | ship, colony (the planet it orbits), star, planet |
| `refit` | `Empire.Refit` | ship, colony, design |
| `arm` | `Empire.Arm` | ship, part, star, target (the ship aimed at), other (its empire) |
| `arm` | `Empire.ArmAtPlanet` | ship, part, star, planet (the one aimed at), other (its owner, −1 for a guarded uninhabited one) |
| `arm` | `Empire.ArmPlanet` | colony, slot (the orbital weapon), star, planet (the colony's), target (the ship aimed at), other (its empire) |
| `arm` | `Empire.ArmPlanetAt` | colony, slot (the orbital weapon), star, planet (the one aimed at), target (its name), other (its owner, −1 for a guarded uninhabited one) |
| `special` | `Empire.UseSpecial` | ship, part, item (the gizmo's name), star, target (a ship's name, "lane N", a planet's name, or none) |
| `tractor` | `Empire.UseTractor` | colony, star, planet (the colony's), target (the ship pulled), other (its empire, the colony's own for its own ship) |
| `study` | `Empire.Study` | tech (not when the topic is picked again) |
| `disarm` | `Empire.Disarm`, `Empire.DisarmPlanet` | ship, or colony (only when a shot was armed) |
| `round` | `Game.StepRound` (a mid-day round, `bot.Step`) | star |
| `shields` | `Empire.SetShields` | ship, on (only when the value changes) |
| `start` | `Colony.Start`, `StartOrbital`, `StartProject` (and `Colony.Next`: the queue's head started, at the close too) | colony, item, cell (surface) or slot (orbit) |
| `enqueue` | `Colony.Enqueue`, `EnqueueOrbital` | colony, item, cell or slot |
| `dequeue` | `Colony.Dequeue` | colony, item, cell or slot (the entry removed) |
| `queue-up` | `Colony.QueueUp` | colony, item, cell or slot (the entry moved up) |
| `cancel` | `Colony.Cancel` | colony, item (ship: a refit's), only when a project was running |
| `demolish` | `Colony.Abandon`, `AbandonOrbital` | colony, item, cell or slot |
| `war` | `Game.DeclareWar` | other |
| `break` | `Game.BreakAlliance` | other |
| `offer` | `Game.OfferPeace`, `OfferAlliance`, `OfferTrade`, `Request` | other, kind (peace, alliance, stars, lanes, research, "war request"...); a request's third party in target ("empire N") |
| `answer` | `Game.Answer` | empire = the one answering, other = the offerer, kind, accept |
| `ability` | `Game.UseAbility` | the empire only |

Not recorded: a refused call (an error returned changes nothing, so it
writes nothing); the day's own resolution (`EndDay`: a Colonise order
founding, a lost ship's refit cancelled, a captured colony's project
dropped, an offer expiring, the invasions, the combat's resolution);
the act a granted request makes the answerer perform (its `answer` is
the record); the fixtures' setup war (`SpawnFixtures`, `SpawnArena`);
a template saved (`AddTemplate`: a design, not a move); the bots'
reasoning (what they weighed, what they tried and were refused). These
go through unexported twins of the entry points (`foundAt`, `cancel`,
`answer`, `declareWar`...).
