// cob lists and extracts the game's COB archives.
//
//	cob ls                 every file: archive, size, name
//	cob cat data/game.pal  one file to stdout
//	cob x -out dir         extract everything under dir
//
// -game picks the install directory ($ASCEND_DIR or ~/dosbox/Ascend).
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rewk/aiscendancy/cob"
)

func main() {
	game := flag.String("game", cob.DefaultDir(), "game directory holding ascend*.cob")
	out := flag.String("out", "cob", "x: output directory")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: cob [-game dir] ls | cat name | x [-out dir]")
		flag.PrintDefaults()
	}
	flag.Parse()
	g, err := cob.OpenGame(*game)
	if err != nil {
		fail(err)
	}
	defer g.Close()
	switch flag.Arg(0) {
	case "ls":
		for _, a := range g.Archives {
			for _, e := range a.Entries() {
				fmt.Printf("%s\t%9d\t%s\n", filepath.Base(a.Path), e.Size, e.Name)
			}
		}
	case "cat":
		b, err := g.Read(flag.Arg(1))
		if err != nil {
			fail(err)
		}
		os.Stdout.Write(b)
	case "x":
		for i, a := range g.Archives {
			for _, e := range a.Entries() {
				dst := filepath.Join(*out, fmt.Sprint(i), filepath.FromSlash(e.Name))
				b, err := a.Read(e.Name)
				if err != nil {
					fail(err)
				}
				if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
					fail(err)
				}
				if err := os.WriteFile(dst, b, 0o644); err != nil {
					fail(err)
				}
			}
			fmt.Printf("%s: %d files -> %s/%d\n", a.Path, len(a.Entries()), *out, i)
		}
	default:
		flag.Usage()
		os.Exit(2)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
