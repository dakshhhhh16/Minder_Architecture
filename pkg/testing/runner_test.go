package testing

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ---- DryRun tests ----

func TestDryRun_ValidFixture_AllCasesPass(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
rule_name: osps-vm-05
test_cases:
  - name: "SECURITY.md present"
    expect: pass
    entity:
      type: repository
      entity:
        owner: "testowner"
        name: "testrepo"
    mock_data:
      git_files:
        "SECURITY.md": "report vulns here"
      http_responses:
        "https://api.github.com/repos/testowner/testrepo/vulnerability-alerts":
          status_code: 200
          body: '{"enabled":true}'
  - name: "no security file"
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
	results, err := DryRun(writeTempFixture(t, yaml))
	if err != nil {
		t.Fatalf("DryRun returned error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	for _, r := range results {
		if r.Skipped {
			t.Errorf("case %q unexpectedly skipped", r.Name)
		}
		if r.Err != nil {
			t.Errorf("case %q: unexpected error: %v", r.Name, r.Err)
		}
	}
}

func TestDryRun_SkippedCasesAreReported(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
rule_name: git-history-rule
test_cases:
  - name: "evaluable case"
    expect: pass
    entity:
      type: repository
      entity:
        owner: "testowner"
        name: "testrepo"
    mock_data:
      git_files:
        "SECURITY.md": "content"
  - name: "needs commit history"
    skip_reason: "requires git commit history, memfs does not support this yet"
`
	results, err := DryRun(writeTempFixture(t, yaml))
	if err != nil {
		t.Fatalf("DryRun returned error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}

	if results[0].Skipped {
		t.Error("first case should not be skipped")
	}
	if !results[1].Skipped {
		t.Error("second case should be skipped")
	}
	if results[1].SkipReason == "" {
		t.Error("second case: SkipReason should not be empty")
	}
}

func TestDryRun_MissingFile_ReturnsError(t *testing.T) {
	t.Parallel()
	_, err := DryRun("/tmp/does_not_exist_fixture_dryrun.yaml")
	if err == nil {
		t.Fatal("expected error for missing fixture file, got nil")
	}
}

func TestDryRun_InvalidFixture_ReturnsError(t *testing.T) {
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
	_, err := DryRun(writeTempFixture(t, yaml))
	if err == nil {
		t.Fatal("expected error for missing version, got nil")
	}
}

func TestDryRun_RelativeURL_CaughtAsError(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
rule_name: some-rule
test_cases:
  - name: "relative url"
    expect: pass
    entity:
      type: repository
      entity:
        owner: "o"
        name: "r"
    mock_data:
      http_responses:
        "/relative/path":
          status_code: 200
          body: "ok"
`
	results, err := DryRun(writeTempFixture(t, yaml))
	if err != nil {
		t.Fatalf("DryRun itself returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Err == nil {
		t.Error("expected error for relative URL, got nil")
	}
}

func TestDryRun_InvalidDSKey_CaughtDuringParse(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
rule_name: ds-rule
test_cases:
  - name: "bad ds key"
    expect: pass
    entity:
      type: repository
      entity:
        owner: "o"
        name: "r"
    mock_data:
      data_source_responses:
        "nodot":
          body: "{}"
`
	_, err := DryRun(writeTempFixture(t, yaml))
	if err == nil {
		t.Error("expected error for invalid data source key, got nil")
	}
}

func TestDryRun_ValidDSKey_Passes(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
rule_name: osv-rule
test_cases:
  - name: "valid ds"
    expect: pass
    entity:
      type: repository
      entity:
        owner: "o"
        name: "r"
    mock_data:
      data_source_responses:
        "osv.query":
          body: '{"vulns": []}'
`
	results, err := DryRun(writeTempFixture(t, yaml))
	if err != nil {
		t.Fatalf("DryRun returned error: %v", err)
	}
	if results[0].Err != nil {
		t.Errorf("expected no error for valid DS key, got: %v", results[0].Err)
	}
}

func TestDryRun_SampleFixtureFile(t *testing.T) {
	t.Parallel()
	_, filename, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(filename), "..", "..")
	path := filepath.Join(root, "examples", "git_rule_test.yaml")

	if _, err := os.Stat(path); err != nil {
		t.Skipf("sample fixture not found at %s: %v", path, err)
	}

	results, err := DryRun(path)
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	for _, r := range results {
		if !r.Skipped && r.Err != nil {
			t.Errorf("case %q: %v", r.Name, r.Err)
		}
	}
}

// ---- Evaluate tests ----

func TestEvaluate_FallsThroughToDryRun(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
rule_name: test-rule
test_cases:
  - name: "simple pass"
    expect: pass
    entity:
      type: repository
      entity:
        owner: "o"
        name: "r"
    mock_data:
      git_files:
        "README.md": "hello"
`
	results, err := Evaluate(writeTempFixture(t, yaml))
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Err != nil {
		t.Errorf("unexpected error: %v", results[0].Err)
	}
}

// ---- ResolveRuleTypePath tests ----

func TestResolveRuleTypePath_Found(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	ruleDir := filepath.Join(base, "rule-types", "github")
	if err := os.MkdirAll(ruleDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	rulePath := filepath.Join(ruleDir, "secret_scanning.yaml")
	if err := os.WriteFile(rulePath, []byte("# rule type"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := ResolveRuleTypePath(base, "secret_scanning")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != rulePath {
		t.Errorf("path = %q, want %q", got, rulePath)
	}
}

func TestResolveRuleTypePath_NotFound(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	_, err := ResolveRuleTypePath(base, "nonexistent_rule")
	if err == nil {
		t.Error("expected error for missing rule type, got nil")
	}
}
