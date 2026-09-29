package l3

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// The asymmetry of verification: an agent can generate far faster than it
// can check, so how quickly and deterministically a repo answers "did my
// change work?" bounds what an agent can safely do there. This signal
// grades that answer from the filesystem. It is the one signal
// internal/scoring treats as a gate (SPEC.md §7 "Verification gate").
//
// It detects the *shape* — a single local command that runs tests, tests
// that exist, and CI that runs them — not one ecosystem's spelling, per
// SPEC.md §6 "the CI gates detect ecosystem shapes".

var (
	testFileRE = regexp.MustCompile(`(?i)(_test\.(go|py|rs)$|(^|/)test_[^/]+\.py$|\.(test|spec)\.[cm]?[jt]sx?$|_spec\.rb$|(^|/)tests?/[^/]+\.(py|rb|rs|[jt]sx?)$|Tests?\.(java|kt|cs)$)`)

	// Makefile/justfile targets. Anchored at column 0 so `.PHONY: test`
	// and recipe bodies do not match.
	makeTestTargetRE = regexp.MustCompile(`(?m)^(test|tests|check|verify|ci)[\w.-]*\s*:`)
	makeLintTargetRE = regexp.MustCompile(`(?m)^(lint|vet|fmt|format|check|verify|ci)[\w.-]*\s*:`)

	// Taskfile.yml nests tasks under `tasks:`, so the key is indented.
	taskTestRE = regexp.MustCompile(`(?m)^\s{1,4}(test|tests|check|verify|ci)[\w:.-]*:\s*$`)
	taskLintRE = regexp.MustCompile(`(?m)^\s{1,4}(lint|vet|fmt|format|check|verify|ci)[\w:.-]*:\s*$`)

	pyTestConfigRE = regexp.MustCompile(`(?m)^\[(tool\.pytest|tool\.tox|tool\.nox)`)

	ciTestRunRE = regexp.MustCompile(`\b(go test|pytest|py\.test|npm (run )?test|npm t|yarn (run )?test|pnpm (run )?test|bun test|cargo test|cargo nextest|make (test|check|ci|verify)|task (test|check|ci)|just (test|check|ci)|tox|nox|mvn (-\S+ )*(test|verify)|gradlew? (\S+ )*test|bundle exec rspec|rspec|dotnet test|jest|vitest|mocha)\b`)
)

// VerificationHarness detects whether an agent can verify its own work
// locally, deterministically, with one command.
type VerificationHarness struct{}

func (VerificationHarness) ID() string        { return "l3.verification-harness" }
func (VerificationHarness) Level() acmm.Level { return acmm.LevelMeasured }
func (VerificationHarness) Family() string    { return "verification" }
func (VerificationHarness) Title() string {
	return "Deterministic local verification: runnable tests, a lint/check command, and CI that runs them"
}

type harnessFacts struct {
	testFile   string
	testEntry  string // path of the file that defines the one-command test entrypoint
	lintEntry  string
	ciTestPath string
}

func (s VerificationHarness) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	f := gatherHarnessFacts(idx)

	hasEntry := f.testEntry != ""
	hasTests := f.testFile != ""
	hasLint := f.lintEntry != ""
	hasCI := f.ciTestPath != ""

	res := acmm.Result{
		Confidence: acmm.ConfidenceMedium,
		Method:     acmm.MethodCrossFile,
	}
	for _, p := range []string{f.testEntry, f.lintEntry, f.testFile, f.ciTestPath} {
		if p != "" && !hasEvidence(res.Evidence, p) {
			res.Evidence = append(res.Evidence, acmm.Evidence{Path: p})
		}
	}

	switch {
	case !hasEntry && !hasTests:
		res.Status, res.Score = acmm.StatusMissing, acmm.ScoreMissing
		res.Confidence = acmm.ConfidenceHigh
		res.Method = acmm.MethodFilenameMatch
		res.Notes = []string{"no test files and no local test entrypoint (Makefile/Taskfile/justfile target, package.json script, or language-native runner)"}
		res.FixHint = "Add one test and one command that runs it (`make test`, an npm " +
			"`test` script, or plain `go test ./...`). A basic test beats none: it " +
			"is the anchor an agent iterates against. Until a repo has this, " +
			"plumbline caps its verdict at L2 — an agent cannot check its own work."
	case !hasEntry || !hasTests:
		res.Status, res.Score = acmm.StatusPartial, acmm.ScoreStubbed
		if !hasEntry {
			res.Notes = []string{"test files exist but no one-command local entrypoint runs them"}
			res.FixHint = "Add a `test` target to a Makefile/Taskfile/justfile (or a `test` script " +
				"in package.json) so an agent can run the suite without reading the CI config."
		} else {
			res.Notes = []string{"a test entrypoint exists (" + f.testEntry + ") but no test files were found"}
			res.FixHint = "Add at least one real test. An entrypoint that runs nothing verifies nothing."
		}
	case !hasCI || !hasLint:
		res.Status, res.Score = acmm.StatusPartial, acmm.ScoreIncomplete
		var missing []string
		if !hasLint {
			missing = append(missing, "a local lint/check command")
		}
		if !hasCI {
			missing = append(missing, "a push/pull_request workflow that runs the tests")
		}
		res.Notes = []string{"local tests are runnable; missing " + strings.Join(missing, " and ")}
		res.FixHint = "Add " + strings.Join(missing, " and ") + ". Local and CI verification " +
			"should be the same command, so a green local run predicts a green PR."
	default:
		res.Status, res.Score = acmm.StatusFound, acmm.ScoreFound
		res.Notes = []string{"tests runnable locally via " + f.testEntry + ", lint/check via " + f.lintEntry + ", and run in CI (" + f.ciTestPath + ")"}
	}
	return res
}

func gatherHarnessFacts(idx *scanner.RepoIndex) harnessFacts {
	var f harnessFacts

	for _, fe := range idx.Files {
		if testFileRE.MatchString(fe.Path) && !isFixturePath(fe.Path) {
			f.testFile = fe.Path
			break
		}
	}

	for _, name := range []string{"Makefile", "makefile", "GNUmakefile"} {
		if data := readOrEmpty(idx, name); data != nil {
			if f.testEntry == "" && makeTestTargetRE.Match(data) {
				f.testEntry = name
			}
			if f.lintEntry == "" && makeLintTargetRE.Match(data) {
				f.lintEntry = name
			}
		}
	}
	for _, name := range []string{"Taskfile.yml", "Taskfile.yaml"} {
		if data := readOrEmpty(idx, name); data != nil {
			if f.testEntry == "" && taskTestRE.Match(data) {
				f.testEntry = name
			}
			if f.lintEntry == "" && taskLintRE.Match(data) {
				f.lintEntry = name
			}
		}
	}
	for _, name := range []string{"justfile", "Justfile"} {
		if data := readOrEmpty(idx, name); data != nil {
			if f.testEntry == "" && makeTestTargetRE.Match(data) {
				f.testEntry = name
			}
			if f.lintEntry == "" && makeLintTargetRE.Match(data) {
				f.lintEntry = name
			}
		}
	}
	for _, p := range idx.ByName["package.json"] {
		test, lint := packageScripts(readOrEmpty(idx, p))
		if f.testEntry == "" && test {
			f.testEntry = p
		}
		if f.lintEntry == "" && lint {
			f.lintEntry = p
		}
	}

	// Python runners are configured, not invoked through a wrapper.
	if f.testEntry == "" {
		for _, name := range []string{"pytest.ini", "tox.ini", "noxfile.py"} {
			if len(idx.ByName[name]) > 0 {
				f.testEntry = idx.ByName[name][0]
				break
			}
		}
	}
	if f.testEntry == "" {
		if data := readOrEmpty(idx, "pyproject.toml"); data != nil && pyTestConfigRE.Match(data) {
			f.testEntry = "pyproject.toml"
		}
	}

	// Language-native toolchains are their own entrypoint: `go test ./...`
	// needs no wrapper and every Go contributor knows it. Requiring a
	// Makefile here would score the standard toolchain as absent — the
	// SPEC.md §6 detector-gap failure. Native entrypoints still need
	// tests to exist; that is checked by the caller.
	if p := firstNonFixture(idx.ByName["go.mod"]); p != "" {
		if f.testEntry == "" {
			f.testEntry = p // `go test ./...`
		}
		if f.lintEntry == "" {
			f.lintEntry = p // `go vet ./...`
		}
	}
	if p := firstNonFixture(idx.ByName["Cargo.toml"]); p != "" {
		if f.testEntry == "" {
			f.testEntry = p // `cargo test`
		}
		if f.lintEntry == "" {
			f.lintEntry = p // `cargo clippy`
		}
	}

	for _, w := range idx.Workflows {
		if !(w.HasPushTrigger() || w.HasPullRequestTrigger()) {
			continue
		}
		if w.AnyRunMatches(ciTestRunRE) {
			f.ciTestPath = w.Path
			break
		}
	}
	return f
}

// packageScripts reports whether a package.json defines a real test
// script and a lint/check script. `npm init` writes a placeholder test
// script that exits 1 ("no test specified"); it is not a test.
func packageScripts(data []byte) (test, lint bool) {
	if data == nil {
		return false, false
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return false, false
	}
	for name, body := range pkg.Scripts {
		switch {
		case (name == "test" || strings.HasPrefix(name, "test:")) && !strings.Contains(body, "no test specified"):
			test = true
		case name == "lint" || strings.HasPrefix(name, "lint:") || name == "check" || name == "typecheck" || name == "format:check":
			lint = true
		}
	}
	return test, lint
}

// isFixturePath excludes files under testdata/fixtures directories: they
// are inputs to tests, and a tool that scans other repos (plumbline's own
// canaries) ships many of them.
func isFixturePath(p string) bool {
	return strings.Contains(p, "testdata/") || strings.Contains(p, "fixtures/")
}

func firstNonFixture(paths []string) string {
	for _, p := range paths {
		if !isFixturePath(p) {
			return p
		}
	}
	return ""
}

func hasEvidence(ev []acmm.Evidence, path string) bool {
	for _, e := range ev {
		if e.Path == path {
			return true
		}
	}
	return false
}

func init() {
	signals.Default.Register(VerificationHarness{})
}
