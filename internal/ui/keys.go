package ui

import (
	"github.com/charmbracelet/bubbles/key"

	"github.com/WinTone01/nabiz/internal/i18n"
)

// Every action key accepts both cases. Requiring shift for one action and not
// another is the kind of detail that makes a keyboard interface feel broken,
// and there is nothing here that needs two meanings for one letter.
type keyMap struct {
	Up, Down, PageUp, PageDown, Home, End key.Binding
	NextTab, PrevTab                      key.Binding
	PrevSel, NextSel                      key.Binding
	Run, Stop, Export, Baseline, AB       key.Binding
	Lang, Help, Quit                      key.Binding
}

func defaultKeys() keyMap {
	bind := func(help, desc string, keys ...string) key.Binding {
		return key.NewBinding(key.WithKeys(keys...), key.WithHelp(help, desc))
	}
	return keyMap{
		Up:       bind("↑", i18n.T("key.scroll"), "up", "k"),
		Down:     bind("↓", i18n.T("key.scroll"), "down", "j"),
		PageUp:   bind("pgup", i18n.T("key.scroll"), "pgup", "ctrl+b"),
		PageDown: bind("pgdn", i18n.T("key.scroll"), "pgdown", "ctrl+f", " "),
		Home:     bind("home", i18n.T("key.top"), "home", "g"),
		End:      bind("end", i18n.T("key.bottom"), "end", "G"),

		NextTab: bind("tab", i18n.T("key.nav"), "tab"),
		PrevTab: bind("⇧tab", i18n.T("key.nav"), "shift+tab"),
		PrevSel: bind("←", i18n.T("key.select"), "left", "["),
		NextSel: bind("→", i18n.T("key.select"), "right", "]"),

		Run:      bind("r", i18n.T("key.run"), "r", "R", "enter"),
		Stop:     bind("s", i18n.T("key.stop"), "s", "S", "esc"),
		Export:   bind("e", i18n.T("key.export"), "e", "E"),
		Baseline: bind("b", i18n.T("key.baseline"), "b", "B"),
		AB:       bind("a", i18n.T("key.ab"), "a", "A"),
		Lang:     bind("l", i18n.T("key.lang"), "l", "L", "f2"),
		Help:     bind("?", i18n.T("key.help"), "?", "f1"),
		Quit:     bind("q", i18n.T("key.quit"), "q", "Q", "ctrl+c"),
	}
}

// ShortHelp is the footer legend.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Run, k.Stop, k.Export, k.Lang, k.Help, k.Quit}
}

// FullHelp is the overlay opened with ?.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown, k.Home, k.End},
		{k.NextTab, k.PrevTab, k.PrevSel, k.NextSel},
		{k.Run, k.Stop, k.Export, k.Baseline},
		{k.AB, k.Lang, k.Help, k.Quit},
	}
}

func (k *keyMap) rebuild() { *k = defaultKeys() }
