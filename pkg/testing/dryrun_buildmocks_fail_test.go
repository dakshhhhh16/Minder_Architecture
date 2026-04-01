// SPDX-FileCopyrightText: Copyright 2026 The Minder Authors
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"testing"
)

// TestDryRun_BuildMocksFailure covers the branch inside DryRun where
// BuildMocks returns an error.  DryRun should not stop processing; it should
// record the failing case in the results and continue to the next one.
// This matters for CI: if one fixture entry is broken you still want to see
// all the other failures, not just the first.
func TestDryRun_BuildMocksFailure_RecordedInResults(t *testing.T) {
	t.Parallel()

	// An empty-string git_files key makes BuildMocks fail because the
	// in-memory filesystem refuses to create a file at that path.
	yaml := `
version: v1
rule_name: some-rule
test_cases:
  - name: "broken case"
    expect: pass
    mock_data:
      git_files:
        "": "this path is invalid"
`
	results, err := DryRun(writeTempFixture(t, yaml))

	// DryRun itself should not return a top-level error; the fixture is valid.
	if err != nil {
		t.Fatalf("DryRun returned unexpected top-level error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err == nil {
		t.Error("expected the result to carry an error for the broken case, got nil")
	}
	if results[0].Skipped {
		t.Error("a case with a build error should not be marked as skipped")
	}
}

// TestDryRun_MixedBrokenAndHealthyCases confirms that DryRun keeps going
// after a BuildMocks failure and records results for all subsequent cases.
func TestDryRun_MixedBrokenAndHealthyCases(t *testing.T) {
	t.Parallel()

	yaml := `
version: v1
rule_name: some-rule
test_cases:
  - name: "broken case"
    expect: pass
    mock_data:
      git_files:
        "": "bad path"
  - name: "healthy case"
    expect: pass
    mock_data:
      git_files:
        "SECURITY.md": "report vulns here"
`
	results, err := DryRun(writeTempFixture(t, yaml))
	if err != nil {
		t.Fatalf("DryRun returned unexpected top-level error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Err == nil {
		t.Error("first case should have an error")
	}
	if results[1].Err != nil {
		t.Errorf("second case should have no error, got: %v", results[1].Err)
	}
}
