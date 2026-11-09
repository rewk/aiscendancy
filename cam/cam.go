// Package cam is the orbit camera of the 3D views (the system display,
// where combat is fought too): bodies are sprites at projected 3D
// positions, in a pinhole projection.
//
// The world is right-handed with Y up and X,Z in the ecliptic plane. A
// Camera orbits the origin. Project first turns the point by Yaw about Y:
//
//	x1 = x cos Yaw − z sin Yaw
//	z1 = x sin Yaw + z cos Yaw
//
// then, in that turned frame, looks from the eye at Dist·(0, sin Pitch,
// cos Pitch) toward the origin. The eye's basis is right = (1,0,0), up =
// (0, cos Pitch, −sin Pitch), forward = (0, −sin Pitch, −cos Pitch), so
// the camera coordinates are dot products with (point − eye):
//
//	camx  = x1
//	camy  = y cos Pitch − z1 sin Pitch
//	Depth = Dist − y sin Pitch − z1 cos Pitch
//
// and the perspective divide gives Scale = Focal/Depth (px per world unit
// at that depth), X = Cx + Scale·camx, Y = Cy − Scale·camy (screen Y
// down). At Yaw 0 the eye looks down −Z, so +Z is the near side: lower
// on screen and bigger. Segments project to segments, so lines need only
// their endpoints; points at or behind the eye (Depth <= 0) are the
// caller's problem, avoided by keeping Dist well past the scene's radius.
package cam

import (
	"math"

	"github.com/rewk/aiscendancy/game"
)

// Camera orbits the origin. Yaw turns the world about its Y axis, Pitch is
// the eye's elevation above the plane (radians), Dist the eye-to-origin
// distance in world units, Focal the pixels a world unit spans at depth 1;
// (Cx,Cy) is where the origin lands on screen.
type Camera struct{ Yaw, Pitch, Dist, Focal, Cx, Cy float64 }

// Point is a projected position: screen X,Y, Scale = pixels per world unit
// at that depth (Focal/Depth), Depth the distance along the view axis
// (bigger = farther, sort descending to paint back to front).
type Point struct{ X, Y, Scale, Depth float64 }

// The system display's view, matched to the original: from 25° above the
// plane, 4 radii away, Focal 1600 makes the unit disc span 1600/4 = 400 px
// either side of the origin, so lanes at radius 1.15 fit the 918 px wide area
// (the views' 1280x960 native px; the original's disc is ~440 px wide in
// its 640x480, ~880 here). The views clamp Pitch and Focal to the limits below.
const (
	DefaultPitch = 25 * math.Pi / 180
	DefaultDist  = 4
	DefaultFocal = 1600

	MinPitch, MaxPitch = 5 * math.Pi / 180, 85 * math.Pi / 180
	MinFocal, MaxFocal = 600, 6000
)

// Default is the default view with the origin at (cx,cy) on screen.
func Default(cx, cy float64) Camera {
	return Camera{Pitch: DefaultPitch, Dist: DefaultDist, Focal: DefaultFocal, Cx: cx, Cy: cy}
}

// Clamp holds Pitch and Focal within their limits.
func (c *Camera) Clamp() {
	c.Pitch = min(max(c.Pitch, MinPitch), MaxPitch)
	c.Focal = min(max(c.Focal, MinFocal), MaxFocal)
}

// Project maps a world point to the screen (see the package comment).
func (c *Camera) Project(p game.Vec3) Point {
	sy, cy := math.Sincos(c.Yaw)
	x1 := p.X*cy - p.Z*sy
	z1 := p.X*sy + p.Z*cy
	sp, cp := math.Sincos(c.Pitch)
	camy := p.Y*cp - z1*sp
	depth := c.Dist - p.Y*sp - z1*cp
	s := c.Focal / depth
	return Point{X: c.Cx + s*x1, Y: c.Cy - s*camy, Scale: s, Depth: depth}
}

// UnprojectPlane is the world point of screen (sx,sy) on the horizontal
// plane y = y0: Project solved backwards. Sy = Cy − Focal·camy/Depth gives
// (Cy−sy)·Depth = Focal·camy, linear in z1 once y = y0:
//
//	z1 = [(Cy−sy)(Dist − y0 sp) − Focal y0 cp] / [(Cy−sy) cp − Focal sp]
//
// then Depth from z1, x1 = (sx−Cx)·Depth/Focal, and the Yaw turned back.
// ok is false when the ray runs parallel to the plane or meets it behind
// the eye.
func (c *Camera) UnprojectPlane(sx, sy, y0 float64) (game.Vec3, bool) {
	sp, cp := math.Sincos(c.Pitch)
	h := c.Cy - sy
	den := h*cp - c.Focal*sp
	if math.Abs(den) < 1e-9 {
		return game.Vec3{}, false
	}
	z1 := (h*(c.Dist-y0*sp) - c.Focal*y0*cp) / den
	depth := c.Dist - y0*sp - z1*cp
	if depth <= 1e-9 {
		return game.Vec3{}, false
	}
	x1 := (sx - c.Cx) * depth / c.Focal
	sy0, cy0 := math.Sincos(c.Yaw)
	return game.Vec3{X: x1*cy0 + z1*sy0, Y: y0, Z: -x1*sy0 + z1*cy0}, true
}

// UnprojectVertical is the height y on the vertical line through (x,·,z)
// whose projection lands on screen row sy. z1 comes from (x,z) as in
// Project, and (Cy−sy)·Depth = Focal·camy is linear in y:
//
//	y = [(Cy−sy)(Dist − z1 cp) + Focal z1 sp] / [Focal cp + (Cy−sy) sp]
//
// ok is false when the line projects to a single row (the denominator ~0)
// or the solution lies behind the eye.
func (c *Camera) UnprojectVertical(sy, x, z float64) (float64, bool) {
	s, co := math.Sincos(c.Yaw)
	z1 := x*s + z*co
	sp, cp := math.Sincos(c.Pitch)
	h := c.Cy - sy
	den := c.Focal*cp + h*sp
	if math.Abs(den) < 1e-9 {
		return 0, false
	}
	y := (h*(c.Dist-z1*cp) + c.Focal*z1*sp) / den
	if c.Dist-y*sp-z1*cp <= 1e-9 {
		return 0, false
	}
	return y, true
}
