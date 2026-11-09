package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/help"
	"github.com/rewk/aiscendancy/ui"
)

// helpTexts are the original's help paragraphs (package help): items by
// game.Item, parts by game.Gizmo, "%s" still in them.
type helpTexts struct {
	items, gizmos []string
}

// helps reads planhelp.txt and gizhelp.txt once; a failure (or no
// assets) logs and leaves its slice empty: every help is then "".
func (a *app) helps() *helpTexts {
	if a.helpTxt != nil {
		return a.helpTxt
	}
	a.helpTxt = &helpTexts{}
	if a.a == nil {
		return a.helpTxt
	}
	read := func(name string) []string {
		b, err := a.a.Game.Read(name)
		if err == nil {
			var ps []string
			if ps, err = help.Parse(b); err == nil {
				return ps
			}
		}
		log.Printf("%s: %v", name, err)
		return nil
	}
	a.helpTxt.items = read("planhelp.txt")
	a.helpTxt.gizmos = read("gizhelp.txt")
	return a.helpTxt
}

// itemHelp is it's paragraph, %s filled with our name; "" when unknown.
// The Colony Base has none: planhelp.txt's is a designers' note ("You
// should not be able to build this."), shown by its structure's dialog.
func (a *app) itemHelp(it game.Item) string {
	if it == game.ColonyBase {
		return ""
	}
	if ps := a.helps().items; it >= 0 && int(it) < len(ps) {
		return help.Fill(ps[it], it.String())
	}
	return ""
}

// ourGizmoHelp is the paragraph of each part of ours (game.NumGameGizmos
// on), which gizhelp.txt does not list.
var ourGizmoHelp = map[game.Gizmo]string{
	game.SpaceHabitat: "Carries three population units between your colonies. Load and Unload them one at a time from the planet screen's ship dialog.",
}

// gizmoHelp is g's paragraph, %s filled with our name; ours from
// ourGizmoHelp; "" when unknown.
func (a *app) gizmoHelp(g game.Gizmo) string {
	if g >= game.NumGameGizmos {
		return ourGizmoHelp[g]
	}
	if ps := a.helps().gizmos; int(g) < len(ps) {
		return help.Fill(ps[g], g.String())
	}
	return ""
}

// smallH is a smallFace line's pitch (ui.Print's 1.25).
var smallH = smallFace.Size * 1.25

// helpCols is how many smallFace characters fit in w native px.
func helpCols(w float64) int { return int(w / ui.Width("m", smallFace)) }

// helpLines is text reflowed to w native px in smallFace, at most n
// lines, each cut to w; none for "" or n <= 0.
func helpLines(text string, w float64, n int) []string {
	return helpLinesIn(smallFace, text, w, n)
}

// helpLinesIn is helpLines in face.
func helpLinesIn(face *text.GoTextFace, text string, w float64, n int) []string {
	if text == "" || n <= 0 {
		return nil
	}
	ls := reflow(paragraph{false, text}, int(w/ui.Width("m", face)))
	ls = ls[:min(len(ls), n)]
	for i := range ls {
		ls[i] = fitWidth(ls[i], w, face)
	}
	return ls
}

// fitLines is how many smallFace lines fit from native y top to bottom.
func fitLines(top, bottom float64) int { return max(0, int((bottom-top)/smallH)) }
