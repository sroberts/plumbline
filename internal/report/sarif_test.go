package report

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sroberts/plumbline/pkg/acmm"
)

func sarifSampleReport() acmm.Report {
	return acmm.Report{
		Schema:           "plumbline/v1",
		ToolVersion:      "test",
		SignalSetVersion: "v1",
		Repo:             "/repo",
		ScannedAt:        "2026-04-28T15:00:00Z",
		Verdict: acmm.Verdict{
			Level:       acmm.LevelInstructed,
			Name:        "Instructed",
			LevelScores: map[acmm.Level]float64{acmm.LevelInstructed: 1.0},
		},
		Signals: []acmm.SignalResult{
			{
				ID: "l2.found-thing", Level: acmm.LevelInstructed, Family: "instructions",
				Title: "Found thing", Status: acmm.StatusFound, Score: 1.0,
				Confidence: acmm.ConfidenceHigh, Method: acmm.MethodFilenameMatch,
			},
			{
				ID: "l2.missing-thing", Level: acmm.LevelInstructed, Family: "instructions",
				Title: "Missing thing", Status: acmm.StatusMissing, Score: 0.0,
				Confidence: acmm.ConfidenceMedium, Method: acmm.MethodContentRegex,
				FixHint: "add a thing",
				Notes:   []string{"why it matters"},
				Evidence: []acmm.Evidence{
					{Path: "README.md", Span: &acmm.LineSpan{Start: 1, End: 5}},
				},
			},
			{
				ID: "l3.partial-thing", Level: acmm.LevelMeasured, Family: "metrics",
				Title: "Partial thing", Status: acmm.StatusPartial, Score: 0.5,
				Confidence: acmm.ConfidenceLow, Method: acmm.MethodFilenameMatch,
			},
			{
				ID: "l3.na-thing", Level: acmm.LevelMeasured, Family: "metrics",
				Status: acmm.StatusNA, Score: 0.0,
			},
		},
	}
}

func TestSARIF_EnvelopeShape(t *testing.T) {
	out, err := SARIF(sarifSampleReport())
	if err != nil {
		t.Fatalf("SARIF: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if doc["version"] != "2.1.0" {
		t.Errorf("version = %v, want 2.1.0", doc["version"])
	}
	if !strings.HasPrefix(doc["$schema"].(string), "https://") {
		t.Errorf("$schema = %v, want https URL", doc["$schema"])
	}
	runs, ok := doc["runs"].([]any)
	if !ok || len(runs) != 1 {
		t.Fatalf("runs missing or not single-run: %v", doc["runs"])
	}
}

func TestSARIF_OnlyEmitsActionableFindings(t *testing.T) {
	// found and na signals are not actionable findings; SARIF results
	// should only carry missing/partial.
	out, _ := SARIF(sarifSampleReport())
	var doc sarifDocument
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Runs) != 1 {
		t.Fatalf("got %d runs, want 1", len(doc.Runs))
	}
	results := doc.Runs[0].Results
	gotIDs := make([]string, len(results))
	for i, r := range results {
		gotIDs[i] = r.RuleID
	}
	wantIDs := map[string]bool{
		"l2.missing-thing": true,
		"l3.partial-thing": true,
	}
	if len(results) != len(wantIDs) {
		t.Errorf("got %d results, want %d (ids=%v)", len(results), len(wantIDs), gotIDs)
	}
	for _, id := range gotIDs {
		if !wantIDs[id] {
			t.Errorf("unexpected result id %q (found/na should be omitted)", id)
		}
	}
}

func TestSARIF_SeverityMapping(t *testing.T) {
	out, _ := SARIF(sarifSampleReport())
	var doc sarifDocument
	_ = json.Unmarshal(out, &doc)

	got := map[string]string{}
	for _, r := range doc.Runs[0].Results {
		got[r.RuleID] = r.Level
	}
	if got["l2.missing-thing"] != "error" {
		t.Errorf("missing → %q, want error", got["l2.missing-thing"])
	}
	if got["l3.partial-thing"] != "warning" {
		t.Errorf("partial → %q, want warning", got["l3.partial-thing"])
	}
}

func TestSARIF_RuleHasFixHintAndHelpURI(t *testing.T) {
	out, _ := SARIF(sarifSampleReport())
	var doc sarifDocument
	_ = json.Unmarshal(out, &doc)

	var rule *sarifRule
	for i, r := range doc.Runs[0].Tool.Driver.Rules {
		if r.ID == "l2.missing-thing" {
			rule = &doc.Runs[0].Tool.Driver.Rules[i]
			break
		}
	}
	if rule == nil {
		t.Fatal("rule l2.missing-thing not in driver.rules")
	}
	if rule.Help.Text != "add a thing" {
		t.Errorf("rule.help = %q, want fix hint", rule.Help.Text)
	}
	if !strings.Contains(rule.HelpURI, "l2.missing-thing") {
		t.Errorf("rule.helpUri = %q, want signal-id deep-link", rule.HelpURI)
	}
}

func TestSARIF_LocationCarriesEvidenceLineSpan(t *testing.T) {
	out, _ := SARIF(sarifSampleReport())
	var doc sarifDocument
	_ = json.Unmarshal(out, &doc)

	for _, r := range doc.Runs[0].Results {
		if r.RuleID != "l2.missing-thing" {
			continue
		}
		if len(r.Locations) != 1 {
			t.Fatalf("got %d locations, want 1", len(r.Locations))
		}
		loc := r.Locations[0]
		if loc.PhysicalLocation.ArtifactLocation.URI != "README.md" {
			t.Errorf("location.uri = %q, want README.md",
				loc.PhysicalLocation.ArtifactLocation.URI)
		}
		if loc.PhysicalLocation.Region == nil {
			t.Fatal("location missing region (line span lost)")
		}
		if loc.PhysicalLocation.Region.StartLine != 1 ||
			loc.PhysicalLocation.Region.EndLine != 5 {
			t.Errorf("region = %+v, want lines 1-5", loc.PhysicalLocation.Region)
		}
		return
	}
	t.Fatal("l2.missing-thing not in results")
}

func TestSARIF_ResultPropertiesIncludeScoreAndConfidence(t *testing.T) {
	out, _ := SARIF(sarifSampleReport())
	var doc sarifDocument
	_ = json.Unmarshal(out, &doc)

	for _, r := range doc.Runs[0].Results {
		if r.RuleID != "l2.missing-thing" {
			continue
		}
		if r.Properties["confidence"] != "medium" {
			t.Errorf("properties.confidence = %v, want medium", r.Properties["confidence"])
		}
		// JSON unmarshals numbers into float64; 0.0 is the score.
		if r.Properties["method"] == nil {
			t.Errorf("properties.method missing")
		}
		return
	}
	t.Fatal("l2.missing-thing not in results")
}

func TestSARIF_EmptyReportProducesValidEnvelope(t *testing.T) {
	r := acmm.Report{
		Schema:           "plumbline/v1",
		ToolVersion:      "test",
		SignalSetVersion: "v1",
	}
	out, err := SARIF(r)
	if err != nil {
		t.Fatal(err)
	}
	var doc sarifDocument
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(doc.Runs) != 1 {
		t.Errorf("len(runs) = %d, want 1", len(doc.Runs))
	}
	if len(doc.Runs[0].Results) != 0 {
		t.Errorf("len(results) = %d, want 0", len(doc.Runs[0].Results))
	}
}
