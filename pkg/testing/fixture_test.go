package testing

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// helper writes content to a temp file and returns the path.
func writeTempFixture(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing temp fixture: %v", err)
	}
	return path
}

func TestParse_ValidFixture(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
rule_name: osps-vm-05
test_cases:
  - name: "SECURITY.md exists"
    expect: pass
    mock_data:
      git_files:
        "SECURITY.md": "report vulns here"
  - name: "No security file"
    expect: fail
    mock_data:
      git_files:
        "README.md": "hello"
`
	f, err := Parse(writeTempFixture(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Version != "v1" {
		t.Errorf("version = %q, want %q", f.Version, "v1")
	}
	if f.RuleName != "osps-vm-05" {
		t.Errorf("rule_name = %q, want %q", f.RuleName, "osps-vm-05")
	}
	if got := len(f.TestCases); got != 2 {
		t.Fatalf("len(test_cases) = %d, want 2", got)
	}
	if f.TestCases[0].Expect != "pass" {
		t.Errorf("test_cases[0].expect = %q, want \"pass\"", f.TestCases[0].Expect)
	}
	if f.TestCases[1].Expect != "fail" {
		t.Errorf("test_cases[1].expect = %q, want \"fail\"", f.TestCases[1].Expect)
	}
}

func TestParse_EmptyPath(t *testing.T) {
	t.Parallel()
	_, err := Parse("")
	if !errors.Is(err, ErrEmptyPath) {
		t.Errorf("expected ErrEmptyPath, got %v", err)
	}
}

func TestParse_FileNotFound(t *testing.T) {
	t.Parallel()
	_, err := Parse("/tmp/does_not_exist_fixture.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestParse_UnsupportedVersion(t *testing.T) {
	t.Parallel()
	yaml := `
version: v99
rule_name: some-rule
test_cases:
  - name: "case"
    expect: pass
`
	_, err := Parse(writeTempFixture(t, yaml))
	if !errors.Is(err, ErrUnsupportedVer) {
		t.Errorf("expected ErrUnsupportedVer, got %v", err)
	}
}

func TestParse_MissingVersion(t *testing.T) {
	t.Parallel()
	yaml := `
rule_name: some-rule
test_cases:
  - name: "case"
    expect: pass
`
	_, err := Parse(writeTempFixture(t, yaml))
	if !errors.Is(err, ErrMissingVersion) {
		t.Errorf("expected ErrMissingVersion for empty version, got %v", err)
	}
}

func TestParse_MissingRuleName(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
test_cases:
  - name: "case"
    expect: pass
`
	_, err := Parse(writeTempFixture(t, yaml))
	if !errors.Is(err, ErrMissingRuleName) {
		t.Errorf("expected ErrMissingRuleName, got %v", err)
	}
}

func TestParse_NoTestCases(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
rule_name: some-rule
test_cases: []
`
	_, err := Parse(writeTempFixture(t, yaml))
	if !errors.Is(err, ErrNoTestCases) {
		t.Errorf("expected ErrNoTestCases, got %v", err)
	}
}

func TestParse_InvalidExpect(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
rule_name: some-rule
test_cases:
  - name: "bad case"
    expect: maybe
`
	_, err := Parse(writeTempFixture(t, yaml))
	if !errors.Is(err, ErrInvalidExpect) {
		t.Errorf("expected ErrInvalidExpect, got %v", err)
	}
}

func TestParse_MissingCaseName(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
rule_name: some-rule
test_cases:
  - expect: pass
`
	_, err := Parse(writeTempFixture(t, yaml))
	if !errors.Is(err, ErrMissingCaseName) {
		t.Errorf("expected ErrMissingCaseName, got %v", err)
	}
}

func TestParse_HTTPMockData(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
rule_name: api-check
test_cases:
  - name: "API returns 200"
    expect: pass
    mock_data:
      http_responses:
        "https://api.github.com/repos/o/r":
          status_code: 200
          body: '{"ok": true}'
`
	f, err := Parse(writeTempFixture(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tc := f.TestCases[0]
	resp, ok := tc.MockData.HTTPResponses["https://api.github.com/repos/o/r"]
	if !ok {
		t.Fatal("expected HTTP mock for api.github.com URL")
	}
	if resp.StatusCode != 200 {
		t.Errorf("status_code = %d, want 200", resp.StatusCode)
	}
}

func TestParse_SkipReason_RelaxesExpectValidation(t *testing.T) {
	t.Parallel()
	// A case with skip_reason set must not fail validation even if expect is
	// empty, because the runner will never evaluate it.
	yaml := `
version: v1
rule_name: some-rule
test_cases:
  - name: "normal case"
    expect: pass
  - name: "skipped case"
    skip_reason: "requires git commit history, not yet supported"
`
	f, err := Parse(writeTempFixture(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.TestCases[1].SkipReason == "" {
		t.Error("expected skip_reason to be set on second case")
	}
}

func TestParse_SampleFixtureFile(t *testing.T) {
	t.Parallel()
	// Derive repo root from the source file location so this test
	// works regardless of the working directory.
	_, filename, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(filename), "..", "..")
	path := filepath.Join(root, "rules", "sample_rule_test.yaml")

	f, err := Parse(path)
	if err != nil {
		t.Fatalf("parsing sample fixture: %v", err)
	}
	if f.RuleName != "osps-vm-05" {
		t.Errorf("rule_name = %q, want %q", f.RuleName, "osps-vm-05")
	}
	if got := len(f.TestCases); got != 3 {
		t.Errorf("sample fixture has %d test cases, want 3", got)
	}
}
