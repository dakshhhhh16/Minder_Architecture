// SPDX-FileCopyrightText: Copyright 2026 The Minder Authors
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"testing"
)

// TestDryRun_ResultCountMatchesTestCaseCount is a contract test.
// DryRun must return exactly one Result per test case in the fixture,
// regardless of whether the case passed, failed, was skipped, or hit an error.
// CI tooling that counts pass/fail/skip ratios depends on this guarantee.
func TestDryRun_ResultCountMatchesTestCaseCount(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name      string
		caseCount int
		yaml      string
	}{
		{
			name:      "one case",
			caseCount: 1,
			yaml: `
version: v1
rule_name: some-rule
test_cases:
  - name: "only case"
    expect: pass
    mock_data:
      git_files:
        "SECURITY.md": "content"
`,
		},
		{
			name:      "three cases mixed",
			caseCount: 3,
			yaml: `
version: v1
rule_name: some-rule
test_cases:
  - name: "passes"
    expect: pass
    mock_data:
      git_files:
        "SECURITY.md": "content"
  - name: "skipped"
    skip_reason: "not yet supported"
  - name: "fails"
    expect: fail
    mock_data:
      git_files:
        "README.md": "hello"
`,
		},
		{
			name:      "five cases all skipped",
			caseCount: 5,
			yaml: `
version: v1
rule_name: some-rule
test_cases:
  - name: "skipped 1"
    skip_reason: "reason"
  - name: "skipped 2"
    skip_reason: "reason"
  - name: "skipped 3"
    skip_reason: "reason"
  - name: "skipped 4"
    skip_reason: "reason"
  - name: "skipped 5"
    skip_reason: "reason"
`,
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			results, err := DryRun(writeTempFixture(t, tt.yaml))
			if err != nil {
				t.Fatalf("DryRun returned error: %v", err)
			}
			if len(results) != tt.caseCount {
				t.Errorf("len(results) = %d, want %d", len(results), tt.caseCount)
			}
		})
	}
}

// TestDryRun_ResultNameMatchesCaseName confirms that the Name field in each
// Result matches the test case name from the fixture in the same position.
// CI output shows this name, so it must round-trip cleanly.
func TestDryRun_ResultNameMatchesCaseName(t *testing.T) {
	t.Parallel()

	yaml := `
version: v1
rule_name: some-rule
test_cases:
  - name: "first case"
    expect: pass
    mock_data:
      git_files:
        "SECURITY.md": "content"
  - name: "second case"
    skip_reason: "not yet supported"
`
	results, err := DryRun(writeTempFixture(t, yaml))
	if err != nil {
		t.Fatalf("DryRun returned error: %v", err)
	}

	wantNames := []string{"first case", "second case"}
	for i, r := range results {
		if r.Name != wantNames[i] {
			t.Errorf("results[%d].Name = %q, want %q", i, r.Name, wantNames[i])
		}
	}
}
