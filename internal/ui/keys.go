package ui

import (
	"github.com/charmbracelet/bubbles/key"

	"github.com/WinTone01/nabiz/internal/i18n"
)

// Every action has a key and every key has one meaning.
//
// Two rules the rest of the interface depends on: enter always activates
// whatever currently has focus and never starts a measurement of its own, and
// esc always steps back out of the innermost thing that is open - a dialog,
// then the palette, then the help sheet. Nothing else is allowed to claim
// either of them, because a key that means two things is a key nobody trusts.
type keyMap struct {
	Up, Down, Left, Right           key.Binding
	PageUp, PageDown, Home, End     key.Binding
	NextPage, PrevPage              key.Binding
	Focus, FocusBack                key.Binding
	Activate, Toggle, Back, Dismiss key.Binding
	Run, Stop, Export, Baseline, AB key.Binding
	Palette, Lang, Help, Quit       key.Binding
	Refresh                         key.Binding
}

func defaultKeys() keyMap {
	bind := func(hint, description string, keys ...string) key.Binding {
		return key.NewBinding(key.WithKeys(keys...), key.WithHelp(hint, description))
	}
	return keyMap{
		Up:       bind("↑/k", i18n.T("key.move"), "up", "k"),
		Down:     bind("↓/j", i18n.T("key.move"), "down", "j"),
		Left:     bind("←/h", i18n.T("key.select"), "left", "h"),
		Right:    bind("→/l", i18n.T("key.select"), "right", "l"),
		PageUp:   bind("pgup", i18n.T("key.page"), "pgup", "ctrl+b"),
		PageDown: bind("pgdn", i18n.T("key.page"), "pgdown", "ctrl+f"),
		Home:     bind("g", i18n.T("key.top"), "home", "g"),
		End:      bind("G", i18n.T("key.bottom"), "end", "G"),

		NextPage: bind("ctrl+n", i18n.T("key.nav"), "ctrl+n"),
		PrevPage: bind("ctrl+p", i18n.T("key.nav"), "ctrl+p"),

		Focus:     bind("tab", i18n.T("key.focus"), "tab"),
		FocusBack: bind("⇧tab", i18n.T("key.focus"), "shift+tab"),
		Activate:  bind("enter", i18n.T("key.confirm"), "enter"),
		Toggle:    bind("space", i18n.T("key.toggle"), " "),
		Dismiss:   bind("d", i18n.T("key.dismiss"), "d", "D"),
		Back:      bind("esc", i18n.T("key.back"), "esc"),

		Run:      bind("r", i18n.T("key.run"), "r", "R"),
		Stop:     bind("s", i18n.T("key.stop"), "s", "S"),
		Export:   bind("e", i18n.T("key.export"), "e", "E"),
		Baseline: bind("b", i18n.T("key.baseline"), "b", "B"),
		AB:       bind("a", i18n.T("key.ab"), "a", "A"),
		Refresh:  bind("f5", i18n.T("key.refresh"), "f5", "ctrl+r"),

		Palette: bind("ctrl+k", i18n.T("key.palette"), "ctrl+k", ":"),
		Lang:    bind("f2", i18n.T("key.lang"), "f2"),
		Help:    bind("?", i18n.T("key.help"), "?", "f1"),
		Quit:    bind("q", i18n.T("key.quit"), "q", "Q", "ctrl+c"),
	}
}

// ShortHelp is the footer legend: the six things worth spending a status bar on.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Run, k.Focus, k.Palette, k.Export, k.Help, k.Quit}
}

// FullHelp is the sheet behind ?, grouped the way the keys are learnt rather
// than the way they are declared.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown, k.Home, k.End},
		{k.Focus, k.FocusBack, k.NextPage, k.PrevPage, k.Left, k.Right},
		{k.Activate, k.Toggle, k.Dismiss, k.Back, k.Refresh},
		{k.Run, k.Stop, k.Export, k.Baseline, k.AB},
		{k.Palette, k.Lang, k.Help, k.Quit},
	}
}

// rebuild re-reads the descriptions after a language switch. The bindings
// themselves never change: the letter r starts a run in every language, because
// muscle memory does not get translated.
func (k *keyMap) rebuild() { *k = defaultKeys() }
