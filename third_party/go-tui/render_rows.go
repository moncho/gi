package tui

// RenderRows uses Pi's fullscreen redraw granularity: compare complete rows,
// clear each changed row from column zero, then write its complete contents.
// No cursor jumps into a previously painted grapheme are needed. Call within
// the app's synchronized update window, like Render and RenderFull.
// full forces every row on startup, resize and resume without clearing scrollback.
func RenderRows(term Terminal, buf *Buffer, full bool) {
	changes := buf.RowDiff(full)
	if len(changes) > 0 {
		term.Flush(changes)
	}
	buf.Swap()
}

// RowDiff returns clear-before-paint changes for each changed row. Blank tails
// remain erased rather than written spaces, preserving terminal copy trimming.
func (b *Buffer) RowDiff(full bool) []CellChange {
	changes := make([]CellChange, 0, b.width)
	for y := 0; y < b.height; y++ {
		changed := full
		lastContent := -1
		for x := 0; x < b.width; x++ {
			idx := y*b.width + x
			cell := b.back[idx]
			if !cell.Equal(b.front[idx]) {
				changed = true
			}
			if !cell.IsEmpty() && !cell.IsContinuation() {
				lastContent = x + int(cell.Width) - 1
			}
		}
		if !changed {
			continue
		}
		// EraseToEOL at column zero is equivalent to Pi's whole-line EL.
		changes = append(changes, CellChange{X: 0, Y: y, EraseToEOL: true})
		for x := 0; x <= lastContent && x < b.width; x++ {
			changes = append(changes, CellChange{X: x, Y: y, Cell: b.back[y*b.width+x]})
		}
	}
	return changes
}
