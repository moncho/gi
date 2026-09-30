package tui

import "time"

// wheelAccelerator ports pi-tui's WheelScrollAccelerator. In auto mode an
// isolated notch moves one line and a fast spin up to six lines per event:
// notches 100 ms apart move 1 line, 50 ms apart 2, 20 ms apart 5. Bursts
// closer than 5 ms (one physical notch split by the terminal, or a
// high-resolution device) move one line each without accelerating.
type wheelAccelerator struct {
	lines      int // fixed lines per event; 0 = auto
	last       time.Time
	direction  int
	averageGap float64 // ms; 0 = unset
	carry      float64
}

const (
	wheelBurstGapMS     = 5
	wheelGestureGapMS   = 200
	wheelReferenceGapMS = 100
	wheelMaxAutoLines   = 6
)

func (w *wheelAccelerator) next(direction int, now time.Time) int {
	if w.lines > 0 {
		return w.lines
	}
	gap := float64(now.Sub(w.last).Milliseconds())
	same := !w.last.IsZero() && direction == w.direction && gap <= wheelGestureGapMS
	w.last, w.direction = now, direction
	if !same {
		w.averageGap, w.carry = 0, 0
		return 1
	}
	if gap < wheelBurstGapMS {
		return 1
	}
	if w.averageGap == 0 {
		w.averageGap = gap
	} else {
		w.averageGap = (w.averageGap + gap) / 2
	}
	lines := wheelReferenceGapMS / w.averageGap
	if lines < 1 {
		lines = 1
	}
	if lines > wheelMaxAutoLines {
		lines = wheelMaxAutoLines
	}
	lines += w.carry
	whole := int(lines)
	w.carry = lines - float64(whole)
	return whole
}
