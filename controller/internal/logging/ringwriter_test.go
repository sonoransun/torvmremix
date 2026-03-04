package logging

import (
	"fmt"
	"sync"
	"testing"
)

func TestRingWriterBasic(t *testing.T) {
	rw := NewRingWriter(5)
	fmt.Fprint(rw, "line1\nline2\nline3\n")

	lines := rw.Lines()
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "line1" || lines[1] != "line2" || lines[2] != "line3" {
		t.Errorf("unexpected lines: %v", lines)
	}
}

func TestRingWriterWrapping(t *testing.T) {
	rw := NewRingWriter(3)
	for i := 1; i <= 5; i++ {
		fmt.Fprintf(rw, "line%d\n", i)
	}

	lines := rw.Lines()
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines after wrapping, got %d: %v", len(lines), lines)
	}
	// Should contain the last 3 lines in chronological order.
	if lines[0] != "line3" || lines[1] != "line4" || lines[2] != "line5" {
		t.Errorf("expected [line3 line4 line5], got %v", lines)
	}
}

func TestRingWriterChronologicalOrder(t *testing.T) {
	rw := NewRingWriter(4)
	for i := 1; i <= 10; i++ {
		fmt.Fprintf(rw, "msg%02d\n", i)
	}

	lines := rw.Lines()
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}
	// Last 4 lines should be msg07..msg10 in chronological order.
	expected := []string{"msg07", "msg08", "msg09", "msg10"}
	for i, want := range expected {
		if lines[i] != want {
			t.Errorf("lines[%d] = %q, want %q (full: %v)", i, lines[i], want, lines)
			break
		}
	}
}

func TestRingWriterPartialLines(t *testing.T) {
	rw := NewRingWriter(5)
	// Write partial data (no newline).
	fmt.Fprint(rw, "partial")
	lines := rw.Lines()
	if len(lines) != 0 {
		t.Errorf("expected 0 lines for partial write, got %d: %v", len(lines), lines)
	}

	// Complete the line.
	fmt.Fprint(rw, " complete\n")
	lines = rw.Lines()
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0] != "partial complete" {
		t.Errorf("expected 'partial complete', got %q", lines[0])
	}
}

func TestRingWriterOnLineCallback(t *testing.T) {
	rw := NewRingWriter(5)
	var received []string
	rw.OnLine(func(line string) {
		received = append(received, line)
	})

	fmt.Fprint(rw, "first\nsecond\n")

	if len(received) != 2 {
		t.Fatalf("expected 2 callbacks, got %d", len(received))
	}
	if received[0] != "first" || received[1] != "second" {
		t.Errorf("unexpected callback lines: %v", received)
	}
}

func TestRingWriterOnLineCallbackNotForPartial(t *testing.T) {
	rw := NewRingWriter(5)
	callCount := 0
	rw.OnLine(func(string) {
		callCount++
	})

	fmt.Fprint(rw, "no newline")
	if callCount != 0 {
		t.Error("callback should not fire for partial line")
	}

	fmt.Fprint(rw, "\n")
	if callCount != 1 {
		t.Errorf("expected 1 callback after completing line, got %d", callCount)
	}
}

func TestRingWriterConcurrentWrites(t *testing.T) {
	rw := NewRingWriter(100)
	var wg sync.WaitGroup
	n := 20

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				fmt.Fprintf(rw, "goroutine%d-line%d\n", id, j)
			}
		}(i)
	}
	wg.Wait()

	lines := rw.Lines()
	if len(lines) != 100 {
		// Ring has capacity 100, we wrote 200 lines total, so should be full.
		t.Errorf("expected 100 lines, got %d", len(lines))
	}
}

func TestRingWriterEmpty(t *testing.T) {
	rw := NewRingWriter(5)
	lines := rw.Lines()
	if len(lines) != 0 {
		t.Errorf("expected 0 lines for empty ring, got %d", len(lines))
	}
}

func TestRingWriterWriteReturnValue(t *testing.T) {
	rw := NewRingWriter(5)
	data := []byte("hello\nworld\n")
	n, err := rw.Write(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(data) {
		t.Errorf("expected n=%d, got %d", len(data), n)
	}
}
