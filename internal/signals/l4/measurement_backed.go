package l4

import (
	"context"
	"regexp"
	"sort"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// The L4 anti-pattern the paper calls autonomy without guardrails:
// automation that changes the repository without the measurement layer
// that would catch it changing the wrong thing. ACMM is explicit that
// levels are sequential — L4 without L3 is not "most of the way to L4",
// it is a machine committing to a codebase nothing is checking.
//
// Framed positively, like l3.metrics-acted-on, so Found keeps meaning
// good across the catalog: is the autonomy in this repo backed by a gate
// that can stop it?
//
// Scope, stated because it bounds what a passing result means: this
// grades whether a blocking gate exists in the same repo as the
// autonomy. It cannot tell whether that gate covers the code path the
// automation touches, nor whether required-status-check settings
// actually enforce it — branch protection is repo configuration, not
// filesystem state, and plumbline does not read it.
var (
	// Automation that mutates the repo or its issue tracker without a
	// human in the path.
	autonomyUsesRE = regexp.MustCompile(`(?i)(peter-evans/create-pull-request|peter-evans/enable-pull-request-automerge|pascalgn/automerge-action|actions/github-script)`)
	autonomyRunRE  = regexp.MustCompile(`(?m)(git\s+push|gh\s+pr\s+(merge|create)|gh\s+issue\s+(create|close|edit)|--auto\b)`)

	// A gate that can stop a change: something that fails, in a workflow
	// that runs on the change itself rather than on a schedule.
	gateRunRE  = regexp.MustCompile(`(?m)(exit\s+1|::error::|go\s+test|pytest|npm\s+(run\s+)?test|cargo\s+test|--fail)`)
	gateUsesRE = regexp.MustCompile(`(?i)(golangci|staticcheck|codecov|sonarsource|reviewdog)`)
)

type MeasurementBacked struct{}

func (MeasurementBacked) ID() string        { return "l4.measurement-backed" }
func (MeasurementBacked) Level() acmm.Level { return acmm.LevelAdaptive }
func (MeasurementBacked) Family() string    { return "guardrails" }
func (MeasurementBacked) Title() string {
	return "Repo-modifying automation is backed by a blocking gate"
}

func (s MeasurementBacked) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	autonomy := map[string]string{} // workflow path -> what was found
	for _, w := range idx.Workflows {
		for _, j := range w.Jobs {
			for _, st := range j.Steps {
				switch {
				case autonomyUsesRE.MatchString(st.Uses):
					recordFirst(autonomy, w.Path, label(st.Name, st.Uses))
				case autonomyRunRE.MatchString(st.Run):
					recordFirst(autonomy, w.Path, label(st.Name, "repo-modifying command"))
				}
			}
		}
	}

	if len(autonomy) == 0 {
		// No autonomy, so no unguarded autonomy. Excluded from level
		// math: the other L4 signals grade whether automation exists.
		return acmm.Result{
			Status:     acmm.StatusNA,
			Score:      acmm.ScoreMissing,
			Confidence: acmm.ConfidenceHigh,
			Method:     acmm.MethodAST,
			Notes:      []string{"no repo-modifying automation — nothing for this signal to grade"},
		}
	}

	gates := blockingGates(idx)
	evidence := evidenceFor(autonomy)

	if len(gates) == 0 {
		return acmm.Result{
			Status:     acmm.StatusMissing,
			Score:      acmm.ScoreMissing,
			Confidence: acmm.ConfidenceMedium,
			Method:     acmm.MethodAST,
			Evidence:   evidence,
			Notes: []string{
				"automation modifies the repo but no push/PR-triggered workflow can fail",
				"autonomy without guardrails: the L4 anti-pattern — a level cannot be skipped",
			},
			FixHint: "Add a push/pull_request workflow that runs the test suite and fails " +
				"on a red result, before adding more automation. L4 is L3 plus a closed " +
				"loop; without the L3 measurement layer, automation is a machine " +
				"committing to a codebase nothing is checking.",
		}
	}

	return acmm.Result{
		Status:     acmm.StatusFound,
		Score:      acmm.ScoreFound,
		Confidence: acmm.ConfidenceMedium,
		Method:     acmm.MethodAST,
		Evidence:   append(evidence, gates...),
		Notes: []string{
			"repo-modifying automation exists alongside a blocking push/PR gate",
			"does not verify the gate covers what the automation touches, or that branch protection requires it",
		},
	}
}

// blockingGates finds workflows that run on the change itself and contain
// something that can fail it. Scheduled workflows are excluded on
// purpose: a nightly suite reports, it does not block a merge.
func blockingGates(idx *scanner.RepoIndex) []acmm.Evidence {
	var out []acmm.Evidence
	for _, w := range idx.Workflows {
		if !(w.HasPullRequestTrigger() || w.HasPushTrigger()) {
			continue
		}
		for _, st := range w.AllSteps() {
			if gateRunRE.MatchString(st.Run) || gateUsesRE.MatchString(st.Uses) {
				out = append(out, acmm.Evidence{Path: w.Path, Excerpt: label(st.Name, "blocking gate")})
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func recordFirst(m map[string]string, path, what string) {
	if _, seen := m[path]; !seen {
		m[path] = what
	}
}

func label(name, fallback string) string {
	if name != "" {
		return name
	}
	return fallback
}

func evidenceFor(m map[string]string) []acmm.Evidence {
	paths := make([]string, 0, len(m))
	for p := range m {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	out := make([]acmm.Evidence, 0, len(paths))
	for _, p := range paths {
		out = append(out, acmm.Evidence{Path: p, Excerpt: m[p]})
	}
	return out
}

func init() {
	signals.Default.Register(MeasurementBacked{})
}
