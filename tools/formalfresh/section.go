// Package formalfresh answers one question: is each TLA+ model in docs/formal
// still checked against the document text it was checked against?
//
// The models are companion artifacts — `state-machines.md` and `build-spec.md`
// stay normative, and on any disagreement the spec wins and the model is wrong.
// That ordering is what makes staleness dangerous rather than merely untidy: a
// normative edit can invalidate a model's coverage while every run recorded in
// the companion still reads as green, because the runs were real — they were
// just run against a different document. Nothing in `make ci` runs TLC (32+
// minutes, needs a JVM), so without this package the only thing standing
// between a spec edit and a silently-void model is somebody remembering.
//
// This is deliberately NOT a model checker. It is a staleness gate, the same
// shape as `tutorial-ka-check`: a derived artifact and its source, with a hash
// between them.
//
// # Section extraction
//
// The unit is a numbered section, because that is the granularity a model's
// correspondence map cites and the granularity at which staleness is real:
// model (b)'s subject sections survived 43 commits to `state-machines.md`
// untouched, so hashing whole documents would have reported it stale 43 times
// and taught the reader to restamp without looking.
//
// Two heading conventions are in play and both are supported:
//
//	docs/state-machines.md   ## §3. FenceReadyLost arbitration
//	docs/build-spec.md       ## 7. Risk, custody & security
//	                         ### 7.1 Sizing for survival
//
// A section runs from its own heading to the next heading of the same or a
// shallower level, so `### 7.1` ends at `### 7.2` or at `## 8`, and a `####`
// nested inside it does not end it.
package formalfresh

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// The three ways extraction can fail. Each is a hard error rather than an empty
// result, and that is the whole point: an extractor that returns "" for a
// heading somebody renamed would make every downstream comparison agree
// forever. A gate whose extractor silently finds nothing is indistinguishable
// from a gate over a document that never changes.
var (
	ErrNotFound  = errors.New("section heading not found")
	ErrAmbiguous = errors.New("section heading appears more than once")
	ErrEmpty     = errors.New("section has no body")
)

// heading reports the ATX heading level of a line and the text after the
// hashes. Level 0 means the line is not a heading.
func heading(line string) (int, string) {
	n := 0
	for n < len(line) && line[n] == '#' {
		n++
	}
	// A run of hashes is only a heading when a space follows it: `#hashtag` is
	// prose, and CommonMark agrees. Six is the maximum level.
	if n == 0 || n > 6 || n >= len(line) || line[n] != ' ' {
		return 0, ""
	}
	return n, strings.TrimSpace(line[n+1:])
}

// isFence reports whether a line opens or closes a fenced code block.
//
// This matters more than it looks. A fenced shell or config example containing
// a `# comment` line parses as an H1 heading, which would end whatever section
// it sits in and truncate that section's hash at the fence. Neither spec
// document contains such a line TODAY, which is exactly why the fence handling
// needs a synthetic fixture: the real documents cannot exercise this branch, so
// a bug in it would be invisible until the day somebody adds a config example.
func isFence(line string) bool {
	t := strings.TrimLeft(line, " ")
	// Up to three leading spaces still opens a fence; four makes it an indented
	// code block, which cannot contain a heading anyway.
	if len(line)-len(t) > 3 {
		return false
	}
	return strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")
}

// sectionID extracts the dotted section number from heading text, or "" when
// the heading is not a numbered section.
//
// The leading `§` is optional (state-machines.md uses it, build-spec.md does
// not) and a trailing `.` is a separator rather than part of the number, so
// `§3.` and `7.1` both yield what a citation would name them: "3" and "7.1".
func sectionID(rest string) string {
	tok, _, _ := strings.Cut(rest, " ")
	tok = strings.TrimPrefix(tok, "§")
	tok = strings.TrimSuffix(tok, ".")
	if tok == "" {
		return ""
	}
	// Digits and dots only, and no empty component — so "Appendix" and
	// "Funding-Rate" are not sections, and neither is a bare "7.".
	for part := range strings.SplitSeq(tok, ".") {
		if part == "" {
			return ""
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return ""
			}
		}
	}
	return tok
}

type hdr struct {
	line  int
	level int
	id    string
}

// headings returns every numbered-section heading in lines, skipping fenced
// code blocks.
func headings(lines []string) []hdr {
	var out []hdr
	inFence := false
	for i, line := range lines {
		if isFence(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		lv, rest := heading(line)
		if lv == 0 {
			continue
		}
		if id := sectionID(rest); id != "" {
			out = append(out, hdr{line: i, level: lv, id: id})
		}
	}
	return out
}

// ExtractSection returns the text of section id in src, heading line included,
// with trailing blank lines removed.
//
// Matching is on the WHOLE dotted number, never a prefix: asking for "7.1" must
// not match "7.10", and asking for "7" must not match "7.1". A prefix match
// would make the gate compare a section against a superset of itself and pass
// on some edits it should catch.
func ExtractSection(src, id string) (string, error) {
	lines := strings.Split(src, "\n")
	hs := headings(lines)

	found := -1
	for i, h := range hs {
		if h.id != id {
			continue
		}
		if found >= 0 {
			return "", fmt.Errorf("%w: §%s at lines %d and %d", ErrAmbiguous, id, hs[found].line+1, h.line+1)
		}
		found = i
	}
	if found < 0 {
		return "", fmt.Errorf("%w: §%s", ErrNotFound, id)
	}

	start := hs[found].line
	end := len(lines)
	for _, h := range hs[found+1:] {
		// Same level or shallower closes the section; a deeper heading is part
		// of it.
		if h.level <= hs[found].level {
			end = h.line
			break
		}
	}

	body := lines[start:end]
	for len(body) > 0 && strings.TrimSpace(body[len(body)-1]) == "" {
		body = body[:len(body)-1]
	}
	// The heading alone is not a section. This fires when a heading is present
	// but its content moved, which is a change the gate must report rather than
	// hash to a stable empty string.
	if len(body) < 2 {
		return "", fmt.Errorf("%w: §%s", ErrEmpty, id)
	}
	return strings.Join(body, "\n"), nil
}

// Digest is the stamped form of a section's text.
func Digest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
