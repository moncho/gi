package tui

import (
	"fmt"
	"testing"

	gotui "github.com/grindlemire/go-tui"
)

func TestTranscriptScrollbarMouse(t *testing.T) {
	for _, size := range [][2]int{{30, 6}, {60, 10}, {100, 20}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			c := &chatTUI{transcriptRef: gotui.NewRef(), stickToBottom: true}
			root := gotui.New(gotui.WithDirection(gotui.Column), gotui.WithWidth(size[0]), gotui.WithHeight(size[1]), gotui.WithScrollable(gotui.ScrollVertical))
			for i := 0; i < 100; i++ {
				root.AddChild(gotui.New(gotui.WithText(fmt.Sprint(i)), gotui.WithHeight(1)))
			}
			root.Render(gotui.NewBuffer(size[0], size[1]), size[0], size[1])
			c.transcriptRegion = root
			c.transcriptRef.Set(root)
			r := root.ContentRect()
			x := r.X + r.Width - 1
			_, maxScroll := root.MaxScroll()
			send := func(action gotui.MouseAction, x, y int) bool {
				return c.HandleMouse(gotui.MouseEvent{Button: gotui.MouseLeft, Action: action, X: x, Y: y})
			}
			if c.handleTranscriptScrollbar(gotui.MouseEvent{Button: gotui.MouseLeft, Action: gotui.MousePress, X: x - 1, Y: r.Y}) || c.scrollbarDragging {
				t.Fatal("content cell treated as scrollbar")
			}
			if c.handleTranscriptScrollbar(gotui.MouseEvent{Button: gotui.MouseLeft, Action: gotui.MousePress, X: x, Y: r.Y - 1}) || c.handleTranscriptScrollbar(gotui.MouseEvent{Button: gotui.MouseLeft, Action: gotui.MousePress, X: x, Y: r.Y + r.Height}) {
				t.Fatal("outside gutter consumed")
			}
			// A track click jumps, and dragging keeps working beyond both edges.
			if !send(gotui.MousePress, x, r.Y+r.Height/2) || !c.scrollbarDragging || c.transcriptScroll <= 0 || c.transcriptScroll >= maxScroll || c.stickToBottom {
				t.Fatal("track click did not jump", c.transcriptScroll)
			}
			if !send(gotui.MouseDrag, x+5, r.Y-20) || c.transcriptScroll != 0 {
				t.Fatal("drag above top", c.transcriptScroll)
			}
			if !send(gotui.MouseDrag, x+5, r.Y+r.Height+20) || c.transcriptScroll != maxScroll || !c.stickToBottom {
				t.Fatal("drag below bottom", c.transcriptScroll)
			}
			if !send(gotui.MouseRelease, x+5, r.Y+r.Height+20) || c.scrollbarDragging {
				t.Fatal("release not handled")
			}
			if send(gotui.MouseDrag, x, r.Y) || c.transcriptScroll != maxScroll {
				t.Fatal("drag continued after release")
			}
			// Grabbing the thumb does not jump until it moves.
			if !send(gotui.MousePress, x, r.Y+r.Height-1) || c.transcriptScroll != maxScroll {
				t.Fatal("thumb press jumped")
			}
			send(gotui.MouseDrag, x, r.Y)
			send(gotui.MouseRelease, x, r.Y)
			if c.transcriptScroll != 0 || c.stickToBottom {
				t.Fatal("thumb did not reach top")
			}
		})
	}
}

func TestTranscriptScrollbarWithoutOverflow(t *testing.T) {
	c := &chatTUI{transcriptRef: gotui.NewRef()}
	root := gotui.New(gotui.WithWidth(30), gotui.WithHeight(10), gotui.WithScrollable(gotui.ScrollVertical))
	root.AddChild(gotui.New(gotui.WithText("short")))
	root.Render(gotui.NewBuffer(30, 10), 30, 10)
	c.transcriptRegion = root
	c.transcriptRef.Set(root)
	if c.handleTranscriptScrollbar(gotui.MouseEvent{Button: gotui.MouseLeft, Action: gotui.MousePress, X: 29, Y: 2}) || c.scrollbarDragging {
		t.Fatal("scrollbar with no overflow")
	}
}
