package l3

import (
	"context"
	"regexp"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// A machine-readable interface contract lets an agent verify a change
// against the *interface* deterministically — schema validation, breaking-
// change diffs, generated-client compilation — instead of by reading prose.

var (
	contractPathRE = regexp.MustCompile(`(?i)((^|/)(openapi|swagger|asyncapi)[\w.-]*\.(ya?ml|json)$|\.proto$|\.(graphql|gql)$|\.schema\.json$)`)

	// Content markers separating a real contract from a stub. A file that
	// merely has the right name earns only the stub score.
	openAPIBodyRE   = regexp.MustCompile(`(?m)^\s*"?(openapi|swagger|asyncapi)"?\s*:`)
	protoBodyRE     = regexp.MustCompile(`(?m)^\s*(service|message|enum)\s+\w+`)
	graphqlBodyRE   = regexp.MustCompile(`(?m)^\s*(type|schema|interface|enum|input|scalar|union|extend)\b`)
	jsonSchemaBody  = regexp.MustCompile(`"(\$schema|type|properties)"\s*:`)
	contractCheckRE = regexp.MustCompile(`(?i)(spectral|redocly|swagger-cli|openapi-generator|openapi-typescript|oapi-codegen|oasdiff|schemathesis|prism|dredd|\bbuf\s+(lint|breaking|generate|build)|protolint|protoc\b|\bajv\b|check-jsonschema|graphql-inspector|graphql-codegen|graphql-schema-linter|apollo (schema|rover)|rover (subgraph|graph) (check|lint))`)
)

// APIContract detects declarative interface contracts (OpenAPI, Protobuf,
// GraphQL, JSON Schema, AsyncAPI) and whether CI validates them.
//
// Absence is NA, not Missing: plumbline cannot tell from the filesystem
// whether a repo *has* an interface, and a CLI or a library with no wire
// contract should not be marked down for lacking one. The signal can add
// credit but never subtract it. See SPEC.md §6 "Verification pillars".
type APIContract struct{}

func (APIContract) ID() string        { return "l3.api-contract" }
func (APIContract) Level() acmm.Level { return acmm.LevelMeasured }
func (APIContract) Family() string    { return "verification" }
func (APIContract) Title() string {
	return "Machine-verifiable interface contract (OpenAPI / Protobuf / GraphQL / JSON Schema) validated in CI"
}

func (s APIContract) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	var real, stub []string
	for _, fe := range idx.Files {
		if !contractPathRE.MatchString(fe.Path) || isContractNoise(fe.Path) {
			continue
		}
		if contractHasBody(fe.Path, readOrEmpty(idx, fe.Path)) {
			real = append(real, fe.Path)
		} else {
			stub = append(stub, fe.Path)
		}
	}

	if len(real) == 0 && len(stub) == 0 {
		return acmm.Result{
			Status:     acmm.StatusNA,
			Confidence: acmm.ConfidenceMedium,
			Method:     acmm.MethodFilenameMatch,
			Notes:      []string{"no OpenAPI/Protobuf/GraphQL/JSON Schema contract found; not graded (a repo with no wire interface is not penalized)"},
			FixHint: "If this repo exposes an HTTP, RPC or file-format interface, describe it " +
				"declaratively (openapi.yaml, a .proto, a JSON Schema) and validate it in CI.",
		}
	}

	res := acmm.Result{
		Confidence: acmm.ConfidenceMedium,
		Method:     acmm.MethodContentRegex,
	}
	for _, p := range append(append([]string{}, real...), stub...) {
		if len(res.Evidence) == 5 {
			break
		}
		res.Evidence = append(res.Evidence, acmm.Evidence{Path: p})
	}

	if len(real) == 0 {
		res.Status, res.Score = acmm.StatusPartial, acmm.ScoreStubbed
		res.Notes = []string{"contract files exist but none contains a recognizable definition (stubs)"}
		res.FixHint = "Fill in " + stub[0] + " with real definitions (paths/operations, messages/services, types) — a contract file with no content verifies nothing."
		return res
	}

	for _, w := range idx.Workflows {
		if !(w.HasPushTrigger() || w.HasPullRequestTrigger()) {
			continue
		}
		if w.AnyRunMatches(contractCheckRE) || w.AnyUsesMatches(contractCheckRE) {
			res.Status, res.Score = acmm.StatusFound, acmm.ScoreFound
			res.Evidence = append(res.Evidence, acmm.Evidence{Path: w.Path})
			res.Notes = []string{"contract present and validated by a push/PR workflow (" + w.Path + ")"}
			return res
		}
	}
	res.Status, res.Score = acmm.StatusPartial, acmm.ScoreIncomplete
	res.Notes = []string{"contract present but no push/PR workflow lints, diffs or generates from it"}
	res.FixHint = "Validate the contract on every PR: spectral/redocly for OpenAPI, `buf lint` + " +
		"`buf breaking` for Protobuf, graphql-inspector for GraphQL, ajv/check-jsonschema for JSON Schema."
	return res
}

func contractHasBody(path string, data []byte) bool {
	switch {
	case protoPath.MatchString(path):
		return protoBodyRE.Match(data)
	case graphqlPath.MatchString(path):
		return graphqlBodyRE.Match(data)
	case jsonSchemaPath.MatchString(path):
		return jsonSchemaBody.Match(data)
	default:
		return openAPIBodyRE.Match(data) || jsonOpenAPIRE.Match(data)
	}
}

var (
	protoPath      = regexp.MustCompile(`(?i)\.proto$`)
	graphqlPath    = regexp.MustCompile(`(?i)\.(graphql|gql)$`)
	jsonSchemaPath = regexp.MustCompile(`(?i)\.schema\.json$`)
	jsonOpenAPIRE  = regexp.MustCompile(`"(openapi|swagger|asyncapi)"\s*:`)
	contractNoise  = regexp.MustCompile(`(?i)(^|/)(testdata|fixtures?|examples?|third_party|node_modules)/`)
)

// isContractNoise excludes sample and test inputs: a contract under
// testdata/ describes someone else's API, not this repo's.
func isContractNoise(p string) bool { return contractNoise.MatchString(p) }

func init() {
	signals.Default.Register(APIContract{})
}
