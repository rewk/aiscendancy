package cam

import (
	"math"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// The expectations are worked out from the formulas in the package comment
// by hand, for the default camera at (320,240).
var c = Default(320, 240)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func samePoint(a, b Point) bool {
	return near(a.X, b.X) && near(a.Y, b.Y) && near(a.Scale, b.Scale) && near(a.Depth, b.Depth)
}

func TestOrigin(t *testing.T) {
	want := Point{320, 240, DefaultFocal / DefaultDist, DefaultDist}
	if got := c.Project(game.Vec3{}); !samePoint(got, want) {
		t.Errorf("origin -> %+v, want %+v", got, want)
	}
}

// +Z (z1 = 1): camy = −sin P, Depth = Dist − cos P. −Z flips both signs.
func TestNearSideIsLowerAndBigger(t *testing.T) {
	sp, cp := math.Sincos(DefaultPitch)
	sNear, sFar := DefaultFocal/(DefaultDist-cp), DefaultFocal/(DefaultDist+cp)
	nearPt, farPt := c.Project(game.Vec3{Z: 1}), c.Project(game.Vec3{Z: -1})
	if want := (Point{320, 240 + sNear*sp, sNear, DefaultDist - cp}); !samePoint(nearPt, want) {
		t.Errorf("+Z -> %+v, want %+v", nearPt, want)
	}
	if want := (Point{320, 240 - sFar*sp, sFar, DefaultDist + cp}); !samePoint(farPt, want) {
		t.Errorf("-Z -> %+v, want %+v", farPt, want)
	}
	if !(nearPt.Y > farPt.Y && nearPt.Scale > farPt.Scale && nearPt.Depth < farPt.Depth) {
		t.Errorf("+Z %+v is not below, bigger and nearer than -Z %+v", nearPt, farPt)
	}
}

// ±X: camx = ±1, camy = 0, Depth = Dist.
func TestXMirror(t *testing.T) {
	s := float64(DefaultFocal) / DefaultDist
	if got, want := c.Project(game.Vec3{X: 1}), (Point{320 + s, 240, s, DefaultDist}); !samePoint(got, want) {
		t.Errorf("+X -> %+v, want %+v", got, want)
	}
	if got, want := c.Project(game.Vec3{X: -1}), (Point{320 - s, 240, s, DefaultDist}); !samePoint(got, want) {
		t.Errorf("-X -> %+v, want %+v", got, want)
	}
}

// +Y: camy = cos P, Depth = Dist − sin P: above the origin on screen.
func TestUpIsHigher(t *testing.T) {
	sp, cp := math.Sincos(DefaultPitch)
	s := DefaultFocal / (DefaultDist - sp)
	got := c.Project(game.Vec3{Y: 1})
	if want := (Point{320, 240 - s*cp, s, DefaultDist - sp}); !samePoint(got, want) {
		t.Errorf("+Y -> %+v, want %+v", got, want)
	}
	if got.Y >= 240 {
		t.Errorf("+Y at screen y %.1f, not above the origin", got.Y)
	}
}

// Yaw π/2: x1 = −z, z1 = x, so +X lands where +Z did at Yaw 0 and +Z
// where −X did.
func TestQuarterTurn(t *testing.T) {
	q := c
	q.Yaw = math.Pi / 2
	if got, want := q.Project(game.Vec3{X: 1}), c.Project(game.Vec3{Z: 1}); !samePoint(got, want) {
		t.Errorf("yawed +X -> %+v, want %+v", got, want)
	}
	if got, want := q.Project(game.Vec3{Z: 1}), c.Project(game.Vec3{X: -1}); !samePoint(got, want) {
		t.Errorf("yawed +Z -> %+v, want %+v", got, want)
	}
}

func TestClamp(t *testing.T) {
	k := Camera{Pitch: -1, Focal: 1e6}
	k.Clamp()
	if k.Pitch != MinPitch || k.Focal != MaxFocal {
		t.Errorf("clamped to pitch %v focal %v", k.Pitch, k.Focal)
	}
	k = Camera{Pitch: 2, Focal: 1}
	k.Clamp()
	if k.Pitch != MaxPitch || k.Focal != MinFocal {
		t.Errorf("clamped to pitch %v focal %v", k.Pitch, k.Focal)
	}
}

// cameras are the views the round trips run on: the default, turned,
// at both pitch and focal limits, the origin off centre.
func cameras() []Camera {
	d := Default(320, 240)
	out := []Camera{d}
	for _, k := range []Camera{
		{Yaw: 1.3, Pitch: MinPitch, Dist: DefaultDist, Focal: 300, Cx: 100, Cy: 400},
		{Yaw: 1.3, Pitch: MaxPitch, Dist: DefaultDist, Focal: 3000, Cx: 500, Cy: 50},
		{Yaw: -2.2, Pitch: DefaultPitch, Dist: 6, Focal: 300, Cx: 236.5, Cy: 335},
		{Yaw: 0.4, Pitch: MaxPitch, Dist: DefaultDist, Focal: 800, Cx: 320, Cy: 240},
	} {
		out = append(out, k)
	}
	return out
}

// Project then UnprojectPlane at the point's own height gives the point
// back; Project then UnprojectVertical through its (x,z) gives its y.
func TestUnprojectRoundTrip(t *testing.T) {
	var pts []game.Vec3
	for _, y := range []float64{0, 0.4, 1.1, -0.3, -1.1} {
		for _, xz := range [][2]float64{{0, 0}, {0.3, -0.2}, {-0.7, 0.5}, {0.9, 0.9}, {-1.0, -0.6}} {
			pts = append(pts, game.Vec3{X: xz[0], Y: y, Z: xz[1]})
		}
	}
	for i, k := range cameras() {
		for _, p := range pts {
			pt := k.Project(p)
			got, ok := k.UnprojectPlane(pt.X, pt.Y, p.Y)
			if !ok || math.Abs(got.X-p.X) > 1e-9 || math.Abs(got.Y-p.Y) > 1e-9 || math.Abs(got.Z-p.Z) > 1e-9 {
				t.Errorf("camera %d: %v -> %v -> %v %v", i, p, pt, got, ok)
			}
			y, ok := k.UnprojectVertical(pt.Y, p.X, p.Z)
			if !ok || math.Abs(y-p.Y) > 1e-9 {
				t.Errorf("camera %d: %v vertical -> %v %v", i, p, y, ok)
			}
		}
	}
}

// Above the plane's horizon the ray never comes down to it: at MinPitch
// the plane y = 0 meets the sky a few px above Cy, so a row far above
// finds it only behind the eye.
func TestUnprojectPlaneMisses(t *testing.T) {
	k := Camera{Pitch: MinPitch, Dist: DefaultDist, Focal: DefaultFocal, Cx: 320, Cy: 240}
	if p, ok := k.UnprojectPlane(320, 240-500, 0); ok {
		t.Errorf("above the horizon: %v", p)
	}
	// The horizon row itself: the ray parallel to the plane.
	horizon := k.Cy - k.Focal*math.Tan(MinPitch)
	if p, ok := k.UnprojectPlane(320, horizon, 0); ok {
		t.Errorf("on the horizon: %v", p)
	}
}
