package ui

import (
	"fmt"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"github.com/muesli/termenv"

	"github.com/WinTone01/nabiz/internal/config"
)

// The colour profile is forced on, because the whole point of these tests is
// that widths are measured through the escape sequences. Under a pipe lipgloss
// would strip every style and the tests would pass on a string the terminal
// never sees.
func TestMain(main *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	zone.NewGlobal()
	os.Exit(main.Run())
}

// A terminal interface has exactly one hard invariant: nothing may be wider
// than the terminal. One cell of overflow wraps the line, which pushes every
// line below it down by one, which makes the whole frame appear to disintegrate
// - and it happens from a single coloured cell being measured with len().
//
// This walks every page at several sizes and checks the frame is exactly the
// size it claims to be.
func TestFrameFitsEverySize(t *testing.T) {
	sizes := []struct{ width, height int }{
		{80, 24},  // the smallest thing anyone actually uses
		{96, 30},  // just under the sidebar breakpoint
		{120, 40}, // a normal window
		{200, 60}, // a wide one
	}
	for _, size := range sizes {
		m := newModel(config.Config{}, "test")
		updated, _ := m.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
		root := updated.(*model)

		for index := range root.nav.items {
			root.gotoIndex(index)
			view := root.View()
			lines := strings.Split(view, "\n")

			if len(lines) > size.height {
				t.Errorf("%dx%d %s: %d rows, want at most %d",
					size.width, size.height, root.nav.current().label, len(lines), size.height)
			}
			for row, line := range lines {
				if width := visWidth(line); width > size.width {
					t.Errorf("%dx%d %s: row %d is %d columns wide, want at most %d\n%q",
						size.width, size.height, root.nav.current().label,
						row, width, size.width, line)
					break
				}
			}
		}
	}
}

// The overlays are composited onto the frame rather than replacing it, which is
// the operation most likely to go wrong: it cuts styled text at a column.
func TestOverlaysStayInsideTheFrame(t *testing.T) {
	m := newModel(config.Config{}, "test")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	root := updated.(*model)

	root.notify("a saved file with a long enough name to wrap onto a second line", toastGood)
	root.showHelp = true
	root.palette.show(root.commands())
	root.askQuit()

	for row, line := range strings.Split(root.View(), "\n") {
		if width := visWidth(line); width > 120 {
			t.Fatalf("row %d is %d columns wide with overlays open, want at most 120", row, width)
		}
	}
}

// dumpView writes one rendered frame per page, so the layout can be looked at
// rather than only asserted about. It is skipped unless NABIZ_DUMP is set.
func TestDumpView(t *testing.T) {
	target := os.Getenv("NABIZ_DUMP")
	if target == "" {
		t.Skip("set NABIZ_DUMP to a path to dump the rendered frames")
	}
	var out strings.Builder
	for _, size := range []struct{ width, height int }{{132, 42}, {84, 28}} {
		m := newModel(config.Config{}, "0.3.1")
		updated, _ := m.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
		root := updated.(*model)

		for index := range root.nav.items {
			root.gotoIndex(index)
			out.WriteString(fmt.Sprintf("=== %dx%d %s ===\n",
				size.width, size.height, root.nav.current().label))
			out.WriteString(root.View())
			out.WriteString("\n\n")
		}
		root.palette.show(root.commands())
		out.WriteString(fmt.Sprintf("=== %dx%d palette ===\n", size.width, size.height))
		out.WriteString(root.View() + "\n\n")
		root.palette.hide()
		root.askQuit()
		out.WriteString(fmt.Sprintf("=== %dx%d dialog ===\n", size.width, size.height))
		out.WriteString(root.View() + "\n\n")
	}
	if err := os.WriteFile(target, []byte(out.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
