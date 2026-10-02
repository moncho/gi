package tui

import (
	"math/rand"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// Frozen pre-optimization terminal-width profile, not an external Unicode spec.
func referenceInRanges(r rune, ranges []runeRange) bool {
	for _, rr := range ranges {
		if r >= rr.min && r <= rr.max {
			return true
		}
	}
	return false
}
func referenceBaseRuneWidth(r rune) int {
	if r < 0 || r > unicode.MaxRune || r < 0x20 || r >= 0x7f && r < 0xa0 {
		return 1
	}
	if referenceInRanges(r, eastAsianWideRanges) || referenceInRanges(r, emojiWideRanges) {
		return 2
	}
	return 1
}
func referenceGraphemeExtend(r rune) bool {
	return r >= 0x1f3fb && r <= 0x1f3ff || unicode.In(r, unicode.Mn, unicode.Me, unicode.Mc, unicode.Join_Control, unicode.Variation_Selector)
}
func referenceExtendWidth(r, base rune, w int) int {
	if r == 0xfe0f {
		return 2
	}
	if r == 0xfe0e && referenceBaseRuneWidth(base) == 2 && referenceInRanges(base, emojiWideRanges) {
		return 1
	}
	return w
}
func referenceNextCluster(s string) (cluster string, width, size int) {
	if len(s) == 0 {
		return "", 0, 0
	}

	// ASCII fast path: only when the next byte is also ASCII (or we are at the
	// end). A non-ASCII next byte (leading byte or continuation byte >= 0x80)
	// forces the full clusterAdvance path. For a bare ASCII byte followed by a
	// raw continuation byte (malformed UTF-8), clusterAdvance correctly returns
	// just the ASCII byte — the continuation byte starts its own cluster.
	// Examples that skip the fast path:
	//   'a' + 0xCC (leading byte of a combining mark) — attaches via graphemeExtend
	//   'a' + 0x80 (raw continuation byte, malformed) — breaks, 'a' is its own cluster
	if s[0] < 0x80 {
		if len(s) == 1 || s[1] < 0x80 {
			return s[:1], 1, 1
		}
	}

	w, size, _ := referenceClusterAdvance(decodeStringStep(s))
	return s[:size], w, size
}

func referenceClusterAdvance(decode decodeStep) (width, size int, base rune) {
	r0, s0, length := decode(0)
	if s0 == 0 {
		return 0, 0, 0
	}
	pos := s0
	w := referenceBaseRuneWidth(r0)

	if regionalIndicator(r0) {
		if pos < length {
			r1, s1, _ := decode(pos)
			if s1 > 0 && regionalIndicator(r1) {
				pos += s1
				return 2, pos, r0
			}
		}
		// Lone RI: w is already 2 (RI is in emojiWideRanges). Fall through so a
		// trailing combining mark or ZWJ sequence still attaches.
	}

	lastWasZWJ := false
	for pos < length {
		r, sz, _ := decode(pos)
		if sz == 0 {
			break
		}
		if referenceGraphemeExtend(r) {
			pos += sz
			w = referenceExtendWidth(r, r0, w)
			lastWasZWJ = isZWJ(r)
			continue
		}
		if lastWasZWJ {
			// Emoji (or any base) after a ZWJ joins the cluster.
			pos += sz
			w = 2
			lastWasZWJ = false
			continue
		}
		break
	}

	return w, pos, r0
}

func TestUnicodeFastPredicatesExhaustive(t *testing.T) {
	for _, ranges := range [][]runeRange{eastAsianWideRanges, emojiWideRanges} {
		for i, rr := range ranges {
			if rr.min > rr.max || i > 0 && rr.min <= ranges[i-1].max {
				t.Fatalf("unsorted ranges: %v", ranges)
			}
		}
	}
	for r := rune(-1); r <= unicode.MaxRune+1; r++ {
		if got, want := baseRuneWidth(r), referenceBaseRuneWidth(r); got != want {
			t.Fatalf("base U+%X: %d != %d", r, got, want)
		}
		if got, want := graphemeExtend(r), referenceGraphemeExtend(r); got != want {
			t.Fatalf("extend U+%X: %v != %v", r, got, want)
		}
		zero := unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf, unicode.Variation_Selector, unicode.Join_Control)
		if isZeroWidthRune(r) != zero {
			t.Fatalf("zero U+%X", r)
		}
		want := referenceBaseRuneWidth(r)
		if zero {
			want = 1
		}
		if RuneWidth(r) != want {
			t.Fatalf("rune width U+%X", r)
		}
	}
}
func TestUnicodeFastClustersDifferential(t *testing.T) {
	inputs := []string{"", "ASCII", "é", "👩🏽‍💻", "🇵🇹🇯🇵🇺", "日本語 / العربية", "✈︎ ✈️ ☺︎ ☺️", "1️⃣", "a\u200cb", "a\u200bb", "\x80a\xff", "\r\n"}
	random := rand.New(rand.NewSource(42))
	alphabet := []rune("abc │┼ 日本語العربيةé👩🏽‍💻🇵🇹✈︎✈️\u200b\u200c\u20e3")
	for i := 0; i < 2000; i++ {
		var s strings.Builder
		for j := 0; j < 32; j++ {
			s.WriteRune(alphabet[random.Intn(len(alphabet))])
		}
		inputs = append(inputs, s.String())
	}
	for i := 0; i < 2000; i++ {
		s := make([]byte, 32)
		random.Read(s)
		inputs = append(inputs, string(s))
	}
	for _, s := range inputs {
		total := 0
		for rest := s; rest != ""; {
			cl, w, n := nextCluster(rest)
			rc, rw, rn := referenceNextCluster(rest)
			if cl != rc || w != rw || n != rn || n <= 0 {
				t.Fatalf("%q: %q/%d/%d != %q/%d/%d", rest, cl, w, n, rc, rw, rn)
			}
			bw, bn, base := nextClusterBytes([]byte(rest))
			r, _ := utf8.DecodeRuneInString(rest)
			if bw != w || bn != n || base != r {
				t.Fatalf("byte path differs: %q", rest)
			}
			total += w
			rest = rest[n:]
		}
		if StringWidth(s) != total {
			t.Fatalf("string width differs: %q", s)
		}
	}
}

func referenceStringWidth(s string) int {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 || s[i] == '\r' && i+1 < len(s) && s[i+1] == '\n' {
			ascii = false
			break
		}
	}
	if ascii {
		return len(s)
	}
	w := 0
	for rest := s; rest != ""; {
		_, cw, n := referenceNextCluster(rest)
		w += cw
		rest = rest[n:]
	}
	return w
}

var unicodeBenchWidth int

func BenchmarkUnicodeWidths(b *testing.B) {
	for name, text := range map[string]string{"ascii": strings.Repeat(" table padding words ", 6), "borders": strings.Repeat("│──┼──", 20), "mixed": strings.Repeat("│ 日本語 ⚠️ 🇵🇹 👩🏽‍💻 é العربية text │", 4)} {
		for _, reference := range []bool{true, false} {
			label := "optimized"
			if reference {
				label = "reference"
			}
			b.Run(name+"/"+label, func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					w := 0
					if reference {
						w = referenceStringWidth(text)
					} else {
						w = StringWidth(text)
					}
					unicodeBenchWidth = w
				}
			})
		}
	}
}

func TestTextMeasurementCacheInvalidation(t *testing.T) {
	e := New(WithText("日本語"))
	check := func(want int) {
		t.Helper()
		for i := 0; i < 2; i++ {
			w, _ := e.IntrinsicSize()
			if w != want {
				t.Fatalf("width %d != %d", w, want)
			}
		}
	}
	check(6)
	e.SetText("🇵🇹")
	check(2)
	WithText("abc")(e)
	check(3)
	e.SetRichText(TextSpan{Text: "👩🏽‍💻"}, TextSpan{Text: "é"})
	check(3)
	WithRichText(TextSpan{Text: "日本語"})(e)
	check(6)
	// Cache only content width; padding/border/dimensions are always current.
	WithPaddingTRBL(1, 2, 1, 3)(e)
	check(11)
	e.SetBorder(BorderSingle)
	check(13)
	e.SetWidth(Fixed(19))
	check(19)
	e.SetWidth(Auto())
	check(13)
	e.SetText("")
	check(0) // empty-container semantics are unchanged
}

func TestUnwrappedClustersCacheAndStyleInvalidation(t *testing.T) {
	e := New(WithRichText(TextSpan{Text: "🇵🇹 👩🏽‍💻"}), WithWrap(false))
	lines := e.unwrappedSpans()
	base := NewStyle().Foreground(RGBColor(12, 34, 56))
	clusters := e.spanLinesCache(lines).lineClusters(lines, 0, base)
	again := e.unwrappedSpans()
	if &lines[0] != &again[0] || &clusters[0] != &e.spanLinesCache(again).lineClusters(again, 0, base)[0] {
		t.Fatal("no-wrap cache not reused")
	}
	changed := base.Background(RGBColor(44, 55, 66))
	recalc := e.spanLinesCache(lines).lineClusters(lines, 0, changed)
	if recalc[0].st != mergeSpanStyle(changed, TextSpan{}.Style) {
		t.Fatal("base background stale")
	}
	if &clusters[0] == &recalc[0] {
		t.Fatal("style change did not resegment")
	}
	e.SetRichText(TextSpan{Text: "日本語"})
	if e.wrapCache.intrinsicValid || e.wrapCache.spans != nil || e.wrapCache.clusters != nil {
		t.Fatal("text change retained cache")
	}
	wrapped := e.wrappedSpans(2)
	if len(wrapped) < 2 {
		t.Fatal("wrap did not apply")
	}
	if len(e.unwrappedSpans()) != 1 {
		t.Fatal("wrap cache confused with no-wrap cache")
	}
}
