package l3

import (
	"context"
	"regexp"
	"strings"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// Agents produce plausible code in *their* style unless a tool rejects
// it. Committed, opinionated lint and format configuration is what turns
// "AI slop" into a failing check instead of a review comment. Presence of
// a linter is not enough (l3.build-lint-gate covers that CI runs one);
// this asks whether the repo *declares its opinions* and covers formatting.

type lintTool struct {
	name     string
	paths    *regexp.Regexp // config file path
	inPyproj string         // pyproject.toml table marker, if any
	strict   *regexp.Regexp // explicit rule selection inside the config
	lints    bool
	formats  bool
}

var lintTools = []lintTool{
	{name: "golangci-lint", paths: regexp.MustCompile(`(^|/)\.golangci\.(ya?ml|toml|json)$`),
		strict: regexp.MustCompile(`(?m)^\s*(enable|enable-all|default)\s*:`), lints: true},
	{name: "ruff", paths: regexp.MustCompile(`(^|/)\.?ruff\.toml$`), inPyproj: "[tool.ruff",
		strict: regexp.MustCompile(`(?m)^\s*(select|extend-select)\s*=`), lints: true, formats: true},
	{name: "biome", paths: regexp.MustCompile(`(^|/)biome\.jsonc?$`),
		strict: regexp.MustCompile(`"(recommended|rules)"`), lints: true, formats: true},
	{name: "eslint", paths: regexp.MustCompile(`(^|/)(\.eslintrc(\.[\w]+)?|eslint\.config\.[cm]?[jt]s)$`),
		strict: regexp.MustCompile(`(extends|rules|recommended|configs)`), lints: true},
	{name: "prettier", paths: regexp.MustCompile(`(^|/)(\.prettierrc(\.[\w]+)?|prettier\.config\.[cm]?[jt]s)$`), formats: true},
	{name: "rubocop", paths: regexp.MustCompile(`(^|/)\.rubocop\.yml$`),
		strict: regexp.MustCompile(`(?m)^\s*(Enabled|inherit_from|AllCops)`), lints: true, formats: true},
	{name: "flake8", paths: regexp.MustCompile(`(^|/)\.flake8$`),
		strict: regexp.MustCompile(`(?m)^\s*(select|extend-select|max-complexity)\s*=`), lints: true},
	{name: "pylint", paths: regexp.MustCompile(`(^|/)\.pylintrc$`),
		strict: regexp.MustCompile(`(?m)^\s*(enable|disable)\s*=`), lints: true},
	{name: "clippy", paths: regexp.MustCompile(`(^|/)\.?clippy\.toml$`), lints: true,
		strict: regexp.MustCompile(`\S`)},
	{name: "rustfmt", paths: regexp.MustCompile(`(^|/)\.?rustfmt\.toml$`), formats: true},
	{name: "stylelint", paths: regexp.MustCompile(`(^|/)(\.stylelintrc(\.[\w]+)?|stylelint\.config\.[cm]?js)$`),
		strict: regexp.MustCompile(`(extends|rules)`), lints: true},
}

// LintConfig detects committed, opinionated lint and format configuration.
type LintConfig struct{}

func (LintConfig) ID() string        { return "l3.lint-config" }
func (LintConfig) Level() acmm.Level { return acmm.LevelMeasured }
func (LintConfig) Family() string    { return "verification" }
func (LintConfig) Title() string {
	return "Opinionated lint and format configuration committed (golangci-lint / ruff / biome / eslint / prettier)"
}

func (s LintConfig) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	var (
		evidence       []acmm.Evidence
		names          []string
		lints, formats bool
		strict         bool
		hasGo          = len(idx.ByName["go.mod"]) > 0
		pyproject      = readOrEmpty(idx, "pyproject.toml")
		seenTool       = map[string]bool{}
		record         = func(t lintTool, path string, data []byte) {
			if !seenTool[t.name] {
				seenTool[t.name] = true
				names = append(names, t.name)
			}
			evidence = append(evidence, acmm.Evidence{Path: path})
			lints = lints || t.lints
			formats = formats || t.formats
			if t.lints && t.strict != nil && t.strict.Match(data) {
				strict = true
			}
		}
	)

	for _, t := range lintTools {
		for _, fe := range idx.Files {
			if isFixturePath(fe.Path) || !t.paths.MatchString(fe.Path) {
				continue
			}
			record(t, fe.Path, readOrEmpty(idx, fe.Path))
			break
		}
		if t.inPyproj != "" && !seenTool[t.name] && pyproject != nil && strings.Contains(string(pyproject), t.inPyproj) {
			record(t, "pyproject.toml", pyproject)
		}
	}
	// package.json can carry eslint/prettier config inline.
	for _, p := range idx.ByName["package.json"] {
		data := string(readOrEmpty(idx, p))
		if !seenTool["eslint"] && strings.Contains(data, `"eslintConfig"`) {
			record(lintTools[3], p, []byte(data))
		}
		if !seenTool["prettier"] && strings.Contains(data, `"prettier"`) {
			record(lintTools[4], p, []byte(data))
		}
	}
	// gofmt ships with the Go toolchain: a Go repo has formatter coverage
	// without any config file, and `go vet` is a built-in linter. Only
	// *opinionated* configuration (a golangci file) counts as strict.
	if hasGo {
		formats = true
	}

	res := acmm.Result{
		Confidence: acmm.ConfidenceMedium,
		Method:     acmm.MethodContentRegex,
		Evidence:   evidence,
	}
	switch {
	case lints && strict && formats:
		res.Status, res.Score = acmm.StatusFound, acmm.ScoreFound
		res.Notes = []string{"explicit lint rules and formatter coverage: " + strings.Join(names, ", ")}
	case lints:
		res.Status, res.Score = acmm.StatusPartial, acmm.ScoreIncomplete
		var gaps []string
		if !strict {
			gaps = append(gaps, "explicit rule selection (config relies on tool defaults)")
		}
		if !formats {
			gaps = append(gaps, "a formatter")
		}
		res.Notes = []string{"linter configured (" + strings.Join(names, ", ") + ") but lacks " + strings.Join(gaps, " and ")}
		res.FixHint = "Declare the rules you want enforced rather than inheriting defaults, and add a " +
			"formatter (prettier/biome/ruff format/gofmt) so style drift fails a check."
	case formats && len(names) > 0:
		res.Status, res.Score = acmm.StatusPartial, acmm.ScoreStubbed
		res.Notes = []string{"formatter configured (" + strings.Join(names, ", ") + ") but no linter configuration"}
		res.FixHint = "Add a linter config (eslint, biome, ruff, golangci-lint) that names the rules to enforce."
	default:
		res.Status, res.Score = acmm.StatusMissing, acmm.ScoreMissing
		res.Confidence = acmm.ConfidenceHigh
		res.Method = acmm.MethodFilenameMatch
		res.Notes = []string{"no committed linter or formatter configuration found"}
		res.FixHint = "Commit an opinionated lint config (.golangci.yml, ruff/biome/eslint config) and a " +
			"formatter. Agents reproduce the style they see; a failing check is the only reliable correction."
	}
	return res
}

func init() {
	signals.Default.Register(LintConfig{})
}
