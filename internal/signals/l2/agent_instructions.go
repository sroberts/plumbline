package l2

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// agentInstructionsPaths lists every agent-instructions filename
// plumbline recognizes, in priority order. The first one found wins
// for evidence-citation purposes; presence of *any* of them is enough
// for the signal to fire.
//
// Priority is "what the user most likely cares about." CLAUDE.md is
// first because it's the most common at the moment; AGENTS.md is the
// emerging multi-agent convention; the rest are tool-specific.
var agentInstructionsPaths = []string{
	"CLAUDE.md",
	"AGENTS.md",
	".github/copilot-instructions.md",
	".cursorrules",
	".windsurfrules",
}

// agentInstructionsLineThreshold is the bar for "substantive" content.
// Lower than CLAUDE.md's old 30-line bar because per-tool rule files
// (Cursor, Windsurf) are routinely shorter and still useful.
const agentInstructionsLineThreshold = 20

// AgentInstructions detects whether the repo encodes preferences for
// any AI coding agent (Claude Code, Codex, Copilot, Cursor, Windsurf,
// or the AGENTS.md convention). Most users use one agent — there's no
// reason to require directives for all of them.
//
// This is a deliberate deviation from the source paper, which lists
// CLAUDE.md and Copilot instructions as separate L2 feedback loops.
// Rationale documented in SPEC.md §6 "Deviations from the source paper".
type AgentInstructions struct{}

func (AgentInstructions) ID() string        { return "l2.agent-instructions" }
func (AgentInstructions) Level() acmm.Level { return acmm.LevelInstructed }
func (AgentInstructions) Family() string    { return "instructions" }
func (AgentInstructions) Title() string {
	return "Agent instructions present (CLAUDE.md / AGENTS.md / copilot-instructions / etc.)"
}

func (s AgentInstructions) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	for _, path := range agentInstructionsPaths {
		data, err := idx.Read(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			continue
		}
		return scoreAgentFile(path, data)
	}

	// None of the known agent-instructions files exist.
	return acmm.Result{
		Status:     acmm.StatusMissing,
		Score:      acmm.ScoreMissing,
		Confidence: acmm.ConfidenceHigh,
		Method:     acmm.MethodFilenameMatch,
		Notes:      []string{"no agent-instructions file found at any known path: " + listKnownPaths()},
		FixHint: "Add ONE agent-instructions file at the repo root with a " +
			"heading and ~20 lines covering this project's conventions, " +
			"architecture, and the kinds of changes you want AI agents to " +
			"avoid. Most common: CLAUDE.md (Claude Code) or AGENTS.md " +
			"(multi-agent convention). copilot-instructions.md, " +
			".cursorrules, and .windsurfrules also work — pick whichever " +
			"matches your team's tooling.",
	}
}

func scoreAgentFile(path string, data []byte) acmm.Result {
	hasHeading := containsHeading(data)
	nonBlank := countNonBlankLines(data)
	hasBody := nonBlank >= agentInstructionsLineThreshold

	score := acmm.ScoreStubbed
	switch {
	case hasHeading && hasBody:
		score = acmm.ScoreFound
	case hasHeading || hasBody:
		score = acmm.ScoreIncomplete
	}

	res := acmm.Result{
		Status:     acmm.StatusFromScore(score),
		Score:      score,
		Confidence: acmm.ConfidenceMedium,
		Method:     acmm.MethodContentRegex,
		Evidence: []acmm.Evidence{{
			Path:    path,
			Excerpt: excerpt(data, 160),
		}},
	}

	if score == acmm.ScoreFound {
		res.Notes = []string{fmt.Sprintf("%s has a heading and %d non-blank lines (≥%d)",
			path, nonBlank, agentInstructionsLineThreshold)}
		return res
	}

	if !hasHeading {
		res.Notes = append(res.Notes, fmt.Sprintf("%s has no markdown heading (a line starting with '#')", path))
	}
	if !hasBody {
		res.Notes = append(res.Notes, fmt.Sprintf("%s has only %d non-blank lines (need ≥%d for Found)",
			path, nonBlank, agentInstructionsLineThreshold))
	}
	switch {
	case !hasHeading && !hasBody:
		res.FixHint = fmt.Sprintf("Add a heading (e.g. '# %s') and expand the file with ~20 lines of project conventions, architecture, and anti-patterns.", path)
	case !hasHeading:
		res.FixHint = fmt.Sprintf("Add a top-level markdown heading at the start of %s.", path)
	case !hasBody:
		res.FixHint = fmt.Sprintf("Expand %s from %d to ≥%d non-blank lines covering conventions, architecture, common pitfalls, and what AI agents should *not* do.",
			path, nonBlank, agentInstructionsLineThreshold)
	}
	return res
}

func listKnownPaths() string {
	out := ""
	for i, p := range agentInstructionsPaths {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}

func init() {
	signals.Default.Register(AgentInstructions{})
}
