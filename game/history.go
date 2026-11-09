package game

// The history: every HistoryEvery days each live empire's totals are
// appended to Empire.History, for the window's graphs (the Stats screen,
// the save list's preview). Record only: no rule and no bot reads it.

// HistoryEvery is the sampling period, days: a sample at day 0 (the first
// EndDay's start), then at every multiple of it (the close of that day).
const HistoryEvery = 10

// Sample is an empire's totals on Day. Industry, Research and Prosperity
// are the colonies' Shown (the planets list's numbers), Fleet the ships'
// Design.Cost, Stars the stars holding a colony of the empire.
type Sample struct {
	Day        int
	Colonies   int
	Pop        int
	Industry   int
	Research   int
	Prosperity int
	Ships      int // the starships flying
	Fleet      int
	Techs      int // known
	Stars      int
	Systems    int // ControlledSystems
	Explored   int // stars visited (Knows)
	Wars       int // empires at War with it
}

// sample is e's Sample today.
func (e *Empire) sample() Sample {
	s := Sample{Day: e.Day, Colonies: len(e.Colonies), Ships: len(e.Ships), Systems: e.ControlledSystems()}
	stars := map[int]bool{}
	for _, c := range e.Colonies {
		ind, res, pro := c.Shown()
		s.Pop += c.Pop
		s.Industry += ind
		s.Research += res
		s.Prosperity += pro
		stars[c.Star] = true
	}
	s.Stars = len(stars)
	for _, sh := range e.Ships {
		s.Fleet += sh.Design.Cost()
	}
	for _, k := range e.Research.Known {
		if k {
			s.Techs++
		}
	}
	for _, v := range e.Visited {
		if v {
			s.Explored++
		}
	}
	for j := range e.Relations {
		if j != e.Index && e.StanceWith(j) == War {
			s.Wars++
		}
	}
	return s
}

// record appends today's Sample to every live empire's History (a dead
// one's stops at its last).
func (g *Game) record() {
	for _, e := range g.Empires {
		if e.Alive() {
			e.History = append(e.History, e.sample())
		}
	}
}
