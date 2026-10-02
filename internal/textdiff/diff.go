// Package textdiff ports jsdiff 8.0.4 (the "diff" package Pi depends on):
// its Myers implementation with jsdiff's diagonal pruning and tie-breaking,
// diffLines and diffWords, so diffs computed by gi pick the same hunks and
// the same changed words as Pi's.
package textdiff

import "strings"

// Change is one jsdiff change object.
type Change struct {
	Value          string
	Count          int
	Added, Removed bool
}

type component struct {
	count          int
	added, removed bool
	previous       *component
}

type pathState struct {
	oldPos int
	last   *component
}

type differ struct {
	equals      func(a, b string) bool
	join        func(tokens []string) string
	postProcess func([]Change) []Change
}

func removeEmpty(tokens []string) []string {
	out := tokens[:0:0]
	for _, t := range tokens {
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func addToPath(p *pathState, added, removed bool, oldPosInc int) *pathState {
	last := p.last
	if last != nil && last.added == added && last.removed == removed {
		return &pathState{oldPos: p.oldPos + oldPosInc, last: &component{count: last.count + 1, added: added, removed: removed, previous: last.previous}}
	}
	return &pathState{oldPos: p.oldPos + oldPosInc, last: &component{count: 1, added: added, removed: removed, previous: last}}
}

func (d differ) extractCommon(p *pathState, newTokens, oldTokens []string, diagonal int) int {
	oldPos := p.oldPos
	newPos := oldPos - diagonal
	common := 0
	for newPos+1 < len(newTokens) && oldPos+1 < len(oldTokens) && d.equals(oldTokens[oldPos+1], newTokens[newPos+1]) {
		newPos++
		oldPos++
		common++
	}
	if common > 0 {
		p.last = &component{count: common, previous: p.last}
	}
	p.oldPos = oldPos
	return newPos
}

func (d differ) buildValues(last *component, newTokens, oldTokens []string) []Change {
	var components []*component
	for c := last; c != nil; c = c.previous {
		components = append(components, c)
	}
	changes := make([]Change, len(components))
	newPos, oldPos := 0, 0
	for i := range components {
		c := components[len(components)-1-i]
		ch := Change{Count: c.count, Added: c.added, Removed: c.removed}
		if !c.removed {
			ch.Value = d.join(newTokens[newPos : newPos+c.count])
			newPos += c.count
			if !c.added {
				oldPos += c.count
			}
		} else {
			ch.Value = d.join(oldTokens[oldPos : oldPos+c.count])
			oldPos += c.count
		}
		changes[i] = ch
	}
	return changes
}

// diff is jsdiff's Diff.diffWithOptionsObj (synchronous, no limits).
func (d differ) diff(oldTokens, newTokens []string) []Change {
	done := func(c []Change) []Change {
		if d.postProcess != nil {
			return d.postProcess(c)
		}
		return c
	}
	newLen, oldLen := len(newTokens), len(oldTokens)
	maxEditLength := newLen + oldLen
	offset := maxEditLength + 1
	bestPath := make([]*pathState, 2*offset+1)
	bestPath[offset] = &pathState{oldPos: -1}
	newPos := d.extractCommon(bestPath[offset], newTokens, oldTokens, 0)
	if bestPath[offset].oldPos+1 >= oldLen && newPos+1 >= newLen {
		return done(d.buildValues(bestPath[offset].last, newTokens, oldTokens))
	}
	const inf = int(^uint(0) >> 1)
	minDiagonal, maxDiagonal := -inf, inf
	for editLength := 1; editLength <= maxEditLength; editLength++ {
		for diagonal := max(minDiagonal, -editLength); diagonal <= min(maxDiagonal, editLength); diagonal += 2 {
			removePath, addPath := bestPath[offset+diagonal-1], bestPath[offset+diagonal+1]
			if removePath != nil {
				bestPath[offset+diagonal-1] = nil
			}
			canAdd := false
			if addPath != nil {
				addPathNewPos := addPath.oldPos - diagonal
				canAdd = 0 <= addPathNewPos && addPathNewPos < newLen
			}
			canRemove := removePath != nil && removePath.oldPos+1 < oldLen
			if !canAdd && !canRemove {
				bestPath[offset+diagonal] = nil
				continue
			}
			var base *pathState
			if !canRemove || (canAdd && removePath.oldPos < addPath.oldPos) {
				base = addToPath(addPath, true, false, 0)
			} else {
				base = addToPath(removePath, false, true, 1)
			}
			newPos = d.extractCommon(base, newTokens, oldTokens, diagonal)
			if base.oldPos+1 >= oldLen && newPos+1 >= newLen {
				return done(d.buildValues(base.last, newTokens, oldTokens))
			}
			bestPath[offset+diagonal] = base
			if base.oldPos+1 >= oldLen {
				maxDiagonal = min(maxDiagonal, diagonal-1)
			}
			if newPos+1 >= newLen {
				minDiagonal = max(minDiagonal, diagonal+1)
			}
		}
	}
	return nil
}

// tokenizeLines is jsdiff's line tokenizer: each line keeps its "\n" or
// "\r\n"; a trailing newline yields no empty final token.
func tokenizeLines(value string) []string {
	var parts []string // split(/(\n|\r\n)/): content, separator, content, ...
	start := 0
	for i := 0; i < len(value); i++ {
		if value[i] == '\n' {
			parts = append(parts, value[start:i], "\n")
			start = i + 1
		} else if value[i] == '\r' && i+1 < len(value) && value[i+1] == '\n' {
			parts = append(parts, value[start:i], "\r\n")
			start = i + 2
			i++
		}
	}
	parts = append(parts, value[start:])
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	var lines []string
	for i, part := range parts {
		if i%2 == 1 {
			lines[len(lines)-1] += part
		} else {
			lines = append(lines, part)
		}
	}
	return lines
}

// Lines is jsdiff's diffLines(old, new) with default options.
func Lines(oldStr, newStr string) []Change {
	d := differ{equals: func(a, b string) bool { return a == b }, join: func(t []string) string { return strings.Join(t, "") }}
	return d.diff(removeEmpty(tokenizeLines(oldStr)), removeEmpty(tokenizeLines(newStr)))
}
