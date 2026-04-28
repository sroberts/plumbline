package l3

import (
	"bytes"
	"context"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// errorMonitorMarkers are substrings whose presence in any dependency
// manifest or source file indicates an error-monitoring SDK is wired up.
var errorMonitorMarkers = [][]byte{
	[]byte("@sentry/"),
	[]byte("github.com/getsentry/sentry-go"),
	[]byte("sentry_sdk"),
	[]byte("@sentry/browser"),
	[]byte("@sentry/node"),
	[]byte("opentelemetry"),
	[]byte("@opentelemetry/"),
	[]byte("go.opentelemetry.io/otel"),
	[]byte("Bugsnag"),
	[]byte("rollbar"),
	[]byte("datadog/dd-trace"),
}

// errorMonitorPaths are dependency manifests we'll inspect.
var errorMonitorPaths = []string{
	"package.json",
	"go.mod",
	"requirements.txt",
	"pyproject.toml",
	"Pipfile",
	"Gemfile",
	"Cargo.toml",
}

type ErrorMonitoring struct{}

func (ErrorMonitoring) ID() string        { return "l3.error-monitoring" }
func (ErrorMonitoring) Level() acmm.Level { return acmm.LevelMeasured }
func (ErrorMonitoring) Family() string    { return "monitoring" }
func (ErrorMonitoring) Title() string     { return "Error-monitoring SDK declared in dependency manifest" }

func (s ErrorMonitoring) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	for _, p := range errorMonitorPaths {
		data := readOrEmpty(idx, p)
		if len(data) == 0 {
			continue
		}
		for _, marker := range errorMonitorMarkers {
			if bytes.Contains(data, marker) {
				return acmm.Result{
					Status:     acmm.StatusFound,
					Score:      acmm.ScoreFound,
					Confidence: acmm.ConfidenceMedium,
					Method:     acmm.MethodContentRegex,
					Evidence:   []acmm.Evidence{{Path: p, Excerpt: string(marker)}},
				}
			}
		}
	}
	return acmm.Result{
		Status:     acmm.StatusMissing,
		Score:      acmm.ScoreMissing,
		Confidence: acmm.ConfidenceLow,
		Method:     acmm.MethodContentRegex,
		Notes: []string{
			"no Sentry / OpenTelemetry / Bugsnag / similar dependency found",
			"low confidence — only checks dependency manifests, not runtime config",
		},
		FixHint: "Wire in an error-monitoring SDK (Sentry, OpenTelemetry, or " +
			"equivalent). Without runtime error signals, AI agents can't " +
			"distinguish 'this fix worked' from 'this fix silently broke prod.'",
	}
}

func init() {
	signals.Default.Register(ErrorMonitoring{})
}
