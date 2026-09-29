package tui

import (
	"strings"
	"unicode"

	"github.com/clipperhouse/uax29/v2/graphemes"
	gotui "github.com/grindlemire/go-tui"
)

// Pi's TruncatedText displays only the first line and uses a three-column
// ellipsis. Sanitize terminal controls before rendering untrusted queue text.
// Keep this local to pending rows; selector IDs and menu labels have their own
// existing width contract.
func pendingRowText(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if before, _, found := strings.Cut(text, "\n"); found {
		text = before
	}
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, text)
	if gotui.StringWidth(text) <= width {
		return text
	}
	ellipsis := "..."
	if width <= len(ellipsis) {
		return ellipsis[:width]
	}
	var out strings.Builder
	used := 0
	iter := graphemes.FromString(text)
	for iter.Next() {
		cluster := iter.Value()
		columns := gotui.StringWidth(cluster)
		if used+columns > width-len(ellipsis) {
			break
		}
		out.WriteString(cluster)
		used += columns
	}
	return out.String() + ellipsis
}
