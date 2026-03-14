package gui

import (
	"image"
	"image/color"
	"math"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

const deg2rad = math.Pi / 180

// GlobeNode represents a relay plotted on the globe.
type GlobeNode struct {
	Lat, Lon    float64
	Fingerprint string
	Nickname    string
	Country     string
	Role        string // "guard", "middle", "exit"
}

// GlobePath represents a circuit path drawn on the globe.
type GlobePath struct {
	Nodes     []GlobeNode
	CircuitID string
	Selected  bool
}

// GlobeWidget renders a 3D-projected globe with relay nodes and circuit paths.
type GlobeWidget struct {
	widget.BaseWidget

	mu       sync.Mutex
	rotation float64 // longitude offset in degrees
	tilt     float64 // latitude offset in degrees
	nodes    []GlobeNode
	paths    []GlobePath

	img         *canvas.Image
	cachedBase  *image.RGBA
	cachedRot   float64
	cachedTilt  float64
	cachedSize  fyne.Size
	lastDragPos fyne.Position

	// OnNodeTapped is called when a relay node is tapped on the globe.
	OnNodeTapped func(fingerprint string)
}

// NewGlobeWidget creates a new globe widget.
func NewGlobeWidget() *GlobeWidget {
	g := &GlobeWidget{
		rotation: -20, // default view: Atlantic
		tilt:     20,
	}
	g.img = canvas.NewImageFromImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	g.img.FillMode = canvas.ImageFillStretch
	g.img.ScaleMode = canvas.ImageScaleSmooth
	g.ExtendBaseWidget(g)
	return g
}

// SetData updates the relay nodes and circuit paths, and re-renders.
func (g *GlobeWidget) SetData(nodes []GlobeNode, paths []GlobePath) {
	g.mu.Lock()
	g.nodes = nodes
	g.paths = paths
	g.mu.Unlock()
	g.renderFrame()
}

// MinSize returns minimum size for the globe.
func (g *GlobeWidget) MinSize() fyne.Size {
	return fyne.NewSize(300, 300)
}

// CreateRenderer implements fyne.Widget.
func (g *GlobeWidget) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(g.img)
}

// Dragged implements fyne.Draggable for globe rotation.
func (g *GlobeWidget) Dragged(ev *fyne.DragEvent) {
	g.mu.Lock()
	g.rotation -= float64(ev.Dragged.DX) * 0.5
	g.tilt += float64(ev.Dragged.DY) * 0.5
	if g.tilt > 90 {
		g.tilt = 90
	}
	if g.tilt < -90 {
		g.tilt = -90
	}
	g.mu.Unlock()
	g.renderFrame()
}

// DragEnd implements fyne.Draggable.
func (g *GlobeWidget) DragEnd() {}

// Tapped implements fyne.Tappable for node hit-testing.
func (g *GlobeWidget) Tapped(ev *fyne.PointEvent) {
	if g.OnNodeTapped == nil {
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	size := g.Size()
	w, h := int(size.Width), int(size.Height)
	if w < 10 || h < 10 {
		return
	}
	r := float64(min(w, h)) / 2.2
	cx, cy := float64(w)/2, float64(h)/2
	rot := g.rotation * deg2rad
	til := g.tilt * deg2rad

	tapX, tapY := float64(ev.Position.X), float64(ev.Position.Y)

	for _, node := range g.nodes {
		px, py, vis := project(node.Lat, node.Lon, rot, til, cx, cy, r)
		if !vis {
			continue
		}
		dx := tapX - px
		dy := tapY - py
		if dx*dx+dy*dy <= 144 { // 12px hit radius
			go g.OnNodeTapped(node.Fingerprint)
			return
		}
	}
}

// renderFrame renders the globe to an image.RGBA and updates the canvas.
func (g *GlobeWidget) renderFrame() {
	g.mu.Lock()
	size := g.Size()
	rot := g.rotation
	til := g.tilt
	nodes := make([]GlobeNode, len(g.nodes))
	copy(nodes, g.nodes)
	paths := make([]GlobePath, len(g.paths))
	copy(paths, g.paths)
	g.mu.Unlock()

	w, h := int(size.Width), int(size.Height)
	if w < 10 || h < 10 {
		return
	}

	img := image.NewRGBA(image.Rect(0, 0, w, h))

	r := float64(min(w, h)) / 2.2
	cx, cy := float64(w)/2, float64(h)/2
	rotRad := rot * deg2rad
	tilRad := til * deg2rad

	// Draw ocean sphere.
	oceanColor := color.RGBA{R: 26, G: 58, B: 92, A: 255}
	spaceColor := color.RGBA{R: 15, G: 15, B: 25, A: 255}
	for y := range h {
		for x := range w {
			dx := float64(x) - cx
			dy := float64(y) - cy
			if dx*dx+dy*dy <= r*r {
				img.SetRGBA(x, y, oceanColor)
			} else {
				img.SetRGBA(x, y, spaceColor)
			}
		}
	}

	// Draw coastlines.
	landColor := color.RGBA{R: 45, G: 90, B: 61, A: 255}
	coastColor := color.RGBA{R: 80, G: 140, B: 100, A: 255}
	for _, poly := range Coastlines {
		for i := 1; i < len(poly); i++ {
			x1, y1, v1 := project(float64(poly[i-1].Lat), float64(poly[i-1].Lon), rotRad, tilRad, cx, cy, r)
			x2, y2, v2 := project(float64(poly[i].Lat), float64(poly[i].Lon), rotRad, tilRad, cx, cy, r)
			if v1 && v2 {
				drawLine(img, int(x1), int(y1), int(x2), int(y2), coastColor)
			}
		}
	}

	// Fill visible land areas with land color (simple point-on-globe check).
	_ = landColor // Land fill via coastline polygon rendering is complex;
	// we rely on coastline outlines for visual clarity.

	// Draw circuit paths.
	for _, path := range paths {
		pathColor := color.RGBA{R: 100, G: 180, B: 255, A: 200}
		lineWidth := 1
		if path.Selected {
			pathColor = color.RGBA{R: 255, G: 200, B: 50, A: 255}
			lineWidth = 2
		}
		for i := 1; i < len(path.Nodes); i++ {
			drawGreatCircleArc(img, path.Nodes[i-1], path.Nodes[i], rotRad, tilRad, cx, cy, r, pathColor, lineWidth)
		}
	}

	// Draw relay nodes.
	for _, node := range nodes {
		px, py, vis := project(node.Lat, node.Lon, rotRad, tilRad, cx, cy, r)
		if !vis {
			continue
		}
		nc := nodeColor(node.Role)
		drawDot(img, int(px), int(py), 4, nc)
		drawDot(img, int(px), int(py), 2, brighten(nc))
	}

	g.mu.Lock()
	g.img.Image = img
	g.mu.Unlock()
	g.img.Refresh()
}

// project transforms lat/lon (degrees) to screen coordinates using
// orthographic projection with the given rotation and tilt.
func project(lat, lon, rot, tilt, cx, cy, r float64) (x, y float64, visible bool) {
	latR := lat * deg2rad
	lonR := lon * deg2rad

	cosLat := math.Cos(latR)
	sinLat := math.Sin(latR)
	cosLon := math.Cos(lonR - rot)
	sinLon := math.Sin(lonR - rot)
	cosTilt := math.Cos(tilt)
	sinTilt := math.Sin(tilt)

	// Visibility check: point is on the visible hemisphere.
	vis := sinTilt*sinLat + cosTilt*cosLat*cosLon
	if vis < 0 {
		return 0, 0, false
	}

	x = cx + r*cosLat*sinLon
	y = cy - r*(cosTilt*sinLat-sinTilt*cosLat*cosLon)
	return x, y, true
}

// drawGreatCircleArc draws a great-circle arc between two nodes.
func drawGreatCircleArc(img *image.RGBA, a, b GlobeNode, rot, tilt, cx, cy, r float64, c color.RGBA, width int) {
	segments := 24
	lat1, lon1 := a.Lat*deg2rad, a.Lon*deg2rad
	lat2, lon2 := b.Lat*deg2rad, b.Lon*deg2rad

	var prevX, prevY float64
	prevVis := false

	for i := range segments + 1 {
		t := float64(i) / float64(segments)

		// Spherical linear interpolation (slerp).
		cosD := math.Sin(lat1)*math.Sin(lat2) + math.Cos(lat1)*math.Cos(lat2)*math.Cos(lon2-lon1)
		if cosD > 1 {
			cosD = 1
		}
		if cosD < -1 {
			cosD = -1
		}
		d := math.Acos(cosD)

		var lat, lon float64
		if d < 1e-6 {
			lat = lat1 + t*(lat2-lat1)
			lon = lon1 + t*(lon2-lon1)
		} else {
			sinD := math.Sin(d)
			fa := math.Sin((1-t)*d) / sinD
			fb := math.Sin(t*d) / sinD

			x := fa*math.Cos(lat1)*math.Cos(lon1) + fb*math.Cos(lat2)*math.Cos(lon2)
			y := fa*math.Cos(lat1)*math.Sin(lon1) + fb*math.Cos(lat2)*math.Sin(lon2)
			z := fa*math.Sin(lat1) + fb*math.Sin(lat2)

			lat = math.Atan2(z, math.Sqrt(x*x+y*y))
			lon = math.Atan2(y, x)
		}

		px, py, vis := project(lat/deg2rad, lon/deg2rad, rot, tilt, cx, cy, r)
		if i > 0 && vis && prevVis {
			for w := range width {
				drawLine(img, int(prevX), int(prevY)+w, int(px), int(py)+w, c)
			}
		}
		prevX, prevY, prevVis = px, py, vis
	}
}

// drawLine draws a line using Bresenham's algorithm.
func drawLine(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	bounds := img.Bounds()
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx := 1
	if x0 > x1 {
		sx = -1
	}
	sy := 1
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy

	for {
		if x0 >= bounds.Min.X && x0 < bounds.Max.X && y0 >= bounds.Min.Y && y0 < bounds.Max.Y {
			img.SetRGBA(x0, y0, c)
		}
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

// drawDot draws a filled circle at (cx, cy) with the given radius.
func drawDot(img *image.RGBA, cx, cy, radius int, c color.RGBA) {
	bounds := img.Bounds()
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			if dx*dx+dy*dy <= radius*radius {
				x, y := cx+dx, cy+dy
				if x >= bounds.Min.X && x < bounds.Max.X && y >= bounds.Min.Y && y < bounds.Max.Y {
					img.SetRGBA(x, y, c)
				}
			}
		}
	}
}

// nodeColor returns a color based on the relay role.
func nodeColor(role string) color.RGBA {
	switch role {
	case "guard":
		return color.RGBA{R: 50, G: 200, B: 50, A: 255}
	case "exit":
		return color.RGBA{R: 220, G: 50, B: 50, A: 255}
	default: // "middle" or unknown
		return color.RGBA{R: 200, G: 150, B: 50, A: 255}
	}
}

// brighten returns a lighter version of a color.
func brighten(c color.RGBA) color.RGBA {
	return color.RGBA{
		R: clampU8(int(c.R) + 80),
		G: clampU8(int(c.G) + 80),
		B: clampU8(int(c.B) + 80),
		A: c.A,
	}
}

func clampU8(v int) uint8 {
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
