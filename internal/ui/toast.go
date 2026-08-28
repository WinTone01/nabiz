package ui

import (
	"strings"
	"time"
)

// Toasts.
//
// Feedback belongs next to the thing that caused it, not in a status bar the
// reader has already stopped looking at. These stack in the top right, carry
// their severity in their border, and expire on their own - a message that has
// to be dismissed is a dialog, and a dialog for "file saved" is an insult.

type toastLevel int

const (
	toastInfo toastLevel = iota
	toastGood
	toastWarn
	toastBad
)

type toast struct {
	text  string
	level toastLevel
	until time.Time
}

type toastStack struct{ items []toast }

// push adds a message. Errors linger, confirmations do not: the cost of missing
// "saved" is nothing and the cost of missing a failure is a wrong conclusion.
func (t *toastStack) push(text string, level toastLevel) {
	life := 4 * time.Second
	if level == toastBad || level == toastWarn {
		life = 9 * time.Second
	}
	t.items = append(t.items, toast{text: text, level: level, until: time.Now().Add(life)})
	if len(t.items) > 4 {
		t.items = t.items[len(t.items)-4:]
	}
}

func (t *toastStack) prune() {
	now := time.Now()
	kept := t.items[:0]
	for _, item := range t.items {
		if item.until.After(now) {
			kept = append(kept, item)
		}
	}
	t.items = kept
}

func (t *toastStack) empty() bool { return len(t.items) == 0 }

func (t *toastStack) mark(level toastLevel) string {
	switch level {
	case toastGood:
		return sOK.Render("✓")
	case toastWarn:
		return sWarn.Render("▲")
	case toastBad:
		return sBad.Render("●")
	}
	return sInfo.Render("·")
}

// view renders the stack as one block, newest at the bottom so a burst of
// messages reads in the order it happened.
func (t *toastStack) view(maxWidth int) string {
	t.prune()
	if len(t.items) == 0 {
		return ""
	}
	width := min(maxWidth, 56)
	var blocks []string
	for _, item := range t.items {
		accent := colInfo
		switch item.level {
		case toastGood:
			accent = colOK
		case toastWarn:
			accent = colWarn
		case toastBad:
			accent = colBad
		}
		body := t.mark(item.level) + " " +
			sText.Render(wrapIndent(item.text, width-8, "  "))
		blocks = append(blocks, card{width: width, accent: accent}.render(body))
	}
	return strings.Join(blocks, "\n")
}
