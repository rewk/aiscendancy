package game

import (
	"slices"
	"strings"
	"testing"
)

// gizmos.txt: after the "#" line, a name line and a number line per gizmo.
func TestGizmoTableMatchesGame(t *testing.T) {
	lines := gameTxt(t, "gizmos.txt")
	k := slices.Index(lines, "#")
	if k < 0 {
		t.Fatal("no # line")
	}
	body := lines[k+1:]
	if len(body) != 2*int(NumGameGizmos) {
		t.Fatalf("%d lines after #, want %d", len(body), 2*NumGameGizmos)
	}
	for g := Gizmo(0); g < NumGameGizmos; g++ {
		name := strings.ReplaceAll(body[2*g], "^", " ")
		n := numbers(t, body[2*g+1])
		if len(n) < 10 || n[len(n)-1] != 255 {
			t.Fatalf("%s: %v", name, n)
		}
		var flags uint8
		for _, b := range n[9 : len(n)-1] {
			flags |= 1 << b
		}
		want := GizmoInfo{name, GizmoCat(n[0]), n[1], n[2], n[3], n[4], n[5], n[6], n[7], Tech(n[8]), flags}
		if Gizmos[g] != want {
			t.Errorf("gizmo %d: game %+v, typed %+v", g, want, Gizmos[g])
		}
	}
}

func TestGizmoNamesAreConstants(t *testing.T) {
	for _, c := range []struct {
		g    Gizmo
		name string
	}{
		{MassBarrageGun, "Mass Barrage Gun"},
		{GizmoTractorBeam, "Tractor Beam"},
		{MurgatroydsKnower, "Murgatroyd's Knower"},
		{GyroInductor, "Gyro-Inductor"},
		{Hyperfuel, "Hyperfuel"},
	} {
		if c.g.String() != c.name {
			t.Errorf("gizmo %d is %q, want %q", c.g, c.g, c.name)
		}
	}
	if Hyperfuel != NumGameGizmos-1 {
		t.Errorf("Hyperfuel = %d, want the game's last", Hyperfuel)
	}
	for g := Gizmo(0); g < NumGizmos; g++ {
		if r := Gizmos[g].ResearchReq; r != NoTech && r >= NumTechs {
			t.Errorf("%s requires tech %d", g, r)
		}
		if Gizmos[g].Cat >= NumGizmoCats {
			t.Errorf("%s: category %d", g, Gizmos[g].Cat)
		}
	}
}
