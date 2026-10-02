package textdiff

import "strings"

// isJSSpace is JavaScript's \s (and the set String.prototype.trim removes).
func isJSSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

// isExtendedWordChar is jsdiff's extendedWordChars class.
func isExtendedWordChar(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == 0xad:
		return true
	case r >= 0xc0 && r <= 0xd6, r >= 0xd8 && r <= 0xf6, r >= 0xf8 && r <= 0x2c6, r >= 0x2c8 && r <= 0x2d7, r >= 0x2de && r <= 0x2ff, r >= 0x1e00 && r <= 0x1eff:
		return true
	}
	return false
}

func jsTrim(s string) string { return strings.TrimFunc(s, isJSSpace) }

func hasJSSpace(s string) bool { return strings.IndexFunc(s, isJSSpace) >= 0 }

// wordParts is value.match(/[word]+|\s+|[^word]/ug).
func wordParts(value string) []string {
	var parts []string
	runes := []rune(value)
	for i := 0; i < len(runes); {
		j := i + 1
		switch {
		case isExtendedWordChar(runes[i]):
			for j < len(runes) && isExtendedWordChar(runes[j]) {
				j++
			}
		case isJSSpace(runes[i]):
			for j < len(runes) && isJSSpace(runes[j]) {
				j++
			}
		}
		parts = append(parts, string(runes[i:j]))
		i = j
	}
	return parts
}

// tokenizeWords is WordDiff.tokenize: whitespace is stitched onto the
// neighbouring word or punctuation tokens.
func tokenizeWords(value string) []string {
	var tokens []string
	prev, havePrev := "", false
	for _, part := range wordParts(value) {
		switch {
		case hasJSSpace(part):
			if !havePrev {
				tokens = append(tokens, part)
			} else {
				tokens[len(tokens)-1] += part
			}
		case havePrev && hasJSSpace(prev):
			if tokens[len(tokens)-1] == prev {
				tokens[len(tokens)-1] += part
			} else {
				tokens = append(tokens, prev+part)
			}
		default:
			tokens = append(tokens, part)
		}
		prev, havePrev = part, true
	}
	return tokens
}

func joinWords(tokens []string) string {
	var b strings.Builder
	for i, t := range tokens {
		if i == 0 {
			b.WriteString(t)
		} else {
			b.WriteString(strings.TrimLeftFunc(t, isJSSpace))
		}
	}
	return b.String()
}

// Words is jsdiff's diffWords(old, new) with default options.
func Words(oldStr, newStr string) []Change {
	d := differ{
		equals: func(a, b string) bool { return jsTrim(a) == jsTrim(b) },
		join:   joinWords,
	}
	d.postProcess = postProcessWords
	return d.diff(removeEmpty(tokenizeWords(oldStr)), removeEmpty(tokenizeWords(newStr)))
}

func postProcessWords(changes []Change) []Change {
	lastKeep, insertion, deletion := -1, -1, -1
	for i := range changes {
		switch {
		case changes[i].Added:
			insertion = i
		case changes[i].Removed:
			deletion = i
		default:
			if insertion >= 0 || deletion >= 0 {
				dedupeWhitespace(changes, lastKeep, deletion, insertion, i)
			}
			lastKeep, insertion, deletion = i, -1, -1
		}
	}
	if insertion >= 0 || deletion >= 0 {
		dedupeWhitespace(changes, lastKeep, deletion, insertion, -1)
	}
	return changes
}

func leadingWs(s string) string {
	return s[:len(s)-len(strings.TrimLeftFunc(s, isJSSpace))]
}

func trailingWs(s string) string {
	return s[len(strings.TrimRightFunc(s, isJSSpace)):]
}

func longestCommonPrefix(a, b string) string {
	ra, rb := []rune(a), []rune(b)
	i := 0
	for i < len(ra) && i < len(rb) && ra[i] == rb[i] {
		i++
	}
	return string(ra[:i])
}

func longestCommonSuffix(a, b string) string {
	ra, rb := []rune(a), []rune(b)
	i := 0
	for i < len(ra) && i < len(rb) && ra[len(ra)-1-i] == rb[len(rb)-1-i] {
		i++
	}
	return string(ra[len(ra)-i:])
}

func replacePrefix(s, oldPrefix, newPrefix string) string {
	return newPrefix + strings.TrimPrefix(s, oldPrefix)
}

func replaceSuffix(s, oldSuffix, newSuffix string) string {
	return strings.TrimSuffix(s, oldSuffix) + newSuffix
}

// maximumOverlap is the longest suffix of a that is a prefix of b.
func maximumOverlap(a, b string) string {
	ra, rb := []rune(a), []rune(b)
	for n := min(len(ra), len(rb)); n > 0; n-- {
		if string(ra[len(ra)-n:]) == string(rb[:n]) {
			return string(rb[:n])
		}
	}
	return ""
}

// dedupeWhitespace is jsdiff's dedupeWhitespaceInChangeObjects; indices are
// -1 when absent.
func dedupeWhitespace(c []Change, startKeep, deletion, insertion, endKeep int) {
	switch {
	case deletion >= 0 && insertion >= 0:
		oldPrefix, oldSuffix := leadingWs(c[deletion].Value), trailingWs(c[deletion].Value)
		newPrefix, newSuffix := leadingWs(c[insertion].Value), trailingWs(c[insertion].Value)
		if startKeep >= 0 {
			common := longestCommonPrefix(oldPrefix, newPrefix)
			c[startKeep].Value = replaceSuffix(c[startKeep].Value, newPrefix, common)
			c[deletion].Value = strings.TrimPrefix(c[deletion].Value, common)
			c[insertion].Value = strings.TrimPrefix(c[insertion].Value, common)
		}
		if endKeep >= 0 {
			common := longestCommonSuffix(oldSuffix, newSuffix)
			c[endKeep].Value = replacePrefix(c[endKeep].Value, newSuffix, common)
			c[deletion].Value = strings.TrimSuffix(c[deletion].Value, common)
			c[insertion].Value = strings.TrimSuffix(c[insertion].Value, common)
		}
	case insertion >= 0:
		if startKeep >= 0 {
			c[insertion].Value = c[insertion].Value[len(leadingWs(c[insertion].Value)):]
		}
		if endKeep >= 0 {
			c[endKeep].Value = c[endKeep].Value[len(leadingWs(c[endKeep].Value)):]
		}
	case startKeep >= 0 && endKeep >= 0:
		newWsFull := leadingWs(c[endKeep].Value)
		delStart, delEnd := leadingWs(c[deletion].Value), trailingWs(c[deletion].Value)
		newWsStart := longestCommonPrefix(newWsFull, delStart)
		c[deletion].Value = strings.TrimPrefix(c[deletion].Value, newWsStart)
		newWsEnd := longestCommonSuffix(strings.TrimPrefix(newWsFull, newWsStart), delEnd)
		c[deletion].Value = strings.TrimSuffix(c[deletion].Value, newWsEnd)
		c[endKeep].Value = replacePrefix(c[endKeep].Value, newWsFull, newWsEnd)
		c[startKeep].Value = replaceSuffix(c[startKeep].Value, newWsFull, newWsFull[:len(newWsFull)-len(newWsEnd)])
	case endKeep >= 0:
		overlap := maximumOverlap(trailingWs(c[deletion].Value), leadingWs(c[endKeep].Value))
		c[deletion].Value = strings.TrimSuffix(c[deletion].Value, overlap)
	case startKeep >= 0:
		overlap := maximumOverlap(trailingWs(c[startKeep].Value), leadingWs(c[deletion].Value))
		c[deletion].Value = strings.TrimPrefix(c[deletion].Value, overlap)
	}
}
