package report

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sroberts/plumbline/pkg/acmm"
)

// DecodeReport parses a serialized report artifact back into an
// acmm.Report. It is the inverse of the TOON / YAML / JSON encoders, so a
// committed `.plumbline.toon` (or `.json` / `.yaml`) snapshot can be read
// back for comparison — the round trip `plumbline snapshot` writes and
// `plumbline diff` reads.
//
// format is one of "toon", "json", "yaml". Non-JSON formats decode to the
// generic tree first, then re-marshal through JSON so the acmm.Report
// json tags apply uniformly (matching how the encoders were built).
func DecodeReport(data []byte, format string) (acmm.Report, error) {
	switch format {
	case "json":
		var r acmm.Report
		if err := json.Unmarshal(data, &r); err != nil {
			return acmm.Report{}, fmt.Errorf("decode json report: %w", err)
		}
		return r, nil
	case "toon":
		tree, err := toonDecode(data)
		if err != nil {
			return acmm.Report{}, err
		}
		return treeToReport(tree)
	case "yaml":
		var tree any
		if err := yaml.Unmarshal(data, &tree); err != nil {
			return acmm.Report{}, fmt.Errorf("decode yaml report: %w", err)
		}
		return treeToReport(tree)
	default:
		return acmm.Report{}, fmt.Errorf("unknown report format %q (want toon|json|yaml)", format)
	}
}

// treeToReport routes a generic tree back through JSON into a typed
// Report so the json struct tags do the field mapping.
func treeToReport(tree any) (acmm.Report, error) {
	b, err := json.Marshal(tree)
	if err != nil {
		return acmm.Report{}, fmt.Errorf("re-marshal decoded tree: %w", err)
	}
	var r acmm.Report
	if err := json.Unmarshal(b, &r); err != nil {
		return acmm.Report{}, fmt.Errorf("decode report: %w", err)
	}
	return r, nil
}

// FormatFromPath guesses the artifact format from a file extension,
// defaulting to toon (the snapshot default) for unknown or empty
// extensions like the bare "-" stdin marker.
func FormatFromPath(path string) string {
	switch {
	case strings.HasSuffix(path, ".json"):
		return "json"
	case strings.HasSuffix(path, ".yaml"), strings.HasSuffix(path, ".yml"):
		return "yaml"
	default:
		return "toon"
	}
}
