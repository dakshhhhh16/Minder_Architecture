package testing

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// writeTempFixture is a small helper that writes YAML content to a temp
// file and returns the path. Makes the tests much cleaner to read.
func writeTempFixture(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing temp fixture: %v", err)
	}
	return path
}

// ---- Basic parsing tests ----

func TestParse_ValidFixture(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
rule_name: osps-vm-05
test_cases:
  - name: "SECURITY.md exists"
    expect: pass
    entity:
      type: repository
      entity:
        owner: "testowner"
        name: "testrepo"
    mock_data:
      git_files:
        "SECURITY.md": "report vulns here"
  - name: "No security file"
    expect: fail
    entity:
      type: repository
      entity:
        owner: "testowner"
        name: "testrepo"
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
	if f.TestCases[0].Entity.Type != "repository" {
		t.Errorf("test_cases[0].entity.type = %q, want \"repository\"", f.TestCases[0].Entity.Type)
	}
}

// ---- Validation error tests ----

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

func TestParse_MissingVersion(t *testing.T) {
	t.Parallel()
	yaml := `
rule_name: some-rule
test_cases:
  - name: "case"
    expect: pass
    entity:
      type: repo
      entity: {}
`
	_, err := Parse(writeTempFixture(t, yaml))
	if !errors.Is(err, ErrMissingVersion) {
		t.Errorf("expected ErrMissingVersion, got %v", err)
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
    entity:
      type: repo
      entity: {}
`
	_, err := Parse(writeTempFixture(t, yaml))
	if !errors.Is(err, ErrUnsupportedVer) {
		t.Errorf("expected ErrUnsupportedVer, got %v", err)
	}
}

func TestParse_MissingRuleName(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
test_cases:
  - name: "case"
    expect: pass
    entity:
      type: repo
      entity: {}
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
    entity:
      type: repo
      entity: {}
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
    entity:
      type: repo
      entity: {}
`
	_, err := Parse(writeTempFixture(t, yaml))
	if !errors.Is(err, ErrMissingCaseName) {
		t.Errorf("expected ErrMissingCaseName, got %v", err)
	}
}

func TestParse_MissingEntity(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
rule_name: some-rule
test_cases:
  - name: "no entity"
    expect: pass
    mock_data:
      git_files:
        "README.md": "hello"
`
	_, err := Parse(writeTempFixture(t, yaml))
	if !errors.Is(err, ErrMissingEntity) {
		t.Errorf("expected ErrMissingEntity, got %v", err)
	}
}

// ---- Data source key validation ----

func TestParse_InvalidDataSourceKey(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
rule_name: ds-rule
test_cases:
  - name: "bad ds key"
    expect: pass
    entity:
      type: repo
      entity: {}
    mock_data:
      data_source_responses:
        "no-dot-separator":
          body: "{}"
`
	_, err := Parse(writeTempFixture(t, yaml))
	if !errors.Is(err, ErrInvalidDSKey) {
		t.Errorf("expected ErrInvalidDSKey, got %v", err)
	}
}

func TestParse_ValidDataSourceKey(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
rule_name: ds-rule
test_cases:
  - name: "valid ds"
    expect: pass
    entity:
      type: repo
      entity: {}
    mock_data:
      data_source_responses:
        "osv.query":
          body: '{"vulns": []}'
`
	f, err := Parse(writeTempFixture(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tc := f.TestCases[0]
	ds, ok := tc.MockData.DataSourceResponses["osv.query"]
	if !ok {
		t.Fatal("expected data_source_responses for osv.query")
	}
	if ds.Body != `{"vulns": []}` {
		t.Errorf("body = %q, want %q", ds.Body, `{"vulns": []}`)
	}
}

// ---- HTTP mock data parsing ----

func TestParse_HTTPMockData(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
rule_name: api-check
test_cases:
  - name: "API returns 200"
    expect: pass
    entity:
      type: repo
      entity: {}
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

// ---- Def and params ----

func TestParse_DefAndParams(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
rule_name: secret_scanning
test_cases:
  - name: "with def and params"
    expect: pass
    def:
      enabled: true
    params:
      severity: high
    entity:
      type: repository
      entity:
        owner: "testowner"
        name: "testrepo"
    mock_data:
      http_responses:
        "https://api.github.com/repos/testowner/testrepo":
          status_code: 200
          body: '{}'
`
	f, err := Parse(writeTempFixture(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tc := f.TestCases[0]
	if tc.Def["enabled"] != true {
		t.Errorf("def.enabled = %v, want true", tc.Def["enabled"])
	}
	if tc.Params["severity"] != "high" {
		t.Errorf("params.severity = %v, want \"high\"", tc.Params["severity"])
	}
}

// ---- Skip reason ----

func TestParse_SkipReason_RelaxesValidation(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
rule_name: some-rule
test_cases:
  - name: "normal case"
    expect: pass
    entity:
      type: repo
      entity: {}
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

// ---- Error expect ----

func TestParse_ErrorExpect_Valid(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
rule_name: some-rule
test_cases:
  - name: "error case"
    expect: error
    error_contains: "missing data source"
    entity:
      type: repo
      entity: {}
`
	f, err := Parse(writeTempFixture(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error for expect=error: %v", err)
	}
	if f.TestCases[0].ErrorContains != "missing data source" {
		t.Errorf("error_contains = %q, want %q", f.TestCases[0].ErrorContains, "missing data source")
	}
}

func TestParse_ErrorExpect_MissingErrorContains(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
rule_name: some-rule
test_cases:
  - name: "error without error_contains"
    expect: error
    entity:
      type: repo
      entity: {}
`
	_, err := Parse(writeTempFixture(t, yaml))
	if !errors.Is(err, ErrMissingErrContains) {
		t.Errorf("expected ErrMissingErrContains, got %v", err)
	}
}

// ---- Sample fixture file ----

func TestParse_SampleFixtureFile(t *testing.T) {
	t.Parallel()
	_, filename, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(filename), "..", "..")
	path := filepath.Join(root, "examples", "git_rule_test.yaml")

	f, err := Parse(path)
	if err != nil {
		t.Fatalf("parsing sample fixture: %v", err)
	}
	if f.RuleName != "osps-vm-05" {
		t.Errorf("rule_name = %q, want %q", f.RuleName, "osps-vm-05")
	}
}
