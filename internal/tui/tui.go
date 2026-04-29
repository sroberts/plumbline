// Package tui implements the Bubble Tea interactive interface. The
// CLI mode is the load-bearing path; the TUI is a peer at feature
// parity (see SPEC.md §8).
package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sroberts/plumbline/internal/fix"
	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/internal/skill"
	"github.com/sroberts/plumbline/internal/textwrap"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// skillIsInstalled reports whether the canonical Claude Code skill
// path is already present in the scanned repo. Kept for backward-compat;
// the [i] install hint now uses anySkillTargetMissing instead so it
// advertises the action whenever ANY supported tool's file is absent.
func skillIsInstalled(idx *scanner.RepoIndex) bool {
	if idx == nil {
		return false
	}
	_, err := idx.Read(skill.Path)
	return err == nil
}

// anySkillTargetMissing reports whether the user could meaningfully
// install for at least one project-scope target. The TUI hides the
// [i] install skill hint only when every supported target is already
// present.
func anySkillTargetMissing(idx *scanner.RepoIndex) bool {
	if idx == nil {
		return true
	}
	for _, t := range skill.Targets() {
		if _, err := idx.Read(t.Path); err != nil {
			return true
		}
	}
	return false
}

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
	screenSkillTargets
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

	// Skill-install picker state. skillCursor tracks the highlighted
	// row; skillGlobal toggles project-scope vs user-scope install.
	skillCursor int
	skillGlobal bool
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
		case screenSkillTargets:
			return m.updateSkillTargets(msg)
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
	case "i":
		return m.startInstallSkill()
	}
	return m, nil
}

// startInstallSkill opens the target picker (screenSkillTargets) so
// the user can choose which coding-agent tool to install for and
// whether to install at project scope or user scope. No-op when the
// scan didn't produce an index yet.
func (m *model) startInstallSkill() (tea.Model, tea.Cmd) {
	if m.idx == nil {
		return m, nil
	}
	m.skillCursor = 0
	m.skillGlobal = false
	m.screen = screenSkillTargets
	return m, nil
}

// updateSkillTargets handles input on the skill-target picker.
//
//	↑/↓ or j/k    move highlight
//	g             toggle project / user scope
//	enter         build the plan and go to screenFixPreview
//	esc           back to results
//	q             quit
func (m *model) updateSkillTargets(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	targets := skill.Targets()
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "esc":
		m.screen = screenResults
		return m, nil
	case "down", "j":
		if m.skillCursor < len(targets)-1 {
			m.skillCursor++
		}
		return m, nil
	case "up", "k":
		if m.skillCursor > 0 {
			m.skillCursor--
		}
		return m, nil
	case "g":
		m.skillGlobal = !m.skillGlobal
		return m, nil
	case "enter":
		if m.skillCursor < 0 || m.skillCursor >= len(targets) {
			return m, nil
		}
		t := targets[m.skillCursor]
		var (
			plan acmm.FixPlan
			err  error
		)
		if m.skillGlobal {
			if !t.SupportsGlobal() {
				m.fixErr = fmt.Errorf("%s has no documented global location; press g to switch back to project scope", t.Name)
				m.screen = screenFixDone
				return m, nil
			}
			plan, err = skill.NewPlanForGlobal(t.ID)
		} else {
			plan, err = skill.NewPlanFor(t.ID)
		}
		if err != nil {
			m.fixErr = err
			m.screen = screenFixDone
			return m, nil
		}
		m.fixer = nil
		m.fixPlan = plan
		m.screen = screenFixPreview
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
		// Cancel: return to whichever screen got us here. Skill installs
		// arrived from the picker; signal fixes from the detail screen.
		m.fixer = nil
		m.fixPlan = acmm.FixPlan{}
		if m.skillGlobal || isSkillPlan(m.fixPlan) {
			m.screen = screenSkillTargets
		} else {
			m.screen = screenDetail
		}
		return m, nil
	case "y":
		root := m.report.Repo
		// Skill installs at user scope use $HOME instead of repo root.
		if m.skillGlobal && isSkillPlan(m.fixPlan) {
			home, err := os.UserHomeDir()
			if err != nil {
				m.fixErr = fmt.Errorf("could not resolve user home dir: %w", err)
				m.screen = screenFixDone
				return m, nil
			}
			root = home
		}
		res, err := fix.Apply(root, m.fixPlan, fix.Options{})
		if err != nil {
			m.fixErr = err
		}
		m.fixResult = res
		m.fixApplied = err == nil
		m.screen = screenFixDone
	}
	return m, nil
}

// isSkillPlan reports whether the given FixPlan came from the
// install-skill flow (vs a Signal fixer). The skill plan IDs are
// "install-skill:<target>"; signal plans use the signal ID.
func isSkillPlan(p acmm.FixPlan) bool {
	return strings.HasPrefix(p.SignalID, "install-skill:")
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
	case screenSkillTargets:
		return m.renderSkillTargets()
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
	hint := "[↑/↓] select   [enter] detail   [r] rescan   [✚=fixable]"
	if anySkillTargetMissing(m.idx) {
		hint += "   [i] install skill"
	}
	hint += "   [q] quit"
	b.WriteString(m.renderHint(hint))
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
	b.WriteString(m.renderHint(hint))
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

	b.WriteString(m.renderHint("[tab/↓] next   [shift+tab/↑] prev   [enter] continue   [esc] cancel"))
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
	b.WriteString(m.renderHint("[y] yes   [n/esc] cancel"))
	return b.String()
}

func (m *model) renderFixDone() string {
	var b strings.Builder
	// fixer is nil for the install-skill flow (no Signal involved); use
	// the FixPlan's SignalID as the heading instead.
	id := m.fixPlan.SignalID
	if m.fixer != nil {
		id = m.fixer.ID()
	}
	b.WriteString(styleHeader.Render(fmt.Sprintf("Fix · %s", id)))
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
	b.WriteString(m.renderHint("[r] rescan   [esc/enter] back   [q] quit"))
	return b.String()
}

// renderSkillTargets shows the install-skill target picker. Each row
// shows the tool name + install path, marked "(installed)" if already
// present. The header shows the current scope (project / user) and
// the [g] toggle.
func (m *model) renderSkillTargets() string {
	var b strings.Builder
	b.WriteString(styleHeader.Render("Install plumbline skill — pick a coding-agent tool"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(60, m.viewWidth())))
	b.WriteString("\n")

	scope := "project (this repo)"
	if m.skillGlobal {
		scope = "user (global, ~/)"
	}
	b.WriteString(fmt.Sprintf("Scope: %s\n", styleHeader.Render(scope)))
	b.WriteString(m.renderHint("[g] toggle scope"))
	b.WriteString("\n\n")

	targets := skill.Targets()
	for i, t := range targets {
		path := t.Path
		marker := "  "
		if m.skillGlobal {
			if !t.SupportsGlobal() {
				path = styleNA.Render("(no global location)")
			} else {
				path = "~/" + t.GlobalPath
			}
		}
		// "installed" badge — for project scope only; global presence
		// is harder to detect without home-dir reads we don't want here.
		if !m.skillGlobal && m.idx != nil {
			if _, err := m.idx.Read(t.Path); err == nil {
				marker = styleFound.Render("✓ ")
			}
		}
		shared := ""
		if t.SharedFile {
			shared = styleNA.Render("  (shared file)")
		}
		line := fmt.Sprintf("%s%-10s  %-22s  %s%s", marker, t.ID, t.Name, path, shared)
		if i == m.skillCursor {
			line = styleSelected.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.renderHint("[↑/↓] select   [enter] preview   [g] scope   [esc] back   [q] quit"))
	return b.String()
}

// ===== helpers =====

func (m *model) viewWidth() int {
	if m.width <= 0 {
		return 80
	}
	return m.width
}

// renderHint word-wraps a footer hint to the current viewport width
// and applies the hint style. Every screen's footer goes through this
// so narrow terminals don't truncate keybindings (issue #19).
func (m *model) renderHint(hint string) string {
	w := m.viewWidth()
	if w <= 0 {
		w = 80
	}
	return styleHint.Render(textwrap.Wrap(hint, w))
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
