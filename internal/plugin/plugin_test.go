package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sroberts/plumbline/pkg/acmm"
)

// writePlugin creates a small shell-script "plugin" that emits the
// given stdout when run. Skips on Windows since /bin/sh isn't there.
func writePlugin(t *testing.T, stdout string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("plugin tests use /bin/sh; skip on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "plugin.sh")
	body := "#!/bin/sh\ncat <<'EOF'\n" + stdout + "\nEOF\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func sha256Of(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

const pluginGoodOutput = `{"id":"x.demo","level":3,"family":"custom","title":"Demo plugin","status":"missing","score":0,"confidence":"medium","method":"filename"}`

func TestLoad_ParsesFirstResult(t *testing.T) {
	path := writePlugin(t, pluginGoodOutput)
	p, err := Load(context.Background(), Spec{Path: path}, "/repo")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.ID() != "x.demo" {
		t.Errorf("ID = %q, want x.demo", p.ID())
	}
	if p.Level() != 3 {
		t.Errorf("Level = %d, want 3", p.Level())
	}
	if p.Family() != "custom" {
		t.Errorf("Family = %q", p.Family())
	}
}

func TestLoad_RejectsEmptyOutput(t *testing.T) {
	path := writePlugin(t, "")
	_, err := Load(context.Background(), Spec{Path: path}, "/repo")
	if err == nil {
		t.Fatal("expected error on empty plugin output")
	}
}

func TestLoad_RejectsBadJSON(t *testing.T) {
	path := writePlugin(t, "this is not json")
	_, err := Load(context.Background(), Spec{Path: path}, "/repo")
	if err == nil {
		t.Fatal("expected error on malformed plugin output")
	}
}

func TestLoad_VerifiesSHA256(t *testing.T) {
	path := writePlugin(t, pluginGoodOutput)

	// Mismatched SHA → reject before invocation.
	_, err := Load(context.Background(),
		Spec{Path: path, SHA256: "00" + strings.Repeat("0", 62)},
		"/repo")
	if err == nil {
		t.Error("expected sha256 mismatch error")
	}
	if !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Errorf("error should mention sha256; got: %v", err)
	}

	// Correct SHA → loads.
	correct := sha256Of(t, path)
	p, err := Load(context.Background(),
		Spec{Path: path, SHA256: correct}, "/repo")
	if err != nil {
		t.Fatalf("with correct sha: %v", err)
	}
	if p.ID() != "x.demo" {
		t.Errorf("ID = %q", p.ID())
	}
}

func TestDetect_ReturnsMatchingResult(t *testing.T) {
	path := writePlugin(t, pluginGoodOutput)
	p, err := Load(context.Background(), Spec{Path: path}, "/repo")
	if err != nil {
		t.Fatal(err)
	}
	got := p.Detect(context.Background(), nil)
	if got.Status != acmm.StatusMissing {
		t.Errorf("Status = %q, want missing", got.Status)
	}
	if got.Confidence != acmm.ConfidenceMedium {
		t.Errorf("Confidence = %q, want medium", got.Confidence)
	}
}

func TestDetect_PluginCrashSurfacesAsMissingResult(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho \"oops\" >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Load probe will fail; that's the expected design (caller should
	// see the failure at startup, not deep into a scan).
	if _, err := Load(context.Background(), Spec{Path: path}, "/repo"); err == nil {
		t.Fatal("expected Load error for crashing plugin")
	}
}

func TestParseSignalResults_NDJSON(t *testing.T) {
	in := `{"id":"a","status":"found"}
{"id":"b","status":"missing"}
`
	got, err := parseSignalResults(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Errorf("got %+v", got)
	}
}

func TestParseSignalResults_TopLevelArray(t *testing.T) {
	in := `[{"id":"a","status":"found"},{"id":"b","status":"missing"}]`
	got, err := parseSignalResults(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("got %d, want 2", len(got))
	}
}

func TestParseSpec_PathOnly(t *testing.T) {
	got, err := ParseSpec("/usr/local/bin/myplugin")
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != "/usr/local/bin/myplugin" || got.SHA256 != "" {
		t.Errorf("got %+v", got)
	}
}

func TestParseSpec_PathAtSHA(t *testing.T) {
	got, err := ParseSpec("/p@abc123")
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != "/p" || got.SHA256 != "abc123" {
		t.Errorf("got %+v", got)
	}
}

func TestParseSpec_Empty(t *testing.T) {
	if _, err := ParseSpec(""); err == nil {
		t.Error("expected error on empty spec")
	}
}

func TestParseSpec_PartialAt(t *testing.T) {
	if _, err := ParseSpec("@deadbeef"); err == nil {
		t.Error("expected error on empty path before @")
	}
	if _, err := ParseSpec("/path@"); err == nil {
		t.Error("expected error on empty sha after @")
	}
}
