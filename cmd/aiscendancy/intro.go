package main

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The species introduction, between Begin and the starmap (captures
// image0044, image0045): pushed over the fresh starmap, popped by any
// click or key but F3; it plays the stinger raceNN, which leaving does
// not cut.
//
// The chrome is 12start.tmp frame 0: a 7 px ring, 8 px bars, opaque
// starfield panels but the picture's hole. Original px:
//
//	top bar   [7,633)x[7,38)    the flag at (10,10), the title centred
//	picture   [7,304)x[46,246)  transparent, 297x200: lgraceNN.shp at zoom
//	right     [312,633)x[46,246) history.txt's intro, white
//	bottom    [7,633)x[254,473) history.txt's text, in the colour
//
// Text offsets measured on the captures (their line pitch 13 px; ours
// the faces' 1.25).
var (
	inTop    = orig(7, 7, 633, 38)
	inPic    = orig(7, 46, 304, 246)
	inRight  = orig(312, 46, 633, 246)
	inBottom = orig(7, 254, 633, 473)
)

const (
	introChrome      = "12start.tmp"
	inFlagX, inFlagY = 10 * zoom, 10 * zoom // top-left
	inPadX           = 11 * zoom
	inRightTop       = 9 * zoom
	inBottomTop      = 16 * zoom
	paraIndent       = "     " // a paragraph's "_____"
)

// pictureBank is sp's picture, lgraceNN.shp (NN = game.Species = specnum).
func pictureBank(sp game.Species) string { return fmt.Sprintf("lgrace%02d", int(sp)) }

// paragraph is one of history.txt's, gaps as the file spaces them (two
// after a full stop).
type paragraph struct {
	indent bool
	text   string
}

// speciesHistory is one species' block of history.txt.
type speciesHistory struct {
	power                          string
	intro, peace, neutral, hostile []paragraph
	text                           []paragraph
}

// atmosphereParas is the section the game's atmosphere picks.
func (h speciesHistory) atmosphereParas(at game.Atmosphere) []paragraph {
	switch at {
	case game.Peaceful:
		return h.peace
	case game.Hostile:
		return h.hostile
	}
	return h.neutral
}

// introParas is the right panel: the intro, the atmosphere's section
// run on into its last paragraph (as image0045 shows).
func (h speciesHistory) introParas(at game.Atmosphere) []paragraph {
	out := append([]paragraph(nil), h.intro...)
	for _, p := range h.atmosphereParas(at) {
		if n := len(out); n > 0 {
			out[n-1].text += "  " + p.text
		} else {
			out = append(out, p)
		}
	}
	return out
}

// parseHistory reads history.txt: per species "specnum N", then
// sections from a keyword line (power, intro, peace, neutral, hostile,
// text) to its "end<keyword>"; "//" lines are comments, "END" closes,
// CRLF, maybe a ^Z. "@@" opens a paragraph, a leading "_" run is its
// indent, line breaks are soft. specnum must run 0..n−1.
func parseHistory(s string) ([]speciesHistory, error) {
	var (
		out     []speciesHistory
		cur     = -1
		section string
		body    []string
	)
	sections := map[string]bool{"power": true, "intro": true, "peace": true, "neutral": true, "hostile": true, "text": true}
	for n, l := range strings.Split(s, "\n") {
		l = strings.TrimRight(l, "\r\x1a")
		switch {
		case section != "":
			if strings.TrimSpace(l) != "end"+section {
				body = append(body, l)
				continue
			}
			h := &out[cur]
			ps := paragraphs(strings.Join(body, " "))
			switch section {
			case "power":
				h.power = strings.TrimSpace(strings.Join(body, " "))
			case "intro":
				h.intro = ps
			case "peace":
				h.peace = ps
			case "neutral":
				h.neutral = ps
			case "hostile":
				h.hostile = ps
			case "text":
				h.text = ps
			}
			section, body = "", nil
		case strings.TrimSpace(l) == "", strings.HasPrefix(l, "//"), l == "END":
		case strings.HasPrefix(l, "specnum "):
			k, err := strconv.Atoi(strings.TrimSpace(l[len("specnum "):]))
			if err != nil || k != len(out) {
				return nil, fmt.Errorf("history.txt:%d: %q, want specnum %d", n+1, l, len(out))
			}
			out = append(out, speciesHistory{})
			cur = k
		case sections[strings.TrimSpace(l)] && cur >= 0:
			section = strings.TrimSpace(l)
		default:
			return nil, fmt.Errorf("history.txt:%d: unexpected %q", n+1, l)
		}
	}
	if section != "" {
		return nil, fmt.Errorf("history.txt: %s never ends", section)
	}
	return out, nil
}

// paragraphs splits a section on "@@", trimmed, the "_" run read as the
// indent, 3+ space runs brought to two.
func paragraphs(s string) []paragraph {
	var out []paragraph
	for p := range strings.SplitSeq(s, "@@") {
		p = strings.TrimSpace(p)
		t := strings.TrimLeft(p, "_")
		ind := len(t) < len(p)
		for strings.Contains(t, "   ") {
			t = strings.ReplaceAll(t, "   ", "  ")
		}
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, paragraph{ind, t})
		}
	}
	return out
}

// reflow wraps p into lines of at most cols characters, breaking at the
// gaps (kept inside a line, dropped at its end); a longer word gets its
// own line.
func reflow(p paragraph, cols int) []string {
	var lines []string
	line, gap := "", 0
	if p.indent {
		line = paraIndent
	}
	fresh := true // nothing but the indent on the line yet
	for _, w := range strings.Split(p.text, " ") {
		if w == "" {
			gap++
			continue
		}
		sp := strings.Repeat(" ", gap+1)
		gap = 0
		switch {
		case fresh:
			line += w
		case utf8.RuneCountInString(line)+len(sp)+utf8.RuneCountInString(w) <= cols:
			line += sp + w
		default:
			lines = append(lines, line)
			line = w
		}
		fresh = false
	}
	return append(lines, line)
}

// introFaces are the texts' sizes, largest first (the original's pitch
// is ~26 native): the largest holding both panels whole is taken.
var introFaces = []*text.GoTextFace{ui.Face(22), ui.Face(20), ui.Face(18), bigFace, smallFace}

var introTitleFace = ui.Face(22)

// layText reflows ps to w px, a blank line between paragraphs; ok when
// they fit h px.
func layText(ps []paragraph, w, h int, face *text.GoTextFace) (lines []string, ok bool) {
	cols := int(float64(w) / ui.Width("m", face))
	for i, p := range ps {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, reflow(p, cols)...)
	}
	return lines, len(lines) <= int(float64(h)/(face.Size*1.25))
}

// fitText lays sets[i] in boxes[i] in the first introFaces where all
// fit, else in the last, cut to the rows.
func fitText(sets [][]paragraph, boxes []rect) ([][]string, *text.GoTextFace) {
	out := make([][]string, len(sets))
	for k, face := range introFaces {
		all := true
		for i, ps := range sets {
			b := boxes[i]
			var ok bool
			out[i], ok = layText(ps, b.X1-b.X0, b.Y1-b.Y0, face)
			if !ok && k == len(introFaces)-1 {
				out[i] = out[i][:int(float64(b.Y1-b.Y0)/(face.Size*1.25))]
			}
			all = all && ok
		}
		if all {
			return out, face
		}
	}
	return out, introFaces[len(introFaces)-1]
}

// The panels' text boxes, native px.
var (
	inRightBox  = rect{inRight.X0 + inPadX, inRight.Y0 + inRightTop, inRight.X1 - inPadX, inRight.Y1}
	inBottomBox = rect{inBottom.X0 + inPadX, inBottom.Y0 + inBottomTop, inBottom.X1 - inPadX, inBottom.Y1}
)

// introScreen is the player's species' introduction, laid out on Enter
// (no text without a.history).
type introScreen struct {
	species game.Species
	colour  int
	title   string

	right, bottom []string
	face          *text.GoTextFace // both panels' (fitText)
}

func (s *introScreen) Enter(a *app) {
	s.species, s.colour = a.e.Species, a.e.Colour
	s.title = s.species.Info().Name
	if int(s.species) < len(a.history) {
		h := a.history[s.species]
		s.title += ": " + h.power
		ls, face := fitText([][]paragraph{h.introParas(a.g.Atmosphere), h.text}, []rect{inRightBox, inBottomBox})
		s.right, s.bottom, s.face = ls[0], ls[1], face
	}
	a.music.stinger(s.species)
}

// typing keeps Space, Tab, F5 and F9 from the app: here they only leave.
func (s *introScreen) typing() bool { return true }

// leave uncovers the starmap begin opened under the screen.
func (s *introScreen) leave(a *app) { a.pop() }

func (s *introScreen) Update(a *app) error {
	for _, b := range []ebiten.MouseButton{ebiten.MouseButtonLeft, ebiten.MouseButtonRight} {
		if inpututil.IsMouseButtonJustPressed(b) {
			s.leave(a)
			return nil
		}
	}
	for _, k := range inpututil.AppendJustPressedKeys(nil) {
		if k != ebiten.KeyF3 { // the music's toggle, app-level
			s.leave(a)
			return nil
		}
	}
	return nil
}

// Render paints the chrome, the picture in its hole and the flag.
func (s *introScreen) Render(a *app, sc *ebiten.Image) {
	sc.Fill(color.Black)
	drawSprite(sc, a.spr(introChrome, 0), 0, 0)
	drawSprite(sc, a.spr(pictureBank(s.species), 0), float64(inPic.X0), float64(inPic.Y0))
	drawSprite(sc, a.flag(s.species, s.colour), inFlagX, inFlagY)
}

// introTone is colourText, the brown (unreadable on the starfield)
// brightened x1.5, hue kept.
func introTone(colour int) color.RGBA {
	c := colourText[colour]
	if colour != game.Brown {
		return c
	}
	up := func(v uint8) uint8 { return uint8(min(255, int(v)*3/2)) }
	return color.RGBA{up(c.R), up(c.G), up(c.B), 255}
}

// Text writes the title and description in introTone, the intro white.
func (s *introScreen) Text(a *app, frame *ebiten.Image) {
	c := introTone(s.colour)
	centred(frame, s.title, introTitleFace, inTop, float64(inTop.Y0+inTop.Y1)/2-introTitleFace.Size/2-zoom, c)
	if s.face != nil {
		ui.Print(frame, strings.Join(s.right, "\n"), s.face, float64(inRightBox.X0), float64(inRightBox.Y0), hoverColour)
		ui.Print(frame, strings.Join(s.bottom, "\n"), s.face, float64(inBottomBox.X0), float64(inBottomBox.Y0), c)
	}
}
