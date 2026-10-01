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

func TestTerminalWheelAccelerationMatchesPi(t *testing.T) {
	for _, platform := range []string{"darwin", "linux", "windows"} {
		for _, ssh := range []string{"", "SSH_CONNECTION", "SSH_CLIENT", "SSH_TTY"} {
			lookup := func(name string) (string, bool) { return "", name == ssh }
			want := platform == "darwin" && ssh == ""
			if got := terminalAcceleratesWheel(platform, lookup); got != want {
				t.Fatalf("%s %s: got %v want %v", platform, ssh, got, want)
			}
		}
	}
	base := time.Unix(1000, 0)
	w := wheelAccelerator{terminalAccelerated: true}
	for i := 0; i < 10; i++ {
		if got := w.next(1, base.Add(time.Duration(i)*20*time.Millisecond)); got != 1 {
			t.Fatalf("double acceleration: %d", got)
		}
	}
	w.lines = 3
	if got := w.next(1, base); got != 3 {
		t.Fatalf("fixed override: %d", got)
	}
}

func TestWheelSettingChangeResetsGesture(t *testing.T) {
	base := time.Unix(1000, 0)
	w := wheelAccelerator{}
	w.next(1, base)
	w.next(1, base.Add(20*time.Millisecond))
	w.configure(3)
	if !w.last.IsZero() || w.carry != 0 || w.averageGap != 0 {
		t.Fatal("setting kept gesture")
	}
	w.configure(0)
	if got := w.next(1, base.Add(40*time.Millisecond)); got != 1 {
		t.Fatalf("new gesture: %d", got)
	}
}
