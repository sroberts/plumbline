// Package tui implements the Bubble Tea interactive interface. The
// CLI mode is the load-bearing path; the TUI is a peer at feature
// parity (see SPEC.md §8).
package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sroberts/plumbline/internal/fix"
	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/internal/textwrap"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// ScanFunc is the work the TUI delegates to the assess pipeline. It
// returns both the human-readable Report (rendered on screen) and the
// underlying RepoIndex (consumed by the fix flow when the user opts
// to scaffold a missing artifact).
type ScanFunc func(ctx context.Context) (acmm.Report, *scanner.RepoIndex, error)

type screen int

const (
	screenScanning screen = iota
	screenResults
	screenDetail
	screenFixForm
	screenFixPreview
	screenFixDone
	screenError
)

type doneMsg struct {
	report acmm.Report
	idx    *scanner.RepoIndex
	err    error
}

type model struct {
	screen screen
	scan   ScanFunc
	report acmm.Report
	idx    *scanner.RepoIndex
	err    error
	cursor int
	width  int
	height int

	// Fix-flow state.
	fixer      signals.Fixer
	fixInputs  []acmm.FixInput
	fixValues  []textinput.Model
	fixCursor  int
	fixPlan    acmm.FixPlan
	fixResult  fix.Result
	fixErr     error
	fixApplied bool
}

// New returns a model wired to the given scan function.
func New(scan ScanFunc) tea.Model {
	return &model{screen: screenScanning, scan: scan}
}

// Run starts the TUI program. Blocks until the user quits or the scan
// errors. Returns the (possibly updated) Report so the caller can
// re-render in CLI mode after the TUI exits.
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
		report, idx, err := m.scan(context.Background())
		return doneMsg{report: report, idx: idx, err: err}
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
		m.idx = msg.idx
		m.screen = screenResults
		return m, nil
	case tea.KeyMsg:
		// Quit is global.
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		// Per-screen handlers.
		switch m.screen {
		case screenResults:
			return m.updateResults(msg)
		case screenDetail:
			return m.updateDetail(msg)
		case screenFixForm:
			return m.updateFixForm(msg)
		case screenFixPreview:
			return m.updateFixPreview(msg)
		case screenFixDone:
			return m.updateFixDone(msg)
		case screenError:
			if msg.String() == "q" {
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m *model) updateResults(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "down", "j":
		if m.cursor < len(m.report.Signals)-1 {
			m.cursor++
		}
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "enter":
		if len(m.report.Signals) > 0 {
			m.screen = screenDetail
		}
	case "r":
		return m.rescan()
	}
	return m, nil
}

// rescan transitions back to the scanning screen and emits a fresh
// scan command. Used after the initial results land or after a fix
// has been applied (the verdict will likely change).
func (m *model) rescan() (tea.Model, tea.Cmd) {
	m.screen = screenScanning
	m.cursor = 0
	return m, func() tea.Msg {
		report, idx, err := m.scan(context.Background())
		return doneMsg{report: report, idx: idx, err: err}
	}
}

func (m *model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "esc":
		m.screen = screenResults
	case "a", "f":
		// Apply / Fix.
		if m.idx == nil || m.cursor >= len(m.report.Signals) {
			return m, nil
		}
		signalID := m.report.Signals[m.cursor].ID
		sig, ok := signals.Default.Get(signalID)
		if !ok {
			return m, nil
		}
		fxr, ok := sig.(signals.Fixer)
		if !ok {
			return m, nil // signal can't fix itself; ignore
		}
		m.fixer = fxr
		m.fixInputs = fxr.Inputs()
		m.fixValues = make([]textinput.Model, len(m.fixInputs))
		for i, in := range m.fixInputs {
			ti := textinput.New()
			ti.Placeholder = in.Default
			ti.Prompt = "› "
			ti.CharLimit = 4096
			ti.Width = 60
			if in.Default != "" {
				ti.SetValue(in.Default)
			}
			m.fixValues[i] = ti
		}
		m.fixCursor = 0
		// If the fixer needs no inputs, jump straight to preview.
		if len(m.fixInputs) == 0 {
			return m.generatePlan()
		}
		m.fixValues[0].Focus()
		m.screen = screenFixForm
	}
	return m, nil
}

func (m *model) updateFixForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenDetail
		return m, nil
	case "tab", "down":
		if m.fixCursor < len(m.fixInputs)-1 {
			m.fixValues[m.fixCursor].Blur()
			m.fixCursor++
			m.fixValues[m.fixCursor].Focus()
		}
		return m, nil
	case "shift+tab", "up":
		if m.fixCursor > 0 {
			m.fixValues[m.fixCursor].Blur()
			m.fixCursor--
			m.fixValues[m.fixCursor].Focus()
		}
		return m, nil
	case "enter":
		// On last field → generate plan; otherwise advance.
		if m.fixCursor < len(m.fixInputs)-1 {
			m.fixValues[m.fixCursor].Blur()
			m.fixCursor++
			m.fixValues[m.fixCursor].Focus()
			return m, nil
		}
		return m.generatePlan()
	}
	// Forward all other keys to the focused textinput.
	var cmd tea.Cmd
	m.fixValues[m.fixCursor], cmd = m.fixValues[m.fixCursor].Update(msg)
	return m, cmd
}

func (m *model) generatePlan() (tea.Model, tea.Cmd) {
	values := make(map[string]string, len(m.fixInputs))
	for i, in := range m.fixInputs {
		values[in.Key] = m.fixValues[i].Value()
	}
	plan, err := m.fixer.Plan(context.Background(), m.idx, values)
	if err != nil {
		m.fixErr = err
		m.screen = screenFixDone
		return m, nil
	}
	m.fixPlan = plan
	m.screen = screenFixPreview
	return m, nil
}

func (m *model) updateFixPreview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "n":
		// Cancel; return to detail.
		m.fixer = nil
		m.fixPlan = acmm.FixPlan{}
		m.screen = screenDetail
		return m, nil
	case "y":
		// Apply.
		res, err := fix.Apply(m.report.Repo, m.fixPlan, fix.Options{})
		if err != nil {
			m.fixErr = err
		}
		m.fixResult = res
		m.fixApplied = err == nil
		m.screen = screenFixDone
	}
	return m, nil
}

func (m *model) updateFixDone(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "esc", "enter":
		m.fixer = nil
		m.fixPlan = acmm.FixPlan{}
		m.fixResult = fix.Result{}
		m.fixErr = nil
		m.fixApplied = false
		m.screen = screenResults
	case "r":
		// Clear fix state and re-run the scan; verdict likely changed.
		m.fixer = nil
		m.fixPlan = acmm.FixPlan{}
		m.fixResult = fix.Result{}
		m.fixErr = nil
		m.fixApplied = false
		return m.rescan()
	}
	return m, nil
}

// ===== styles (SPEC.md §8.2.6 palette) =====

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
	case screenFixForm:
		return m.renderFixForm()
	case screenFixPreview:
		return m.renderFixPreview()
	case screenFixDone:
		return m.renderFixDone()
	}
	return ""
}

func (m *model) renderResults() string {
	var b strings.Builder
	r := m.report
	b.WriteString(styleHeader.Render(fmt.Sprintf("plumbline · %s", r.Repo)))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(60, m.viewWidth())))
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
		marker := " "
		if _, ok := signals.Default.Get(s.ID); ok {
			if sig, _ := signals.Default.Get(s.ID); sig != nil {
				if _, fixable := sig.(signals.Fixer); fixable {
					marker = styleHint.Render("✚")
				}
			}
		}
		line := fmt.Sprintf("  %s %-30s %s  %.2f", marker, s.ID, padStatus(s.Status), s.Score)
		line = statusStyle(s.Status).Render(line)
		if i == m.cursor {
			line = styleSelected.Render(line)
		}
		b.WriteString(line + "\n")
	}

	b.WriteString("\n")
	b.WriteString(styleHint.Render("[↑/↓] select   [enter] detail   [r] rescan   [✚=fixable]   [q] quit"))
	return b.String()
}

func (m *model) renderDetail() string {
	if m.cursor < 0 || m.cursor >= len(m.report.Signals) {
		return ""
	}
	s := m.report.Signals[m.cursor]
	bodyWidth := m.viewWidth() - 2

	var b strings.Builder
	b.WriteString(styleHeader.Render(fmt.Sprintf("%s · %s", s.ID, statusStyle(s.Status).Render(string(s.Status)))))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(60, m.viewWidth())))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Title:      %s\n", s.Title))
	b.WriteString(fmt.Sprintf("Level:      %d (%s)\n", s.Level, s.Level.Name()))
	b.WriteString(fmt.Sprintf("Family:     %s\n", s.Family))
	b.WriteString(fmt.Sprintf("Score:      %v\n", s.Score))
	b.WriteString(fmt.Sprintf("Confidence: %s\n", s.Confidence))
	b.WriteString(fmt.Sprintf("Method:     %s\n", s.Method))
	b.WriteString("\n")

	if len(s.Notes) > 0 {
		b.WriteString(styleHeader.Render("Why:"))
		b.WriteString("\n")
		for _, n := range s.Notes {
			b.WriteString(textwrap.Indent("  · ", n, bodyWidth))
			b.WriteByte('\n')
		}
		b.WriteString("\n")
	}

	if len(s.Evidence) == 0 {
		b.WriteString(styleNA.Render("Evidence: (none)\n"))
	} else {
		b.WriteString(styleHeader.Render("Evidence:"))
		b.WriteString("\n")
		for _, e := range s.Evidence {
			b.WriteString(fmt.Sprintf("  %s\n", e.Path))
			if e.Excerpt != "" {
				b.WriteString(styleNA.Render(textwrap.Indent("    ", e.Excerpt, bodyWidth)))
				b.WriteByte('\n')
			}
		}
	}

	if s.FixHint != "" {
		b.WriteString("\n")
		b.WriteString(styleHeader.Render("Fix:"))
		b.WriteString("\n")
		b.WriteString(textwrap.Indent("  ", s.FixHint, bodyWidth))
		b.WriteByte('\n')
	}

	b.WriteString("\n")
	hint := "[esc] back   [q] quit"
	if sig, _ := signals.Default.Get(s.ID); sig != nil {
		if _, ok := sig.(signals.Fixer); ok {
			hint = "[a] apply fix   [esc] back   [q] quit"
		}
	}
	b.WriteString(styleHint.Render(hint))
	return b.String()
}

func (m *model) renderFixForm() string {
	var b strings.Builder
	b.WriteString(styleHeader.Render(fmt.Sprintf("Apply fix · %s", m.fixer.ID())))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(60, m.viewWidth())))
	b.WriteString("\n\n")

	if len(m.fixInputs) == 0 {
		b.WriteString(styleNA.Render("(no inputs needed; press enter to preview)\n"))
		return b.String()
	}

	for i, in := range m.fixInputs {
		labelStyle := styleNA
		if i == m.fixCursor {
			labelStyle = styleHeader
		}
		b.WriteString(labelStyle.Render(in.Label))
		if in.Required {
			b.WriteString(styleMissing.Render(" *"))
		}
		b.WriteString("\n")
		if in.Help != "" {
			b.WriteString(styleNA.Render("  " + in.Help))
			b.WriteString("\n")
		}
		b.WriteString("  ")
		b.WriteString(m.fixValues[i].View())
		b.WriteString("\n\n")
	}

	b.WriteString(styleHint.Render("[tab/↓] next   [shift+tab/↑] prev   [enter] continue   [esc] cancel"))
	return b.String()
}

func (m *model) renderFixPreview() string {
	var b strings.Builder
	b.WriteString(styleHeader.Render("Preview · "))
	b.WriteString(stylePartial.Render(m.fixPlan.SignalID))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(60, m.viewWidth())))
	b.WriteString("\n\n")

	if m.fixPlan.Summary != "" {
		b.WriteString(textwrap.Wrap(m.fixPlan.Summary, m.viewWidth()-2))
		b.WriteString("\n\n")
	}

	for _, op := range m.fixPlan.Ops {
		b.WriteString(styleHeader.Render(fmt.Sprintf("[%s] %s", op.Kind, op.Path)))
		b.WriteString("\n")
		b.WriteString(styleNA.Render(strings.Repeat("─", min(40, m.viewWidth()-2))))
		b.WriteString("\n")
		// Show first ~20 lines of body.
		lines := strings.Split(string(op.Body), "\n")
		shown := lines
		if len(shown) > 24 {
			shown = shown[:24]
		}
		for _, line := range shown {
			b.WriteString(styleNA.Render("│ "))
			b.WriteString(line)
			b.WriteString("\n")
		}
		if len(lines) > len(shown) {
			b.WriteString(styleNA.Render(fmt.Sprintf("… %d more lines …\n", len(lines)-len(shown))))
		}
		b.WriteString("\n")
	}

	b.WriteString(styleMissing.Render("Apply this fix? "))
	b.WriteString(styleHint.Render("[y] yes   [n/esc] cancel"))
	return b.String()
}

func (m *model) renderFixDone() string {
	var b strings.Builder
	b.WriteString(styleHeader.Render(fmt.Sprintf("Fix · %s", m.fixer.ID())))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(60, m.viewWidth())))
	b.WriteString("\n\n")

	if m.fixErr != nil {
		b.WriteString(styleMissing.Render("Failed: "))
		b.WriteString(m.fixErr.Error())
		b.WriteString("\n")
	} else {
		b.WriteString(styleFound.Render("✓ Applied:"))
		b.WriteString("\n")
		for _, op := range m.fixResult.Operations {
			marker := "  → "
			if op.Wrote {
				b.WriteString(marker + fmt.Sprintf("%s %s (%d bytes)\n", op.Kind, op.Path, op.Bytes))
			}
		}
		b.WriteString("\n")
		b.WriteString(styleHint.Render("Press r to rescan and see the updated verdict."))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(styleHint.Render("[r] rescan   [esc/enter] back   [q] quit"))
	return b.String()
}

// ===== helpers =====

func (m *model) viewWidth() int {
	if m.width <= 0 {
		return 80
	}
	return m.width
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
