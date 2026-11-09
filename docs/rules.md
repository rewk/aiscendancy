# The rules, as established

Every rule lives in `game/`; this records where each came from and which
are still assumptions. Sources: the game's own tables (`formats.md`), the
fan manual at https://www.b-sting.nl/ascendancy/ (planets-types.html,
planets-buildpower.html, species.html, research-list.html), Nathan
Williams' 2005 tech-tree guide and James Friel's building guide
(`docs/third-party/`), and our own checks in DOSBox.

## Planets and squares

- Bonus squares: red = industry, green = prosperity, blue = research
  (planitem.txt column order, manual, confirmed in game).
- A structure on a bonus square of its kind gets **+1** to that output,
  only if it already makes at least 1 of it (`Planet.Output`; checked in
  game).
- A new structure must share an edge with an existing one
  (`Planet.Reachable`); the first goes anywhere. Transport Tubes cross
  black squares; only Tubes (and Terraforming) go on black. The data
  flags the Outpost too (flag 6), the original refuses it there (only
  the Orfa put one): as antag.exe (after keeping
  it a while on the fuzzer's 38.8 vs 39.5 colonies).
- Item flags: 0 special placement, 1 project, 2 weapon, 3 shield, 5
  ongoing, 6 may sit on a black square (ASSUMPTION on 0 and 5).
- Home worlds: size and type per species (`game/species.go`, from the
  manual's species page; Snovemdomas = Enormous Primordial as seen). The
  base lands on the centre, forced white, its four edge neighbours forced
  off black (ours, seed 7 hemmed the base in; not for the
  Orfa).

## Species hooks

The passive abilities (Recharge 0) are `Traits`: one method per place a
rule bends, a base answer for everyone (`game.Base`, used by sandboxes),
a type per species overriding what its ability changes; colonies and
ships carry their Species and the rules ask `Species.Traits()`. Done:
Orfa `BuildsOnBlack` (any structure on black, home base not forced
open), Chronomyst `LaneFactor` 2 (lane days halved), Snovemdomas
`HullFactor` 2, Minions `InvadesAlways` (SuperMarines: "Invasions"),
Oculons `KnowsStarLanes` and Kambuchka `KnowsHomeStars` ("Fog of war"),
Hanshaks `KnowsRaces` ("Diplomacy").

The timed abilities are not Traits but used on demand ("Species
abilities"). All fourteen are in (the Baliflids and the
Marmosians in "Diplomacy" lot 3, the twelve others below).

## Species abilities (`game/ability.go`)

The frame (diplomacy lot 3): `Empire.AbilityUsed` is the day of the last
use, `Game.AbilityState(i)` says ready or the days left of the species'
`Recharge`, `Game.UseAbility(i)` uses it: passive refused, recharging
refused, `AbilityUsed` = Day on success, an AbilityUsed Report for the
user first; a refusal changes nothing. A timed ability starts EMPTY
(checked in DOSBox): `AbilityUsed` is 0 in a new game, so the
first use is ready on day Recharge. The close where it recharges (Day −
AbilityUsed == Recharge exactly, once a charge, a living empire only)
Reports an AbilityReady "Special ability ready" to its owner; the window
shows the original's notice on the player's (docs/screens.md), the bots
ignore it. Asleep (`AbilityAsleep`, the empire controlling no system; on
the Ungooma bumping ships with nothing left) it
is refused too; it wakes with a system controlled again; asleep, its
recharge pauses: each close moves AbilityUsed a
day on, at most Recharge − 1 behind, so a charge full before the sleep
reports AbilityReady the close after the waking.

Source: help.txt's abilNN / abresNN texts. Each in a line, our readings
flagged:

- Fludentri SA_HealShips (60): every ship of ours at `MaxHull`; refused
  "nothing to repair" when none is damaged.
- Swaparamans SA_DoublePower (62): every ship's Power left today
  doubled; the next `Refill` gives the plain supply. Refused with no
  ship.
- Shevar SA_SapAlienPower (90): every other empire's ship out of the
  lanes at a star where we are `Present` has Power 0, its armed shots
  dropped and its shields' reservation taken too:
  on ones go dark and `Sapped`, switching them off refunds nothing until
  the Refill (a knocked-down shield, unlike it, refunds: measured); a
  Report per empire hit ("The Shevar drain our ships' power"); refused
  when there is none.
- Dubtaks SA_StealResearch (63): every tech we lack that two other
  empires or more know is Granted, a Discovered "<Tech> stolen" each.
  ASSUMPTION: any empire of the game counts, met or not.
- Chamachies SA_FreeResearch (89): the topic's Progress set to its Cost,
  so the close's `Advance` discovers it with no research at all (Advance
  of 0 points discovers a topic at its Cost). Refused with no topic.
- Nimbuloids SA_IndustryBoost (68): every project not ongoing gains its
  colony's day's industry (`Output`) at once, capped at its Cost; the
  close adds its own and completes it on reaching the Cost ("double
  industry for one day"). Refused when no project runs.
- Mebes SA_MoreMaxPop (72): `Planet.PopBonus` + `MebesPopBonus` (2) on
  every colony's planet, which `Capacity` adds; +2 a use measured in
  DOSBox (the help says only "increase the maximum
  population").
- Capelons SA_ShieldPlanets (66): `Empire.Invincible` = Day;
  `IsInvincible(day)` holds on that day and the next. Why two: the use
  falls in day D, whose rounds run at Day D, but `Game.EndDay` increments
  Day to D+1 BEFORE the invasions of that close; covering D+1 also covers
  the next day's rounds (one more day of rounds than of invasions).
  While invincible an Invade is refused ("the Capelons's colonies are
  invincible today": `Order`, and the close drops the order as any
  refused invasion, modules kept), and a round's shot at their orbital
  structures is spent and lands nothing (Damage 0, nothing razed). Ships
  in orbit are ships: still hit.
- Ungooma SA_BumpShipsInLanes (70): every ship of every empire in a lane
  is back at the star it left, in space at the lane's entry (FreeSpot
  when taken, Empires then Ships order), out of orbit; its lane order
  (GoToStar, a GoToSystem route) kept, so it sets off again at the next
  close, the days flown lost (dropping it lost
  every route), any other order dropped; a Report per other empire hit.
  Refused with no ship in a lane.
- Frutmaka SA_RepelAlienShips (77): at every star holding a colony of
  ours, every other empire's ship out of the lanes (in orbit or in
  space), listed before any move, goes to the far end of the star's
  lowest-index lane, in space at that lane's endpoint (FreeSpot); a lane
  order (GoToStar, GoToSystem) goes on as a GoToSystem to its star from
  there, dropped when it landed there (the bump's rule), any other order
  dropped. ASSUMPTION: a star with no lane
  keeps its intruders. A Report per empire hit naming the stars. Refused
  when no ship moves. The warp leaves a ship under refit at its Docks.
- Arbryls SA_BlockStarLanes (100): `Empire.BlockedUntil` = Day +
  `BlockDays` (20, ASSUMPTION: the duration is not in the files). While
  Day < BlockedUntil no ship of another empire, allies included, ENTERS
  a lane whose far end holds an Arbryls colony (`LaneBlocker`): `Order`
  refuses ("the lane is blocked by the Arbryls"), and a ship ordered
  before waits at the entry, its order kept (the close and the rounds
  swap it for a move to the entry, `holdAtBlock`, spending nothing once
  there), entering at the first move after the block. Ships in the lane
  go on; the Arbryls' own pass. The Lane Blocker special closes one lane
  both ways the same way (`Empire.LaneBlocks`, "Ship specials"):
  `LaneBlocker` reads both, so every consumer (Order's refusal,
  `holdAtBlock` in the close and the rounds, the window's `BlockedLane`)
  honours either.
- Govorom SA_Cornucopia (150): our colony of lowest Pop among those not
  yet a rich world (a Cornucopia whose squares but the Colony Base's are
  all bonus ones; the first in Colonies order on a tie) becomes Type
  Cornucopia: its bonus squares stay, every black or white one becomes
  the bonus kind fewest on the world so far (shuffled order, ties
  drawn), the three as near a third each as the kept ones allow
  (the first version only whitened the black, the
  second re-rolled every square, recolouring the standing structures'),
  deterministic (seed, empire, day, planet), the Colony Base's square
  kept, the structures and ruins where they stand, tubes on black
  removed. Refused when every colony is a rich world already (a use
  on a rich world would change nothing and burn the recharge).

The bot's abilities (`bot/ability.go`): in Act after the
diplomacy, when `AbilityState` says ready, one predicate per species
(`wantAbility`) reads only what the fog shows (Hostiles, LaneTraffic,
CombatStars, its own empire); the Reports go to the bot's news, a
refusal is dropped and asked again the next Act.
- Fludentri: a ship of theirs under RepairAt × MaxHull (the Docks'
  repair gene). Swaparamans: an armed ship of theirs at a combat star.
  Shevar: a hostile ship seen where they are Present. Capelons and
  Frutmaka: a hostile ship at a star holding a colony of theirs.
- Ungooma: a lane ending at a colony star of theirs carries traffic and
  no ship of theirs is in a lane. Arbryls: at war with such traffic.
- Chamachies: a topic with at least FreeResearchDays (a gene, neutral
  30) left at their research rate (Σ Output research, `Research.Days`;
  never at rate 0 counts). Nimbuloids: a project not ongoing runs.
- Dubtaks and Mebes: whenever ready. Govorom: from their second colony
  on (ASSUMPTION: the home alone waits, the rich world is better spent
  on a young colony; their home is a Cornucopia already).

Measured: sim -galaxy seed 1, 3 empires, 1200 days: six Govorom
Cornucopias (days 311 to 1061, each on a world not yet rich), no
Capelons use (no war in that game); solo seeds 1-8, 1500 days, mean days
to a quarter / half of the stars (the win capped at 1500): Chamachies
894/1158 → 836/1131, Nimbuloids 820/1106 → 791/1070, Govorom 776/1083 →
771/1047 (win 1307 → 1273), Mebes 832/1123 → 814/1114 (win 1365 →
1339); the Dubtaks and the Fludentri unchanged (alone, nothing to steal,
nothing damaged).

## Colony output (manual's curves, confirmed by an in-game 1 → 3 prosperity)

    industry   = round(floor(Σ industry × hyperpower)^0.85)
    prosperity = round(floor(Σ prosperity × fertilization + party)^0.85) − floor(pop/4)
    research   = floor(Σ research × internet) + takeover

The curve, measured (sums 1-19; 9 → 7 the original's quirk, reproduced
in `power`):

    sum    1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19
    shown  1 2 3 3 4 5 5 6 7  7  8  8  9  9 10 11 11 12 12

Measured on the test saves 01-18 (sav/testdata), the planet record's own
outputs (+68/+70/+72, docs/formats.md) against these formulas on its
squares (a square's byte 2 of 3 counts as built): the player's colonies
agree on all 870, Scientist Takeover's q added after the Internet
included, with one quirk: a floored sum of 9 gives 7, not round(6.47) = 6
(84 times, industry and prosperity alike; every other sum from 1 to 60
is round(Σ^0.85)), reproduced in `power`.
`savedump -colonies` prints the comparison per colony, and
`TestColoniesSaves` holds it on every test save. The AIs cheat (antag.exe's do): their
industry is floor(curved × ~1.3) in most games (1.0 in saves 02 and 09,
~1.13-1.25 for some empires: by difficulty, ASSUMPTION), not modelled.
Each plant multiplies by 1.5 once. The −pop/4 is the upkeep: a lone Colony
Base stops growing at 4 people. The Cloning Plant doubles a
positive prosperity on its way into the growth bank, not the output
shown (the planets guide: "doubles your population growth"); a
negative one is not doubled. `Item.Unique` names the five one-per-planet
bonus structures (the three plants, the Cloning Plant, the Observation
Installation): a second is allowed and adds nothing.

**The ongoing projects** (`Item.Ongoing`): Endless Party and
Scientist Takeover never complete. While one runs the industry is
switched off and its share q = ⌊Σ industry × hyperpower⌋ / 4 (integer
division, `OngoingShare`; the manual's quarter) is diverted: the party
adds q to the prosperity sum before the curve (`party` above), the
takeover adds q to research (`takeover`); `Colony.Diverted` gives the
item and q. Ours: an ongoing project takes no industry, starts and runs
with no event, and gives way: any start (a structure, an orbital, a
ship, a refit, a planet-wide project, the other ongoing one) replaces it
(`Colony.Running` is false on it, `common` lets the start through); the
same one again is refused, "already running". `DaysLeft` is false on
it; it reserves no worker (UsePop 0). Alien Hospitality is the third
(diplomacy lot 2): industry off, its q adds to nothing in
the colony and goes to the other empires' ledgers ("Diplomacy"); its q
is ours, the CURVED industry's quarter, `Colony.industry` / 4 (the raw
quarter let a big colony escape the curve);
Interstellar Trucking carries the same flag 5 and stays
`ErrNotModelled`. The bot (`planColony` → `idle`): a colony that starts
nothing runs an Endless Party while (Capacity − Pop) / Capacity > the
gene `IdleParty` (neutral 0: while any room is left; clamp [0, 1]), else
a Scientist Takeover; the one researched when only one is. A colony on
an ongoing project plans as an idle one (its picks weighed on its output
without it: `yielded`), so a real build replaces it, and it switches
when the colony fills up. Measured (solo, 21 species × seeds 1-4, 1500
days, Default): idle colony-days 55.2% → 13.2% (party 7.9%, takeover
34.0%), techs known at day 1500 57.1 → 58.9; the win days barely move.

Of the other planet-wide projects only the Lush Growth Bomb is modelled
("Population capacity" below, `Colony.StartProject`);
`Colony.CanStartProject` refuses the rest with `ErrNotModelled`.
Terraforming is: started like a structure on an empty reachable black
square, where Transport Tubes go on black, or on black under tubes,
cost 50 and 1 worker from the table, it turns the
square white when done (`Planet.Terraform`; the guides' "black squares
into white squares"). Tubes on it are removed: the
squares they reached may become unreachable, as after a demolition;
reachability is only "a structure next to it". The bot terraforms once
it is known, while a black square is left (`terraformAt`, "The fixed
policy's build and research").

**Automation** (our decisions; `Square.Automated`): a
targeted project on a built surface structure, the Colony Base
included, never an orbital one (`Colony.CanStart` at the structure's
cell; refused unresearched (tech 51), on an empty square, on a structure
automated already, and by the common checks: a project running, an
ongoing one giving way, no free worker). Its cost is **110 + the
structure's own Cost** (`AutomationExtra`, Project.Target read off the
square at the start; checked in the game: "110 Industry Points on top of
the base structure's cost"); the table's Cost 1 is unused. While it
builds it takes 1 free worker (its UsePop, as any project); once
complete the structure is Automated and employs nobody
(`Planet.Population` skips its UsePop): two workers free against the
build's, one against before it; capacity (MaxPop) and outputs
unchanged. Event: Built "<structure> automated on <planet>" (ours: the
original has only the generic "Construction of %s is complete on %s.").
Demolish and Terraform clear the mark with the structure; an Automation
whose target was demolished or replaced meanwhile completes as a dropped
build (no event, `Planet.Automate` refuses). An invasion takes the
colony as it is, the marks kept.

The bot (`automateAt`, before the greedy pick, under `Greedy`): a full
colony (Pop ≥ Capacity) whose last free worker is the one an Automation
takes (Free 1), with more than one empty Factory square, automates its
non-automated structure of least `AutomateCostW` × Cost (ties in Cells
order; the structure's Cost is the Automation's on top of
AutomationExtra: Tubes 10, a Factory 30, the Colony Base 120; every
surface item frees one worker), so it alternates automating and building
instead of stalling with squares left. The gene (neutral 0: Cells order,
the Colony Base included; [-1, 1]) was chosen after the
cheapest-first order measured WORSE (solo, 21 species × seeds 1-4, 1500
days: 50 wins against 51, the 50 won both ways 13 days later on average,
9 later and 1 sooner), the mechanism unknown: the costs differ little
(140 against 160 or 230) and the dates cascade. Measured (same runs):
2036 Automations started over the 84 games; 10 rows of 84 move: the 50
games won both ways win 13.7 days sooner on average (8 sooner, 1 later),
Ungooma seed 4 now wins (51 wins against 50).

## The day (`game/colony.go`)

Industry into the project (Built on reaching the cost), research into
the species' shared tree (Discovered), prosperity into the growth bank
(Grew at **50 points**, measured: 50 days at prosperity 1). Every
completion's text (Built, a ship's, Refitted) ends with the colony's
free population after it and the day's growth, "(free 2)" (can it chain
another project? The original's dialog says
"<planet> has 0 free population for additional projects"). Starting a
project needs free population ≥ the item's UPOP ("No Free Population"),
research allowance, a reachable square; the project reserves its
workers. A Colony Base is never a project (`ErrNotBuildable`): it lands
with the colony. A cancelled project loses the industry sunk into it.

A structure is abandoned through `Colony.Abandon` / `AbandonOrbital`
(the window's structure dialog): never the Colony Base, nor the last
Shipyard while a ship is built, nor the last Orbital Docks while a refit
runs (else the project would complete without them); the
project keeps running otherwise (an Automation whose target goes
completes as a dropped build). A structure is **replaced** through
`Colony.Replace(r, at, it)` (`game/replace.go`, for the
bots): Abandon then Start at the same square, journalled as both,
atomic: `CanReplace` refuses with nothing changed on CanAbandon's
refusals, the same item, a Running project (an ongoing one gives way, as
to any start), a non-surface item, or CanStart's verdict on the square
as emptied (the old structure's worker counts as free; an Automated one
frees nobody). `Colony.OutputWith(at, it)` is Output with one square
swapped.

**Construction queue** (`game/queue.go`): a
colony's `Queue` of constructions tied to a square or an orbit slot:
the surface structures, Terraforming, Automation, the dig, the orbital
structures but the Ship; never a ship, a refit nor a planet-wide
project. `CanEnqueue` / `CanEnqueueOrbital` refuse those, a Colony Base,
an unknown item, and a target CanStart's target checks refuse on the
**planned planet** (`Colony.Planned`: a copy with the running project
and every entry applied in order: a structure placed, Terraforming
whitening, the dig clearing the ruins, an Automation marking Automated),
so a square next to a queued structure is reachable, a Factory follows
its square's Terraforming, an Automation a queued structure, and a
target taken twice is refused by the plan itself. Free workers are not
checked then. `Dequeue` and `QueueUp` may leave a later entry invalid:
it is dropped in its turn. `Colony.Next` starts the head while nothing
Runs (an ongoing project gives way to it, as to any start): short of
workers it waits, any other refusal drops it with an `Unqueued` event
and the next is tried. It runs last in `Colony.EndDay` (after the
growth: a worker born today counts; after a Ship's launch, in
`Empire.endDay`, so the crew has left the Pop) and after the player's
cancel; nowhere else. A captured colony loses its queue. The bots never
queue (their games are byte-identical to before).

A fresh colony has 2 people ("Colonies always start with one free
population"). ASSUMPTIONS (named constants): industry surplus and idle
industry lost; workers busy from the project's start; a structure
produces from the day after it is built; negative prosperity empties the
bank, never kills.

**Population capacity** (`Colony.Capacity`), MEASURED in the original's
saves (11 DOSBox saves; planet record +74, formats
in `sav/sav.go`): a bare planet holds **5 + ⌊N/3⌋**, N its surface
squares that are not black (`CapacityBase`, `CapacityPerSquares`); type
and size act only through N. Exact on all 1560 planets with nothing
built (`sav` TestBareCapacity). The finished structures add their MaxPop
(Colony Base 2, Habitat 3, Metroplex 2, Outpost 1): exact on 321 of the
359 colonised planets; a project in progress adds nothing. What the
data flags for black (flag 6: the Outpost, the Transport Tubes) employs nobody
once built (`Item.Staff`; checked in the original,
planitem.txt's UsePop 1 notwithstanding): its project takes a worker,
the finished structure gives it back (an Outpost adds its room),
abandoning one frees nobody; nor is it offered an Automation (nobody to
free: `CanStart` refuses it). No structure is abandoned while the
population needs its room (`Colony.CanAbandon`: Pop over Capacity less
its MaxPop; the original greys the Outpost's removal). The Habitat is
planitem.txt's (seen in the original: an Agriplot's prosperity, 2 gross,
+1 shown net of the upkeep, a worker, 3 of room; the b-sting guide's 0
and the Agriplot's 1 read the net), the Metroplex too: of its 2 units of
room, one works its +1 in the three outputs. Terraforming raises it by
itself (black → white). The **Lush Growth Bomb** (a planet-wide project,
cost 200, research 62) adds `LushBonus` = 10, once per planet
(`Planet.Lush`; the fan guide: one per planet works): ~20 planets of
03.SAV, where it was used, sit at exactly +10. The bot never starts
it. Known gaps, unexplained: a few planets of 02.SAV (a 2018 game) at
+22..+32, ~10 big colonies at −1..−3.

Validated against the 2005 guide's day-by-day opening (Agriplot 30,
growth 37, Factory 67, Lab 92, Lab 117, Factory 132, Shipyard 212 at
cost 240): our sim with his script reproduces every day. His advice:
Agriplot first (growth gates everything), Lab on blue, two Factories
before the Shipyard; research Orbital Structures, Xenobiology, Tonklin
Diary, Interplanetary Exploration, Environmental Encapsulation,
Spacetime Surfing, then Advanced Chemistry, Power Conversion,
Hyperlogic.

## Research (`game/research.go`)

Cost = restree.txt cost × 1 for index 0-10, × 2·(index−10) for 11-65:
derived from the manual's "days" and CONFIRMED in DOSBox (Momentum
Deconservation shows 280 with one Laboratory). Progress is kept per tech
when the topic changes; the surplus of the discovering turn is lost
(both confirmed). **Dense clusters** (ours, with
the QoL a 100-star game's colonies ran the whole tree out by day 1424):
from index `DenseFrom` = 15 the cost is × max(1, n / `DenseStars`), n
the cluster's stars (`Research.Stars`, set by NewGame): ×1.5 Dense, ×2
Very Dense, none at Average and below (the fuzzer's density: its genomes
hold). A Xeno dig grants a tech (`Grant`, below). `Unlocks`
derives what a tech gives from the item and gizmo tables. `Days(t,
rate)` = ⌈(cost − progress) / rate⌉ (0 known, −1 at no research) for the
views, `rate` being `Empire.ResearchRate`, the colonies' research of
today.

**The scientific singularity** (ours): once every
tech is known (`Research.Complete`, derived, nothing saved) each
colony's `Output` moves res / `SingularityShare` (2, floored) of its
final research (Internet and takeover in) to its industry; an ongoing
project still switches all the industry off, its q still the planet's
industry's; the research left is lost (no topic). Scientist Takeover is
refused (`CanStartProject`) and every running one stops as the tree
completes (`Empire.stopTakeovers`: after the colonies' day, a theft, an
exchange). The tech completing the tree, by any path (study, dig,
theft, exchange), is followed once by a Discovered, Tech NoTech, with
`SingularityText` (short, for the news); the window's discovery popup
of that last tech adds `SingularityStory`, the long telling. The bots aim their output mix at `ResShare` 0 from
then on and never pick the takeover.

**Xeno ruins and the dig**. In the original's saves
(11 DOSBox saves, `sav`): a square's colour code 8-12 is Xeno
ruins on a square of code−8; a dig in progress shows item 38 on it; dug
ruins are gone (the planets with ruins fall from 15% at day 0 to 6% at
day 1466). The 5 day-0 saves (797 planets, 265 stars): 123 planets
(15.4%) hold ruins, always exactly ONE square; neither type nor size
matters measurably (6-20% per type on ~70 planets each, 12-18% per
size); per system and the square's kind: docs/galaxy.md, step 6. Ours:
`Square.Ruins` on a square of any Kind, which still counts (a ruin on
black is black for the capacity, as the saves' 5 + N/3 with code 8
black); `Planet.RuinsAt`. The generator (docs/galaxy.md) gives each
non-home planet ruins with `RuinChance` = 0.154 on a uniform square, a
tiny one with `RuinChanceTiny` = 0.25 (a kept divergence, a reason to settle the tiny ones).

The **Xeno Archeological Dig** (table: cost 50, research 6, 1 worker; a
Targeted project) starts only on the ruins square (`Colony.CanStart`:
"no Xeno ruins at" elsewhere), black included, empty and reachable as
any build. Its completion clears the ruins, places nothing, and grants
ONE tech not yet known, uniform over all the unknown ones, prerequisites
ignored (the help's "the key to the discovery of advanced
technologies"; in the original a dig once gave the Lush Growth Bomb very early: it can
decide a game); all known, nothing. Events: Built "Dig done on
<planet>", then Discovered "<tech> found in ruins" (at the colony). The
draw keeps no PRNG state: a fresh PCG seeded from `DigKey(Game.Seed,
empire, day)` mixed with the colony's star and planet indices
(SplitMix64), so a loaded game digs up what the unbroken run would have;
a lone `Empire.EndDay` uses seed 0. **Nothing is built on ruins**
(to protect the dig): `CanBuild`, `CanTerraform`
and so `BestCell` (a Colony Base's landing included) refuse the ruins
square, `CanStart` says "<cell> holds Xeno ruins: dig them first",
`Offers` lists the dig alone there. Before this rule every fuzz game (63
solo, 8 versus) ended up with structures on ruins, up to 15 at once.

**The bot and the ruins**. `planColony` starts the dig
before any other choice of an idle colony, when research allows it and
`CanStart` takes it on the planet's `RuinsAt` (unreachable ruins wait
for the colony to grow next to them). Colonisation: the gene `RuinWorth`
(neutral `ruinWorth` = 13 lane days, one blue lane at one drive: the
mean blue LaneDays at LaneStrength 1 on Average clusters, seeds 1-10,
15.2 with the measured lane days, 7.2 before them, when it was
6; also the nil genome's; clamp [0, 63]) makes `route` pick the target
star of least lane days less RuinWorth when it holds an unclaimed
foundable planet with ruins (fog: only at a star the empire Knows), ties
the first settled; the bot's choice has no planet qualities, the lane
days ARE its score, so a ruins world beats a comparable one up to a lane
nearer, not two. In its own system a colony ship takes a ruins planet
first (in-system flight is not costed in days), and, without ruins
there, leaves for a ruins star of fewer lane days than RuinWorth.

## Ships (`game/ship.go`)

**Designs.** Hulls Small/Medium/Large/Enormous with 5/10/15/25 slots,
techs Interplanetary Exploration (Small+Medium), Advanced Exploration,
Large Scale Construction. A Design = hull + parts; cost = hull + Σ
Industry; valid when research allows every part, the generators' Power
covers the other parts' (Specials draw on use, `UseSpecial`: left out,
ASSUMPTION), and an engine is aboard. A `Template` is a design whose
parts may be "Best <category>", realised against the research of the
day (`Realise`: the bot's colony ship is one). `Empire.Templates` are
the player's own, optional, none at the start (the
bots use none): `AddTemplate` refuses undiscovered material, a hull or
fixed part research does not allow (Valid's predicates) or a Best part
whose category has no allowed gizmo yet, so a saved template always
realises whole. A ship is the orbital `Ship` project built where a
Shipyard is; its crew (UsePop 1) leaves at completion.

**Movement by energy** (gizmos.txt: a drive's Level is "the
maximum distance that can be travelled, in sugar cube units", its Power
"the cost of operating the device", a generator's Power what it
supplies; battle.txt: "how far a ship can travel in a single move is
determined by the engines on board", "a ship that is out of power cannot
be used again until the following day"): one move goes up to `Reach` = Σ
drive Level sugar cubes (0.1 radius) and costs `MoveCost` = Σ drive
Power off the ship's `Power`, refilled to `PowerPerDay` = Σ generator
Power at the day's close (Tonklin Motor 2 cubes for 1, Ion Banger 4/1,
Graviton 6/3, Inertia Negator 6/1, Space Bender 10/5; Proton Shaver 2
... Nanotwirler 10). A shorter move costs the full MoveCost.

Measured in the original (a Minions game, test saves
12-18, a Small with one Tonklin and one Proton Shaver; a grid square =
160 file units = one sugar cube): energy is an integer (ship record
+140), refilled in full each day to Σ generator Power (2; 6 on three
Shavers); 1 square cost 1; 4 squares straight cost 2; a 4×4-square
diagonal (905 units) went two hops of 224,224 (316.8 ≤ 320 = Reach) and
stopped short, out of energy, its destination kept. The original's hop
ends on the 32-unit grid (0.02 here), the nearest point: ours are not
snapped (the 4-square move with its 32-unit slant, 641.6, took 2 hops
there, 3 here). Its energy bar draws 3 segments per unit. A shot (test
saves 12-13 of a later game, day 1467: Nanomanipulator, Nanotwirler,
Space Bender, Nanoshell) cost the weapon's Power, 6 (energy 10 → 4), and
one of its Uses (3 → 2, the part entry's byte 1), no shield draw: as
ours. The shields RESERVE their Power, combat or not, and a hit on
raised shields costs their draw again: both measured, "Combat".

The day: on the display an order is one Hop, the Repeat Move button
another, Auto hops every ordered ship while the power lasts; `Step` at
the close spends what is left on the order, refills, then proceeds with
the new day's power (the original's Auto, done for the player). `ETA`
simulates a copy and counts the days, 0 = today. A Star Lane Drive is
needed for lanes (`LaneDrives` > 0, Check).

**Lane days** (measured in DOSBox, the original's
ETA read on the day the ship enters the lane, lane lengths from the save
09.SAV by `savedump`, in file units, radius 1 = 440): `LaneDays` =
max(1, ⌈L × (`RedLaneFactor` 3 if red) / (`LaneSpeed` × S × the
species' LaneFactor)⌉), `LaneSpeed` = 10/440 (10 file units a day), S =
`Design.LaneStrength` = √(Star Lane Drives aboard) + 2 × √(Star Lane
Hyperdrives aboard) (each kind's Level × √count). The 16 measurements,
all fitted exactly (`TestLaneDaysMeasured`), L_file colour drives →
days:

| L_file | colour | 1D | 2D | 3D | 1H | 2H | 1D+1H |
|---|---|---|---|---|---|---|---|
| 149.5 | blue | 15 | 11 | 9 | 8 | | 5 |
| 113.2 | blue | 12 | | | 6 | | |
| 74.8 | blue | 8 | | | 4 | | |
| 77.8 | blue | | | | 4 | | |
| 141.2 | red | 43 | | | 22 | | |
| 212.3 | red | 64 | 46 | | 32 | 23 | |

(2H = 23 rules out additive Hyperdrives, 16, and a half-step, 22.) The
gizhelp text agrees: "regular Star Lane Drives allow slow travel through
red links, the Hyperdrive makes the use of red links practical". There
is no entry day: a ship that enters a lane on day D (the Hop from its
endpoint, MoveCost paid, by hand or at the close) has Arrive = D +
LaneDays and the ETA shown on D is LaneDays (`TestLaneEntryETA`); with
no power left on D the entry is the next day's first Hop. A lane may be
closed to an empire's entries (`LaneBlocker`: the Arbryls'
SA_BlockStarLanes, a Lane Blocker special; "Species abilities", "Ship
specials"); the Lane Destabilizer and the Lane Magnetron shorten a
crossing to the next close.

**Orders** go through `Empire.Order`; `Starship.Check` says why one is
refused (not adjacent, no lane drive, not in orbit, no colonizer, in a
lane, inhabited planet). GoToPlanet, GoToStar (a lane), `GoToPoint` (any
point within `SystemReach` = 1.5 radii of the star, ours: the original's
limit is not known), Colonise ("Each time a world is colonized, the
Colonizer is used up": the Colony Base lands on `BestCell`, 2 people),
`Invade` ("Invasions"), `GoToSystem` ("Routes"). `Empire.EndDay` runs
colonies, shared research, ships (one order each). The player founds
and leaves orbit at once instead ("il faut le
résoudre avant la fin du tour"): `Empire.Found` / `FoundAt(s, cell)`
lands the Colony Base on the square the player clicked
(`Planet.CanSettle`: empty, no ruins, reachable, black allowed) and
spends the Colonizer; `Empire.LeaveOrbit` puts the ship in space at
`FreeSpot(LeavePoint)`, order cleared, no power spent (the original's,
"discutable"), refused while it is being refitted. The bots keep the
Colonise order at the close (`BestCell`); the sim is byte-identical.

**Routes** (ours, multi-hop routing,
`game/route.go`; the original orders one lane at a time): `GoToSystem`
sends a ship to any star, `Order.Star` the destination.
`Empire.Route(s, dest)` is the way: Dijkstra over the lanes the empire
sees (`KnowsLane`) between the stars it has explored (`Knows`; an
unexplored star is no stop, but it is a destination down a seen lane: a
scout's errand), each lane weighted by its `LaneDays` at the ship's
`LaneStrength` and Traits (red lanes ×3), so the fewest days win over
the fewest lanes; a lane `LaneBlocker` closes today is skipped; ties go
to the lower star index, then the lower lane. The in-system legs are not
weighed. `Empire.Order` refuses the order without a route (no lane
drive, already there, no known way). At every close (`Empire.endDay`'s
ships, the same phase as a GoToStar) a ship out of a lane recomputes its
route and flies the first lane as a GoToStar (`steer`/`unsteer`,
holdAtBlock's swap idiom: the movement code is GoToStar's), the order
kept through the lanes; so a lane learnt, explored or blocked on the way
changes the route at the next stop. The stops make no Arrived (only the
destination's, on the lane exit); at the destination the order is done;
with no route left it is dropped with a `NoRoute` event at the ship's
star. A combat round's move flies the same lane. Bots do not use it (sim
and fuzz byte-identical).

**Orbit** (the original's rule, kept on purpose): a
ship is in orbit when it occupies one of the planet's ten orbit slots
(the 2x5 grid of the colony screen); anywhere else it is in space,
movable, and does not follow the planet. Not ergonomic, but it matters:
a ship in orbit takes damage when the planet is shot at and blocks
invasions, a shield. `Starship.Orbit` is set on reaching the planet's
Pos and cleared on any move; TO MODEL: the slot taken on entering
(refused or waiting when the orbit is full) and freed on leaving.

**The orbital guard** (`game/guard.go`; battle.txt info24:
"Planets which have ships or defenses in orbit are protected from
invasion by enemy species. These are indicated by a colored circle
surrounding the planet in the system area. Foreign ships are prevented
from orbiting a planet that has such orbital protection."; planhelp,
Orbital Shields: "As long as a colony is protected by Orbital Shields,
alien ships may not enter orbit."; the defenses are the orbital
shields, or ANY ship in orbit, whatever the ship). An orbit is a place,
not a point of space: a ship in orbit is no obstacle (ShipGap); the
system display does not draw it (the orbit exists only on the
planet screen; the system display shows only the circle).

`Game.Guard(star, pl)`: a COLONY is guarded by its owner alone, when an
Orbital Shield or Orbital Mega Shield stands in its orbit slots or one
of the owner's ships is in its orbit (a foreign ship in a colony's orbit
guards nothing: it cannot bar the owner's own ships, else the owner is
locked out of its own orbit); an
unowned planet by the empire of the first ship in its orbit (Empires
order, then Ships). Every other empire is refused that orbit, whatever
its stance (allies too), `Empire.Barred`.
`Empire.Order` accepts a GoToPlanet there: the ship BOUNCES (the original:
the shields must be down first; it does not cancel the
order). Empire has no Game: an unexported `peers` link to
`Game.Empires`, set by NewGame and restored by `Game.Relink` (not
saved), nil for a lone Empire (itself only).

A ship whose GoToPlanet target is guarded flies on, and the Hop that
would end within `PlanetClear` (0.2, two sugar cubes, outside the views'
orbit ring: ASSUMPTION) of the planet enters no orbit: the
ship stops in space at `FreeSpot`(the planet's Pos pushed back
PlanetClear along its approach, the ships in space there; along +X from
the centre itself), never on the planet's centre, its order KEPT,
MoveCost paid (Hop's `barred`, read live); from closer than PlanetClear
it backs off to it; standing bounced already (PlanetClear to PlanetClear
+ ShipGap out) it holds, spending nothing, and enters once the guard is
gone. Same-day race: ships move in the existing order (the close's
Empires then Ships, a round's sequential moves): the first to reach a
free orbit enters and guards it, the later ones bounce. A ship already
in orbit stays (a foreign shield is built only by the colony owner, so
it cannot rise under a guest); a ship in orbit of a planet another
empire colonised is left there (it guards nothing). The bots never
target a barred planet (`foundable`), nor a barred Docks (`docksFor`).
`ETA` ignores the guards. The rounds' `Move.Orbit` is the orbit a move
entered (−1: space or a lane), for the replay. Versus fuzz (3 empires,
seeds 1-8, 2000 days) was identical before and after: 225 barred
verdicts in the bots' planning, none changing a choice, no stop short,
no refused order (the bots never bring two empires' ships to one orbit).
The planets as targets and shooters, the ships in orbit untargetable:
"Planets in combat".

**Exclusion radius** ("les vaisseaux se
superposent"; `game/space.go`): a ship in space (out of the lanes and
out of orbit, `InSpace`) keeps `ShipGap` = 0.05 (half a sugar cube) from
the others, any empire. Ships in orbit are drawn on the orbit ring: no
obstacle, never blocked. A Hop that ends in space (every move but the
one entering an orbit or a lane, a GoToPlanet's on the way too) is
shortened along its straight path to the farthest point at ShipGap or
more from each ship in space there (`clearEnd`: the path may pass a
ship, only the end counts); a ship already within ShipGap of another may
leave or slide past it but not end closer than it started; a move
shortened to under 1e-9 is none (no power spent, order kept). GoToPoint
arrives only on its point exactly, but a taken point (an obstacle within
ShipGap) is replaced by its `FreeSpot` each day, so a second ship sent
to it arrives there and its order clears. A ship placed in space (a lane
exit, the arena's side B) goes to `FreeSpot`: its point if free, else
the first free p + k·ShipGap·u (k = 1, 2, …; u the 26 normalised
directions, x then y then z) within SystemReach. The obstacles: at the
close (`Step`), the other ships in space at that star where they are
when the ship steps (empires, then Ships, in order: the earlier ones are
where they went); in a round, the same in the round's ship order (the
ships resolved already where they ended, the others where they
started); a view's Hop, where they are (`Game.Obstacles`). `ETA` ignores
them. So no two ships in space ever end a move within ShipGap, unless
they started it closer (then not closer than they started). Rejected:
round-start obstacles (moves independent of their order) let two movers
stack: two ships leaving one orbit for one point ended on the same spot,
two side by side converged a little every round (ships must
never overlap).

**Orbits** (in the original the planets turn on
their orbits each turn, the inner ones faster, so ranges vary; the lane
endpoints stay on their destination's side; `game/orbitmotion.go`): at
each close (`Game.EndDay`), before the ships move, every planet of every
system turns about its star's vertical axis by 2π/P(r) radians, its
horizontal radius r = hypot(X, Z) and its height Y kept, counterclockwise
seen from above (+X towards −Z). P(r) = `OrbitDays0` × (r / 0.25)^1.5
days, `OrbitDays0` = 30 (ASSUMPTION, ours: the original's speeds are
unknown, tuned by playing later): 30 days at the innermost 0.25, ~68 at
0.5, ~130 at 0.9; a day's step is at most ~0.05 (the innermost). No state
of its own, no draw: the step applies to the saved Pos. The lanes do not
move. A ship in a planet's orbit takes its new Pos (an orbiting ship's
Pos IS its planet's). A ship in space the planet ran over (`swept`:
within PlanetClear of the new Pos, or of the segment old → new at a point
strictly inside it; one the planet only moved away from is left, a push
would pull it in) is pushed PlanetClear off the new Pos, away from it
(along the planet's motion when on its centre), then to the `freeOf` spot
clear of the other ships in space (ShipGap, live) and of that planet:
stars, planets and Empires then Ships in order, its order kept, not
journalled (no decision). The views' orders by planet index read the
planet's Pos live (Hop, the bounce, LeavePoint, the tractor, the bots'
targets and stands, all recomputed each day); a GoToPoint keeps its
point; `ETA` assumes the planets stand still (a GoToPlanet's forecast
may be off by a day). A lone `Empire.EndDay` (no Game: the single-colony
sim, unit tests) turns nothing.

The bodies never meet: `PlanetRadius`(size) = 0.025 + 0.015·size (the
system view's), and two planets' orbits keep, where they come closest
(aligned on one side: hypot(r1 − r2, y1 − y2)), PlanetRadius(a) +
PlanetRadius(b) + `OrbitMargin` (0.02, ours). The generator enforces it
after rolling a system, drawing nothing (every cluster's names, types,
sizes, surfaces and lanes as before): each planet in index order, too
close to an earlier one, moves its Y to the nearest clear height in
[0, 0.6] (a tie goes up), else [0, 0.8], else above all (`spreadOrbits`,
again after a home world replaces planet 0). Seeds 1-200 Average: 2728
systems of 10000 touched, 3457 planets of 30001 moved (1750 up, 1707
down), none out of [0, 0.6]. Saves keep their positions (no heal):
A day-1486 game has 38 pairs too close in 31
systems, whose planets pass through each other.

**Refit (Orbital Docks)** (`game/refit.go`; planhelp.txt:
"Orbital Docks provide a maintenance platform for the repair and
refitting of ships. Any ship orbiting a colony with Orbital Docks can be
refitted. When a ship is refitted, its contents may be changed, and its
hull is repaired."; static.txt: "Being Refitted at %s"; help.txt's event
`Refit of ship "%s" is complete on %s.`; ours "<ship> refitted at
<planet>"): a colony PROJECT (`Empire.Refit`), Item OrbitalDocks (it
occupies the Docks' worker, UsePop 1), `Project.Refit` naming the ship
(names are unique in an empire: every ship comes from `Spawn`, named
"<class> <numeral>" with the first numeral of its class no ship or ship
project bears, a loss's freed again: `ShipNameFor`),
`Project.Design` the target. `CanRefit`: the ship and the colony are the
empire's, the planet holds Orbital Docks, the ship orbits it, the target
keeps the ship's hull (contents change, the hull does not) and is Valid,
something changes (another multiset than the parts aboard, or a damaged
hull), the ship is not being refitted already, then the colony's usual
checks (one project, research, a free worker). The player's target is
the parts chosen on the design screen's refit mode, edited by hand
(antag.exe lets you change the parts by hand); the
ship's template realised today (`RefitTarget`, a template named as its
class) is the one-click option there; a bot passes its own. Cost
(ASSUMPTION, ours: the original's price is not known): the parts ADDED
(multiset target − the parts aboard, `Starship.Aboard`: a spent
Colonizer or Invasion Module is gone, re-adding it is paid) at their
Industry, removed parts refunded nothing, plus the repair pro rata of
the hull's cost, ⌈(MaxHull − Hull) × HullCost / MaxHull⌉ (ASSUMPTION,
ours); a refit of the same design is a pure repair. While it runs the
ship is HELD (`Empire.Order` refuses any order) but still fights with
its current design and can be destroyed: a lost ship's refit is
cancelled at the next close, before the colonies' day. Completed (event
`Refitted`), the ship takes the target: hull full (MaxHull of the new
design), its Colonizers aboard, per-part state reset (Shots, no weapon
chosen), the power left capped at the new day's. A refit is not a ship
for the fleet cap.

ASSUMPTIONS: hull costs 70/140/280/500 (Small ≈ 70 from the guide's
207-industry ship); 3 ships per controlled system (the help says
"proportional"; controlled = the only colonies there,
`StarSystem.Controller`, "Winning"), enforced at the day's close: the
newest ships scrapped (`game/defeat.go`); the Range of weapons and
scanners is in `RangeUnit` = SugarCube / 10 = 0.01 disc radius (gizmos.txt
states sugar cubes for drive Levels only, and
read in cubes every gun covered the whole system, Fourier Missiles 40
twice the disc's diameter): a Mass Barrage Gun's 25 reaches 0.25, a
Tonklin Motor move is 0.2; the exclusion radius `ShipGap` = 0.05 (ours,
the original's spacing unknown).

## Several empires (`game/game.go`)

`game.Game` holds the cluster and its empires in a fixed order and
EndDay plays them one after another (the original takes the species'
turns in sequence). Home stars: the first is the cluster's, the next
ones the 2-3 lane star farthest in lane hops from the homes already
chosen (no PRNG draw, the cluster stays as rolled; seed 1 Average, 4
empires: stars 16, 31, 37, 11, 6-15 hops apart). Each planet carries its
owner's index; "inhabited" is any empire's, so no bot settles another's
planet. Species: the player's, then others drawn from the seed without a
double.

## History (`game/history.go`)

Record only, for the window's graphs (the Stats screen, the save list's
preview): `Game.EndDay` appends a `Sample` to every live empire's
`Empire.History` at day 0 (the first EndDay's start, before the day
runs) and at the close of every `HistoryEvery` = 10th day (after the
invasions, the caps, the diplomacy and the abilities). A Sample: Day,
Colonies, Pop, Industry, Research, Prosperity (the colonies' `Shown`, the
planets list's numbers, an ongoing project's diversion in), Ships,
Fleet (the ships' `Design.Cost`), Techs (known), Stars (stars holding a
colony of the empire), Systems (`ControlledSystems`), Explored (stars
known) and Wars (empires at war with it). A defeated empire stops
sampling, its History kept. No rule and no bot reads it: sim and fuzz
(solo, versus, duel) stay byte-identical. Saved with the empire (about
270 B a sample in the indented JSON: a 4-empire 2000-day save grows
24%, 933 to 1153 KB).

## Fog of war (`game/fog.go`)

An empire sees every star's position and type from day 1 (the original
shows the whole cluster's stars), but a star's name, its planets and the
lanes leaving it only once one of its ships or colonies has been there
(`Visited`, `Visit`, `Revisit` at the day's close; `Knows`). A lane is
known from either end (`KnowsLane`), or on its own once learnt in an
exchange of lanes (`LanesKnown`, `LearnLane`, diplomacy lot 2); the
Oculons know every lane from day 1 (`KnowsStarLanes` trait), not the
stars behind them. Knowledge is live, not a snapshot: a known system
shows its planets and their owners as they are today (ASSUMPTION).
Foreign ships: the partial fog below (it replaced the first rule, "seen
where the empire is `Present`", a colony or a ship out of lanes;
`Present` stays for its other uses); the scanners read what a sighting
hides ("Scanners"). The bot obeys the fog: it cannot target the planets
of an unknown star and explores the nearest unknown adjacent star with a
lane-drive colony ship when no known target remains. The Kambuchka
(`KnowsHomeStars`) have Visited every empire's home star at
day 0 (`NewGame`): explored like their own, so its name, planets, lanes
and ships show from the start and the starmap flags every home
(ASSUMPTION: the original's "see all home stars" read as explored
systems, not a mere marker). The Hanshaks' `KnowsRaces` is in
"Diplomacy".

Partial fog, the original's (the rules layer is
in, `Game.ShipsSeen`/`Hostiles`/`LaneTraffic`, shown by the window too):
- An explored system is watched live. Its foreign ships are seen with
  their owner, hull size and position, but **not their armament**. Its
  planets are seen down to their surface. `ShipsSeen(e, star)`: when e
  knows star, every other empire's ship there out of the lanes as a
  `Sighting` (owner, hull size, star, position, orbit; the design,
  parts, cargo, hull points and shields stay hidden), in empire then
  ship order; nothing at an unknown star.
- Ships in transit on a known lane are seen with their owner and how far
  along, nothing about what they are: `LaneShips(e)`, the starmap's
  pennants (as the original shows them). The bots
  read only `LaneTraffic(e, lane)`, a count (0 on a lane e does not
  know; the Oculons know them all).
- The bots do not cheat: they decide only from this view, so a threat is
  judged by its Scan when scanned, else by hull size (the genes
  HullThreat, HullReach; "Scanners"). The bot's rounds target from
  `HostilesAll` and arm through `CanFireAt`/`Empire.Arm`; `EnemiesAt`
  stays the rules' own list.

The cloakers (planitem.txt's Orbital and Surface
Cloaker, which had no effect before):
- An **Orbital Cloaker** standing in a planet's orbit hides that orbit
  from every empire but the planet's owner (`Empire.SeesOrbit`): its
  structures (the system's planet panel shows no tile, "orbit: cloaked"
  for the guard line, no guard ring; the planet screen no orbital) and
  the ships in it (`ShipsSeen`, hence `Hostiles`, `SightingOf`, the
  player's `Engaged` and the "Combat at" news, skip a foreign ship in a
  cloaked orbit). A ship out of the orbit is seen again; the cloaker
  razed (the cascade's first layer, "Planets in combat"), the orbit
  shows. A shot at the planet stays possible for an empire at war with
  its owner: `CanHitPlanet` judges a cloaked orbit from what the shooter
  sees (hostile through the owner alone, never "nothing to hit", a
  hidden guard never counted), the shooter does not know
  what it hits. Nothing hidden is named to it: a ship destroyed or a
  structure razed in the cloaked orbit is "something in <planet>'s
  orbit" in its news and round log, an invasion refused by a hidden
  guard "<planet>'s orbit is guarded", a Phase Bomb's razing on a
  cloaked surface "something razed"; the starmap's colonists order
  counts no cloaked colony's people.
- A **Surface Cloaker** hides the colony's surface (`SeesSurface`): the
  planet screen draws its squares as bare markers, no structure, ruins,
  people, outputs nor project; name, type, size and owner stay, with
  "surface cloaked". Invasion is unchanged (the invader does not know
  the shields).
- The owner and `ScanAll` (the `-reveal` cheat) see through. **The bots
  are not blinded**: they read `ShipsSeenAll`/`HostilesAll`,
  the partial fog without the cloakers, and the planets' orbits from the
  model as before; the cloakers change their play only through the
  cascade (a cloaker's 15 points in `planetSide`, and the fights a
  cloaker-only colony now starts).

## Scanners (`game/fog.go`)

Sources: gizhelp.txt ("always active and consumes no power", "the status
of other ships"), help.txt ("Scanners allow you to view the status of
other ships"), and b-sting.nl `ships-scan.html` (the table: range
25/50/75/100/200/1000 cubes, strength 1/1/2/3/4/5). The Range is read in
`RangeUnit` (0.01 radius, not cubes; see "Ships"): 0.25 to 10.
- A foreign ship within Range × RangeUnit of one of the empire's
  scanners, at the same star and both out of the lanes, carries a `Scan`
  in its Sighting: its design, hull points (and MaxHull), shields (on,
  ShieldsDown), cargo and energy (and PowerPerDay, Absorb and
  ShieldHits, which the window prints only with the parts).
- The strength is ignored (the range is enough, the original's
  scanners were overpowered). Their behaviour will likely be improved
  later.
- A Cloaker aboard the target blocks every scanner (gizhelp.txt: "blocks
  all sensors").
- The parts need X Ray Megaglasses ("Ship specials"):
  without them a Scan's Design is its name and hull, `Scan.Parts` false.
- A colony's Observation Installation scans the foreign ships at its
  star within `ObservationRange` 100 (the Hyperwave Tympanum's;
  ASSUMPTION, the guide gives no number) of the planet's Pos, from a
  colony of the empire, one per planet enough. Other planets
  do not scan. The Surface and Orbital Cloakers hide a planet's surface
  and its orbit from aliens ("Fog of war", the cloakers).
- **The damage ledger**: without a scan an empire
  still knows the hull points its own shots took off a ship (it saw them
  land). `Starship.Dealt[e]`, saved, counts empire e's hull damage: a
  round's shots, its ships' and its planets' orbital weapons alike, and
  the specials that hurt a hull (`hurt`). Every heal (repair, refit, the
  Sacrificial Orb, SA_HealShips) clamps each entry to MaxHull − Hull.
  `Sighting.Dealt` is the seeing empire's entry, scanned or not.

## The bot's combat policy (`bot/combat.go`)

A choice of the bots, not a rule. The bot sees foreign ships only
through `Game.Hostiles` (the fog's view, scans included), never another
empire's `Ships`; every estimate is redone each day from what is seen
(no memory, nothing saved). The genes tune it (`Genome.Combat`; a nil
genome plays `NeutralCombat`): HullThreat 2/4/6/8, FleeRatio 1,
AttackRatio 0.5, ThreatAvoid 2 (lane days per point of damage, clamped
to [0, 21]: 1 and [0, 10] before the measured lane days doubled the blue
lane's days), WarshipShare 0.1, DefendW 1; for the rounds HullReach 0.45
Pos units for every hull (one Tonklin Motor's move, Level 2 × SugarCube
= 0.2, plus the Mass Barrage Gun's Range 25 × RangeUnit = 0.25: the
first parts research gives, the fixture Raider's; clamped to [0, 5]),
Standoff 0.9, Rounds 30 = `RoundCap`; for the refits RefitGain 0.25
(clamped to [0, 4]) and RepairAt 0.5 ([0, 1]); for the shields
ShieldFirst 1 ([0, 1], GA scale 0.5). `Genome.Clamp` keeps AttackRatio ≤
FleeRatio: a star attack picks is never one the ship would flee on
arrival.
- **Either it flees or it stays** (going back and
  forth is the worst behaviour). A ship leaves a fight only on pressure
  (or unarmed under a gun), never because its shots are spent; and it is
  never sent into, nor through, a star where it would flee: **danger**
  (s, v) is, for an armed s, the pressure of the hostiles at v (Act's
  estimate) on my ships there with s counted once > FleeRatio; for an
  unarmed one, damage seen at v. `search`'s `blocked` (a blocked star is
  never settled, returned nor expanded; the start never blocked) takes
  it in attack and planShip; without hostiles it is never true (games
  without contact play as before, byte for byte).
- **Estimate** at a known star. Theirs: per hostile, damage per day =
  its Scan's `DesignDamage(design, 0)`, else `HullThreat[hull]`; hull =
  its Scan's hull points, else the hull's strength × its owner's
  `HullFactor` less what my shots took off it (`Sighting.Dealt`, at
  least 1; assuming a full hull, every scannerless fight
  fled after the first exchange, the corvette mirror timed out 16/16).
  Plus, per hostile colony there holding an orbital defence
  (`planetSide`, lot B2): damage = Σ Damage × Shots of its
  orbital weapons, hull = Σ Strength of its weapons and shields, at full
  strength (a foreign planet's damage is hidden: OrbitDamage is never
  read). So pressure, flee, danger, the warship want and attack see
  planetary defences, and a star holding only a defended hostile colony
  is an attack target when the pressure is below AttackRatio. Mine: my
  ships there out of the lanes, `DesignDamage` of their design and their
  hull points left (my colonies' defences are not counted).
- **Pressure** = (their damage / my hull) / (my damage / their hull):
  how much faster they kill my side than I kill theirs; 0 when they deal
  none, +Inf when they do and I deal none (so an unarmed ship under a
  gun always flees, whatever FleeRatio short of +Inf).
- **Flee**: a ship out of the lanes where their damage > 0 and the
  pressure > FleeRatio goes to its **refuge**: among the adjacent stars
  not in danger for it (all of them when every one is), the one of least
  seen damage (ties: one of my colonies, then fewer lane days, then lane
  order), unless it is colonising in orbit (it founds today) or has no
  lane drive. It plans nothing else that day.
- **Attack**: an idle warship (armed, no Colonizer) stays where hostiles
  are seen at pressure ≤ FleeRatio; else it heads for the nearest known
  star (lane days, through known stars) whose hostiles, with it added to
  my side there, put the pressure below AttackRatio; else back to the
  nearest star of one of my colonies; both searches skip the stars in
  danger for it, and with no safe colony star it stays. The rounds do the
  shooting (`Bot.Round`, below; `bot.Day` plays the rounds, "Combat
  rounds").
- **Round tactics** (`bot/rounds.go`, lot 2). Each round at each combat
  star, the sides are recomputed from what is seen at the round's start
  (Hostiles, not Act's estimate: ships die between rounds), P their
  pressure; then each of my ships there out of the lanes, in Ships order:
  - **Defence** (lot B2) first: each colony of mine there arms each
    orbital weapon with a shot left today (`ArmPlanet`) at the hostile
    ship in space (not in orbit) within its Range × RangeUnit of the
    planet's Pos of least estimated hull, ties the nearest, then
    Hostiles order; with none in range, at the nearest planet of the
    system it may hit (`CanArmPlanetAt`, ties the lower index;
    `planetAim`); refusals skipped. Their side in the round counts
    their colonies' defences too (as above).
  - A ship being **refitted** does nothing (Order refuses its moves, Arm
    a shooter in orbit).
  - **Flee** when unarmed (a colony ship, a scout) and the hostiles deal
    damage, or armed, P > FleeRatio and it can get away (a lane drive
    with a GoToStar or a refuge, or a safest point out of every
    hostile's reach): an armed ship that cannot escape stays and fights
    or holds, running in vain it would never fire. It arms nothing.
    Colonising in orbit, it stays (founds today). With a lane drive it
    keeps a GoToStar order, else is ordered GoToStar the adjacent star
    `flee` picks (its dest dropped): a lane is safe. Without one (or
    refused): GoToPoint to the safest point among where it is and the 26
    directions (each of dx, dy, dz in {−1, 0, 1}, normalised) at
    SystemReach from the system's centre, the score of a point being min
    over hostiles of (distance − its reach: scanned, its move when its
    energy covers MoveCost plus its longest Range × RangeUnit;
    unscanned, HullReach of its hull); staying put clears a GoToPoint.
    An unarmed ship among hostiles that deal no damage does nothing.
  - **Hold** when armed, P ≤ FleeRatio, and no weapon can still shoot
    today (Uses, energy: a Mass Barrage Gun fires once a day), or
    nothing is there to shoot at (no hostile ship in space, no hostile
    planet `CanHitPlanet` accepts): it arms nothing and any order but a
    GoToStar (a lane departure decided) is cleared. It stays: fleeing,
    attack would send it back tomorrow.
  - **Fight** otherwise (outgunned, cornered ships too). The target is
    the hostile ship in space (one in orbit is hit through its planet
    only) of least estimated hull points (ties nearest, then Hostiles
    order); without one, the nearest planet of the star it can hit
    (`CanHitPlanet` nil: a hostile colony's defences, a planet guarded
    by a hostile ship, ties planet order), armed with `ArmAtPlanet`, the
    stand taken from the planet's Pos. A ship in orbit arms nothing (it
    must leave orbit to fire): its move to the stand takes it out, it
    fires from the next round. The stand is Standoff × its longest
    affordable range from the target on its own side (+X when on it),
    scaled into SystemReach; away from it, the ship is ordered GoToPoint
    there (an older move order replaced, its dest kept so attack plans
    tomorrow), at it a GoToPoint is cleared (it must not drift on). Out
    of its longest range of the target the move comes first: it arms at
    the target, higher Level first, then part order, within its
    generators less one for the move, never a shot that would leave less
    than MoveCost of energy (`Round` moves only when the shots it fired
    leave a generator and MoveCost). Within range the shots come first:
    it arms with every generator and moves to its stand only with a
    generator and MoveCost left (else a GoToPoint is cleared), and with a
    shot armed never to a stand farther from the target (the
    moves resolved first then, and two ships each backing off to their
    stand ended out of range, every shot dropped; kept, though no longer
    needed, since Round fires first). Without it a ship with
    energy left for one action spent it chasing its stand and never
    fired. Range is not checked: an out-of-range shot is dropped after
    the moves at no cost.
  - Act clears the GoToPoint of my ships at a star that is no combat
    star today (attack and planShip plan only ships without an order).
  - **Shields** (ShieldFirst; the draw a reservation). At ShieldFirst ≥
  0.5 (Neutral and a nil genome: 1) the
    bot never touches the switch: on from Spawn, its shots get the
    energy the reservation leaves. Below, a fighting ship whose energy
    with the shields on (Power, or Power − ShieldDraw while off) is short
    of the shots it wants (the weapons that can shoot, higher Level
    first, up to its generators; none in orbit) turns its shields off,
    the draw refunded, and fires; on otherwise (dark too: a dark shield
    still holds its reservation). A ship that flees, holds or is
    unarmed, and every ship at Act, turns them back on (refused, left
    off, when the energy cannot pay the draw). Fight's energy is Power
    as it stands (the stand's move only with MoveCost left).
  - Each bot plays at most its Rounds gene's rounds a day at a star.
- **Avoidance**: a colony ship's route costs ThreatAvoid × the damage
  seen at a star on top of the lane days into it, and a star where
  damage is seen is no target. planShip's route also blocks the stars in
  danger for the ship, i.e. for an unarmed one every star where damage
  is seen, so there ThreatAvoid only shapes `reachable` (the shipyard's
  "is there a target", unblocked).
- **Warships**: only with **contact** (a known star holds a hostile ship
  or another empire's planet), at a shipyard, before the colony ship:
  `game.WarshipOn` (a lane drive when allowed; on a Medium or larger hull
  the best Scanner allowed right after it, its Scan reads
  the foe's hull points) on the largest known hull the colony builds
  within `WarshipDays` (60, `Colony.DaysFor`), down to Medium, kept
  whatever its days (an Enormous took a colony 100 days and
  more; `bot.warship`), carrying one or two specials after the scanner
  (`game.WarshipWith`; `bot.fitted`, random-looking
  and reproducible): drawn among the Specials research allows but the
  Colonizer, the Invasion Module, the lane drives and the Lane
  Destabilizer, by a SplitMix hash of the game's seed, the empire, the
  day and the colony's place, a second one on the hash's low bit; the
  most of them that still leave a weapon; a refit keeps the ship's, while the warships flying and building number
  below ⌈WarshipShare × colonies⌉, or when the damage seen at the
  colony's star and its neighbours exceeds DefendW × my damage there.
  Alone, the bot never builds one: one-empire games play as before.
- **Colony ships' hull** (a larger hull settles
  several planets on one trip): with Colonizers in flight, the smallest
  hull's `game.ColonyShip` within inFlightMax as before; with none,
  `game.ColonyShipOn` every known hull with at most spare Colonizers,
  the one carrying the most that the colony builds within `ColonyDays`
  (60), the smallest hull on a tie, else the smallest hull's
  (`bot.colonyShip`). Measured as the scouts (below), per empire at day
  1290: colonies 17.1 → 18.6, population 534 → 567, systems 8.5 → 9.0;
  Medium to Enormous colony ships fly late in the game.
- **Scouts** (the bots explored only the stars
  they settled, so they rarely met): at a shipyard, after the warship,
  `game.ScoutShip` ("Scout": the smallest hull, engine, Star Lane Drive,
  the best scanner when it fits, generators) while fewer than
  min(`scouts` 2, systems controlled − 1) fly or build, a fleet slot
  stays free after it, and a frontier star is left (`wantScout`). An
  idle scout goes to the nearest unexplored unclaimed star, the
  hostiles' stars blocked (`explore`); once every star is known the
  idle ones are abandoned (`retireScouts`). The slot and the second
  system: two scouts and a warship filled a one-system cap
  (ShipsPerSystem 3) and froze the empire. Measured, mksave dense, 7
  empires, seeds 1-3, 1300 days, per empire: stars explored 37 → 75,
  empires met 2.3 → 5.6 (alliances, a war, an empire eliminated);
  colonies 22.0 → 17.1, population 548 → 534, systems 8.3 → 8.5,
  research 319 → 281 (more contact: courted systems, warships, Docks).
- **Refits** (`bot/refit.go`; the rule: "Refit (Orbital
  Docks)"). Docks: only with contact, ONE per empire: at a shipyard
  colony, right after the Shipyard rule, once research allows them and
  no colony of the empire holds Docks or builds them (a refit is an
  OrbitalDocks project too, told apart by its `Refit` name), waiting for
  a free worker like the Shipyard. **Want** (an armed ship s): the
  warship research gives today on s's hull (`game.WarshipOn`, a lane
  drive when s has one), armed, with other parts, when it deals at least
  (1 + RefitGain) × s's damage (`DesignDamage` against no shield); else,
  s's hull below RepairAt × its full hull, a repair, to that design when
  it deals more than s's, else to s's own. A refit `CanRefit` would call
  "nothing to refit" is never wanted. **Start**: in Act, after assess
  and before any colony plans, each ship in orbit of one of my Docks
  colonies whose project slot is free, NoOrder (or the GoToPlanet that
  brought it), no hostile seen at its star, that wants a refit is
  refitted there, the first in Ships order taking the colony: planning
  never hands the docks of a waiting ship another project. Only a
  **dockable** Docks colony counts, for the start, the wait and the
  route: it makes industry, a worker is free once its running project
  ends, and that project's days left plus the refit's (⌈RefitCost /
  industry⌉) are at most `refitMaxDays` = 60 (a policy constant:
  without it warships parked for hundreds of days at a Docks
  colony with no free worker). **Route**, after flee and before attack:
  a ship being refitted holds; one that wants a refit (out of the lanes,
  NoOrder, no hostile at its star) holds in orbit of a Docks colony
  (waiting for it), at a Docks star is ordered GoToPlanet the Docks
  planet, else heads for the nearest Docks star (search, lane days,
  never into nor through a star in danger for it, as attack; the star in
  dest). No Docks reachable, no lane drive away from one, hostiles at
  its star: attack decides as before. In the rounds a ship being
  refitted cannot escape (`Order` refuses its moves): it fights where it
  is, never flees in vain. Without contact no Docks are built, so
  one-empire games play as before, byte for byte.
- **Guarded orbits** (with the bounce rule): Act clears the
  GoToPlanet of my ships to a planet another empire guards (`Barred`:
  Hop bounces them off it, the order kept), so they plan again instead
  of bouncing for ever. Invasion ships do not wait on a guard either.
- **Invasions** (`bot/invade.go`; the rule: "Invasions").
  Two genes, in the combat block: **InvadeMargin** (modules built past
  the fewest that win, rounded; neutral 1, clamped to [0, 10]: a Surface
  Shield raised during the trip still falls) and **InvadeShare**
  (invasion ships per armed warship; neutral 0.5, clamped to [0, 4]: the
  warships clear the guard first, two where one would flee a defended
  colony). A target is a colony I see at a known star (planets are seen
  down to their surface), of an empire Hostile to mine, guarded by no
  Orbital (Mega) Shield and by no other empire's ship seen in its orbit
  (the rule's invasion guard, read through `ShipsSeen`); its worth is
  `ShieldWorth` summed over its surface (0 for the Minions, who always
  win). **Build**: at a shipyard, after the warship and before the
  colony ship, only with contact, research allowing the module, no
  damage seen at the colony's star, the loaded invasion ships flying
  (Invaders left) plus the Ship projects carrying modules fewer than
  ⌈InvadeShare × armed ships flying and building⌉, and a target: the
  nearest from the colony (its own system first, nearest planet; else
  `search`, lane days, with a lane drive when allowed, never into nor
  through a star where damage is seen). The design (`game.InvasionShip`,
  a policy helper like `ColonyShip`): the best engine, the generators its
  draw needs, a lane drive when allowed, N = ⌈(worth+1)/ModuleWorth⌉ +
  InvadeMargin modules, on the smallest allowed hull holding them (fewer
  modules when none does, never fewer than the ⌈(worth+1)/ModuleWorth⌉
  that win). **Run** (each Act, no memory but dest): a ship carrying
  modules targets the nearest colony as above whose worth its modules
  beat (worth < Invaders × ModuleWorth) and no other order of mine
  claims today; a GoToPlanet to a target still valid is kept, to one no
  longer valid (guarded, captured, shielded past its modules) cleared
  and planned again; in orbit of a target it orders Invade (all its
  modules); in its system GoToPlanet the nearest; else GoToStar the
  first hop (dest the target's star, never into nor through a star in
  danger for it); else it waits. It is unarmed: it flees as a colony
  ship does, except that in orbit with an Invade order it stays (flee
  and the rounds, as Colonise). Emptied, it is idle (no Colonizer:
  planShip gives it nothing); it still counts in the fleet cap.
  Measured (versus, Average, seeds 1-8, 2000 days): 3 empires, 522
  invasion ships, 443 captures, 0 repelled (no bot builds a Surface
  Shield: InvadeMargin is untested by play), the first on days 855-993;
  2 winners of 8 change and the leader's share grows (mean 22.6 -> 29.1
  systems of 50, the last's 12.5 -> 6.3): invasions snowball. 4
  empires: 593 ships, 502 captures, 1 winner of 8 changes. Solo and duel
  unchanged byte for byte (no contact, no module researched).
- **Defences** (`bot/fortify.go`: a bot colony
  fell to one module; "au pire, ça ira dans les niveaux de
  difficulté"). Only while my empire is at War with some empire (at
  peace no cost), on a colony of Pop ≥ **ShieldPop** (neutral 6, [2,
  20]), in `build` right after the warship and before the scout, the
  invasion and colony ships and the economic picks, one project at a
  time, each step while its target is short, counted on
  `Colony.Planned()` (what stands, the running project, the queue: a
  shield razed or an orbital destroyed is rebuilt): surface shields
  until their `ShieldWorth` sum reaches **ShieldWorth** (neutral 4,
  [0, 10]: two Surface Shields or one Mega, a two-module invasion
  falls; the Mega when known, at its `BestCell`); orbital shields up to
  **OrbitalShields** (neutral 1, [0, 4]; the Mega when known; one is the
  guard: no orbit, no invasion); orbital weapons up to **OrbitalGuns**
  (neutral 1, [0, 4]; the best known by Damage × Shots then Range,
  `bestGun` over `game.OrbitalDefences`: Long Range Whopper, Short Range
  Whopper, Missile Base); then an Orbital Cloaker up to
  **OrbitalCloakers** and a Surface Cloaker up to **SurfaceCloakers**
  (neutral 1, [0, 1], when known; their effect is a later game/ lot).
  All rounded, 0 skips a step; GA combat scalars at scale 1. A step that
  cannot start (unknown, no square, no free orbit slot) falls through to
  the next; short of free people it waits, as the Shipyard does, but
  only while the colony can grow (at Capacity the wait would be for
  ever). Measured (versus, Average, 4 empires, seeds 1-12, 3000 days, a
  zz probe over `bot.Day`'s events): wars in 3 games of 12 (the 9 others
  byte-identical); captures 266 → 88, repelled 0 → 0 (the invasion ship
  is sized on the shields it sees, InvadeMargin above them, so it never
  fails; the drop comes from the orbital guard, the guns and the
  slower expansion), population in those 3 games 14769 → 12817 (−13%,
  the defences' cost), colonies 419 → 420; ~1440 defence structures and
  490 cloakers built. sim -galaxy seed 1 600 days unchanged (no war).
- **Captures** in the bookkeeping: `Note` diffs the colonies by
  identity: a colony in neither `Founded` nor `Captured` is new,
  captured that day when Act left an Invade order on its planet, else
  founded; the colonies lost leave both maps. The home's opening script
  plays on `Colonies[0]` only while it is a colony the bot started with
  (`New` founds every colony it is given on day 0): a loser whose home
  was taken plays no script on its next colony. `State` saves Founded
  and Captured aligned with `Colonies` (−1 none); `save.Version` 17.
  cmd/sim prints the invasions with the day's news, `c<day>` in the
  summary's founded column for a captured colony, and "no colony" in the
  header for an empire a loaded save left without any.

## Combat (`game/combat.go`; the 2005 guide and the fan manual)

Hull strength 10/20/30/40 for Small/Medium/Large/Enormous
(`HullStrength` × the species' `HullFactor`). A shot of weapon w does
w.Level damage minus the target's active shield score (Σ shield Level,
"the hull takes the damage of a shot minus the absorbing power of your
shields"; 3 Ion Wraps vs a Plasmatron 4 → 1), at most w.Uses shots per
day (every weapon has a limit, 1 to 5: the fan manual's ships-weapon
page agrees with gizmos.txt on power, uses, strength and range, not on
the industry costs: 10/20/30/40/50/50/70/50/100/100 against the file's
15/25/40/50/60/70/70/70/100/100, perhaps the "hack in
Ship::init_global_gizmos()" the file's header admits; the file wins
until measured in game), w.Power off the day's power, within w.Range ×
RangeUnit (0.01 radius; ASSUMPTION, see "Ships"), same system, neither
ship in a lane.

**Shields** are an on/off switch (on at launch) that RESERVES their
Power (`ShieldDraw`) off the energy, combat or not ("shields only
consume power when active"; measured on test saves 12-13 day
1471: Nanoshell on 10 → 7, off back to 10; next day still 7 with it on,
off gives 10; the record's +148 holds the active shields' Σ Level, 0
once knocked down, the shield's part entry bytes 3-6 ffffffff):
`Starship.SetShields` on takes it at once, refused when the energy is
short; off gives it back in full; `Refill` sets PowerPerDay then takes
it again while on (a design whose day cannot carry it has them switched
off). No round draws anything (an earlier per-round draw is
withdrawn). `Starship.ShieldHits` is the hits they can still take
before a hit knocks them down, as takeShot pays them: Power /
ShieldDraw while they absorb, 0 else, `NoShieldFall` (−1) for shields
drawing nothing (a Wave Scatterer alone, Power 0: never knocked down).
Invariant: while `Shields`, ShieldDraw is already off
`Power` (a refit refunds the old draw and takes the new; a shield part
lost while on refunds its Power). Dark (`ShieldsDown`, Absorb 0: the
Shield Blaster, or a hit the energy could not pay the draw for, below)
keeps the switch and the reservation until the refill.

**A hit on raised shields costs their draw** (measured on
test saves 12-16 day 1488, one save per shot; target
"testweapons", Nanoshell on: Power 3, Level 5, energy 7 after its
reservation, hull 6):

| shot | energy | shields (record +148) | hull |
|---|---|---|---|
| 1. Mass Barrage Gun (Level 1) | 7 → 4 | 5 | 6: absorbed |
| 2. Mass Barrage Gun | 4 → 1 | 5 | 6: absorbed |
| 3. Mass Barrage Gun | 1 | 5 → 0 | 6: knocked down (energy 1 < draw 3), no damage |
| 4. Plasmatron (Level 4) | 1 | 0 | 6 → 2: full damage |

and a Plasmatron absorbed earlier cost exactly 3 too: the cost is the
target's ShieldDraw, fixed, whatever the shot's Level.
`Starship.takeShot`, the one place a weapon's shot lands on a ship
(Round's `land`, ships' and planets' guns alike): shields up (Absorb >
0) and Power ≥ ShieldDraw: Power −= ShieldDraw (the reservation spent
and held again at once) and the hull takes max(0, Level − Absorb) (a
shot stronger than the shield passes its excess, measured too: test
saves 12-13 day 1502, a Nanomanipulator, Level 13, on a Medium behind a
Nanoshell on: energy 7 → 4, hull 20 → 12 = 13 − 5); Power short of the
draw: `ShieldsDown`, no hull damage, Power untouched; dark or off: the
full Level. An Invulnerable target takes nothing, its shields untouched.
Within a round every shot is paid first, then the hits land in Round's
order, so a hit's draw never takes the energy of its target's own shots
that round. Refill clears the dark and holds the reservation again: the
next day the shields are up at PowerPerDay − ShieldDraw (the original
sometimes fails to; not reproduced). The specials' damage keeps its own
rules, no draw: the Carbonizer, the Smart Bomb and the Self
Destructotron are absorbed (Absorb), the Backfirer and the Moving Part
Exploiter not.

Fire is armed (`Empire.Arm`: hostile = at War, `Game.Hostile`) and
resolved by the rounds (`Game.Round`, "Combat rounds"); a
ship at 0 hull points is destroyed (it leaves its empire; a Destroyed
report for each side). The manual's shields-vs-weapons page gives the
planet weapons: Orbital Missile Base 7 damage 1 shot, Short Range
Whopper 7 x3, Long Range Whopper 8 x3, Myrmidonic Carbonizer 20 x1; and
a ship out of power takes full damage, its shields dark (as measured:
the hit above). The specials (repair at sea, the Accutron's range, the
Invulnerablizer, the targeted ones...) are in "Ship specials"; the bot's
shield policy is its ShieldFirst gene ("The bot's combat policy", round
tactics).

**Planets in combat** (lot B1, `game/orbit.go`; our
rules after b-sting.nl's "Shields vs. Weapons", the fan manual and
battle.txt). The table `OrbitalDefences` (by Item, `Item.Defence`):

| structure | Strength | damage × shots/day | Range (RangeUnit) |
|---|---|---|---|
| Orbital Cloaker | 15 (ASSUMPTION) | | |
| Orbital Shield | 15 (b-sting) | | |
| Orbital Mega Shield | 35 (b-sting) | | |
| Orbital Missile Base | 15 (ASSUMPTION) | 7 × 1 (manual) | 40 (ASSUMPTION) |
| Short Range Orbital Whopper | 15 (ASSUMPTION) | 7 × 3 (manual) | 40 (ASSUMPTION) |
| Long Range Orbital Whopper | 15 (ASSUMPTION) | 8 × 3 (manual) | 100 (ASSUMPTION) |

The Long Range Orbital Whopper's 8 is measured (test saves 07-08 day
1502: 8 hull off an unshielded enemy ship).

- **A ship in orbit does not fire** (the
  original's behaviour: it must leave orbit to fight). `Arm` and
  `ArmAtPlanet` refuse a shooter in orbit ("<ship> is in orbit of
  <planet>: leave orbit to fire"); a shot armed before the shooter's
  move into an orbit in the same round is dropped at no cost. It still
  guards the planet and is hit through its cascade.
- **A planet as a target.** A ship in orbit can no longer be fired at
  (`Arm`: "the target is in orbit of <planet>: fire at the planet", the
  ship unnamed as a foreign ship always is; a shot
  whose target entered an orbit during the round's moves is dropped at
  no cost): it is hit through its planet. `Empire.ArmAtPlanet(s, part,
  pl)` arms a ship's weapon at planet pl of its star, refused (after
  Arm's shooter refusals) unless `Game.PlanetHostile` (not the
  shooter's own colony; a colony of a hostile empire, or a planet
  guarded, `Game.Guard`, by one) or a hostile ship is in its orbit
  (an enemy in orbit of your own colony was out of
  every reach); and something to hit in its orbit (`Game.CanHitPlanet`:
  "<planet>: nothing to hit in orbit"). Range to the planet's Pos,
  refused at the arming ("<weapon>: out of range",
  `Starship.PlanetInRange`, the shot was armed then dropped
  by Round in silence) and checked again by Round. The cascade (`orbitTarget`, `orbitLayer`): the
  Orbital Cloakers first, then the orbital shields
  (plain and Mega alike) in slot order, then the orbital weapons in slot
  order, all three only at a PlanetHostile planet (no friendly fire on the
  shooter's or a neutral's structures), then the ships in orbit of
  empires hostile to the shooter (Empires then Ships order; the
  shooter's own skipped).
  Structures absorb nothing ("always take damage with each shot and do
  not recover"): a shot's full Level goes on the slot's hidden
  `Planet.OrbitDamage`; at the Strength the structure is razed (the slot
  freed, its damage reset, Kind `Razed` reports to both sides), the
  excess LOST (no carry-over; the source's shots-to-kill table agrees
  either way). A ship in orbit takes the shot as from a ship (its
  shields absorb). The cloaker is a layer, not a shield: it guards
  nothing (`orbitShielded` names the two shields), so a cloaked orbit
  without an orbital shield is entered and invaded as an open one; a
  cloaker counts as an orbital defence for `CombatStars` (as a shield). Only a rebuild restores a structure
  (`BuildOrbital`/`DemolishOrbital` reset the slot's damage). The guard
  lifts by itself when the last shield and the last ship in orbit go.
- **The blockade** (`game/blockade.go`; ours, the
  original lets the ship under construction be hit instead). A ship of
  an empire at War with a colony's owner, in its planet's orbit, stops
  the colony's work in orbit (`Project.InOrbit`: an orbital structure,
  a ship, a refit): the project stays with its progress, but each
  blockaded day's industry is lost, nothing carried. The blockader must
  see the orbit (`SeesOrbit`): an Orbital Cloaker over the planet keeps
  its yard working. Planet by planet: one ship per orbit held. Marked
  at the day's start (`Game.blockades`, before any ship moves), read by
  `Colony.EndDay`; `Colony.Blockaded`/`Blockader` live for the views
  ("blockaded" where the days left show) and `DaysLeft` (false). News
  (Kind `Blockade`, both sides, `Game.blockadeNews` after the invasions
  and the close of control: a blockader defeated or scrapped that close
  is gone): "<planet> blockaded by the <species>" to the owner,
  "<planet> blockaded" to the blockader, "Blockade of <planet> lifted"
  to both (not to a defeated blockader); the saved `Colony.BlockedBy` (blockader + 1, 0 none) remembers the
  last close's; a capture clears it without news.
- **Simultaneous.** A round's planet shots are distributed over the
  round's pending damage: a structure whose damage plus pending reaches
  its Strength, or a ship whose hull less pending is 0, counts as fallen
  for the later shots, which strike the next layer; a shot finding every
  layer fallen is dropped at no cost. The damage lands with the ships'.
- **A planet as a shooter** ("used exactly like ship components").
  `Empire.ArmPlanet(col, slot, t Sighting)` arms the orbital weapon in a
  colony's slot at a hostile ship in space: once a round, `Shots` a day
  per structure (`Colony.OrbitShots`, zeroed at the close where the ships
  refill), no energy nor generator (planets have no power: ASSUMPTION).
  A ship in orbit is reached through its planet:
  `Empire.ArmPlanetAt(col, slot, pl)` arms the weapon at another planet
  of the system, its cascade struck as by a ship's shot (the original's
  orbital weapons fire at planets; the earlier
  rule "a planet does not fire at a planet" is gone). Refused
  on `ArmPlanet`'s own grounds (not the empire's colony, no weapon in the
  slot, armed already, no shot left, the Tractor Beam used), then no such
  planet, its own ("cannot fire at itself"), `CanHitPlanet`'s verdict
  (a cloaked orbit judged from what the shooter sees), then out of its
  Range × RangeUnit planet to planet; `CanArmPlanetAt` is the check
  alone (the window's label, the bot). Round re-checks the range and
  drops a shot whose cascade fell this round, no shot spent. Round
  fires them after the ships' shots (empire, then Colonies, then Armed
  order), range from the planet's Pos, each doing Damage − the target's
  Absorb (a structure takes the full Damage); the reports name
  "<structure> of <planet>". A colony whose gun reaches a planet it may
  fire at (`planetFoe`: CanHitPlanet and the range) makes its star a
  combat star, no ship needed (the original's bots
  fire at every valid target; mksave -empires 4 -days 1500, seeds 4-9:
  seed 9 alone changes, Bodkin II shelling Bodkin IV from day
  1290). `Fired` carries a planet shooter (`FromPlanet`,
  `ShooterPlanet`, `ShooterSlot`, `ShooterItem`) and a planet target
  (`AtPlanet`, `TargetPlanet`; `Target` the ship in orbit struck, or nil
  with `HitSlot`/`HitItem` the structure); `RoundLog.Razed` lists the
  structures razed.
- **Combat stars.** Besides two hostile empires' ships out of the lanes,
  a colony with an orbital defence facing a hostile ship out of the
  lanes that it can shoot (it has an orbital weapon, the ship is in
  space: an unarmed ship in orbit makes no combat star) or that can
  shoot it (the ship has a weapon part).
- **Seen.** Foreign orbit slots stay visible (a known system is seen
  live); the damage is hidden (the original): views read
  `Planet.DefenceLeft(viewer, slot)`, the owner's only, never
  `OrbitDamage`. Saved (`save.Version` 15).
- Fixtures: `SpawnFixtures` puts the Gunboats and the Raiders in space
  on the home planet (FreeSpot, ShipGap apart), the colony ships in
  orbit: in orbit the fixture fight could not be fought.
- **The planetary Tractor Beam** (`game/tractor.go`). The surface
structure (planitem: cost 50, Stasis
  Field Science) "generates a powerful stasis field to overcome a ship's
  drives and pull it toward a planet. You can use a planet's Tractor
  Beam in the System Display by selecting the planet and then selecting
  the Tractor Beam from its item list" (planhelp.txt); the fan manual:
  "pull ships closer to your planet to use short range weapons or bring
  a damaged and lame ship back home"; James Friel's guide: "you can't
  shoot and use this device at the same time". `Empire.UseTractor(col, t
  Target)` (a ShipTarget: `Mine` one of e's ships, else a `Ship`
  sighting; `CanTractor` the refusals alone, `TractorEnd` the end point
  for the bots) pulls any ship, own, allied or hostile, in space at the
  colony's star (not in a lane, not in orbit: a planet does not act on a
  ship in orbit), at once. ASSUMPTION, no number for the planet in the
  files: the ship gizmo's, `Gizmos[GizmoTractorBeam]`: Range 120 from
  the planet's Pos, the pull its Special1 65 (`specialReach`, 0.65)
  straight toward the planet, stopped PlanetClear off it by `displace`
  (and ShipGap off any ship in the way). Refused: not e's colony, no
  beam on the planet, none left this round, no such ship (a wreck,
  gone), in a lane, at another star, in orbit, out of range, already
  alongside (within PlanetClear, or nowhere clear to come). One pull a
  round per Tractor Beam on the surface (`Planet.Count`), no energy. The
  cost is the planet's round: a pull drops the colony's armed orbital
  shots and `ArmPlanet` refuses "Tractor Beam used this round". The
  count (`Colony.Tractors`, not saved) is reset with `Armed` by `Round`
  for every colony at the star, gunless ones too, by the close (a pull
  out of combat: one a beam a day) and by an invasion. Reports:
  "<planet>'s Tractor Beam: <ship> is moved", Kind SpecialUsed, to the
  user and to the ship's owner when another empire; journal act
  `tractor`. The bot (`bot/tractor.go`, in Round before `defend`): each
  colony with a pull left brings home its own ship below RepairAt ×
  MaxHull (one no other colony of its pulled this round), else pulls the
  first hostile in space the pull brings within reach of an orbital
  weapon with a shot left that does not reach it now: another colony's
  that has not pulled, or its own when none of its weapons reaches a
  hostile now.

`Starship.DamagePerDay(t)` is a day's firepower against t without a
move, range and positions ignored: the day's energy as Refill gives it
(PowerPerDay, less the shields' draw while they are on and the day
covers it: the reservation, taken once), spent in rounds of at most
`Generators` shots, one per weapon a round in part order, each weapon up
to its Uses shots (0 counted as one), each shot its Power while the
energy covers it, each landing as takeShot does on t's shields as they
stand (Absorb while t's energy left now, all of it kept for them, pays
the draw per hit; nothing for the knock-down hit; then the full Level);
the rounds stop when one fires nothing. `DesignDamage(d, shield)` is the
same for a fresh ship of d (shields on) against a shield score whose
energy is unknown: every hit absorbed, never knocked down (the bot's
estimate of its own designs, against 0: exact). `DesignDamageOn(d, t)`
is against a fresh ship of design t (its energy PowerPerDay less the
reservation): the archetypes' figures.

The abstract combat (`Game.AbstractCombat`: one simultaneous
volley per star at the day's close, for the fuzzer) is withdrawn: the
rounds are the one rule.

Fixtures: `-foes N` Raiders of empire 1 at the player's home,
`-warships N` Gunboats (same design: Mass Barrage Gun 1 against Ion Wrap
1, a stalemate).

Ship archetypes (`game/fixtures.go`, arena names): one
warship per hull, each from one tech era's parts, no Special but the
lane drive. Composition rule: the day's supply pays one round of
everything at once (the shield draw, every weapon once, one move) plus a
second round's draw, `Generators ≥ Weapons + 1`, the slots full
(composed under the per-round draw; the reservation takes the draw once,
the figures below DesignDamageOn since the knock-down).

| name | hull | era parts | supply / gens / draw | hull pts / shield |
|---|---|---|---|---|
| corvette | Small | Lancer's: 2 Proton Shaver, Tonklin Motor, Fourier Missiles, Ion Wrap (no room for scanner or lane drive) | 4 / 2 / 1 | 10 / 1 |
| frigate | Medium | 4 Subatomic Scoop, Ion Banger, 2 Molecular Disassociator, Concussion Shield, Subspace Phase Array, Star Lane Drive | 16 / 4 / 2 | 20 / 2 |
| cruiser | Large | 6 Quark Express, Inertia Negator, 4 Ueberlaser, Plasmatron, Deactotron, Aural Cloud Constructor, Star Lane Drive | 36 / 6 / 2 | 30 / 3 |
| battleship | Enormous | 11 Nanotwirler, Nanowave Space Bender, 6 Nanomanipulator, 4 Hypersphere Driver, Nanoshell, Hyperwave Tympanum, Star Lane Hyperdrive | 110 / 11 / 3 | 40 / 5 |

Each outclasses the next smaller one: its shield nullifies or blunts the
smaller's guns (DesignDamageOn frigate 6 vs corvette 0 a day, cruiser 32
vs frigate 2, battleship 155 vs cruiser 8), and a scripted 1 v 1 always
goes to the bigger (`game/archetype_test.go`). Two cruisers against a
battleship: mostly the battleship still, but its Nanoshell's energy runs
out under their shots (`fuzz -mode duel -arena
battleship:cruiser,cruiser -seeds 1-8 -swap`: 16-0 for the battleship
before the knock-down, 10-6 after).

## Population transport (`game/empire.go` Load/Unload)

Ours, not the original's: a young colony's limit is its population, so
a ship carries people. The original's own hook, Interstellar Trucking
(planitem.txt, ResearchReq 254, "Interstellar Trucking is a myth."), was
moving industry: rejected as unbalancing. The Space Habitat (`Gizmo` 76,
past gizhelp's 76): a passive Special, Power 0, Industry 50, Special1 3 =
the units one module berths, research Planetary Replenishment (the
Habitat's). `Starship.Pop` aboard, `Berths()` = modules × 3. Load: one
unit, in orbit of our own colony, from its `Free()` (neither employed
nor reserved), the colony keeping one. Unload: one unit onto our own
colony with room; it lands free. No ally's or foreign colony, no
founding (the Colonizer's). A refit to fewer berths than Pop, an Abandon
with Pop aboard: refused. A ship lost with Pop: its owner alone hears
"N colonists lost aboard X" (combat, specials, self-destruct, scrap); a
lost module (LosePart) caps Pop at the berths left. The window: Load and
Unload in the planet screen's ship dialog, one unit a click, the box
kept open, an "Aboard" row of circles by module; the POP panel above
shows the colony. The bots' (`bot/habitat.go`): a donor
is a full colony (Pop at Capacity) with a free unit, a recipient one
at Pop ≤ `HabitatFill` × Capacity; at a shipyard, after the colony
ship, a Habitat ship (`game.HabitatShipOn`, the hull of most berths
built within `ColonyDays`) while the Habitat ships flying and building
number fewer than ⌈`HabitatShare` × min(donors' Free, recipients'
room) / berths⌉ and a fleet slot stays free. Each ferries: in orbit, it
unloads on a recipient, else loads at a donor, until refused; loaded,
it goes to the nearest recipient (`invasionTarget`'s routing, danger
avoided) whose room the people already sent there leave open, empty to
the nearest donor; an order stands while its target qualifies; never
abandoned. Genes `HabitatShare` (0 builds none) and `HabitatFill`, 0.5
each for a nil genome and Neutral; a genome file without them reads 0.
Warships never carry one (`carried`). A crew launched at a
donor leaves it one short of full, no donor until it regrows.

## Invasions (`game/invade.go`, the bots': "The bot's combat policy")

Sources: gizhelp, Invasion Module: "transports a planetary infiltration
team and their camouflaged base of operations to a planet's surface from
orbit. It can be destroyed by a planetary Surface Shield unless multiple
Invasion Modules are launched to overload the Surface Shield's tracking
ability. It is expensive to assemble and can be used only once."
battle.txt info24: ships or defences in orbit protect from invasion.

The rule: exactly like colonising, on a planet a hostile
empire owns. `Starship.Invaders` is the modules left (the Design keeps
the parts, as `Cargo` the Colonizers; Spawn and a refit load them);
`Usable` offers "invade". The `Invade` order (appended to OrderKind) is
given in orbit with a module left (`Check`); `Empire.Order` refuses an
unowned planet, one's own, and a planet guarded by anyone but the
invader: the owner's Orbital (Mega) Shields or a ship of any other
empire in its orbit (the guard must be cleared first, planet combat).
The order moves nothing, has no ETA.

The window's Invade button resolves it AT ONCE (`Game.InvadeNow`, the
modules and shields are spent and the colony
changes hands on the click, journalled "invade"); the bots' Invade
orders resolve at the day's close, after every empire's (`Game.EndDay`
-> `Game.Invade`), Empires then Ships order, re-checked as Order checks
it (a refused one is dropped silently, its modules kept, as Found drops
a refused Colonise). The modules launched are spent: on a success only
the fewest whose worth beats the shields' total strictly (not all of them, as first read; the rest
stay aboard for another planet), on a failure all of them. Each module
is worth one Surface Shield, a Surface Mega Shield 2.5 (`ModuleWorth` 2,
`ShieldWorth` 2/5, in halves: exact integers). The shields fall in cell
order (`Planet.Cells`, plain and Mega alike, as the orbital cascade goes
in slot order: no preference invented) while the modules' worth left
covers theirs; one too dear survives and a later cheaper one may still
fall. Success iff the modules exceed the shields' total STRICTLY (2 vs 2
fails: the shields fell, nobody is left to land). Examples: 3 vs 2
shields, success, both destroyed, 3 launched; 6 vs none, success, 1
launched, 5 kept; 4 vs a Mega, 3 launched; 3 vs 5, failure, 3 destroyed,
2 left; 2 vs a Mega, failure, the Mega stands; 3 vs a Mega, success; 3
vs shield+Mega (cell order), failure, the shield falls, the Mega stands.
Minions (SA_SuperMarines, `Traits.InvadesAlways`): always a success on
ONE module, every surface shield destroyed, the rest kept.

Success: the colony changes hands AS IS (structures, Pop, Growth); its
builds follow the winner's traits (Orfas build on
a captured planet's black squares; a colony has no species of its own,
`Colony.Traits` are its owner's); out of the loser's Colonies (order
kept), appended to the winner's, `Planet.Owner` set; only its project is lost (a refit there
with it), its orbital weapons' day (`Armed`, `OrbitShots`) cleared. Same
day, later invaders judge the new state: the winner's other ships are
refused (their own planet), a third empire's are refused by the winner's
ship in orbit (the guard; two foreign empires' ships in one orbit, an
old state, guard it against each other). An empire with no colony left
controls no system: it is defeated at the day's close ("Winning").

The events (Kind `Invaded` / `Repelled`, at the planet, for both sides,
appended to each empire's slice of `Game.EndDay`), help.txt's texts:
invadesuccess "We have captured %s, losing %d invader%s in the battle.
We now have %d colon%s in our empire." (one space after "battle.", the
file has two); invadefailure "Our invasion of %s failed. We lost %d
invader%s in the battle."; EP_COLONYINVADED "Our colony on %s was
invaded by the %s." for the loser; and OURS for a repelled defender:
"Our colony on %s repelled an invasion by the %s, losing %d surface
shield%s." Saved (`save.Version` 16).

## Combat rounds (SPEC, the rule engine in, lot 1)

The original mixes two things in its "power"; ours separates them.

**Energy and power.**
- **Energy** is the day's budget: `Starship.Power`, refilled at the
  day's close to Σ generator Power. A move pays MoveCost and a shot pays
  its weapon's Power. Shields that are on hold their draw off it, a
  reservation taken when switched on and at each refill and given back
  when switched off, never per round; a hit the raised shields absorb
  pays their draw again off the target's energy, the hit short of it
  darkens them (`ShieldsDown`) and does nothing, the next ones the full
  Level (`takeShot`; both measured: "Combat").
- **Power** is what can run at the same time: the **number of
  generators** aboard (`Design.Generators`). Within one round the engine
  takes one generator if the ship moves, and each weapon that fires
  takes one. Shields take none. Power renews every round.
- So two weapons fire together only with two generators ("a
  generator is needed for a gun"). The original could not fire several
  guns at once.

**The round** is the day's unit inside a system where hostile ships
meet (both out of the lanes: a combat star). A round is at most one move
per ship, plus the shots its power and energy allow. Each side decides
blind, without seeing the others' choices, and then the round resolves:
1. Every armed shot fires first (shots before
   moves), on the round's starting positions, instantaneous and
   simultaneous: all are computed on the state before any damage, the
   damage lands together, and a ship destroyed in the round has still
   fired. A shot is dropped, at no cost and taking no generator, when
   its target is gone (destroyed, in a lane, in an orbit) or is out of
   range at the round's start; a target that leaves the range, enters
   an orbit or a lane this round is hit all the same.
2. Then every surviving ship with an order moves one Hop toward it, one
   after another in the round's ship order (empire, then Ships: see
   "Exclusion radius", an earlier mover takes a spot first). A ship
   moves only if a generator and the MoveCost are left after the shots
   it fired (and the draws its raised shields paid for the hits). A ship
   destroyed in the round neither moves nor blocks a spot.
3. A ship that enters a lane leaves the combat from that round on.

Fire stays manual, as in the original: activate a weapon, pick a target
in the system (`Empire.Arm`), with several weapons in one round as power
allows. Orders persist over rounds, so a GoToPoint advances one Hop a
round while the energy lasts: the round only makes explicit the move
sequence the day already is. A system with no hostile plays no round,
and its ships Step at the close.

**The day's end.**
- A bot is done at a star when it neither moves nor fires, or at its
  round cap (a bot setting).
- The player plays rounds where they want. On "next", if one of their
  ships still has energy at a combat star, the game says so and offers
  to confirm or to go there (the original's heuristic).
- Once the day is confirmed, the player's ships at combat stars pass
  their remaining rounds (no move, no shot) while the bots play on, off
  camera: they may destroy them, the price of a hasty "next".
- The day ends when every side has passed. Ships at a star that played
  rounds do not Step at the close, while the fight goes on there; a
  fight ended during the day (the foe destroyed or gone) lets them Step.

**Seen.** The partial fog is unchanged. A scanner in range adds the
target's energy left to its Scan: without one, you do not know whether
an enemy can still fire.

**One rule.** The round engine is the combat rule for the window, the
bots and the fuzzer alike, with the energy simulated. The abstract
combat is withdrawn. If the rounds prove too slow for the fuzzer, the
engine is optimised; no second rule is added.

**As implemented (lot 1, `game/rounds.go`, `bot.Day`).**
- `Design.Generators` is the power; `Starship.Power` stays the energy.
  `Starship.Armed` holds the round's `Shot`s (part, target Sighting):
  `Empire.Arm` refuses not one of the empire's ships, not a weapon,
  already armed this round, no shot left today, no generator free, the
  energy short of every armed shot (the shields' reservation already off
  it), a lane, a gone/own/elsewhere target; not the range. `Disarm`
  clears. Armed is not saved (a Shot's target is a live pointer): Refill
  and every Round clear it, a day never starts armed.
- `Game.CombatStars`: the stars, ascending, where two mutually Hostile
  empires have ships out of the lanes, or a colony's orbital defence
  faces a foe's ship (`facesFoe`) or its gun a planet (`planetFoe`). `Game.Round(star, passed)`,
  shots before moves: first the shots on the round's starting
  positions (dropped at no cost, taking no generator, when the target is
  gone, in a lane or an orbit or out of range, when the shooter is in an
  orbit or Scrambled, or when the energy left no longer covers it, in
  part order: a shield switched on after arming), every shot paid first,
  then the hits landing in order through takeShot (the target's shields
  paying their draw per hit, dark short of it), the colonies' orbital
  weapons after the ships', the damage summed per target and landing
  together; the destroyed get their two Reports, credited to the first
  shooter in empire, ship, Armed order. Then the survivors' moves (one
  Hop each, in empire then Ships order, when a generator and MoveCost
  are left after the shots they fired and the draws their shields paid;
  each keeps `ShipGap` from the other surviving round ships in space as
  they stand at its turn, the earlier ones where they ended: sequential,
  so no two end within ShipGap; a destroyed ship blocks nothing).
  acted = a move or a shot; the third return, a `RoundLog` (the moves
  From→To, a lane entry to its endpoint; the shots that fired with their
  weapon, the round's starting positions as ends, Damage and Absorbed =
  Level − Damage, ShieldsDown on the one hit that knocked the target's
  shields down; the destroyed in order), is for the
  window's replay, `bot.Day` returning every round's in play order; the
  star's count of the day's rounds goes up (`Game.RoundsToday`, below):
  at the close its ships out of the lanes are refilled, not Stepped (a
  Colonise order still founds).
- `bot.Day`: every bot Acts (orders only), then at each combat star
  (computed once) rounds 0.. up to `bot.RoundCap` (30): the bots'
  `Round` in empire order (lot 2's tactics, "The bot's combat policy"; a
  bot passes from its Rounds gene's round on), the player (a nil bot)
  playing round 0 only (its armed shots, its orders) and passing after;
  stop at a round that does nothing. Then `Game.EndDay`. `Scan.Energy`
  is the target's energy left.
- The window's fire (F, the fire mode's click) arms; the shot fires in
  the next round played.
- The window's day in steps (`bot.DayRun`: `Begin`, `Star`,
  `Round(manual)`, `Auto`, `Finish`; `bot.Day` = `Begin(..).Finish()`,
  byte-identical). Since combat mode the window's close
  never pauses: `cmd/aiscendancy/roundmode.go` plays every combat star by
  `Auto`, then `Finish`; the player fights during the day instead (the
  next item), warned at the day's end of ships with energy left where it
  is engaged. Where the player is engaged the window hops no ship by
  hand (an order is only set): the rounds move it, a hop each, as the
  bots'.

- Mid-day rounds (combat mode lot 1; screens.md "Combat
  mode"): `Game.StepRound(e, star, passed)` is the player's step, an
  entry point: refused unless `Game.Engaged` (e knows the star, it is a
  combat star, e sees a hostile ship there or a hostile colony with an
  orbital defence in an orbit it sees or that one of e's guns there may
  hit, blind at a cloaked orbit (before, the gun's owner
  stayed out of a fight its gun started, its ships there held off the
  close's Step with no round to play), and e has a colony there or a
  ship out of the lanes; a shielded planet alone used to
  leave the player's combat mode shut, "Combat at X: <species> planets"), else one `Round`, journalled ("round", the
  star). `bot.Step(g, bots, star)` wraps it for the window: refused at
  `RoundCap` rounds there today, else every bot below its Rounds cap
  plays `Bot.Round` first (the others passing), then StepRound for the
  empire without a bot. `Round` counts into `Game.RoundsToday` (per
  star, saved, nil after EndDay), which replaces the unsaved fought
  marks: a star with rounds today, still a combat star at the close
  (`Game.EndDay` masks the count with `CombatStars`), has its ships
  refilled, not Stepped, at the close, also after a save and a load mid-day. The close's
  `DayRun` reads the same count: a bot passes once the day's rounds
  there reach its cap, a star is done at RoundCap of them (one reached
  mid-day is dropped unplayed), so the caps hold over the whole day.
  The player still plays the close's FIRST round at each star (its
  standing orders and armed shots) and passes after, unless stepped by
  `Round(true)`. `Starship.UsableEnergy`: energy left for a hop
  (MoveCost, with a drive) or a shot of one of its weapons, for the
  close's warning.

**The display** (combat mode, screens.md "Combat mode"):
the system panel's left button steps a round where the player is
engaged, Repeat Move and Auto unchanged (refused there). Lot 2 is the bot's round play, lot 3 the arena (`mksave -arena`, 2
v 2 around a planet, and `fuzz -mode duel`), lot 4 the display.

## Ship specials (`game/special.go`)

Source: gizhelp.txt's text per gizmo (quoted below and on each effect's
func), gizmos.txt's numbers (Power, Uses, Special1, Special2: read from
`Gizmos`, never retyped).

**The frame.** `Empire.UseSpecial(s, part, Target)` uses a special AT
ONCE (the original fires one when clicked; not armed for the round as a
weapon) and returns its Reports: reps[0] the user's (Kind `SpecialUsed`
at the ship's star, a past-tense line), then the victims' (a kill's
Destroyed pair, the Phase Bomb's Razed; at a foreign ship whose owner
the effect told nothing, that owner gets the user's line, the user's
name replaced by "a <Species> ship": a Disarmer
took a gun unannounced); the window shows reps[0] and
logs them all, as an ability's. The user's line never names a foreign
target either: "a <Species> ship" in its place (`hideName`, after the
user's own name). The user's report and the
victim's copy carry `Event.Use`, a `SpecialUse` (`json:"-"`: never
saved): user, gadget, star, target kind, target ship and owner (−1
none), planet or lane index, and Outcome, each effect's short result
naming no ship nor planet ("tied down", "-3", "Ueberlaser destroyed",
"shrugged off", "2 ships moved"), for the window's round log. The
planet's Tractor Beam (`UseTractor`, no ship user) carries none. `Target` is {Kind: NoTarget, ShipTarget,
LaneTarget, PlanetTarget; Ship a Sighting of another empire's ship, or
Mine one of the user's own (exactly one of the two for a ShipTarget);
Index the lane's in System.Lanes or the planet's in System.Planets}; the
kinds a special takes come from its flags (GzTargetShip, GzTargetLane,
GzTargetSurface; none of them: NoTarget). Refusals in order: not one of
e's ships; being refitted (held at its Docks like a ship under
construction: no weapon, no special); no such part;
not a special; the Colonizer and the Invasion Module ("used by an
order": Colonise, Invade); passive ("<name> is always on"); a target
kind it does not take ("needs a ship target", "takes no ship target");
not modelled yet; in a star lane; in orbit ("in orbit: leave it first":
a ship in orbit only leaves it, colonises or
invades; gizhelp names orbit for the Colonizer and the Invasion Module
alone); no use left today (its `Shots` entry, parallel to Parts and
zeroed by Refill as a weapon's, against Uses, 0 no limit); its Power
above the energy left; then the target's (`targetCheck`, below); then
its own refusal. A refusal changes nothing. A success pays one use and
the Power (no energy is reserved for armed shots: one left unpaid is
dropped by the Round at no cost), applies the effect, and a one-time
device is lost (`LosePart`: a NEW Parts slice, the Design's being shared
with its template and sister ships; Shots and the armed shots' parts
shifted; Weapon −1 from the lost index on; Cargo and Invaders capped at
the Colonizers and modules left). No Game is needed: the other empires
are the Empire's peers, a kill goes through `destroyedIn`
(Game.destroyedBy's one implementation). A refit restores the full
design, paying the lost parts again (RefitCost counts them as added).
`Starship.Usable` gives "use" or the same refusal text from the same
helper (`specialReady`, the checks needing neither the empire nor the
target: a modelled targeted special reads "use", the aim is
UseSpecial's). `Boosted` and `Invulnerable` are day-scoped, cleared by
Refill, saved (save.Version 28).

**The target's refusals** (`targetCheck`, lot 2): a target kind the
effect does not aim at is "not modelled yet" (the Carbonizer's lane
and planet targets, never to be modelled; since lot 3 every non-passive special has an
effect, so specialCheck's own "not modelled yet" names none). Then the
range: the gizmo's Range × RangeUnit from the user's Pos to the
target's (never Boosted: the Accutron is the weapons').
- Hostile-targeted (every ship-targeted one but the Sacrificial Orb and
  the Plasma Coupler): `Empire.Arm`'s own refusals, shared code: the
  user in orbit (refused earlier by the frame), a Mine target "not
  hostile", then `fireTarget`: gone (Hull out, or no longer in its
  empire's Ships: a stale Sighting; Arm reads it too now), not hostile
  (not at War), in a lane, another system, in orbit ("fire at the
  planet").
- Ally-targeted (Sacrificial Orb, Plasma Coupler): Mine one of the
  user's other ships ("not on itself"), or a Sighting of an empire at
  Alliance ("not an ally"); out of the lanes, same star, orbit allowed.
- Planet-targeted (Phase Bomb): Index a planet of the user's star
  ("no planet N"), owned by an empire at War ("not hostile").
- Hostile or friendly (lot 3: Tractor Beam, Hyperswapper; gizhelp
  "another ship"): the user in orbit refused by the frame; a
  Mine target or a Sighting of an ally goes through the ally rules, any
  other Sighting through the hostile ones; a target in orbit refused
  either way ("is in orbit", "fire at the planet").
- Lane-targeted (lot 3): Index a lane endpoint of the user's star's
  System.Lanes ("no lane N"; the cluster lane is Star.Lanes[Index]), the
  range measured to that endpoint. The user may be in orbit.

**Day flags** (lot 2, saved: save.Version 29; cleared by Refill):
`TiedDown` (Hop moves nothing, the order and the energy kept: the
Round's move pass finds no move), `Scrambled` (Arm refuses "crew
scrambled"; Round drops a shot armed before at no cost), `ShieldsDown`
(Absorb 0, Shields and their reservation untouched: back tomorrow;
switching them off while dark refunds the draw; set by the Shield
Blaster or by a hit the energy could not pay the shields' draw for).

The actives (self-targeted):
- Hyperfuel (P0, Special1 40): "a one-time-use power reserve of great
  capacity": +Special1 energy, the part lost. The energy lasts the day
  (Refill gives the plain supply).
- Toroidal Blaster (P0, Uses 1, Special1 40; the file spells it
  Torroidal): "a huge boost in engine performance, but it is hard on the
  engines and usually damages some of them": +Special1 energy, the FIRST
  drive aboard lost (ASSUMPTION: always one). Refused "no engine
  aboard". With no drive left a ship takes no move (Check: "no
  engine"; Hop drops the order).
- Cannibalizer (P0, Uses 1, Special1 5, Special2 20): "convert some of
  the mass of its own hull into an energy reserve": Hull −Special1,
  energy +Special2 (ASSUMPTION: S1 the hull, S2 the energy). Refused
  "would scuttle the ship" at Hull ≤ Special1.
- Replenisher (P8, Uses 1): "fully recharges all of the weapons aboard a
  ship": every weapon's Shots back to 0. Refused "nothing to replenish"
  when none fired today.
- Remote Repair Facility (P30, Uses 0, Special1 5): "repair damage
  without having to enter Orbital Docks": Hull +Special1, capped at
  MaxHull, as often as the energy lasts. Refused "hull is full".
- Accutron (P7, Uses 1): "increases the effective range of all the
  weapons on a ship": Boosted, `WeaponRange` (Range × RangeUnit) doubled
  (the 2005 guide's "Range w/ Accutron" is twice) for every ship-weapon
  range check (CanFire, the Round's ship and planet targets); planet
  guns and scanners are not boosted. Refused "already active today".
- Invulnerablizer (P20, Uses 1, one-time): "the shield will not allow
  the ship to come to harm ... can be used only once": Invulnerable for
  the day, the part lost; in a Round every shot at it (a ship's, a
  planet gun's, or a planet shot whose cascade reaches it in orbit) is
  paid and lands nothing (Damage 0, Absorbed the full Level/Damage, no
  credit), as a Capelon structure under SA_ShieldPlanets. Refused
  "already invulnerable today".
- Recaller (P7, Uses 0): "moves instantly into the shunt and arrives
  immediately at its home system": `Empire.HomeStar` (the star NewGame
  put its home world on, pickHomes over the Game's empires: no field,
  no PRNG, the same whoever holds that world today), in space at the
  FreeSpot of the home world's (planet 0) LeavePoint against every
  empire's ships in space there, its order and armed shots dropped.
  Refused "already home" (a refit and an orbit: the frame's refusals).

The passives:
- Star Lane Hyperdrive (Level 2): "a more powerful version of the Star
  Lane Drive that improves speed through star lane space":
  `Design.LaneStrength` counts it 2 × √(Hyperdrives aboard) against a
  Drive's √(Drives aboard) (measured, "Lane days"), and it is a lane
  drive for Check (`LaneDrives`).
- Gyro-Inductor: "generate power when it travels up the gravitational
  gradient of a planet. When the ship leaves orbit, the Gyro-Inductor
  creates power": a Hop that leaves an orbit (the bounce off a guarded
  one included) gets its MoveCost back with a Gyro-Inductor aboard,
  once whatever their number (ASSUMPTION, no number in the table: the
  leaving move is free; otherwise two of them would mint
  energy per orbit cycle). `LeaveOrbit` spends nothing
  already: no gain there.
- X Ray Megaglasses: "allow you to view the contents of any ship you are
  able to scan": a Scan's parts are known (`Scan.Parts`, transient)
  only when one of the empire's ships at that star, both out of the
  lanes, carries them (the table's Range 120 is not read: ASSUMPTION),
  or under ScanAll; otherwise Design is {Name, Hull} with Parts nil,
  hull points, shields, cargo and energy still read. The bot's
  `sightingSide` falls back on HullThreat without the parts.

The targeted actives (lot 2, `game/targeted.go`; "once": the part lost
after the use; a kill is the Destroyed pair credited to the user, the
victim's ledger counting it; an Invulnerable ship takes no damage from
any of them):
- Molecular Tie Down (P7 R50 U1): "temporarily immobilizes it": TiedDown.
- Intellect Scrambler (P7 R50 U1): "can turn an experienced crew into a
  group of bumbling rookies": Scrambled, its armed shots dropped
  (ASSUMPTION: no crew experience is modelled; the rookies cannot fire
  today).
- Brunswik Dissipator (P15 R50 U1): "drains the target ship's
  generators, leaving it powerless": its energy 0, armed shots dropped.
- Disarmer (P5 R60 U1): "selectively destroys weapons": its first
  Weapon part lost. Refused "no weapon aboard".
- Containment Device (P5 R50 U1): "destroys one colonizer or invasion
  module": the first one still loaded (Cargo or Invaders > 0: a spent
  Colonizer's part stays in the Design and is skipped) lost, its count
  one down. Refused "no colonizer nor invasion module aboard".
- Shield Blaster (P7 R50 U1): "temporarily shuts down the defensive
  systems": ShieldsDown.
- Backfirer (P10 R50 U1): "the more weapons the target ship carries, the
  more it is damaged by its own weapons": Σ Level of its Weapon parts,
  its shields not absorbing (ASSUMPTION: the damage is internal).
  Refused "no weapon aboard".
- Moving Part Exploiter (P10 R50 U1): "the more intricate and advanced
  the equipment ... the more internal damage": one hull point per part
  aboard, no absorb (ASSUMPTION).
- Gizmogrifier (P4 R60 U1): "will destroy the first major device it
  locks onto": its Parts[0] lost (ASSUMPTION: design order). Refused
  "no part aboard".
- Specialty Blaster (P5 R60 U1): "destroys one specialized device": its
  first Special part lost. Refused "no special aboard".
- Disintegrator (P25 R25 U1, once): "cannot be blocked by normal
  defenses": the target destroyed outright; an Invulnerable one shrugs
  it off, the use spent (ASSUMPTION: the Invulnerablizer's "will not
  allow the ship to come to harm" beats "normal defenses").
- Myrmidonic Carbonizer (P4 R40 L20 S1 20 U1): "grows in strength for a
  while then dissipates. There is an ideal range": round(Level × (1 −
  |d − ideal| / ideal)), ideal = Special1 × RangeUnit, never below 0
  (ASSUMPTION: a triangle, 0 at the user and at Range = 2 × ideal),
  less the target's Absorb. Ship targets only (lane and planet "not
  modelled yet").
- Sacrificial Orb (P3 R70 U0, S1 6): "repair damage to another ship by
  absorbing that damage itself": the target +min(Special1, its damage)
  hull, the user − the same. Refused "nothing to repair", "would
  scuttle the ship" (the user's Hull ≤ the amount).
- Plasma Coupler (P4 R70 U1): "beam some of its power to another ship":
  half the energy left after the cost (floor) to the target's
  (ASSUMPTION: no number). Refused "no energy to beam" when that is 0.
- Phase Bomb (P4 R25 U1, once): "launched at a planet and destroys
  structures on the planet's surface": the first structure in cell order
  (`Planet.Cells`) other than the Colony Base demolished (`Planet.Demolish`,
  Abandon's core: the workers stay, free); the owner's report Kind
  `Razed` (now any structure destroyed in combat; no reader in cmd/
  tells orbital from surface), Item the structure. Refused "nothing to
  destroy". An owner invincible today (SA_ShieldPlanets) shrugs it off,
  the use spent.

The area actives (NoTarget; the victims collected before any damage
lands, in empire then Ships order, in space only: not in orbit, not in a
lane):
- Smart Bomb (P7 L7 S1 15 U0, once): "fires in multiple directions at
  every ship it has determined to be hostile": every ship of an empire
  at War within Special1 × RangeUnit takes shotDamage(Level, its
  Absorb). Refused "no hostile ship in range".
- Self Destructotron (P0 L20 R40 U0): "destroys the ship using it ...
  a huge explosion that greatly damages other ships nearby": every other
  ship of every empire, the user's own included, within Range ×
  RangeUnit takes shotDamage(Level, its Absorb), kills credited to the
  user (an own ship's pair goes to the user's empire both ways, no
  ledger); then the user is destroyed, its SpecialUsed report "X
  self-destructs" its only one. Never refused but by the frame.

**The bot's specials** (`bot/special.go`). In each Round,
before anything is armed (`specials`, ahead of the sides, defend and the
flee/hold/fight switch), each ship of the bot's at the star, out of the
lanes and not being refitted, tries the rules below in this order, each
re-read after the uses before it; a rule that holds uses the first part
of its gizmo `Usable` calls "use" (`UseSpecial`; a refusal costs nothing,
the use's Reports go to the bot's news: the SpecialUsed line, the
victims' Destroyed and Razed; `bot.Day` gathers that news after each
Round, the same day). The ships are walked on a snapshot of
Ships (a self-destruct or a blast shrinks it); after a use a ship no
longer mine or at the star ends its turn, and the hostiles and their
damage are read again (a kill, a part lost: never a dead sighting aimed
at), none left ends the pass; Round fights the hostiles as last read.
Each needs hostiles seen there (Hostiles);
"under fire" is their side's damage > 0 (`theirSide`, Round's own sum:
sightingSide + planetSide). The ship's own state is read directly.
- Recaller: under fire, Hull ≤ RecallAt × MaxHull, not at HomeStar (it
  leaves: nothing else is tried, Round's switch skips it, no longer at
  the star).
- Invulnerablizer: under fire, Hull ≤ ShieldAt × MaxHull.
- Accutron: a weapon aboard (so the standoff and the shots-first test,
  `longestRange` = `WeaponRange`, read the doubled range).
- Replenisher: a weapon with Uses > 0 aboard and every such one spent
  (Shots ≥ Uses).
- Hyperfuel: energy < MoveCost + the cheapest weapon's Power (0 without).
- Cannibalizer: the same energy test, Hull > CannibalAt × MaxHull.
- Toroidal Blaster: energy < MoveCost, two drives aboard or more (a
  last-resort burst that burns one).

Then the targeted and area ones (`targetRules`, lot 2), each once a ship,
read from the Sightings alone. "In range": a hostile in space (its
scanned hull points above 0) within the gizmo's Range × RangeUnit;
"armed": its parts known (Scan.Parts) with a weapon among them, or its
parts unknown and HullThreat of its hull > 0; "strongest": the greatest
sightingSide damage, ties the fewest hull points, the nearest, Hostiles
order.
- Disintegrator, Intellect Scrambler, Disarmer, Gizmogrifier, Myrmidonic
  Carbonizer: the strongest armed in range.
- Brunswik Dissipator: the same, its energy unscanned or > 0; Shield
  Blaster: its shields unscanned or on; Specialty Blaster: a Special part
  known aboard.
- Backfirer: the one in range with the most weapon parts known (at least
  one); Moving Part Exploiter: the most parts known, at least
  `partsExploitAt` 5 (a constant).
- Molecular Tie Down: at a star with a colony of mine, the nearest in
  range not known armed whose cargo is unscanned or > 0 (a colony ship,
  an invader); Containment Device: the same anywhere.
- Smart Bomb: at least SmartBombAt hostiles in space within Special1 ×
  RangeUnit.
- Self Destructotron: their damage ≥ my Hull, no escape (`canEscape`), a
  hostile in space within Range × RangeUnit.
- Sacrificial Orb: my Hull − Special1 > ShieldAt × MaxHull: the most
  damaged other ship of mine there in range at Hull ≤ ShieldAt × its
  MaxHull. Plasma Coupler: my energy ≥ 2 × MoveCost + its Power: my other
  ship there in range of least energy, short of its MoveCost.
- Phase Bomb: the nearest planet of the star in range owned by an
  empire at War.
- The positional ones (lot 3, appended in this order). "threatAt" a
  point: the sightingSide damage of the hostiles whose threatReach
  covers it, summed; "cornered": no escape (`canEscape`).
- Positron Bouncer: the strongest armed in range whose threatReach
  covers its distance to me (it can hit me).
- Tractor Beam: a gun aboard able to fire (`longestRange` > 0): the
  nearest in range not known armed, its cargo unscanned or > 0,
  farther than `longestRange` (pulled into my guns); never a mate.
- Hyperswapper: their damage ≥ my Hull, cornered, threatAt my Pos > 0:
  the one in range whose spot, once swapped (it at my Pos), has the
  least threatAt, strictly below mine.
- Fleet Disperser: the strongest armed in range with `disperseAt` 2 (a
  constant) other hostiles in space within its Special1. Mass
  Condensor: the same, and no ship of mine in space within Special1 of
  the target (the user included: it is pulled too).
- Gravity Distorter: `disperseAt` hostiles in space within Special1 of
  me, no other ship of mine in space there. Gravimetric Condensor:
  `disperseAt` hostiles in space within Special1 of the star and beyond
  Special2 (the ones it pulls), no ship of mine in space within
  Special1.
- Gravimetric Catapult: under fire, cornered, threatAt −Pos below
  threatAt Pos.
- Then the lane ones (`laneRules`, a LaneTarget: Index the endpoint in
  System.Lanes; `LaneBlocks` is keyed by the cluster lane,
  `Stars[star].Lanes[Index]`). Lane Magnetron: fleeing (`fleeing`,
  Round's own test: unarmed under fire, or armed, pressure > FleeRatio
  and an escape), a lane drive aboard: the lane of its GoToStar order
  (kept), else of its `refuge` (`roundFlee`'s), its endpoint in range.
  Lane Destabilizer: never: `LaneTraffic` is a count of every other
  empire's ships in a lane, neither their side nor their way, and one
  inbound to my star would be hurried in.
- Remote Repair Facility, a quiet-day pass in Act after the ability
  (`repairs`): a ship out of the lanes at a star with no hostile seen,
  Hull < MaxHull, uses it while usable and each use raises the hull and
  spends energy.
- The quiet-day lane pass (`lanes`, Act after `repairs`): a ship out of
  the lanes at a star with no hostile seen tries each once a day. Lane
  Blocker: at a star with a colony of mine, at War with someone: the
  first endpoint in range whose far star holds no colony of mine, its
  lane not blocked by me today. Lane Endoscope: the first endpoint in
  range whose far star I do not know. The quiet-day passes (repairs,
  lanes) skip a ship being refitted.

Four genes, combat-side (`Combat`), scalars in the GA after
FreeResearchDays (scalarGenes 32, mutated at their Neutral magnitude in
`dipScales`): CannibalAt 0.5, ShieldAt 0.5, RecallAt 0.25, clamped [0,
1]; SmartBombAt 2, clamped [1, 10], scale 1. Dormant today in solo and
versus games: the bots' Warship and ColonyShip designs carry no special;
the rules are proved on fixture ships (bot/special_test.go).

The positional actives (lot 3, `game/positional.go`). Special1 and
Special2 are distances in the Range unit (× RangeUnit; ASSUMPTION: the
Positron Bouncer's 100 is a system radius, the Tractor Beam's 65 two
thirds). A displacement (`displace`) aims at a point clamped to
SystemReach and travels the segment to it, stopping at the farthest
point ShipGap off every other ship in space at the star (any empire's,
live) and PlanetClear off every planet (`clearPath`: clearEnd with a
radius per obstacle; a ship starting inside one may leave, not end
closer); one that cannot move is "not moved", the use spent. A moved
ship is in space (Orbit −1), its order kept. Targets in orbit are
refused (targetCheck), the area effects skip ships in orbit and in
lanes; their victims are collected first (empire then Ships order),
then moved one after another against the live positions (a second
ship on one ray stops ShipGap off where the first went). Refused "no
ship in range" when an area effect has nobody to move. No report but
the user's.
- Positron Bouncer (P2 R60 U1, S1 100, hostile): "buffeting it away
  without damaging it": the target pushed Special1 straight away from
  the user.
- Tractor Beam (P3 R120 U1, S1 65, hostile or friendly): "pull another
  ship toward it": the target pulled Special1 toward the user, at most
  to ShipGap from it. Refused "already alongside" within ShipGap.
- Hyperswapper (P3 R120 U1, hostile or friendly): "causes them to swap
  positions": the two Pos exchanged exactly (both in space).
- Fleet Disperser (P5 R120 U1, S1 45, hostile): "repels other ships away
  from its target": every other ship in space within Special1 of the
  target, the user included, pushed straight away from it out to
  Special1.
- Gravity Distorter (P4 U1, S1 65, NoTarget): "pushes small objects,
  such as space vessels, away from the ship": every other ship within
  Special1 of the user pushed out to Special1 from it.
- Mass Condensor (P8 R120 U1, S1 50, S2 5, hostile): "All ships will be
  attracted toward the affected ship": every other ship within Special1
  of the target (the user included) and farther than Special2 pulled to
  Special2 (= ShipGap) from it.
- Gravimetric Condensor (P5 U1, S1 65, S2 10, NoTarget): "all the ships
  in the system to be pulled toward the star": every ship in space
  within Special1 of the star (the origin; the user included) and
  farther than Special2 pulled to Special2 from it.
- Gravimetric Catapult (P3 U1, NoTarget): "whips past the sun, stopping
  opposite its previous position": Pos → −Pos, then the free spot near
  it (FreeSpot's search, planets' PlanetClear included). Refused "at
  the star" within ShipGap of it (a refit and an orbit: the frame's).

The lane actives (lot 3, a LaneTarget):
- Lane Blocker (P3 R40 U1, ASSUMPTION one-time: the help does not say):
  "Star Lane Drives and Star Lane Hyperdrives can no longer overcome
  it": `Empire.LaneBlocks[lane]` = Day + BlockDays (the Arbryls'
  duration; saved, save.Version 30), the lane closed BOTH ways to every
  other empire, allies included, as SA_BlockStarLanes ("Species
  abilities": Order refuses "the lane is blocked by the X", a ship
  ordered before waits at the entry, the ships inside go on); the
  user's own pass. The
  expired entries are dropped at the next use (no cleanup otherwise:
  the comparison with the day decides). Refused "already blocked".
- Lane Destabilizer (P7 R40 U1, one-time: "itself is destroyed"): "the
  ships inside the star lane to be thrown quickly toward their
  destination point": every ship inside that lane, either way, any
  empire, Arrive = min(Arrive, Day + 1): out at the next close. Refused
  "no ship in the lane". One report, the user's.
- Lane Magnetron (P10 R50 U1, one-time): "slip through the star lane
  almost instantly": the user takes the GoToStar order through the lane
  (Order's refusals: Check's "no star lane drive" / "no engine", the
  lane blocked; a refit, an orbit and a Tie Down refused before: its
  engines damped; the Recaller and the Catapult take a tied-down ship, a
  shunt, a gravity pull), enters it now at its endpoint (armed shots
  dropped), Arrive = Day + 1.
- Lane Endoscope (P10 R50 U0): "scan the system at the other end": the
  far star explored (`Visit`, as an arrival: its planets and lanes
  known) and the lane learnt. Refused "already explored".

Not yet: the Cloaker. Never: the Carbonizer at a lane or a planet.

## Galaxy run (seed 1, Average, one empire)

The bot's expansion: a ship routes (Dijkstra in lane days
through the stars its empire Knows, an unknown star a leaf) to the
nearest star with free foundable planets, else the nearest unexplored
one, and keeps its destination in bot.State. Colony ships are built
against a budget, the open places (targets, 3 per unexplored frontier
star) less the Colonizers not yet sent. Solo, `cmd/fuzz` gives 63 wins
of 63 (21 species x seeds 1-3, Average), a median win near day 1450
(Chronomyst 1163, its double lane speed). Before, the one-lane horizon
left 20 species of 21 stuck at 4/50 systems on seed 1. Shipyard day
237, first Small colony ship day 333 (Tonklin Motor, Proton Shaver,
Colonizer x3), first colony day 335, 4 colonies by day 400, 7 by day
600.

**Colonisation pace against the original's saves** ("too many planets, too
big"). The galaxies match: 2.7-3.2
planets per star and uniform sizes in the 12 saved galaxies, 3.03 and
uniform over our 30 seeds (the census in formats.md). The pace did not:
planets colonised by every empire together, 50 stars, 5 empires (a
planet with a built Colony Base in the save; ours by mksave -empires 5,
seeds 1-2):

| day | the original's saves | our bots |
|---|---|---|
| 302 | 15 | 8-10 |
| 443 | 33 | 26-34 |
| 710 | 40 | 72-93 |
| 1030 | 62 | 137-142 |
| 1466 | 107 | 137-142 (every planet taken) |

Equal to day 450, then ours saturate: a bot holds ~28 colonies at day
1000 where the original's AI holds ~12. Not bigger: the original's AI
takes sizes 4-5 first (12 of its 15 colonies at day 302), ours take
every size alike (14 of 34 are size 1-2 at day 443). The cause is the
policy, not a rule: a shipyard builds a Colonizer whenever it has a
spare and `targets` finds a foundable planet of any size (9-16 colony
ships per empire before day 710 on seed 1; the first warships near day
655, two or three each).

Two genes slow it (`bot/bot.go`), tuned on the saves'
curve. `FoundFloor` (rounded, 1 to 5), the planet-worth floor by Size: a
planet below it is not foundable for the bot (targets, spare, routes, a
ship in its orbit) while the empire knows a settleable planet of that
Size or more, claimed or not; when none is known the floor drops to 1.
`InFlight` (rounded, 1 to 50), the cap on the Colonizers in flight:
those aboard the empire's ships under orders or in a lane (an idle
ship's leftover, say a driveless ship's last Colonizer with nothing left
in its system, would block the empire for good) plus those of the colony
ships being built; no colony ship starts past it, but one always may
when none is in flight (a two-Colonizer ship under a cap of 1). Tuned
(fixed policy and Neutral): 4 and 2, from a grid of floors 1, 3, 4, 5 by
caps 1, 2, 3, 6, 50 (mksave -empires 5, seeds 1-3, colonies at the
saves' days; the floor moves the sizes, not the count):

| day | saves | cap 50 (before) | cap 2 |
|---|---|---|---|
| 302 | 15 | 8-9 | 7-9 |
| 443 | 33 | 26-32 | 21-29 |
| 710 | 40 | 62-84 | 46-57 |
| 1030 | 62 | 125-131 | 70-80 |
| 1466 | 107 | 137-142 | 96-100 |

(Measured with this count: an idle ship able to go
is in flight, a driveless one with nothing at its star is not; the count
first tried, every idle ship out, let last night's colony ship slip past
the cap, 91-106 at day 1030.) Cap 1 plays as 2 (one ship always goes,
and a colony ship carries 2-3 Colonizers): the cap cannot slow further.
Floor 4 at day 443, seed 2: 18 of 20 colonies size 4-5 (floor 1: 10 of
21); seed 1 barely moves (the known big planets run out, the floor
drops). Left: the early pace (half the saves' at day 302) and the middle
(a fifth over the saves' at day 1030). Kept as is: it matches what
antag.exe plays.

**Difficulty** ("un réglage de difficulté, avec
juste ce changement pour l'instant"; `bot.Config.Difficulty`, the
new-game screen's Bots line, saved with each bot). Original plays as
above. Hard lifts inFlightMax to at least 6 (`hardInFlight`) and plays an
evolved genome (`bot/hard.json`,
`bot.AtLevel`; `-genome` wins over it): fuzz evolve on Orfa (-fresh,
seeds 1-6, pop 24, 8 generations; HabitatShare and HabitatFill came out
0.5, a sweep of them moved nothing). Solo over every species, seeds 1-4:
both win all, median day 1833 against the floor alone's 2324; as empire
0 of 4 against the default bots (every species, seeds 1-3, 3000 days)
it ends with more systems in 62 games of 63, its fitness 0.546 against
0.393. Its diplomacy genes are extreme (Temper −46, Rage 252), untried
against a human. Why this
lever: a human player's Orfa on a quick save (seed 907789, Very Dense, 7
empires) held 80 colonies at day 1332, the bot Orfa in the same seat
14, stuck since day 1120 with 62 settleable planets known: since the
larger colony ships (one 12-22 Colonizer ship fills the cap of 2) a
single ship flies at a time, settling one planet a day. Each lock
lifted alone on that galaxy (colonies at day 1332, Orfa / the mean of
the 7): none 14/19, courted off 19/17, floor 1 27/31, InFlight 6
19/35, all three 29/38. InFlight over Very Dense seeds 1-3 (per empire,
colonies, pop, research): 2: 17.5, 638, 349; 4: 24.4, 741, 452; 6:
28.8, 802, 438; 10: 28.0, 771, 488. Against the saves' curve (mksave
-empires 5, seeds 1-3, colonies of all empires at days 302, 443, 710,
1030, 1466; the saves 15, 33, 40, 62, 107): cap 2 7-11, 23-26, 43-53,
68-74, 74-132 (still the original's pace); cap 6 7-12, 32-33, 61-69,
111-120, 137-148. Later: a policy per species and level, say
an invader sending 3-4 Invasion Module ships at colonised worlds.

`AbandonIdle` (`bot/bot.go` abandonStranded; rounded days, 0
to 1000, 0 never; fixed policy and Neutral 60): a ship whose Design
holds a Colonizer or an Invasion Module (whatever is left aboard), idle
after every planner (out of a lane, no order, not under a refit) that
many Acts in a row, is Abandoned, freeing its fleet-cap slot; the count
is not saved (a loaded bot restarts it). Measured, mksave -empires 5 to
day 1466: 61 abandons on seed 1, 60 on seed 2, all emptied colony ships
(59 with a lane drive); the fleets fall from 12-19 ships to 1-5;
colonies unchanged on seed 1, 116 to 127 at day 1466 on seed 2 (91 both
at day 1030).

## Winning

The game's own end texts (`end-*.txt` in the COB archives: a short
description, 10 rank names, a congratulation) give four victories and
one defeat:

- `end-ctrl`: control of **more than two-thirds of the stars** (its
  header comment still says "over half": an older threshold, the texts
  say 2/3).
- `end-capt`: the home systems of every other species captured.
- `end-dest`: every other species destroyed.
- `end-unit`: the galaxy under one alliance, the player alive.
- `end-xtnc`: the player extinct.

The fan manual (b-sting.nl `winning-conditions.html`) adds the details
the files do not hold:
- Capturing a home system takes holding a planet in it, and destroying
  its colonies is not enough.
- Destroying a species means every colony and every ship.
- Control is counted outward from the home system along the star lanes,
  and a non-allied alien ship or system in the way cuts off the systems
  behind it ("Starlanes under your control will light up in your species
  colour", checked in the game).

The rank, 1 to 10, comes from ship count, colony count and inventions,
with percentage bands 0-20, 21-29 and 30+ for the first three ranks. The
formula is unknown.

The victories (`game/victory.go`; a player allied with every
species met had not won): `Game.VictoryOf` judges, in this order,
destruction (every other empire defeated: a defeat destroys its ships
already), capture (a colony of the empire at every other's HomeStar,
defeated or not), unification (every other living empire allied) and
control (ControlledSystems × 3 > 2 × the stars: local control, the lane
rule left out). At each close, after the close of control, a living
empire not yet a winner that wins keeps it (`Empire.Won`, `WonDay`) and
a Won Report goes to it and to every other living empire; nothing ends,
the game goes on, a bot's victory is news only. The window's popup
(`victory.go`) shows the end text's enddesc, the day and a rank of its
ten: 9 − WonDay / 200, clamped (OUR ASSUMPTION, the sooner the higher;
the original's rule is in the exe).

Control of one system (help.txt, the Ships box: "A species controls a
system by having the only colonies there") is `StarSystem.Controller`:
the owner of every colony in it, nobody when two empires
share it; `Empire.ControlledSystems`, the fleet cap's base, counts those
(before, any star with a colony of the empire's). The system screen's
strip flags it. The lane rule is not modelled; a system's control is
local. `cmd/fuzz` solo measures the bare systems count, with no
lane-connectivity rule (alone, nothing cuts it off).

The close of control (`game/defeat.go`, `Game.EndDay` after
every empire's close and the invasions, before the diplomacy: control
moves only by a colony founded or captured):
- The defeat (the original's end-dest; it was
  once "controls no system", which wiped out an empire
  whose colonies all stood in shared systems): an empire with no colony
  left is out, `Empire.Lost` = the day (`Alive` false, saved). Its ships
  are gone (with no system controlled the scrap takes them anyway), the
  offers from, to or about it dropped, its lane blocks lifted. An empire
  controlling no system but holding colonies lives on with a fleet cap
  of 0 until they are invaded. Every loser of a close is picked first:
  two empires losing their last colonies the same day both go. A
  `Defeated` event for it and one for every other living empire (Other
  the loser), at its home star. Diplomacy and abilities refuse it
  (`ErrDefeated`), its bot does nothing; the window's player defeated
  gets the end prompt.
- The scrap (help.txt `EP_SCRAPNEWESTSHIP`: "Loss of control of a
  system forces us to scrap our most recent ship."): while an empire
  flies more than ShipsPerSystem × its controlled systems, its newest
  ship (the last of `Ships`, birth order) is destroyed, wherever it is
  (a lane, a refit: cancelled at the next close), one `Scrapped` event
  each. A Ship project in excess is never cancelled: it completes and
  its ship is scrapped at that close, the industry lost. Measured: none
  in `sim -galaxy -seed 1 -days 600 -empires 3`; 1 and 3 scraps, no
  defeat, in 2000 days of 4 empires on seeds 1 and 2.

## Diplomacy (lots 1 to 3 in; the game's `.dip` files)

The original's diplomacy is data-driven: `race00.dip` to `race20.dip`
(one per species, in `Species` order) and `player.dip` in the COB
archives (grammar: formats.md). Each file holds a personality sketch for
the dialogue writers (comments above `DIPVALUES`), a `DIPVALUES` table,
the fifteen actions' texts (`DA_Discover`, `DA_DeclareWar`,
`DA_ProposeAlliance`, `DA_TradeStarlanes`, `DA_TradeStars`,
`DA_TradeResearch`, `DA_OfferPeace`, `DA_BreakAlliance`,
`DA_DiscoverAskHelp`, `DA_HelpControlSystem`, and the third-party ones
`DA_OfferPeaceToOtherRace`, `DA_DeclareWarOnOtherRace`,
`DA_BreakAllianceWithOtherRace`, `DA_ProposeAllianceToOtherRace`;
`DA_NoDipAction`) and the accept/reject responses (`DR_ACCEPT_…`,
`DR_REJECT_…`, plus `DR_NOSHIPS_`, `DR_NOWAR_`, `DR_NOMORESHIPS_`,
`DR_MORESHIPS_HelpControlSystem`). Placeholders: `%y` you, `%m` me, `%t`
the third party, `%s` the system. The formulas are in the exe only (not
decompiled); the tables, typed from the files:

| species | DECLARE_WAR | ACCEPT_PEACE | PROPOSE_PEACE | BREAK_ALLIANCE | ACCEPT_ALLIANCE | PROPOSE_ALLIANCE | START_ATTITUDE | SHIP_PATIENCE | BIO_PERIOD | BIO_MAXPOS | BIO_MAXNEG | SIN_X | SIN_HALF_X | SIN_TWICE_X | NEG_PLAYER_TURNS |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| Minions | 290 | 360 | 450 | 510 | 600 | 750 | 340 | 95 | 400 | 4 | -4 | 3 | 0 | 0 | 99 |
| Snovemdomas | 250 | 350 | 400 | 450 | 600 | 760 | 340 | 35 | 800 | 4 | -4 | 2 | 0 | 1 | 319 |
| Orfa | 210 | 310 | 410 | 470 | 570 | 750 | 420 | 60 | 750 | 4 | -4 | -1 | 0 | 2 | 74 |
| Kambuchka | 250 | 330 | 450 | 470 | 600 | 800 | 350 | 95 | 600 | 4 | -2 | -1 | 0 | 1 | 53 |
| Hanshaks | 150 | 200 | 350 | 470 | 600 | 750 | 440 | 50 | 500 | 2 | -1 | -1 | 0 | 1 | 50 |
| Fludentri | 300 | 400 | 500 | 550 | 700 | 750 | 380 | 40 | 500 | 2 | -2 | -3 | 0 | 2 | 91 |
| Baliflids | 220 | 240 | 300 | 440 | 500 | 680 | 320 | 72 | 350 | 4 | -4 | 2 | 0 | -2 | 43 |
| Swaparamans | 270 | 310 | 400 | 475 | 575 | 700 | 390 | 80 | 200 | 3 | -3 | 2 | 0 | -1 | 87 |
| Frutmaka | 300 | 400 | 500 | 600 | 700 | 800 | 310 | 100 | 180 | 2 | -2 | -1 | -2 | 1 | 79 |
| Shevar | 450 | 530 | 670 | 750 | 800 | 900 | 455 | 110 | 666 | 2 | -3 | 4 | -5 | -1 | 120 |
| Govorom | 200 | 300 | 400 | 430 | 600 | 700 | 350 | 45 | 250 | 3 | -3 | 1 | 1 | 1 | 111 |
| Ungooma | 200 | 300 | 400 | 450 | 600 | 750 | 410 | 140 | 200 | 3 | -3 | 2 | 3 | -1 | 59 |
| Dubtaks | 250 | 300 | 450 | 580 | 600 | 700 | 620 | 145 | 500 | 3 | -3 | 3 | 0 | -1 | 48 |
| Capelons | 250 | 350 | 450 | 570 | 650 | 790 | 310 | 80 | 800 | 5 | -5 | 0 | 3 | -2 | 72 |
| Mebes | 170 | 290 | 360 | 450 | 550 | 690 | 370 | 110 | 700 | 3 | -3 | 0 | -2 | 1 | 66 |
| Oculons | 300 | 360 | 450 | 480 | 600 | 750 | 440 | 80 | 333 | 3 | -3 | 1 | 1 | 1 | 43 |
| Arbryls | 210 | 300 | 370 | 420 | 620 | 780 | 400 | 95 | 180 | 2 | -2 | -1 | 2 | 1 | 50 |
| Marmosians | 320 | 450 | 560 | 570 | 700 | 850 | 360 | 90 | 288 | 2 | -2 | 2 | 0 | 0 | 59 |
| Chronomyst | 150 | 250 | 450 | 540 | 670 | 690 | 430 | 40 | 500 | 3 | -3 | -1 | -1 | 1 | 109 |
| Chamachies | 210 | 300 | 450 | 550 | 590 | 700 | 320 | 80 | 700 | 1 | -1 | 1 | 0 | -1 | 129 |
| Nimbuloids | 130 | 310 | 440 | 490 | 590 | 760 | 320 | 90 | 777 | 3 | -3 | 1 | -3 | 1 | 71 |

Our reading (a hypothesis, nothing measured): one attitude per ordered
pair of empires on a scale of about 0 to 1000, starting at
`START_ATTITUDE`, crossing thresholds in a fixed order (DECLARE_WAR <
ACCEPT_PEACE < PROPOSE_PEACE < BREAK_ALLIANCE < ACCEPT_ALLIANCE <
PROPOSE_ALLIANCE in every file): war is declared when the attitude falls
under DECLARE_WAR, peace accepted over ACCEPT_PEACE and proposed over
PROPOSE_PEACE, an alliance broken under BREAK_ALLIANCE, accepted over
ACCEPT_ALLIANCE, proposed over PROPOSE_ALLIANCE. Every species starts at
peace, none allied (START between DECLARE_WAR and ACCEPT_ALLIANCE), and
several start a few points from war: Frutmaka 310 against 300, Shevar
455 against 450, Marmosians 40 points, Minions 50, Capelons 60, the
Dubtaks 370 away. The BIO_ values are a mood: a sum of sines of period
BIO_PERIOD days, the three weights on sin x, sin x/2, sin 2x, clipped to
[BIO_MAXNEG, BIO_MAXPOS]. SHIP_PATIENCE reads as the tolerance to the
other's ships in one's systems. NEG_PLAYER_TURNS reads as the
Antagonizer's bias against the human player: its readme says Neutral
"biases the AI players against the player diplomatically" and Hostile
"to the point where AI opponents will probably all end up at war with
the player" (Peaceful: "just you against the AI code, without cheats").

What the game's texts state (species.txt, help.txt): the attitude
depends on "their personality, your actions toward them, and their
overall standing in the game"; allies "will neither attack without
provocation nor colonize systems you control" and "help you in battles
against common enemies"; species at war "have no diplomatic options but
to offer peace"; third-party options (ask a peace offer, a war
declaration, an alliance with a third species) are for allies only; a
knowledge exchange adds stars, lanes or techs (red rings in the research
display). Players' experience (GameFAQs board): an exchange proposed
every few days raises the attitude until peace comes, slowly for the
aggressive species; an ally accepts a peace with a third party much more
readily; "most species are stupid enough to declare war at their least
tactical moment" (the fan manual). Alien Hospitality turns a colony's
industry off and pours it into the "alien diplomacy rating", at peace
only (the fan manual, the 2005 planets guide): ours since lot 2, below.
Three abilities touch diplomacy: the Hanshaks contact every species at
the start, the Baliflids force peace on everyone (100 days), the
Marmosians make everyone dislike whoever is at war with them (100 days).

Ours (the facts in `game/`, the judgement in the
genome). `game/diplomacy.go`: a `Stance` per
pair (Unknown, Peace, War; Alliance since lot 2), kept equal both ways
in `Empire.Relations[j]` with the CONTACT day and a LEDGER of counts
(days at war, wars j declared on me, my ships j destroyed, my colonies j
invaded, j's armed ship-days at stars where I hold a colony and j none
while at peace, my offers j refused, the day of my last offer).
Everybody starts Unknown; two empires both `Present` at a star (a
colony, or a ship out of the lanes) at the day's close are in contact
and at Peace; the Hanshaks ("talk to all races at the start") meet
everyone on day 0. `Game.Hostile` is War only: nothing fires, bombs or
invades before a declaration (`invadeErr`: "we are not at war with the
X"); a guarded orbit still bars every other empire (lot 2 kept it
closed). `DeclareWar(a, b)` is unilateral and immediate from Peace;
`OfferPeace` (at War, one pending offer per direction, `OfferCooldown` 5
days between offers) waits in `Game.Offers` for `Answer` (accept: Peace;
refuse: the offerer's Refused grows) or expires refused after
`OfferExpiry` 10 days. Every transition returns Reports (Contacted,
WarDeclared, PeaceOffered, PeaceMade, OfferRefused; `Event.Other` names
the other empire) that the callers log. The fixtures (`SpawnFixtures
-foes`, `SpawnArena`) put empires 0 and 1 at war: a fixture is a fight.
No bias against the player, no mood (the BIO_ sines) for now. Not
modelled: the mood, the fleet requests (`DA_HelpControlSystem`,
`DA_DiscoverAskHelp`). (The other timed abilities came later: "Species
abilities".)

The ATMOSPHERE (the New Game screen's third
choice): `Game.Atmosphere` is Peaceful (−1), Neutral (0, the zero value:
mksave, sim and fuzz games) or Hostile (+1), saved, read by nothing in
`game/`. The bot adds `AtmosphereShift` 100 per step to its six
thresholds, as its Temper gene does (`Bot.Thresholds`): Hostile puts
every species whose START_ATTITUDE sits within 100 of DECLARE_WAR (most)
at war on contact, Peaceful makes a war 100 points harder and an
alliance 100 points easier. Toward every empire, the original's "general
aggressiveness of species", not the Antagonizer's bias against the
player alone. A constant, not a gene: the fuzzer plays Neutral games and
could not tune it.

Lot 2 (the rules in `game/`; the bot and the window build on
them). The ALLIANCE: `OfferAlliance(a, b)` at Peace only, accepted by
`Answer` into Alliance both ways (AllianceMade each, a crossed alliance
offer dropped); `BreakAlliance(a, b)` back to Peace, unilateral, the
victim's ledger counting it (Broken), the pending offers between them
dropped either way. War is declared from Peace only: an ally breaks
first (the original's BREAK_ALLIANCE). One pending `Offer` per ordered
pair whatever its kind, `OfferCooldown` after any offer. The ledger's
four new counts: AllyDays (a day allied), Broken (alliances j broke with
me), Trades (exchanges concluded), Hospitality (j's Alien Hospitality
points toward me); Intrusions stay counted at Peace only (an ally's
ships are welcome). The EXCHANGES (`OfferTrade(a, b, k)`, k one of
TradeStars, TradeLanes, TradeResearch), at Peace or Alliance, refused
when neither side would learn anything: `Gain(to, from, k)` is the stars
`from` Knows and `to` does not, the lanes `from` KnowsLane and `to` does
not, or ONE tech, the cheapest `Research.Cost` that `from` knows and
`to` does not, ties by index (one tech each side,
the cheapest). Accepted, both sides learn their Gain as it stands then
(the stars Visited, the lanes in `Empire.LanesKnown`, the tech Granted),
a side with nothing to learn learning nothing; Trades counts on both
sides; a crossed exchange of the same kind (the same exchange) is
dropped. ALIEN HOSPITALITY is an ongoing project (industry off, the
quarter q of `Colony.Diverted` adding to nothing in the colony): every
day each empire's q summed over its colonies running it adds to the
Hospitality that every empire at Peace or Alliance with it holds about
it (nothing at War or unmet). FRIENDSHIP BARS WAR (ours: nothing in the
original): `Relation.Friendship` is what j gave me, weighed as a neutral
bot's goodwill: `FriendTrade` (30) per exchange concluded, `FriendHelp`
(30) per request of mine j granted, `FriendHosp` (0.2) per Alien
Hospitality point j poured toward me, each close keeping 1 −
`FriendFade` (1%) of it. While a's Friendship about b holds `FriendBar`
(50) or more, `DeclareWar(a, b)` is refused, an ally's granted request
for that war too (a bot asked refuses it: `bot.Decide`; accepted
anyway, `Answer` makes it a refusal, Refused counted, "(the Mebes are
our friends)" in the refuser's report only), "the Capelons are your
friends: no war on them for 10 days" (`Game.WarBarred`: the closes left
were b to give nothing more). A war declared on me wipes my Friendship
toward the attacker (it broke it). A bot does not want a war the game
bars: no declaration, no arming for it (`warTarget`), alliances and
exchanges as at a mild attitude. With these (same fuzz as below), the
twelve games come out byte for byte as with no bar at all (wars 7.42,
alliances at the end 6.83, unification 9): a bot's goodwill grows from
the same gifts as its Friendship, so it seldom wants war on one it is
barred from; the bar bit on revenge wars after an attack, which the
wipe frees.
For every empire, bots included. The player sees it: the Species
screen's friendship column (green from FriendBar), its ledger's last
fact "no war for N days", Talk's "friendship: 69, no war for 23 days";
none of it at War with b (b declared it) nor b defeated.
The bots' goodwill counts the granted requests too (gene HelpW, 30).
Measured (fuzz versus, 6 empires, seeds 1-12, 2000 days, against the
first version below): wars 7.17 → 7.33 a game, alliances at the end
7.08 → 6.75, unification 9 games of 12 (mean day 1419) unchanged; the
bar holds on 44-60% of the bots' pair-days at Peace (seeds 1-3,
Friendship up to 1340: a steady big host bars war some 320 days after
it stops).
A first version (same day) barred war on a fading hospitality alone
(1/30 a day, bar 30): fuzz versus, 6 empires, seeds 1-12, 2000 days,
wars 7.42 → 7.17 a game, alliances at the end 6.83 → 7.08, unification
9 games of 12 either way (mean day 1419); among the bots the bar held
on 40-63% of the pair-days at Peace yet rarely stopped a war they
wanted (their attitude already weighs the hospitality). ALLIES DO NOT COLONISE each other's
systems (species.txt): `Found`/`FoundAt` refuse a star where an ally
holds a colony (`CanFound`, `ErrAllySystem`, `Empire.AllySystem`). The
orbital guard stays closed to every other empire, allies included.
Reports: AllianceOffered, AllianceMade, AllianceBroken, TradeOffered,
TradeMade (what we learnt, then what we taught, "4 stars from Orfa; taught
them 2 stars", "Hyperlogic from Orfa; taught
them Xenobiology", "Nothing learnt from Orfa; taught them 1 lane", "…;
taught them nothing"); refusals name the
kind ("The Orfa refuse our alliance", "We refuse the Mebes's exchange of
research").

Lot 3 (the rules in `game/`; the bot and the window build on
them). The THIRD-PARTY REQUESTS (species.txt: for allies only):
`Request(a, b, k, t)`, k one of AskPeaceWith, AskWarOn, AskBreakWith,
AskAllianceWith, is a's request to its ally b to act toward t: refused
unless a and b are at Alliance, t is neither of them and a has met t,
and b's stance with t is the one the act starts from (`requestValid`:
peace asked of an empire at War with t, war or an alliance of one at
Peace, a break of one allied). It is an `Offer` carrying `Third` t,
under the same rule as the others (one pending per ordered pair, the
cooldown, the expiry). Accepted, b performs the act itself on a's behalf
(`OfferPeace`, `DeclareWar`, `BreakAlliance` or `OfferAlliance` from b
to t, with their own Reports and ledgers: a war asked is b's
declaration, t counts it in b's WarsOn), and a's ledger counts it
(Helped: the requests b granted me); a peace or alliance request b has
already made to t (its offer pending) is granted as it stands, nothing
added. A request no longer valid (b acted by itself,
the stances moved) is MOOT: answered either way it is dropped, nothing
counted (a refusal does not count); pending, the day's close
drops it silently. Reports: HelpAsked, HelpGiven ("The Orfa grant our
request: war on the Mebes"), refusals "The Orfa refuse our request:
peace with the Mebes".

The TIMED ABILITIES' frame (`Game.UseAbility`, "Species abilities"): two
are here, the other twelve there. The Baliflids' SA_ForcePeace ("Force
all aliens to make peace with you", 100 days): every empire at War with
them is at Peace both ways, the offers between them dropped, a PeaceMade
each side; refused when at war with nobody. The Marmosians' SA_CauseWar
("Make aliens dislike anyone at war with you", 100 days): for each
empire x at War with them, every other empire that has met x counts it
in its ledger about x (Defamed), for the bots to weigh; refused when at
war with nobody. ASSUMPTION: the pheromones are not announced (no Report
for x nor for the listeners), and the dislike is a fact in the ledger,
not a stance change: what it does is the bots' judgement.

The bot (`bot/diplomacy.go`): an ATTITUDE toward each empire met,
0..1000, recomputed every Act: attitude = Start − grudge − EnvyW ×
max(0, their systems − mine) + weariness (their systems over the stars I
Know), where grudge fades by Forgive a day (neutral 0.01) and grows with
the ledger's deltas: a ship lost GrudgeShip 40, a colony invaded
GrudgePlanet 150, a war declared on me GrudgeWar 100, an offer refused
GrudgeRefusal 20, an intrusion IntrudeW 2 × 50 / the species'
SHIP_PATIENCE; the weariness grows WearyW (1) per day at war and fades
by Forgive too, the one term over Start ("war has no value to us":
without it no species but the Dubtaks ever reached its PROPOSE_PEACE,
and a war was for good; at the fade's equilibrium it
is worth WearyW / Forgive = 100). Against the species' DIPVALUES
(`Personalities`, Temper added to the six thresholds): under DECLARE_WAR
the bot declares war when its armed ships, one at least, number at least
WarCaution (1) × the other's ships it sees (none seen: no caution); at
war, over PROPOSE_PEACE it offers peace; it accepts an offer over
ACCEPT_PEACE. Nineteen genes in the GA (scalarGenes 27, lot 2's six and
lot 3's three below), mutated at their Neutral magnitude (`dipScales`:
Temper 100, Forgive 0.01), as a unit step is a leap on a daily share and
a crawl on the attitude scale. The player is an empire like another to
it. Measured (sim -galaxy seed 1, 3 empires): the first contact on day
811, no war by day 2500: the grudges stay 0 (nobody intrudes before a
war), only envy moves the attitudes, and the species under its threshold
(Snovemdomas at 230 toward the Govorom's 22 systems) sees more Govorom
ships than its 4 warships: WarCaution holds it. Versus (seeds 1-2, 4
empires, 1200 days): the ranks move (seed 1: Govorom 19/50 → 15/50
first, Snovemdomas 14 → 12 fourth, Minions 9 → 14 second) with no fight
at all: the bots build warships against ships they no longer attack.

The bot's lot 2. A GOODWILL term beside the grudge, fading
by Forgive too: attitude = Start − grudge − envy + weariness + goodwill,
the goodwill growing TradeW (30) per exchange concluded with j (and
HelpW 30 per request of mine j granted) and HospW
(0.2) per Hospitality point j poured toward me, the grudge GrudgeBreak
(150) per alliance j broke with me. The decisions, per empire met in
index order: every pending offer from j answered, a peace at
ACCEPT_PEACE, an alliance at ACCEPT_ALLIANCE, an exchange when FAIR:
worth(what I learn) ≥ TradeFair (0.5) × worth(what j learns), worth =
stars + lanes + 1 for a tech (`Game.Gain`). At Peace: war under
DECLARE_WAR as before (and nothing else that day), else an alliance
offered over PROPOSE_ALLIANCE, else an exchange; allied: the alliance
broken under BREAK_ALLIANCE (a war, if still wanted, comes from Peace at
a later Act), else an exchange. The exchange proposal: every TradeZeal
(60) days per empire, the kind worth the most to me (stars, lanes,
research, ties to the first), none when it is worth nothing; the day is
kept (`State.LastTrade`) whatever the game answers, so a refusal is not
retried daily. ALIEN HOSPITALITY: an idle colony runs it before the
party or the takeover when researched, some empire met is at Peace
(neither war nor alliance: it courts the undecided) and fewer of my
other colonies run it than ⌈Hospitable (0.1) × my colonies⌉: the first
idle colonies in `Colonies` order take the share, the same from day to
day (the home first when idle). The Colonizers skip an ally's systems
(`foundable` reads `Empire.AllySystem`; every colony target, route and
count goes through it). Measured (sim -galaxy seed 1, 3 empires, 1200
days): the three contacts unchanged (811, 882, 913); 31 exchange
proposals, 17 concluded (the first ones big: 18 and 29 lanes, 13 and 26
stars, at contact), 14 refused as unfair (each refusal is the proposer's
GrudgeRefusal 20 against TradeW 30 for a success); no alliance, no war:
the attitudes are 300 to 433 on day 1200, the goodwill's equilibrium at
one exchange per 60 days being about TradeW / (1 − 0.99⁶⁰) ≈ 66, far
from PROPOSE_ALLIANCE (690-900); 15 colonies of 142 on Alien Hospitality
on day 1200 (the share exactly: 4 + 6 + 5), Govorom Prime among them;
techs 33/41/44 → 36/48/46 (the exchanges), colonies 37/54/51 →
38/55/49. Versus (seeds 1-2, 4 empires, 1200 days): seed 1 Govorom
15/50 first → Minions 16/50 first, Govorom 16 second (Minions 14 → 16),
seed 2 Orfa first still (25 → 23/50), Ungooma 11 → 14.

HOSPITALITY ON THE CURVE (after an easy
unification: a big empire's 48 points a day lifted each of five bots
9.6 a day, an equilibrium of 960 over every PROPOSE_ALLIANCE): Alien
Hospitality's q is the curved industry's quarter, no longer the raw
sum's (a sum of 400: 40 points a day, not 100). Measured (fuzz versus,
6 empires, seeds 1-12, 2000 days): alliances at the end 6.83 → 2.83 a
game, unifications 9 → 2 (day 1632), wars 7.42 → 8.50; the bots ally
mostly through hospitality. Measured and rejected the same day: HospW
and FriendHosp halved to 0.1 (2.08 alliances, 1 unification; with the
curve 0.33 and none: it hits small empires as hard as big ones);
sharing each day's points among the empires at peace (many contacts
dilute it); capping hospitality's goodwill at ACCEPT_ALLIANCE − Start
(the bots never propose an alliance then: 0.00, no unification; at
PROPOSE_ALLIANCE − Start, 1.50 and none).

The bot's lot 3. Three genes: FavourW (100), AskZeal (90),
DefameW (100). An ally's REQUEST is judged on the attitude toward the
third party (`Decide`, the asker's attitude unread), against the
threshold of the act the bot would perform toward it, bent by FavourW
the ally's way: a war on it under DECLARE_WAR + FavourW and with the
arms of a war of its own (`warReady`: armed ships, one at least, at
least WarCaution × its ships seen), a peace offer at PROPOSE_PEACE −
FavourW, a break under BREAK_ALLIANCE + FavourW, an alliance offer at
PROPOSE_ALLIANCE − FavourW (the proposing thresholds: the act is the
bot's). The bot's own requests: allied with j, after the break check,
every AskZeal days per ally (`State.LastAsk`, kept whatever the game
answers, as LastTrade), war on the first empire it is at War with and j
at Peace with (index order); a request due and sent takes the pair's
one pending offer, so no exchange is proposed to j that Act. It asks
nothing else (peace, break and alliance requests are the player's). The
pheromones: grudge + DefameW per defamation of j heard (ΔDefamed). The
ABILITIES, after the diplomacy in Act, when ready: the Baliflids force
peace when some empire at War with them has their attitude at
ACCEPT_PEACE or more (a war they would end; the others are freed with
it), the Marmosians defame their enemies whenever at war; the twelve
others' rules are in "Species abilities", "The bot's abilities".
Measured: sim -galaxy seed 1, 3 empires, 1200 days, and versus seeds
1-2, 4 empires, 1200 days, byte-identical: no alliance ever forms under
the neutral genes (lot 2's goodwill equilibrium), so no request is made,
and none of those games has Baliflids or Marmosians.

Encroachment and rearmament (in the original,
colonising a system someone holds brought war at once, and our bots were
"clearly too peaceful"). The original's help has allies "never colonize
systems you control" (CanFound already refuses an ally's system). The
ledger's `Encroached` counts the colonies j founded at a star where I
held one, once met (`Empire.encroach`, at the founding); the bot's
grudge grows GrudgeEncroach (100, as a war declared) per one, so a
single encroachment takes the Snovemdomas from Start 340 under their
DECLARE_WAR 250. The bot settles no system where an empire at Peace with
it holds a colony while it means to keep the peace (its attitude at or
over DECLARE_WAR: `courted`); wanting war, it does. Rearmament: an
empire I am at War with, at Peace under my DECLARE_WAR, or at Peace
under my PROPOSE_PEACE (wary) with one armed ship of its seen near me
(a wary bot answers armed ships in kind), sets
`warTarget` = ⌈WarCaution × its armed ships seen⌉ + 1, at least 2, and
`wantWarship` builds warships up to it; the ships seen are counted
without those a parts scan shows unarmed (`armedSeen`, once a day), in
`warReady` too. Measured: versus seeds 1-24, 4 empires, 3000 days, wars
per game 0.08 → 0.00 with encroachment alone → 0.00 with both
(empire-days at war 100 → 0, armed ships at the end 16.5 → 15.8): the
bots of equal size no longer envy each other under their thresholds once
they stop settling in each other's systems; solo 3000 days
byte-identical in shares (no contact). Against a dominant player (the
day-1108 endgame save, 66 colonies, 11 armed ships seen), 300 days: the
three hostile species rearm (Minions 2 → 6, Baliflids 2 → 8 armed ships)
but stay short of the 12 the target asks, and none declares. Hence
two loosenings: the parity counts the other's armed
ships seen NEAR my systems only (at a star of mine or one lane from it:
a war is fought at the border), in warReady and warTarget; and a RAGE
(gene Rage, 100): under DECLARE_WAR − Rage any armed ship of mine will
do. Measured: versus seeds 1-24, wars per game 0.12, empire-days at war
205, ships destroyed 1.6, colonies invaded 6.8, no empire defeated,
armed ships at the end 18.6; on the endgame save (the player idle) the
Minions declare within 50 days and take 4 colonies in 300, the Baliflids
and the Arbryls declare later.

## The fixed policy's build and research (`bot/bot.go`)

A nil genome (the window's and mksave's bots, `bot.Default`) built only
Factory, Agriplot, Laboratory, Habitat, Outpost, replaced nothing and
researched `Available()[0]` after the guide: at day 896 of a human's game
(seed 264797, 7 empires) no bot held an upper tier, ~6 industry a colony
against the player's 17, and the colonies with nothing left to build ran
Alien Hospitality. Now (option A, the fixed policy extended):

- **Build list** (`greedyItems`): per output family the best tier
  research allows (`family`, read from the item table: no MaxPop and
  exactly one output; `bestTier`: most output, then cheaper): industry
  Factory 1 → Industrial Megafacility 3, research Laboratory 1 →
  Research Campus 3, prosperity Agriplot 2 → Artificial Hydroponifer 3;
  the mixed producers (Logic Factory, Engineering Retreat) in none. Plus
  every known housing item (MaxPop, the Colony Base aside): Habitat,
  Metroplex, Outpost. `GreedyPick`'s order: at Capacity the most MaxPop
  first (the worker who just arrived builds room for the next), then the
  wanted output's maker (the balance below), the cheapest, the lowest
  index. `GreedyItems` stays the Neutral genome's Build list.
- **Base buildings first** (`upgradeOpen`): a family's upper
  tier is built or replaced into only once the planet holds `Upgrade`
  (rounded) structures of that family, any tier (a Factory swapped for a
  Megafacility keeps the count); below it the base item (Factory,
  Laboratory, Agriplot) is built, the upper tier known or not.
- **The balance** (`wanted`, replacing "the scarcest output
  first", which fed prosperity most days through the −pop/4 upkeep and
  let the Hydroponifers swamp the rest). On the colony's outputs without
  its ongoing project: prosperity when it is negative, or 0 with room to
  grow (the upkeep covered first: a colony does not starve); else the
  output k furthest under its share of the total T, target_k·T −
  output_k, ties industry, research, prosperity. Targets IndShare,
  ResShare, prosperity the rest (≥ 0). At Capacity prosperity grows
  nothing: industry and research alone, their targets rescaled to a sum
  of 1, over T = industry + research. Equal shares replay the old
  scarcest-first pick (Upgrade 0, shares 0.34/0.33: seed 264797 x7's
  day-900 save identical to the previous policy's).
- **Industry first and no stall** ("le bot construit
  des labos en premier"): a fresh colony's Colony Base makes industry 1,
  prosperity 1, so the balance met industry's share and picked a
  Laboratory. Now, after the prosperity rule, industry while the
  structures' industry (`Planet.Output`, before the power curve: a
  Factory 1, a Megafacility 3, a red square +1) is under `IndFloor` (3:
  the Colony Base and a Factory, a red square's base included); the
  home's script is Agriplot, Factory. And the prosperity rule keeps the
  last free worker when the next unit's upkeep (pop/4) would bring
  prosperity to 0: a Factory taking it froze the colony (prosperity 0,
  nobody free for the Agriplot; at a floor of 4 on the output, 81 of 97
  colonies idle at day 900). Measured, 168 solo runs (8 seeds, every
  species, to day 3000) and 8 versus games of 4 empires to day 2000,
  mean day to a quarter / runs to a half / systems held in versus:
  before 1904 / 12 / 312; the guard alone (IndFloor 0) 1886 / 14 / 321;
  IndFloor 2 1889 / 14 / 319, 3 1905 / 12 / 314, 4 1912 / 11 / 324. All
  within about 1%: the floor is a matter of taste, 3 keeps a Factory
  before the Laboratory everywhere at no measurable cost.
- **Replacement** (`tierReplace`): when GreedyPick finds nothing AND the
  population is capped or no candidate has a square left (`roomFor`; a
  colony merely waiting for a worker to grow does not), a structure of a
  family is swapped for that family's best known tier only, the Upgrade
  threshold waived (no base structure fits any more, and a
  small full planet never reaches 3 of a family), with
  replacePick's rule (shared `bestSwap`): gain = Σ(OutputWith(at, new) −
  Output()), loss = ⌈Cost(new) / industry without it⌉ × the output lost,
  qualifies at gain > 0 and gain × 100 ≥ loss (`replaceDays`), best
  gain/loss wins. One at a time per colony: the swap is the colony's
  project.
- **Terraforming** (as soon as it is known):
  right after the dig and the home's script, before the Shipyard, a
  colony with a black square `CanTerraform` allows whitens it
  (`terraformAt`: an empty one, else one under Tubes, in Cells order).
  Not an aim of the research.
- **Research** (`aimTopic`): once GuideResearch is done, toward the
  techs of `AimItems` in order (Habitat's Planetary Replenishment,
  Metroplex's Large Scale Construction, the Megafacility's Positron
  Guidance, then Campus and Hydroponifer, the guide's already): the aim
  when Available, else the cheapest Available tech among its unknown
  prerequisites, transitively (ties the lower index). Every aim known:
  `Available()[0]` as before. A guide head not yet studiable still falls
  back on `Available()[0]`. The ongoing projects stay the last resort
  (`idle` after build).

Measured (before → after; the Genome branch byte-identical, sim -galaxy
seed 1 x3 1200 days with a replacing genome, 308 swaps):

| | before | after |
|---|---|---|
| solo, 21 species x seeds 1-8, 3000 days: runs reaching 1/4 | 131/168, mean day 2325 | 166/168, mean day 2071 |
| solo: reaching 1/2 (no win either way) | 0 | 4 |
| solo: mean systems, colonies at day 3000 | 14.5, 43.5 | 17.9, 52.6 |
| versus 4 x seeds 1-8, 2000 days: systems held, all empires | 281 | 307 |
| seed 264797 x7 at day 900: colonies | 67 | 89 |
| same: upper tiers standing (Mega/Campus/Hydro/Habitat) | 0/0/0/0 | 54/105/177/13 |
| same: colonies on Alien Hospitality / no project | 12% / 48% | 0% / 4% |
| 4 empires, seeds 1-3, day 900: Alien Hospitality / no project | 5% / 49% | 0% / 12% |

Industry a colony and techs barely move at day 900 (6.2 → 5.1 without the
ongoing project, 17-25 → 17-22 techs: the aims are dear techs, and the
new colonies are young); prosperity is the scarcest output most days
(the −pop/4 upkeep), so the Hydroponifer is the commonest upper tier.
The colonisation pace (mksave -empires 5, seeds 1-3, colonies of all
empires) stays near the table of "Galaxy run": 302: 8/7/9 (same), 443: 25/22/21
(was 29/21/28), 710: 57/44/50 (57/50/46), 1030: 82/72/77 (80/78/70),
1466: 106/96/115 (100/107/100).

**The economy genes** ("faire varier l'envie de
faire des bâtiments améliorés avec un gène"): `Upgrade` [0, 20],
`IndShare` and `ResShare` [0, 1] each, scaled down to a sum of 1 past it
(`bot.Economy`; GA scales 2, 0.1, 0.1). A nil genome plays
`NeutralEconomy` (b.econ, the combat and diplomacy genes' idiom), so the
window's bots and the GA share the pick. The Genome branch moved through
them: GenomePick's Balance bonus now goes to the wanted output's makers
(not the scarcest's), and GenomePick and replacePick skip an upper tier
under Upgrade. Neutral: Upgrade 3, IndShare 0.5, ResShare 0.3, the best
of a grid at seed 264797 x7, day 900 (per colony; Upgrade by mix):

| Upgrade, ind/res/pros | ind | res | pros | techs | Megafacility | Metroplex |
|---|---|---|---|---|---|---|
| 0, 0.34/0.33/0.33 (= scarcest first) | 4.2 | 4.8 | 3.2 | 19.1 | 54 | 0 |
| 0, 0.5/0.3/0.2 | 7.9 | 5.9 | 3.5 | 21.3 | 223 | 0 |
| 2, 0.5/0.3/0.2 | 7.9 | 5.6 | 3.3 | 21.9 | 192 | 0 |
| 3, 0.4/0.3/0.3 | 5.4 | 4.7 | 3.7 | 20.9 | 63 | 0 |
| 3, 0.45/0.35/0.2 | 6.5 | 5.9 | 3.1 | 22.4 | 127 | 2 |
| **3, 0.5/0.3/0.2** | **8.5** | **5.7** | **3.4** | **23.4** | 205 | 18 |
| 5, 0.5/0.3/0.2 | 7.4 | 5.0 | 3.2 | 23.3 | 123 | 26 |

Solo seeds 1-4 (84 runs, 3000 days), mean day to 1/4: 2032 at the
chosen genes, 2005 at Upgrade 2, 1999 at 0, 2206 at 0.4/0.3, 2163
before. Before → after the lot:

| | before | after |
|---|---|---|
| solo 21 x seeds 1-8: runs reaching 1/4, mean day | 166/168, 2071 | 167/168, 1926 |
| solo: reaching 1/2; wins | 4; 0 | 12; 0 |
| solo: mean systems, colonies at day 3000 | 17.9, 52.6 | 19.2, 55.9 |
| versus 4 x seeds 1-8, 2000 days: systems held | 307 | 318 |
| seed 264797 x7, day 900: colonies, techs (mean) | 89, 19.1 | 93, 23.4 |
| same, per colony: industry / research / prosperity | 4.2 / 4.8 / 3.2 | 8.5 / 5.7 / 3.4 |
| same: Factory, Laboratory, Agriplot | 83, 57, 18 | 221, 183, 172 |
| same: Megafacility, Campus, Hydroponifer | 54, 105, 177 | 205, 76, 78 |
| same: Habitat, Metroplex, Outpost | 13, 0, 110 | 43, 18, 100 |
| same: Alien Hospitality / no project | 0% / 4% | 3% / 23% |

The 23% with no project (21 colonies): 9 wait for a worker to grow, 1
is capped with none free, 11 have a free worker on a full planet:
mostly small planets holding fewer than 3 structures of any family
(2 Factories, 2 Laboratories), so the threshold bars their upgrade for
good. The colonisation pace runs faster (colonies of all empires,
mksave -empires 5, seeds 1-3): 302: 7/6/11 (was 8/7/9), 443: 32/24/29
(25/22/21), 710: 61/52/57 (57/44/50), 1030: 85/77/87 (82/72/77), 1466:
108/109/123 (106/96/115), past the cap-2 band of "Colonisation pace"
from day 710 on; FoundFloor and InFlight left as tuned.

## The bot's genome and the GA

`bot.Genome` (bot/genome.go) holds the weights: Build per structure,
Tech per tech, Balance, CostW, ResCostW, ShipyardFactories,
FrontierWorth, RuinWorth, IdleParty and AutomateCostW (with the scalars
in the GA), then the combat genes (the specials' CannibalAt, ShieldAt,
RecallAt, the shields' ShieldFirst and the defences' ShieldWorth,
ShieldPop, OrbitalShields, OrbitalGuns among them, GA scalars). A nil
genome plays the fixed policy, byte-identical. `cmd/fuzz -mode evolve`
breeds genomes on solo fitness: 2 − winDay/cap for a win, else the share
of the win's systems. The first run (Snovemdomas, seeds 1-4, pop 24, 8
generations, 9 min) scored 1.554 against Default's 1.502, a win about
150 days sooner. Every seed was won from generation 0, so the signal is
only the win day. A weight matters only for an item the research
reaches, so a `never` there is not yet a verdict.

**Replacement genes** (replace the structures by
better ones, waiting or not for the planet to be full, "c'est justement
à évaluer"; `bot/replace.go`). In `build`, with a Genome under Greedy,
right before GenomePick: when `ReplaceDays` > 0 and (`ReplaceFull` <
0.5 or no surface item of Build > 0 research allows has a BestCell
left), `replacePick` and `Colony.Replace`. Candidates: an old surface
structure with an output and no MaxPop (never a Habitat, Metroplex,
Outpost, Tubes, defence, plant, Colony Base) and a new one alike with
Build > 0, CanReplace nil. On the colony without its ongoing project:
gain = Σ(OutputWith(at, new) − Output()) over the three outputs,
unweighted (a weight on the scarcest output, the first version, made
the bots trade Factory and Laboratory back and forth: Σ output only
grows swap after swap, so no trade is ever reversed), loss = ⌈Cost(new) /
industry of OutputWith(at, NoItem)⌉ × Σ(Output() − OutputWith(at,
NoItem)) (the output lost while it builds); it qualifies at gain > 0
and gain × ReplaceDays ≥ loss, the best gain/loss wins (loss 0 first;
ties Cells order, then item index). Neutral 0/0 (never): every
existing genome plays as before. Clamp [0, 400] and [0, 1]; GA scales
100 and 0.5. Neutral's Build holds only the GreedyItems (Factory,
Agriplot, Laboratory: equal outputs, never a gain); the higher tiers
need Build > 0 from the GA.

## Oracles for "useless buildings"

The 2005 guide's dead-end ratings (Cloaking, Superconductivity,
Repulsion Beam, Snooping, Gravity Flow Control: never; Advanced
Chemistry, Advanced Planetary Armaments: always) and James Friel's
verdicts (Fertilization Plant, Cloning Plant, Logic Factory: worthless;
his account of the ongoing projects: docs/README.md). Nathan Williams'
ballistics guide holds measured weapon damage per hull and power unit
for the combat model.
