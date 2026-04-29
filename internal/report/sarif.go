package report

import (
	"encoding/json"
	"fmt"

	"github.com/sroberts/plumbline/internal/buildinfo"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// SARIF emits a SARIF 2.1.0 document representing an acmm.Report. Each
// missing or partial signal becomes one SARIF "result"; signals with
// status `found` and `na` are omitted (they're not actionable findings).
//
// Why SARIF: GitHub's code-scanning surface ingests SARIF and renders
// findings inline in PRs. Without SARIF, CI gates have to scrape JSON
// or markdown. SPEC.md §9 names sarif as a first-class output format.
//
// Schema: SARIF 2.1.0 (OASIS) —
// https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html
//
// Severity mapping (per SPEC.md §8.2.6 color palette):
//
//	missing → error
//	partial → warning
//	found   → omitted
//	na      → omitted
func SARIF(r acmm.Report) ([]byte, error) {
	rules := make([]sarifRule, 0, len(r.Signals))
	results := make([]sarifResult, 0, len(r.Signals))

	for _, s := range r.Signals {
		if s.Status == acmm.StatusFound || s.Status == acmm.StatusNA {
			continue
		}
		rules = append(rules, sarifRule{
			ID:               s.ID,
			Name:             s.Title,
			ShortDescription: sarifText{Text: s.Title},
			HelpURI:          fmt.Sprintf("https://github.com/sroberts/plumbline/blob/main/SPEC.md#%s", s.ID),
			Help:             sarifText{Text: s.FixHint},
			DefaultConfiguration: &sarifConfig{
				Level: severityFor(s.Status),
			},
			Properties: map[string]any{
				"acmm.level":  int(s.Level),
				"acmm.family": s.Family,
			},
		})
		results = append(results, signalToResult(s))
	}

	doc := sarifDocument{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool: sarifTool{
				Driver: sarifDriver{
					Name:           "plumbline",
					Version:        buildinfo.Version,
					InformationURI: "https://github.com/sroberts/plumbline",
					Rules:          rules,
					Properties: map[string]any{
						"signal_set_version": r.SignalSetVersion,
					},
				},
			},
			Results: results,
			Properties: map[string]any{
				"acmm.level":      int(r.Verdict.Level),
				"acmm.level_name": r.Verdict.Name,
				"scanned_at":      r.ScannedAt,
			},
		}},
	}
	return json.MarshalIndent(doc, "", "  ")
}

func signalToResult(s acmm.SignalResult) sarifResult {
	msg := s.Title
	if msg == "" {
		msg = s.ID
	}
	res := sarifResult{
		RuleID:  s.ID,
		Level:   severityFor(s.Status),
		Message: sarifText{Text: msg},
		Properties: map[string]any{
			"score":      s.Score,
			"confidence": string(s.Confidence),
			"method":     string(s.Method),
		},
	}
	if len(s.Notes) > 0 {
		// SARIF "result.message" already carries one human string;
		// stash structured notes in properties so consumers can drill
		// in without parsing the message text.
		res.Properties["notes"] = s.Notes
	}
	for _, ev := range s.Evidence {
		// SARIF "physicalLocation" requires a URI. Plumbline evidence
		// is always repo-relative path, optionally with a line span.
		loc := sarifLocation{
			PhysicalLocation: sarifPhysicalLocation{
				ArtifactLocation: sarifArtifactLocation{URI: ev.Path},
			},
		}
		if ev.Span != nil {
			loc.PhysicalLocation.Region = &sarifRegion{
				StartLine: ev.Span.Start,
				EndLine:   ev.Span.End,
			}
		}
		res.Locations = append(res.Locations, loc)
	}
	return res
}

func severityFor(st acmm.Status) string {
	switch st {
	case acmm.StatusMissing:
		return "error"
	case acmm.StatusPartial:
		return "warning"
	default:
		// StatusFound / StatusNA shouldn't reach here (filtered upstream),
		// but if they do, "note" is the harmless fallback per SARIF
		// 2.1.0 §3.27.10.
		return "note"
	}
}

// SARIF 2.1.0 envelope, modeled to match the spec's required fields.
// We omit optional fields plumbline doesn't currently populate (taxonomies,
// invocations, conversion); GitHub's ingestor accepts a minimal envelope.

type sarifDocument struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool       sarifTool      `json:"tool"`
	Results    []sarifResult  `json:"results"`
	Properties map[string]any `json:"properties,omitempty"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string         `json:"name"`
	Version        string         `json:"version,omitempty"`
	InformationURI string         `json:"informationUri,omitempty"`
	Rules          []sarifRule    `json:"rules,omitempty"`
	Properties     map[string]any `json:"properties,omitempty"`
}

type sarifRule struct {
	ID                   string         `json:"id"`
	Name                 string         `json:"name,omitempty"`
	ShortDescription     sarifText      `json:"shortDescription"`
	HelpURI              string         `json:"helpUri,omitempty"`
	Help                 sarifText      `json:"help,omitempty"`
	DefaultConfiguration *sarifConfig   `json:"defaultConfiguration,omitempty"`
	Properties           map[string]any `json:"properties,omitempty"`
}

type sarifConfig struct {
	Level string `json:"level,omitempty"`
}

type sarifResult struct {
	RuleID     string          `json:"ruleId"`
	Level      string          `json:"level,omitempty"`
	Message    sarifText       `json:"message"`
	Locations  []sarifLocation `json:"locations,omitempty"`
	Properties map[string]any  `json:"properties,omitempty"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           *sarifRegion          `json:"region,omitempty"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine int `json:"startLine,omitempty"`
	EndLine   int `json:"endLine,omitempty"`
}

type sarifText struct {
	Text string `json:"text"`
}
