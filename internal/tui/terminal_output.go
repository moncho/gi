package tui

import "strings"

// plainTerminalOutput removes terminal controls from untrusted subprocess output
// before it enters a transcript. The TUI owns styling; forwarding an embedded
// SGR, OSC (including hyperlinks), or cursor command can corrupt later rows.
// Preserve whitespace and line breaks so source code remains readable.
func plainTerminalOutput(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for i := 0; i < len(text); {
		ch := text[i]
		if ch == 0x1b || ch == 0x9b || ch == 0x9d || ch == 0x90 || ch == 0x9f || ch == 0x9e || (ch == 0xc2 && i+1 < len(text) && text[i+1] >= 0x90 && text[i+1] <= 0x9f) {
			control := ch
			if ch == 0xc2 {
				control = text[i+1]
				i += 2
			} else {
				i++
				if ch == 0x1b && i < len(text) {
					control = text[i]
					i++
					switch control {
					case '[':
						control = 0x9b
					case ']':
						control = 0x9d
					case 'P':
						control = 0x90
					case '_':
						control = 0x9f
					case '^':
						control = 0x9e
					}
				}
			}
			switch control {
			case 0x9b: // CSI: parameters/intermediates followed by a final byte.
				for i < len(text) && text[i] >= 0x20 && text[i] <= 0x3f {
					i++
				}
				if i < len(text) && text[i] >= 0x40 && text[i] <= 0x7e {
					i++
				}
			case 0x9d, 0x90, 0x9f, 0x9e: // OSC/DCS/APC/PM: BEL or ST.
				for i < len(text) {
					if text[i] == 0x07 {
						i++
						break
					}
					if text[i] == 0x1b && i+1 < len(text) && text[i+1] == '\\' {
						i += 2
						break
					}
					if text[i] == 0xc2 && i+1 < len(text) && text[i+1] == 0x9c {
						i += 2
						break
					}
					// An unterminated escape must not eat the rest of the transcript.
					if text[i] == '\n' {
						break
					}
					i++
				}
			}
			continue
		}
		if ch == '\n' || ch == '\r' || ch == '\t' || ch >= 0x20 && ch != 0x7f {
			b.WriteByte(ch)
		}
		i++
	}
	return b.String()
}
