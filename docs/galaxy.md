# The star cluster generator (`game/cluster.go`)

The generator is NOT in the game's files. What they give: `starstat.txt`
(13 star types, their shares, 2.24 mean lanes per star), `names.txt` (100
names), static.txt (five densities), cosmos.txt STARS 50, windows.txt
NUM_STARS 100. Star counts per density, MEASURED in saves of
fresh games: Vacuous 15, Sparse 25, Average 50, Dense 75, Very Dense 100.

## What the original does (measured in 12 saved galaxies with `sav/`)

| density (stars) | lanes/star | red | blue len | red len | red/blue | dead ends | y/xz |
|---|---|---|---|---|---|---|---|
| Vacuous (15), 2 saves | 1.9-2.0 | 0% | 0.9 sp | – | – | 27-47% | 1.0 |
| Sparse (25) | 2.16 | 7% | 0.83 | 1.9 | 2.3 | 28% | 0.9 |
| Average (50), 6 saves | 2.12-2.24 | 6-11% | 0.63-0.70 | 1.3-1.9 | 1.8-2.9 | 18-24% | 0.85-1.14 |
| Dense (75) | 2.19 | 8.5% | 0.66 | 1.28 | 1.95 | 23% | 1.0 |
| Very Dense (100), 2 saves | 2.20-2.32 | 11-14% | 0.59-0.63 | 1.05-1.25 | 1.8-2.0 | 16-24% | 1.05 |
| **mean** | **2.15** | **9.4%** | 0.72 | 1.45 | **2.14** | **24%** | **1.00** |

(sp = cbrt(bounding-box volume / n): NOT the unit of cluster.go's constants,
see below. The mean row is over the galaxies with reds for the red columns.)

- **Positions: concentric SHELLS, no squash.** Distance from the origin is
  440 − 75k (440, 365, 290, 215, 140, 65) plus a few at 50; about half the
  stars on the outer shell and each shell inward holds about half the
  next (50 stars: 26/12/6/3/1/1; 100: 51/24/12/6/3/2; 15: 8/3/2/1/1). The
  cloud is round (y/xz = 1.00). Nearest pairs are much closer than our
  0.9-spacing minimum (15-54 units at 50 stars, i.e. ~0.1-0.4 sp).
- **Blue lanes alone are NOT connected**: 3-13 islands (one at Vacuous),
  about n−1 to n blue lanes; each star's nearest neighbour is almost
  always a blue partner (the nearest-neighbour graph ⊂ blue in 10 of 12).
- **Red lanes are the bridges between the blue islands**: every red joins
  two different islands, the red count ≈ the island count, 0 reds when
  blue is one island; a red is 1.0-2.5× the shortest possible link
  between its islands, often exactly the shortest: a near-Kruskal bridge.
- Fewer lanes than our third design (2.15 against 2.4; starstat's 2.24
  is an upper value), less red (9.4% against 11-12%), shorter reds (2.1×
  blue against 2.5-2.7×), more dead ends (24% against 16-21%); degree 4
  rare, degree 5 once.

## Ours (fourth design)

`NewCluster` in `game/cluster.go`, one PCG for the whole cluster, steps in
a fixed order (names, types, positions, lanes, home, systems).

1. Names shuffled, types by starstat's shares.
2. **Positions on the shells**: per star a shell k = heads of a fair coin
   capped at 5 (P = 1/2, 1/4, … , 1/32 for the last two), radius
   `shellRadii` = (440 − 75k)/440, a direction uniform on the sphere (a
   normalised Gaussian triple), redrawn while closer than `minSep` = 0.03
   (unit ball; ≈ 13 file units) to an earlier star, up to 200 times.
3. **Blue lanes**, no PRNG draw. First every star's **nearest neighbour**
   (ties by lowest index), whatever its length (no star
   may hang on a single red, as a lone star bridged by a red did;
   the saves have nnB 98-100%). Then, over all pairs by length (ties by
   index), a Kruskal forest of the pairs ≤ `blueCap`/√n (3.4, recalibrated
   from 3.65 when the NN lanes merged 1.6 islands at Average; lane units
   of 1/√n, 0.48 = 212 file units at 50 stars) joins those components and
   leaves I islands, none of one star; then extras of the same cap
   between stars of ≤ `extraMaxDeg` = 2 lanes, shortest first, until the
   blues number round(2.15·n/2) − (I − 1). The NN lanes and the forest
   are n − I lanes, always under that target: the extras never vanish.
   Some NN lanes are longer than the cap: 2% of the
   blues at Vacuous, 5% at Average, 10% at Very Dense.
4. **Bridge reds**, no PRNG draw: Kruskal over the islands, I − 1 of them,
   each round the cross-component pair of lowest length ×
   `redPenalty`^(ends with ≥ `redHubDeg` = 3 lanes), `redPenalty` = 1.5.
   **Cycle reds**, no PRNG draw: while two stars lie more than
   `maxHops(n)` = ceil(`maxHopsFrac`·n) lanes apart (`maxHopsFrac` =
   0.33: 5, 9, 17, 25, 33 hops for 15-100 stars), among those pairs the
   one of lowest same score gets a red, the hops (a BFS from every star)
   recomputed after each. Same `Red: true` in the data; a cycle red may
   also join two islands.
   **Cover reds** (in a 100-star game one red held 46
   stars alone), no PRNG draw: while a lane alone holds more
   than `chokeFrac` = 10% of the stars (a bridge, its loss cutting them
   off; `chokes`, Tarjan's low links), the pair crossing the most such
   chokes per score gets a red: one red between two chains' ends lifts
   several, and lands on the dead ends. +0.6 to +1.5 reds a galaxy; that
   map would take 2 (the worst choke 46 → 9 stars, 33 → 26 hops). The
   first try, the shortest pair across the worst choke, one at a time,
   took 16 there; linking stars close in space but far in hops from the
   blue graph alone did worse (20+ reds at 100 stars for the same).
5. Home and systems unchanged (2-3 lanes; 1-5 planets uniform, see the
   census in `docs/formats.md`).
6. **Xeno ruins** (`layRuins`), by a PCG of their own (seed,
   0x3c6ef372fe94f82b) so steps 1-5 draw what they drew before: each
   planet, the homes excepted, holds ruins with probability `RuinChance`
   = 0.154 on one surface square drawn uniformly (the saves: 123 of 797
   day-0 planets, always one square, type and size not mattering;
   docs/rules.md), a tiny planet with `RuinChanceTiny` = 0.25 (ours:
   a reason to settle the tiny ones; the same draws,
   only the threshold moves; seeds 1-20: 25.4% of the tiny, 15.1% of the
   others, 17.1% in all). The per-system counts below predate it. Ruins
   per system, seeds 1-20 Average (1000 stars) against the 5 day-0 saves
   (265 stars): 0: 61.4% / 62%, 1: 31.1% / 30%, 2: 7.0% / 7.5%, 3: 0.5%
   / 0.4%. The square's kind follows the planet's mix (31% black, all
   squares 32%); the saves lean a little to black (41%), not reproduced.

**Units.** Positions are unit-ball (outer shell 1 = 440 file units).
`blueCap` is in units of 1/√n, not cbrt(1/n): half the stars sit on the
outer SPHERE, so the near-neighbour spacing goes like n^-1/2. With
cbrt(1/n) no cap gave the saves' island growth (3 → 5 islands from 15 to
100 stars, against 1 → 12). sav.Stats' "sp" is a third unit
(cbrt(box/n) ≈ 2·cbrt(1/n) for a radius-1 ball): never compare a constant
with a stats column by eye, measure both with savedump.

**Side by side** (saves: the distinct galaxies of each density; ours: seeds
1-30, `savedump -stats -gen <d>:1-30`; red share and r/b averaged over the
galaxies that have reds, brdg over those with a red across two islands,
as savedump's mean row does). isl = blue islands, nnB = stars whose
nearest neighbour is a blue partner, brdg = red length / the closest pair
of the two islands it joins, diam = the most lanes on a shortest path
(hops), cyc = cycle and cover reds per galaxy (reds − (isl − 1)); the
"ours" rows measured with the cover reds.

| density | | l/s | red | r/b | dead | isl | nnB | brdg | diam | cyc |
|---|---|---|---|---|---|---|---|---|---|---|
| Vacuous (15) | saves (2) | 1.94 | 0% (none) | – | 37% | 1 | 90% | – | 8.5 | – |
| | ours | 2.48 | 15.8% (30/30 have reds) | 2.10 | 7% | 1.3 | 100% | 1.26 | 4.9 | 2.6 |
| Sparse (25) | saves (1) | 2.16 | 7.4% | 2.29 | 28% | 2 | 100% | 1.92 | 10 | – |
| | ours | 2.35 | 10.6% (30/30) | 2.05 | 14% | 1.8 | 100% | 1.23 | 8.3 | 2.3 |
| Average (50) | saves (5) | 2.17 | 8.5% | 2.29 | 21% | 4.6 | 99% | 1.51 | 20.6 | – |
| | ours | 2.25 | 9.6% | 1.91 | 18% | 4.2 | 100% | 1.11 | 15.5 | 2.2 |
| Dense (75) | saves (1) | 2.19 | 8.5% | 1.95 | 23% | 7 | 99% | 1.21 | 26 | – |
| | ours | 2.22 | 11.2% | 1.77 | 19% | 7.9 | 100% | 1.04 | 21.1 | 2.4 |
| Very Dense (100) | saves (2) | 2.26 | 12.4% | 1.87 | 20% | 12.5 | 100% | 1.19 | 31 | – |
| | ours | 2.21 | 12.8% | 1.69 | 19% | 12.8 | 100% | 1.03 | 26.7 | 2.3 |

**Where we diverge on purpose: the hop cap.** By choice, no
star is more than 33% of the star count in lanes from another, a red
joining the two far ends of the universe when needed (a
global cycle). The saves are chain-like: their diameters are 27-60% of n
(15 stars: 8-9 hops; 25: 10; 50: 16-24; 75: 26; 100: 27-35), and ours
were 35-57% before the cap. So this is QoL, not fidelity, and it costs:
more lanes and reds at the small densities (Vacuous 2.47 lanes per star
and 15% red against the saves' 1.94 and none), fewer dead ends
(the cycle reds land on the chains' ends: Vacuous 7% against 37%).

What does not match:
- **brdg ≈ 1.0-1.1 against 1.2-1.9**: a global Kruskal takes the shortest
  bridge. The saves' reds start at a star (often a blue dead end) and go
  to the nearest star outside ITS island; that is not the islands'
  closest pair (rank up to 176 among the cross pairs), and there are
  about I reds, not I − 1 (3 reds on 3 islands in 01.sav, 16 on 13 in
  08.SAV). Hence also r/b short at Average and below.
  `redPenalty` (1.5 or 2.5) and a dead-end preference barely move it.
- Vacuous dead ends (27% and 47% in the two saves) and Very Dense lanes
  per star (2.26): `lanesPerStar` is one number for every density; at
  Vacuous the hop cap (5 hops) adds 2.5 reds per galaxy on top.

Re-measure, after changing a constant:

```bash
go build -o bin/savedump ./cmd/savedump
bin/savedump -stats ~/dosbox/Ascend/*.sav ~/dosbox/Ascend/*.SAV ~/dosbox/Ascend/*.GAM \
    ~/dosbox/Ascend/*.gam   # plus any other DOSBox install's Ascend/*.SAV
for d in vacuous sparse average dense verydense; do bin/savedump -stats -gen $d:1-30 | tail -1; done
```

## Rejected designs

- MST + short extras to 1.12 lanes per star, longest fifth red: long
  chains, red lanes barely longer than blue.
- Positions in groups with Kruskal bridges between the blue components:
  islands, "ça manque de connexité".
- Third design: uniform ball squashed 0.7 in Y, 0.9-spacing
  separation, blue = the 3D relative neighbourhood graph pruned to 2.1
  lanes per star, red = 13% of n random chords 1.8-4 spacings long.
  Replaced: no shells, blue connected by itself (no islands), reds as
  long chords (2.5-2.7× blue), 2.4 lanes per star, 16-21% dead ends.
