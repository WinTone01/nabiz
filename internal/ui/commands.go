package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/WinTone01/nabiz/internal/i18n"
	"github.com/WinTone01/nabiz/internal/suite"
)

// Everything the program can do, in one list.
//
// This is the palette's contents and, more usefully, the honest inventory of
// the interface: if an action is not here it does not exist, and if it is here
// it works from here. Actions that cannot run right now stay listed and sort to
// the bottom rather than vanishing, so the answer to "where did export go" is
// always "it is there, greyed out, because nothing has been measured yet".
func (m *model) commands() []command {
	busy := m.app.Busy()
	hasRun := m.app.LastRun() != nil
	_, canRun := m.page().(runnablePage)
	_, canAB := m.page().(abPage)

	commands := make([]command, 0, len(m.nav.items)+16)

	navSection := i18n.T("cmd.section.go")
	for _, item := range m.nav.items {
		target := item.id
		commands = append(commands, command{
			id:      fmt.Sprintf("go:%d", item.id),
			title:   i18n.T("cmd.goto", item.label),
			section: navSection,
			hint:    item.key,
			enabled: true,
			run:     func() tea.Cmd { m.gotoPage(target); return nil },
		})
	}

	runSection := i18n.T("cmd.section.run")
	for _, name := range suite.Names {
		suiteName := name
		commands = append(commands, command{
			id:      "run:" + name,
			title:   i18n.T("cmd.run_suite", name),
			section: runSection,
			enabled: !busy,
			run: func() tea.Cmd {
				if m.app.Busy() {
					return nil
				}
				m.app.SuitePick = suiteIndex(suiteName)
				m.gotoPage(pageTest)
				return m.startSuite(suiteName)
			},
		})
	}

	actionSection := i18n.T("cmd.section.action")
	commands = append(commands,
		command{
			id: "act:run", title: i18n.T("cmd.run_current"), section: actionSection,
			hint: "r", enabled: canRun && !busy,
			run: func() tea.Cmd { return m.actionRun() },
		},
		command{
			id: "act:stop", title: i18n.T("cmd.stop"), section: actionSection,
			hint: "s", enabled: busy,
			run: func() tea.Cmd { m.actionStop(); return nil },
		},
		command{
			id: "act:export", title: i18n.T("cmd.export"), section: actionSection,
			hint: "e", enabled: hasRun,
			run: func() tea.Cmd { m.actionExport(); return nil },
		},
		command{
			id: "act:baseline", title: i18n.T("cmd.baseline"), section: actionSection,
			hint: "b", enabled: hasRun,
			run: func() tea.Cmd { m.actionBaseline(); return nil },
		},
		command{
			id: "act:ab", title: i18n.T("cmd.ab"), section: actionSection,
			hint: "a", enabled: canAB && !busy,
			run: func() tea.Cmd {
				if page, ok := m.page().(abPage); ok {
					m.askAB(page.ABTarget())
				}
				return nil
			},
		},
		command{
			id: "act:apply", title: i18n.T("cmd.apply"), section: actionSection,
			enabled: m.app.SelectedCount() > 0 && !busy,
			run:     func() tea.Cmd { m.gotoPage(pageAdvice); m.askApply(); return nil },
		},
		command{
			id: "act:rollback", title: i18n.T("cmd.rollback"), section: actionSection,
			enabled: m.app.HasSnapshots && !busy,
			run:     func() tea.Cmd { m.askRollback(); return nil },
		},
		command{
			id: "act:refresh", title: i18n.T("cmd.refresh"), section: actionSection,
			hint: "f5", enabled: true,
			run: func() tea.Cmd {
				m.notify(i18n.T("ui.refreshed"), toastInfo)
				return refreshEnvCmd()
			},
		},
		command{
			id: "act:lang", title: i18n.T("cmd.lang", i18n.Name(i18n.Other())),
			section: actionSection, hint: "f2", enabled: true,
			run: func() tea.Cmd { return m.switchLanguage() },
		},
		command{
			id: "act:help", title: i18n.T("cmd.help"), section: actionSection,
			hint: "?", enabled: true,
			run: func() tea.Cmd { m.showHelp = true; return nil },
		},
		command{
			id: "act:quit", title: i18n.T("cmd.quit"), section: actionSection,
			hint: "q", enabled: true,
			run: func() tea.Cmd { return m.quit() },
		},
	)
	return commands
}

func suiteIndex(name string) int {
	for index, candidate := range suite.Names {
		if candidate == name {
			return index
		}
	}
	return 0
}
