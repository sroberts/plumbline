package report

import (
	"gopkg.in/yaml.v3"

	"github.com/sroberts/plumbline/pkg/acmm"
)

// YAML encodes an acmm.Report as a YAML document — the "force" format for
// users who prefer YAML over the default TOON maturity snapshot.
//
// Like TOON, it is produced from the same generic tree as `--report json`
// (see toGeneric), so keys, omitempty elisions, and value shapes match
// the JSON output. yaml.v3 marshals map keys in sorted order, keeping the
// artifact deterministic and diff-friendly.
func YAML(r acmm.Report) ([]byte, error) {
	v, err := toGeneric(r)
	if err != nil {
		return nil, err
	}
	return yaml.Marshal(v)
}
