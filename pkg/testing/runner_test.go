// SPDX-FileCopyrightText: Copyright 2026 The Minder Authors
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDryRun_ValidFixture_AllPass(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
rule_name: osps-vm-05
test_cases:
  - name: "SECURITY.md present"
    expect: pass
    mock_data:
      git_files:
        "SECURITY.md": "report vulns here"
      http_responses:
        "https://api.github.com/repos/o/r/vulnerability-alerts":
          status_code: 200
          body: '{"enabled":true}'
  - name: "no security file"
    expect: fail
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

func TestDryRun_SkippedCasesReported(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
rule_name: git-history-rule
test_cases:
  - name: "evaluable case"
    expect: pass
    mock_data:
      git_files:
        "SECURITY.md": "content"
  - name: "needs commit history"
    skip_reason: "requires git commit history, not yet supported by memfs"
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
	if results[0].Err != nil {
		t.Errorf("first case: unexpected error: %v", results[0].Err)
	}

	if !results[1].Skipped {
		t.Error("second case should be skipped")
	}
	if results[1].SkipReason == "" {
		t.Error("second case: SkipReason should be non-empty")
	}
}

func TestDryRun_BadFixturePath_ReturnsError(t *testing.T) {
	t.Parallel()
	_, err := DryRun("/tmp/does_not_exist_fixture_dryrun.yaml")
	if err == nil {
		t.Fatal("expected error for missing fixture file, got nil")
	}
}

func TestDryRun_InvalidFixtureContent_ReturnsError(t *testing.T) {
	t.Parallel()
	yaml := `
rule_name: some-rule
test_cases:
  - name: "case"
    expect: pass
`
	_, err := DryRun(writeTempFixture(t, yaml))
	if err == nil {
		t.Fatal("expected error for invalid fixture, got nil")
	}
}

func TestDryRun_InvalidURL_ReturnsResultError(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
rule_name: some-rule
test_cases:
  - name: "bad url"
    expect: pass
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
		t.Error("expected Err for relative URL, got nil")
	}
}

func TestDryRun_DataSourceURLValidated(t *testing.T) {
	t.Parallel()
	yaml := `
version: v1
rule_name: ds-rule
test_cases:
  - name: "bad data source url"
    expect: pass
    mock_data:
      data_source_responses:
        "not-a-url":
          status_code: 200
          body: "{}"
`
	results, err := DryRun(writeTempFixture(t, yaml))
	if err != nil {
		t.Fatalf("DryRun itself returned error: %v", err)
	}
	if results[0].Err == nil {
		t.Error("expected Err for invalid data_source_response URL, got nil")
	}
}

func TestDryRun_SampleFixtureFile(t *testing.T) {
	t.Parallel()
	_, filename, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(filename), "..", "..")
	path := filepath.Join(root, "rules", "sample_rule_test.yaml")

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
