package main

import "fmt"

// located is a screen standing for one place: push drops the stack back
// to an earlier screen of the same place and puts the new one there, so
// the stack never loops. A screen without a place stacks freely.
type located interface{ place() string }

// A pick-mode starmap (navigate.go) stacks over the system it came from.
func (s *starmapScreen) place() string {
	if s.nav != nil {
		return "navigate"
	}
	return "starmap"
}

func (s *systemScreen) place() string   { return fmt.Sprintf("system %d", s.star) }
func (s *planetScreen) place() string   { return fmt.Sprintf("planet %d %d", s.star, s.idx) }
func (s *logScreen) place() string      { return "log" }
func (s *researchScreen) place() string { return "research" }
func (s *fleetScreen) place() string    { return "fleet" }
func (s *planetsScreen) place() string  { return "planets" }
func (s *speciesScreen) place() string  { return "species" }
