package l3

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sroberts/plumbline/pkg/acmm"
)

const ciRunsTests = `
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: make test
`

func file(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }

func TestVerificationHarness(t *testing.T) {
	cases := []struct {
		name  string
		files fstest.MapFS
		want  float64
	}{
		{"nothing", fstest.MapFS{"main.go": file("package main")}, acmm.ScoreMissing},
		{"tests but no entrypoint (JS, no scripts)", fstest.MapFS{
			"src/a.test.js": file("test('x',()=>{})"),
		}, acmm.ScoreStubbed},
		{"npm init placeholder is not a test entrypoint", fstest.MapFS{
			"package.json":  file(`{"scripts":{"test":"echo \"Error: no test specified\" && exit 1"}}`),
			"src/a.test.js": file("x"),
		}, acmm.ScoreStubbed},
		{"entrypoint but no tests", fstest.MapFS{
			"Makefile": file("test:\n\tpytest\n"),
		}, acmm.ScoreStubbed},
		{"make test + tests, no CI, no lint", fstest.MapFS{
			"Makefile":  file(".PHONY: test\ntest:\n\tpytest\n"),
			"test_a.py": file("def test_a(): pass"),
		}, acmm.ScoreIncomplete},
		{"go native entrypoint + tests, no CI", fstest.MapFS{
			"go.mod":    file("module x\n"),
			"a_test.go": file("package x"),
		}, acmm.ScoreIncomplete},
		{"full loop: make test/lint, tests, CI", fstest.MapFS{
			"Makefile":                 file("test:\n\tgo test ./...\nlint:\n\tgo vet ./...\n"),
			"a_test.go":                file("package x"),
			".github/workflows/ci.yml": file(ciRunsTests),
		}, acmm.ScoreFound},
		{"npm scripts + CI", fstest.MapFS{
			"package.json":             file(`{"scripts":{"test":"vitest","lint":"eslint ."}}`),
			"src/a.spec.ts":            file("x"),
			".github/workflows/ci.yml": file("on: push\njobs:\n  t:\n    runs-on: x\n    steps:\n      - run: npm test\n"),
		}, acmm.ScoreFound},
		{"testdata tests do not count", fstest.MapFS{
			"go.mod":             file("module x\n"),
			"testdata/a_test.go": file("package x"),
		}, acmm.ScoreStubbed},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := runOn(t, VerificationHarness{}, c.files)
			if got.Score != c.want {
				t.Errorf("score = %v, want %v (notes: %v)", got.Score, c.want, got.Notes)
			}
			if got.Score < acmm.ScoreFound && got.FixHint == "" {
				t.Error("non-Found result must carry a FixHint")
			}
		})
	}
}

func TestAPIContract(t *testing.T) {
	openapi := "openapi: 3.0.0\ninfo:\n  title: x\npaths: {}\n"
	validateCI := "on: pull_request\njobs:\n  c:\n    runs-on: x\n    steps:\n      - run: npx @stoplight/spectral-cli lint openapi.yaml\n"

	cases := []struct {
		name   string
		files  fstest.MapFS
		want   float64
		status acmm.Status
	}{
		{"no contract is NA, not Missing", fstest.MapFS{"main.go": file("x")}, 0, acmm.StatusNA},
		{"openapi present, unvalidated", fstest.MapFS{"openapi.yaml": file(openapi)}, acmm.ScoreIncomplete, acmm.StatusPartial},
		{"swagger.json in api/", fstest.MapFS{"api/swagger.json": file(`{"swagger":"2.0"}`)}, acmm.ScoreIncomplete, acmm.StatusPartial},
		{"openapi validated in CI", fstest.MapFS{
			"openapi.yaml":             file(openapi),
			".github/workflows/ci.yml": file(validateCI),
		}, acmm.ScoreFound, acmm.StatusFound},
		{"proto with service", fstest.MapFS{"api/v1/svc.proto": file("syntax = \"proto3\";\nservice S {}\nmessage M {}\n")}, acmm.ScoreIncomplete, acmm.StatusPartial},
		{"proto validated with buf", fstest.MapFS{
			"svc.proto":                file("message M {}\n"),
			".github/workflows/ci.yml": file("on: push\njobs:\n  b:\n    runs-on: x\n    steps:\n      - run: buf lint && buf breaking --against .git#branch=main\n"),
		}, acmm.ScoreFound, acmm.StatusFound},
		{"graphql schema", fstest.MapFS{"schema.graphql": file("type Query { a: Int }\n")}, acmm.ScoreIncomplete, acmm.StatusPartial},
		{"json schema", fstest.MapFS{"config.schema.json": file(`{"$schema":"x","type":"object"}`)}, acmm.ScoreIncomplete, acmm.StatusPartial},
		{"stub named openapi.yaml", fstest.MapFS{"openapi.yaml": file("# TODO\n")}, acmm.ScoreStubbed, acmm.StatusPartial},
		{"contract under testdata is someone else's", fstest.MapFS{"testdata/openapi.yaml": file(openapi)}, 0, acmm.StatusNA},
		{"scheduled-only check does not gate PRs", fstest.MapFS{
			"openapi.yaml":            file(openapi),
			".github/workflows/n.yml": file("on:\n  schedule:\n    - cron: '0 0 * * *'\njobs:\n  c:\n    runs-on: x\n    steps:\n      - run: spectral lint openapi.yaml\n"),
		}, acmm.ScoreIncomplete, acmm.StatusPartial},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := runOn(t, APIContract{}, c.files)
			if got.Status != c.status || (c.status != acmm.StatusNA && got.Score != c.want) {
				t.Errorf("status/score = %v/%v, want %v/%v (notes: %v)", got.Status, got.Score, c.status, c.want, got.Notes)
			}
		})
	}
}

func TestLintConfig(t *testing.T) {
	cases := []struct {
		name  string
		files fstest.MapFS
		want  float64
	}{
		{"nothing", fstest.MapFS{"a.py": file("x")}, acmm.ScoreMissing},
		{"golangci with explicit linters (Go has gofmt built in)", fstest.MapFS{
			"go.mod":        file("module x\n"),
			".golangci.yml": file("version: \"2\"\nlinters:\n  default: none\n  enable:\n    - govet\n"),
		}, acmm.ScoreFound},
		{"golangci default-only, no explicit rules", fstest.MapFS{
			"go.mod":        file("module x\n"),
			".golangci.yml": file("version: \"2\"\nrun:\n  timeout: 5m\n"),
		}, acmm.ScoreIncomplete},
		{"ruff with select (lint+format)", fstest.MapFS{
			"pyproject.toml": file("[tool.ruff]\nline-length = 100\n[tool.ruff.lint]\nselect = [\"E\", \"F\"]\n"),
		}, acmm.ScoreFound},
		{"biome recommended", fstest.MapFS{"biome.json": file(`{"linter":{"rules":{"recommended":true}}}`)}, acmm.ScoreFound},
		{"eslint without formatter", fstest.MapFS{"eslint.config.js": file("export default [{ rules: {} }]")}, acmm.ScoreIncomplete},
		{"eslint + prettier", fstest.MapFS{
			"eslint.config.js": file("export default [{ rules: {} }]"),
			".prettierrc":      file("{}"),
		}, acmm.ScoreFound},
		{"prettier alone", fstest.MapFS{".prettierrc.json": file("{}")}, acmm.ScoreStubbed},
		{"go with no lint config is not credited as linted", fstest.MapFS{"go.mod": file("module x\n")}, acmm.ScoreMissing},
		{"fixture configs ignored", fstest.MapFS{"testdata/.golangci.yml": file("linters:\n  enable: [a]\n")}, acmm.ScoreMissing},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := runOn(t, LintConfig{}, c.files)
			if got.Score != c.want {
				t.Errorf("score = %v, want %v (notes: %s)", got.Score, c.want, strings.Join(got.Notes, "; "))
			}
		})
	}
}
