package tui

import (
	"testing"
	"time"
)

func TestWheelAcceleratorMatchesPi(t *testing.T) {
	base := time.Unix(1000, 0)
	at := func(ms int) time.Time { return base.Add(time.Duration(ms) * time.Millisecond) }
	var w wheelAccelerator
	if got := w.next(1, at(0)); got != 1 {
		t.Fatalf("first notch %d", got)
	}
	// 100 ms apart: 1 line each.
	if got := w.next(1, at(100)); got != 1 {
		t.Fatalf("100ms %d", got)
	}
	w = wheelAccelerator{}
	w.next(1, at(0))
	total := 0
	for i := 1; i <= 5; i++ {
		total += w.next(1, at(i*20))
	}
	if total != 25 { // 20 ms apart: 5 lines per event
		t.Fatalf("fast spin moved %d", total)
	}
	if got := w.next(1, at(102)); got != 1 { // burst < 5 ms
		t.Fatalf("burst %d", got)
	}
	if got := w.next(-1, at(150)); got != 1 { // direction change starts a new gesture
		t.Fatalf("reverse %d", got)
	}
	if got := w.next(-1, at(500)); got != 1 { // gesture gap
		t.Fatalf("pause %d", got)
	}
	fixed := wheelAccelerator{lines: 3}
	if fixed.next(1, at(0)) != 3 || fixed.next(1, at(1)) != 3 {
		t.Fatal("fixed lines")
	}
}
