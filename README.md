# AIscendancy

Claude-assisted remake of Ascendancy (1995) in Go + Ebiten. No decompiled
code; needs the original game's files.

*Ascendancy* (The Logic Factory, 1995) is a space 4X for DOS. This remake
uses [Ebiten](https://ebitengine.org/). It is inspired by the original
rather than identical to it. The game's rules are reimplemented from its own
data tables. The interface is redone at 1280x960 to show in one place what
the original spreads over several screens: sortable lists, build queues, a
log, a research tree. Some additions are not in the original: multi-hop
routes for ships and population transport by ship.

**You need a copy of the original game.** This repository holds no file of
the game. The program reads the art, the texts and the music straight from
the original's archives, `ascend00.cob`, `ascend01.cob` and `ascend02.cob`.

## Build and run on Linux

You need [Go](https://go.dev/dl/) 1.24 or later, a C compiler and the
development headers Ebiten links against. On Debian or Ubuntu:

```bash
sudo apt install gcc pkg-config libc6-dev libgl1-mesa-dev libxcursor-dev \
    libxi-dev libxinerama-dev libxrandr-dev libxxf86vm-dev libasound2-dev
```

For other distributions, see [Ebiten's install guide](https://ebitengine.org/en/documents/install.html).

Build and start the game:

```bash
go build -o aiscendancy ./cmd/aiscendancy
./aiscendancy -game /path/to/Ascend
```

`/path/to/Ascend` is the directory that holds `ascend00.cob`,
`ascend01.cob` and `ascend02.cob`. Without `-game`, the program looks in
`$ASCEND_DIR`, then in `~/dosbox/Ascend`.

F5 saves to `~/.local/share/aiscendancy/quick.json.gz`. Use `-save` to
choose another file.

To avoid typing the same flags every time, put them in an
`aiscendancy.json` file in the directory you start the game from. Copy
`aiscendancy.json.example` to begin. The flags you type on the command line
take precedence over the file.

```json
{ "game": "/path/to/Ascend" }
```

`./aiscendancy -h` lists every flag.

## Build and run on Windows (untested)

The author has not tested this part. Ebiten needs no C compiler on
Windows. Install Go 1.24 or later, then run in a terminal from the
repository:

```bat
go build -o aiscendancy.exe ./cmd/aiscendancy
aiscendancy.exe -game C:\path\to\Ascend
```

You can also build the Windows binary from Linux:

```bash
GOOS=windows GOARCH=amd64 go build -o aiscendancy.exe ./cmd/aiscendancy
```

The `aiscendancy.json` file works the same way as on Linux. In JSON,
double the backslashes: `{ "game": "C:\\Games\\Ascend" }`. F5 saves to
`%LOCALAPPDATA%\aiscendancy\quick.json.gz`.

In the game:
- The mouse is enough to play everything.
- Space or the mouse's forward button ends the day.
- Esc or the back button goes back.
- F5 saves and F9 loads. F2 opens the settings and F3 toggles the music.

## What is in it

- `game/`: the rules, as a headless and deterministic model with no pixels.
- `bot/`: the computer players. A bot's behaviour is a set of weights (its genome). Its decisions use no random numbers.
- `cmd/aiscendancy`: the game itself.
- `cmd/sim`: a headless simulator.
- `cmd/fuzz`: plays many bot-only games, and evolves bot genomes with a genetic algorithm.
- `cmd/mksave`: builds saves offline.
- `cmd/savestat`: compares saves.
- `cob/`, `shp/`, `flic/`, `dip/`, `help/`, `sound/`: readers for the original's file formats.
- `sav/`, `cmd/savedump`: a reader for the original's save games. It was used to calibrate the cluster generator.

The `docs/` directory explains the work in detail:
- `formats.md`: the file formats.
- `rules.md`: every rule, with its source.
- `screens.md`: each screen, against the original's.
- `galaxy.md`: the cluster generator.

```bash
go test ./...      # tests that need the game's files skip without them
```

## Licence

The code is under the MIT licence (`LICENSE`). *Ascendancy* and its assets
belong to their owners. This project is not affiliated with them.
