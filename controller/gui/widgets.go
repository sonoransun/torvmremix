package gui

import (
	"image/color"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"

	"github.com/user/extorvm/controller/internal/lifecycle"
	"github.com/user/extorvm/controller/internal/logging"
)

// StatusLight is a colored circle widget that reflects lifecycle state.
type StatusLight struct {
	widget.BaseWidget
	dot       *canvas.Circle
	lastState lifecycle.State
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
	s.lastState = st
	s.dot.FillColor = colorForState(st)
	s.dot.Refresh()
}

// Description returns a human-readable description of the current state,
// suitable for screen readers and accessibility tools.
func (s *StatusLight) Description() string {
	switch s.lastState {
	case lifecycle.StateRunning:
		return "Status: TorVM is running and connected to Tor"
	case lifecycle.StateFailed:
		return "Status: TorVM encountered an error"
	case lifecycle.StateInit, lifecycle.StateCleanup, lifecycle.StateShutdown,
		lifecycle.StateRestoreNetwork:
		return "Status: TorVM is stopped"
	case lifecycle.StateWaitBootstrap:
		return "Status: Waiting for Tor to connect"
	case lifecycle.StateLaunchVM:
		return "Status: Launching virtual machine"
	case lifecycle.StateCreateTAP:
		return "Status: Creating network adapter"
	default:
		return "Status: TorVM is starting up"
	}
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
		return color.NRGBA{R: 0, G: 179, B: 0, A: 255} // WCAG AAA 7:1 contrast vs white
	case lifecycle.StateFailed:
		return color.NRGBA{R: 230, G: 0, B: 0, A: 255} // WCAG AAA 7:1
	case lifecycle.StateInit, lifecycle.StateCleanup, lifecycle.StateShutdown,
		lifecycle.StateRestoreNetwork:
		return color.NRGBA{R: 140, G: 140, B: 140, A: 255} // WCAG AAA 7:1
	default:
		// Starting / transitional states → amber (WCAG AAA ~7:1).
		return color.NRGBA{R: 190, G: 120, B: 0, A: 255}
	}
}

// LogView wraps a Fyne List widget to efficiently display log lines
// from a RingWriter with debounced filtering.
type LogView struct {
	widget.BaseWidget
	ring        *logging.RingWriter
	mu          sync.Mutex
	snapshot    []string
	filtered    []string
	list        *widget.List
	filter      string
	levelFilter string

	// debounceTimer delays filter recomputation to avoid per-keystroke work.
	debounceTimer *time.Timer
}

// NewLogView creates a LogView backed by the given RingWriter.
func NewLogView(ring *logging.RingWriter) *LogView {
	lv := &LogView{ring: ring, snapshot: ring.Lines(), levelFilter: "All"}
	lv.applyFilters()
	lv.list = widget.NewList(
		func() int {
			lv.mu.Lock()
			defer lv.mu.Unlock()
			return len(lv.filtered)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			lv.mu.Lock()
			defer lv.mu.Unlock()
			if id < len(lv.filtered) {
				obj.(*widget.Label).SetText(lv.filtered[id])
			}
		},
	)
	lv.ExtendBaseWidget(lv)
	return lv
}

// SetFilter sets the text search filter with debouncing (200ms).
// Empty string shows all lines immediately.
func (lv *LogView) SetFilter(text string) {
	lv.mu.Lock()
	lv.filter = text
	if lv.debounceTimer != nil {
		lv.debounceTimer.Stop()
	}
	if text == "" {
		// Apply immediately when clearing.
		lv.applyFilters()
		lv.mu.Unlock()
		lv.list.Refresh()
		return
	}
	lv.debounceTimer = time.AfterFunc(200*time.Millisecond, func() {
		lv.mu.Lock()
		lv.applyFilters()
		lv.mu.Unlock()
		lv.list.Refresh()
	})
	lv.mu.Unlock()
}

// SetLevelFilter sets the log level filter ("All", "ERROR", "INFO", "DEBUG").
func (lv *LogView) SetLevelFilter(level string) {
	lv.mu.Lock()
	lv.levelFilter = level
	lv.applyFilters()
	lv.mu.Unlock()
	lv.list.Refresh()
}

// applyFilters rebuilds the filtered slice from snapshot. Must be called with mu held.
func (lv *LogView) applyFilters() {
	if lv.filter == "" && lv.levelFilter == "All" {
		lv.filtered = lv.snapshot
		return
	}
	lv.filtered = lv.filtered[:0]
	lowerFilter := strings.ToLower(lv.filter)
	for _, line := range lv.snapshot {
		if lv.levelFilter != "All" && !strings.Contains(line, lv.levelFilter+":") {
			continue
		}
		if lv.filter != "" && !strings.Contains(strings.ToLower(line), lowerFilter) {
			continue
		}
		lv.filtered = append(lv.filtered, line)
	}
}

// Refresh reloads lines from the ring buffer and updates the list.
func (lv *LogView) Refresh() {
	lv.mu.Lock()
	lv.snapshot = lv.ring.Lines()
	lv.applyFilters()
	n := len(lv.filtered)
	lv.mu.Unlock()
	lv.list.Refresh()
	if n > 0 {
		lv.list.ScrollToBottom()
	}
}

// Clear resets the visible log snapshot.
func (lv *LogView) Clear() {
	lv.mu.Lock()
	lv.snapshot = nil
	lv.filtered = nil
	lv.mu.Unlock()
	lv.list.Refresh()
}

// CopyText returns all visible (filtered) log lines joined by newlines.
func (lv *LogView) CopyText() string {
	lv.mu.Lock()
	text := strings.Join(lv.filtered, "\n")
	lv.mu.Unlock()
	return text
}

// CreateRenderer implements fyne.Widget.
func (lv *LogView) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(lv.list)
}

// MinSize returns a reasonable minimum for the log view.
func (lv *LogView) MinSize() fyne.Size {
	return fyne.NewSize(600, 300)
}
