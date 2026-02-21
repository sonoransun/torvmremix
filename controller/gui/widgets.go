package gui

import (
	"image/color"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"

	"github.com/user/extorvm/controller/internal/lifecycle"
	"github.com/user/extorvm/controller/internal/logging"
)

// StatusLight is a colored circle widget that reflects lifecycle state.
type StatusLight struct {
	widget.BaseWidget
	mu    sync.Mutex
	state lifecycle.State
	dot   *canvas.Circle
}

// NewStatusLight creates a StatusLight starting in Init state.
func NewStatusLight() *StatusLight {
	s := &StatusLight{}
	s.dot = canvas.NewCircle(colorForState(lifecycle.StateInit))
	s.dot.StrokeWidth = 1
	s.dot.StrokeColor = color.Black
	s.ExtendBaseWidget(s)
	return s
}

// SetState updates the displayed state color.
func (s *StatusLight) SetState(st lifecycle.State) {
	s.mu.Lock()
	s.state = st
	s.mu.Unlock()
	s.dot.FillColor = colorForState(st)
	s.dot.Refresh()
}

// MinSize returns the minimum size for the status light.
func (s *StatusLight) MinSize() fyne.Size {
	return fyne.NewSize(24, 24)
}

// CreateRenderer implements fyne.Widget.
func (s *StatusLight) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(s.dot)
}

func colorForState(st lifecycle.State) color.Color {
	switch st {
	case lifecycle.StateRunning:
		return color.NRGBA{R: 0, G: 200, B: 0, A: 255}
	case lifecycle.StateFailed:
		return color.NRGBA{R: 220, G: 0, B: 0, A: 255}
	case lifecycle.StateInit, lifecycle.StateCleanup, lifecycle.StateShutdown,
		lifecycle.StateRestoreNetwork:
		return color.NRGBA{R: 160, G: 160, B: 160, A: 255}
	default:
		// Starting / transitional states → yellow.
		return color.NRGBA{R: 230, G: 200, B: 0, A: 255}
	}
}

// LogView wraps a Fyne List widget to efficiently display log lines
// from a RingWriter.
type LogView struct {
	widget.BaseWidget
	ring     *logging.RingWriter
	mu       sync.Mutex
	snapshot []string
	list     *widget.List
}

// NewLogView creates a LogView backed by the given RingWriter.
func NewLogView(ring *logging.RingWriter) *LogView {
	lv := &LogView{ring: ring, snapshot: ring.Lines()}
	lv.list = widget.NewList(
		func() int {
			lv.mu.Lock()
			defer lv.mu.Unlock()
			return len(lv.snapshot)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			lv.mu.Lock()
			defer lv.mu.Unlock()
			if id < len(lv.snapshot) {
				obj.(*widget.Label).SetText(lv.snapshot[id])
			}
		},
	)
	lv.ExtendBaseWidget(lv)
	return lv
}

// Refresh reloads lines from the ring buffer and updates the list.
func (lv *LogView) Refresh() {
	lv.mu.Lock()
	lv.snapshot = lv.ring.Lines()
	n := len(lv.snapshot)
	lv.mu.Unlock()
	lv.list.Refresh()
	if n > 0 {
		lv.list.ScrollToBottom()
	}
}

// CreateRenderer implements fyne.Widget.
func (lv *LogView) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(lv.list)
}

// MinSize returns a reasonable minimum for the log view.
func (lv *LogView) MinSize() fyne.Size {
	return fyne.NewSize(600, 300)
}
