// Package acmm holds the public types for plumbline's ACMM assessor.
//
// These types are the API surface consumed by --json output, the JSON
// schemas published via `plumbline schema`, and any external tools that
// parse plumbline results. Backwards-incompatible changes here require
// a major version bump of the schema $id (see SPEC.md §7 signal-set
// versioning and §9.5 schema contract tests).
package acmm

// Level is an ACMM maturity level (1–5). Level 1 is the implicit floor
// — every repo has it. Levels 2–5 each correspond to a feedback-loop
// topology described in the source paper.
type Level int

const (
	LevelAssisted       Level = 1
	LevelInstructed     Level = 2
	LevelMeasured       Level = 3
	LevelAdaptive       Level = 4
	LevelSelfSustaining Level = 5
)

// Name returns the human-readable name of the level.
func (l Level) Name() string {
	switch l {
	case LevelAssisted:
		return "Assisted"
	case LevelInstructed:
		return "Instructed"
	case LevelMeasured:
		return "Measured"
	case LevelAdaptive:
		return "Adaptive"
	case LevelSelfSustaining:
		return "Self-Sustaining"
	default:
		return "Unknown"
	}
}

// Status is the qualitative result of running a single signal against
// a repository. Status is derived from Score per the rubric; signals
// do not set Status directly.
type Status string

const (
	StatusFound   Status = "found"
	StatusPartial Status = "partial"
	StatusMissing Status = "missing"
	StatusNA      Status = "na"
)

// Score values from the mandatory four-step partial-credit rubric.
// See SPEC.md §6.
const (
	ScoreMissing    = 0.0  // artifact absent
	ScoreStubbed    = 0.33 // named or stubbed only
	ScoreIncomplete = 0.67 // present but incomplete
	ScoreFound      = 1.0  // fully wired
)

// StatusFromScore maps the rubric score onto a Status. Any score not
// equal to one of the four rubric values is treated as Partial; signal
// authors are expected to use only the rubric values.
func StatusFromScore(score float64) Status {
	switch score {
	case ScoreMissing:
		return StatusMissing
	case ScoreFound:
		return StatusFound
	default:
		return StatusPartial
	}
}

// Confidence is independent of Score and answers a different question:
// how trustworthy is this verdict? See SPEC.md §6 confidence ladder.
type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

// AtLeast reports whether c is at or above the named minimum.
func (c Confidence) AtLeast(min Confidence) bool {
	rank := map[Confidence]int{ConfidenceLow: 0, ConfidenceMedium: 1, ConfidenceHigh: 2}
	return rank[c] >= rank[min]
}

// Method describes how a signal arrived at its result. The verdict
// JSON exposes this so a CI gate can reason about strictness without
// reading evidence.
type Method string

const (
	MethodFilenameMatch Method = "filename"
	MethodContentRegex  Method = "content-regex"
	MethodAST           Method = "ast"
	MethodCrossFile     Method = "cross-file"
)

// LineSpan is an optional location within a file used by Evidence.
type LineSpan struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Evidence is a citation produced by a signal — the path, optional
// line range, and an excerpt the user can verify by hand.
type Evidence struct {
	Path    string    `json:"path"`
	Span    *LineSpan `json:"span,omitempty"`
	Excerpt string    `json:"excerpt,omitempty"`
}

// DiagEntry is one line of detection diagnostics, populated only when
// --debug is set. See SPEC.md §8.2.7.
type DiagEntry struct {
	Path   string `json:"path"`
	Action string `json:"action"`
	Hit    bool   `json:"hit"`
	Detail string `json:"detail,omitempty"`
}

// Result is what every signal returns. Score must come from the rubric
// constants above; Status is derived from Score.
type Result struct {
	Status     Status      `json:"status"`
	Score      float64     `json:"score"`
	Confidence Confidence  `json:"confidence"`
	Method     Method      `json:"method"`
	Evidence   []Evidence  `json:"evidence,omitempty"`
	Notes      []string    `json:"notes,omitempty"`
	Diag       []DiagEntry `json:"diag,omitempty"`
}

// SignalResult is one signal's entry within a verdict.
type SignalResult struct {
	ID         string      `json:"id"`
	Level      Level       `json:"level"`
	Family     string      `json:"family"`
	Title      string      `json:"title,omitempty"`
	Status     Status      `json:"status"`
	Score      float64     `json:"score"`
	Confidence Confidence  `json:"confidence"`
	Method     Method      `json:"method"`
	Evidence   []Evidence  `json:"evidence,omitempty"`
	Notes      []string    `json:"notes,omitempty"`
	Diag       []DiagEntry `json:"diag,omitempty"`
}

// Verdict is the top-level result of an `assess` run. Its shape is the
// `verdict` schema published via `plumbline schema verdict`.
type Verdict struct {
	Level                Level             `json:"level"`
	Name                 string            `json:"name"`
	LevelScores          map[Level]float64 `json:"level_scores"`
	NextGap              []string          `json:"next_gap"`
	MinConfidenceApplied Confidence        `json:"min_confidence_applied"`
}

// Report is the top-level JSON document emitted by `assess --json`.
type Report struct {
	Schema           string         `json:"schema"`
	ToolVersion      string         `json:"tool_version"`
	SignalSetVersion string         `json:"signal_set_version"`
	CISystem         string         `json:"ci_system"`
	Repo             string         `json:"repo"`
	ScannedAt        string         `json:"scanned_at"`
	Verdict          Verdict        `json:"verdict"`
	Signals          []SignalResult `json:"signals"`
}
