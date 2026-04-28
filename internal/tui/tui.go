// Package tui implements the Bubble Tea interactive interface. The
// CLI mode is the load-bearing path; the TUI is a peer at feature
// parity (see SPEC.md §8).
package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sroberts/plumbline/pkg/acmm"
)

// ScanFunc is the work the TUI delegates to the assess pipeline.
// The TUI doesn't know about scanner / signals / scoring directly;
// it just kicks off ScanFunc and renders the resulting Report.
type ScanFunc func(ctx context.Context) (acmm.Report, error)

type screen int

const (
	screenScanning screen = iota
	screenResults
	screenDetail
	screenError
)

type doneMsg struct {
	report acmm.Report
	err    error
}

type model struct {
	screen screen
	scan   ScanFunc
	report acmm.Report
	err    error
	cursor int
	width  int
	height int
}

// New returns a model wired to the given scan function.
func New(scan ScanFunc) tea.Model {
	return &model{screen: screenScanning, scan: scan}
}

// Run starts the TUI program. Blocks until the user quits or the scan
// errors. The returned report is what was emitted on screen — the
// caller can use it for --report output once the TUI exits.
func Run(scan ScanFunc) (acmm.Report, error) {
	p := tea.NewProgram(New(scan), tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return acmm.Report{}, err
	}
	if m, ok := final.(*model); ok {
		if m.err != nil {
			return acmm.Report{}, m.err
		}
		return m.report, nil
	}
	return acmm.Report{}, nil
}

func (m *model) Init() tea.Cmd {
	return func() tea.Msg {
		report, err := m.scan(context.Background())
		return doneMsg{report: report, err: err}
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case doneMsg:
		if msg.err != nil {
			m.err = msg.err
			m.screen = screenError
			return m, nil
		}
		m.report = msg.report
		m.screen = screenResults
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "down", "j":
			if m.screen == screenResults && m.cursor < len(m.report.Signals)-1 {
				m.cursor++
			}
		case "up", "k":
			if m.screen == screenResults && m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			if m.screen == screenResults && len(m.report.Signals) > 0 {
				m.screen = screenDetail
			}
		case "esc":
			if m.screen == screenDetail {
				m.screen = screenResults
			}
		}
	}
	return m, nil
}

// Color palette per SPEC.md §8.2.6.
var (
	styleHeader   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffffff"))
	styleHint     = lipgloss.NewStyle().Foreground(lipgloss.Color("36"))
	styleFound    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	stylePartial  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleMissing  = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleNA       = lipgloss.NewStyle().Faint(true)
	styleSelected = lipgloss.NewStyle().Reverse(true)
)

func statusStyle(s acmm.Status) lipgloss.Style {
	switch s {
	case acmm.StatusFound:
		return styleFound
	case acmm.StatusPartial:
		return stylePartial
	case acmm.StatusMissing:
		return styleMissing
	default:
		return styleNA
	}
}

func (m *model) View() string {
	switch m.screen {
	case screenScanning:
		return styleHeader.Render("plumbline · scanning...") + "\n\n" +
			styleHint.Render("Press q to abort.")
	case screenError:
		return styleMissing.Render("Error: ") + m.err.Error() + "\n\n" +
			styleHint.Render("Press q to quit.")
	case screenResults:
		return m.renderResults()
	case screenDetail:
		return m.renderDetail()
	}
	return ""
}

func (m *model) renderResults() string {
	var b strings.Builder
	r := m.report
	b.WriteString(styleHeader.Render(fmt.Sprintf("plumbline · %s", r.Repo)))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", 60))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Assessed level: %s\n\n",
		styleHeader.Render(fmt.Sprintf("%d (%s)", r.Verdict.Level, r.Verdict.Name))))

	for _, l := range []acmm.Level{
		acmm.LevelInstructed, acmm.LevelMeasured,
		acmm.LevelAdaptive, acmm.LevelSelfSustaining,
	} {
		score := r.Verdict.LevelScores[l]
		bar := levelBar(score, 20)
		marker := ""
		switch {
		case l == r.Verdict.Level:
			marker = styleFound.Render("  PASS")
		case l == r.Verdict.Level+1:
			marker = stylePartial.Render("  NEXT")
		}
		b.WriteString(fmt.Sprintf("  L%d %-16s  %s  %5.1f%%%s\n",
			l, l.Name(), bar, score*100, marker))
	}

	if len(r.Verdict.NextGap) > 0 {
		b.WriteString("\n")
		b.WriteString(styleHeader.Render(fmt.Sprintf("Next-level gap (to reach L%d):", r.Verdict.Level+1)))
		b.WriteString("\n")
		for _, id := range r.Verdict.NextGap {
			b.WriteString(fmt.Sprintf("  · %s\n", styleMissing.Render(id)))
		}
	}

	b.WriteString("\n")
	b.WriteString(styleHeader.Render("Signals:"))
	b.WriteString("\n")
	for i, s := range r.Signals {
		line := fmt.Sprintf("  %-30s %s  %.2f", s.ID, padStatus(s.Status), s.Score)
		line = statusStyle(s.Status).Render(line)
		if i == m.cursor {
			line = styleSelected.Render(line)
		}
		b.WriteString(line + "\n")
	}

	b.WriteString("\n")
	b.WriteString(styleHint.Render("[↑/↓] select   [enter] detail   [q] quit"))
	return b.String()
}

func padStatus(s acmm.Status) string {
	return fmt.Sprintf("%-8s", s)
}

func levelBar(score float64, width int) string {
	filled := int(score * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
}

func (m *model) renderDetail() string {
	if m.cursor < 0 || m.cursor >= len(m.report.Signals) {
		return ""
	}
	s := m.report.Signals[m.cursor]
	var b strings.Builder
	b.WriteString(styleHeader.Render(fmt.Sprintf("%s · %s", s.ID, statusStyle(s.Status).Render(string(s.Status)))))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", 60))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Title:      %s\n", s.Title))
	b.WriteString(fmt.Sprintf("Level:      %d (%s)\n", s.Level, s.Level.Name()))
	b.WriteString(fmt.Sprintf("Family:     %s\n", s.Family))
	b.WriteString(fmt.Sprintf("Score:      %v\n", s.Score))
	b.WriteString(fmt.Sprintf("Confidence: %s\n", s.Confidence))
	b.WriteString(fmt.Sprintf("Method:     %s\n", s.Method))
	b.WriteString("\n")

	if len(s.Evidence) == 0 {
		b.WriteString(styleNA.Render("Evidence: (none)\n"))
	} else {
		b.WriteString(styleHeader.Render("Evidence:"))
		b.WriteString("\n")
		for _, e := range s.Evidence {
			b.WriteString(fmt.Sprintf("  %s\n", e.Path))
			if e.Excerpt != "" {
				b.WriteString(fmt.Sprintf("    %s\n", styleNA.Render(e.Excerpt)))
			}
		}
	}

	if len(s.Notes) > 0 {
		b.WriteString("\n")
		b.WriteString(styleHeader.Render("Notes:"))
		b.WriteString("\n")
		for _, n := range s.Notes {
			b.WriteString(fmt.Sprintf("  %s\n", n))
		}
	}

	b.WriteString("\n")
	b.WriteString(styleHint.Render("[esc/enter] back   [q] quit"))
	return b.String()
}
