package l3

import (
	"context"
	"regexp"
	"sort"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/internal/workflows"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// The L3 anti-pattern the paper calls the dashboard graveyard: metrics
// are collected diligently and then acted on by nobody. ACMM grades
// feedback-loop topology, so a publisher with no consumer is not a
// half-built L3 loop — it is an open loop wearing the costume of a
// closed one, and the cost is worse than not measuring, because the
// dashboard makes the team feel measured.
//
// This signal grades the *topology*, not the plumbing: does anything in
// this repo fail, block, or report on the numbers it collects? Whether a
// given upload authenticates is a runtime fact a static scan cannot see,
// and this detector does not pretend otherwise.
var (
	// Steps that ship a metric somewhere else to be looked at.
	metricsPublisherRE = regexp.MustCompile(`(?i)(codecov/codecov-action|coverallsapp/github-action|coveralls|sonarsource/|sonarqube|codeclimate|deepsource|datadog|honeycomb|actions/upload-artifact)`)

	// Uploads of things that aren't metrics. actions/upload-artifact is
	// how build outputs, logs, and fuzz corpora move between jobs, and
	// grading a repo for uploading its binaries would be nonsense.
	metricsArtifactRE = regexp.MustCompile(`(?i)(coverage|lcov|cobertura|junit|test-results|benchmark|profile|metrics)`)

	// Evidence that a number is consumed rather than merely displayed.
	// Both halves are required: a step that fails, *and* a number for it
	// to fail on. `exit 1` alone matches every gofmt check in existence,
	// which would let a repo pass this signal on the strength of a step
	// that has nothing to do with the metrics it publishes.
	metricsConsumerRE = regexp.MustCompile(`(?i)(exit\s+1|::error::|fail[-_ ]?(ci|below|under|on)|--fail)`)
	metricContextRE   = regexp.MustCompile(`(?i)(coverage|--cov|lcov|cobertura|benchmark|perf|latency|error[ _-]?rate|flake|flaky|threshold|floor|budget|regression|ratchet)`)

	uploadArtifactRE = regexp.MustCompile(`(?i)actions/upload-artifact`)
)

type MetricsActedOn struct{}

func (MetricsActedOn) ID() string        { return "l3.metrics-acted-on" }
func (MetricsActedOn) Level() acmm.Level { return acmm.LevelMeasured }
func (MetricsActedOn) Family() string    { return "metrics" }
func (MetricsActedOn) Title() string     { return "Collected metrics are acted on, not just published" }

func (s MetricsActedOn) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	publishers := map[string][]string{} // workflow path -> step descriptions

	for _, w := range idx.Workflows {
		for _, st := range w.AllSteps() {
			if !metricsPublisherRE.MatchString(st.Uses) {
				continue
			}
			// upload-artifact is only a metrics publisher when what it
			// uploads is a metric.
			if isGenericArtifactUpload(st) {
				continue
			}
			label := st.Name
			if label == "" {
				label = st.Uses
			}
			publishers[w.Path] = append(publishers[w.Path], label)
		}
	}

	if len(publishers) == 0 {
		// Nothing is published, so there is no graveyard to find. The
		// other L3 signals grade whether measurement exists at all; this
		// one only grades what happens to it afterwards.
		return acmm.Result{
			Status:     acmm.StatusNA,
			Score:      acmm.ScoreMissing,
			Confidence: acmm.ConfidenceHigh,
			Method:     acmm.MethodAST,
			Notes:      []string{"no metrics publishers in CI — nothing for this signal to grade"},
		}
	}

	paths := make([]string, 0, len(publishers))
	for p := range publishers {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	consumers := metricsConsumers(idx)
	evidence := make([]acmm.Evidence, 0, len(paths))
	for _, p := range paths {
		evidence = append(evidence, acmm.Evidence{Path: p, Excerpt: publishers[p][0]})
	}

	if len(consumers) == 0 {
		return acmm.Result{
			Status:     acmm.StatusMissing,
			Score:      acmm.ScoreMissing,
			Confidence: acmm.ConfidenceMedium,
			Method:     acmm.MethodAST,
			Evidence:   evidence,
			Notes: []string{
				"metrics are published but no step compares them against a threshold or fails on them",
				"dashboard graveyard: the L3 anti-pattern — collected, never acted on",
			},
			FixHint: "Make one of these numbers block something. A coverage floor that " +
				"fails the build, a perf budget that rejects a regression, an error-rate " +
				"threshold that pages someone. A metric nobody can fail is decoration, " +
				"and it costs more than not measuring because it feels like measuring.",
		}
	}

	// Published and consumed. Both halves of the loop exist; whether the
	// consumer reads the same metric the publisher ships is beyond what a
	// static scan can establish, hence medium confidence.
	evidence = append(evidence, consumers...)
	return acmm.Result{
		Status:     acmm.StatusFound,
		Score:      acmm.ScoreFound,
		Confidence: acmm.ConfidenceMedium,
		Method:     acmm.MethodAST,
		Evidence:   evidence,
		Notes:      []string{"metrics are published and at least one gate acts on a collected number"},
	}
}

// isGenericArtifactUpload reports whether an upload-artifact step is
// shipping something other than a metric — a binary, a log, a fuzz corpus.
func isGenericArtifactUpload(st workflows.Step) bool {
	if !uploadArtifactRE.MatchString(st.Uses) {
		return false
	}
	for _, v := range st.With {
		if metricsArtifactRE.MatchString(v) {
			return false
		}
	}
	return !metricsArtifactRE.MatchString(st.Name)
}

// metricsConsumers finds steps that act on a number: a threshold
// comparison that fails the job, or a scheduled analysis that writes a
// tracked artifact back into the repo.
func metricsConsumers(idx *scanner.RepoIndex) []acmm.Evidence {
	var out []acmm.Evidence
	seen := map[string]bool{}
	for _, w := range idx.Workflows {
		for _, st := range w.AllSteps() {
			scope := st.Run + "\n" + st.If + "\n" + st.Name
			if !metricsConsumerRE.MatchString(scope) || !metricContextRE.MatchString(scope) {
				continue
			}
			if seen[w.Path] {
				continue
			}
			seen[w.Path] = true
			label := st.Name
			if label == "" {
				label = "threshold comparison"
			}
			out = append(out, acmm.Evidence{Path: w.Path, Excerpt: label})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func init() {
	signals.Default.Register(MetricsActedOn{})
}
