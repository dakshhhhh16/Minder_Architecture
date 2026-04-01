// SPDX-FileCopyrightText: Copyright 2026 The Minder Authors
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"testing"
)

// TestDryRun_AllCasesSkipped verifies that DryRun exits cleanly when every
// test case in the fixture has a skip_reason.  This is the expected state for
// rules that depend on git commit history: you scaffold the fixture, mark all
// cases as skipped, and come back to fill them in later.  DryRun should not
// treat a fully-skipped fixture as an error.
func TestDryRun_AllCasesSkipped_NoErrors(t *testing.T) {
	t.Parallel()

	yaml := `
version: v1
rule_name: git-history-rule
test_cases:
  - name: "needs default branch name"
    skip_reason: "requires git branch metadata, not yet supported by memfs"
  - name: "needs commit count"
    skip_reason: "requires git commit history, not yet supported by memfs"
  - name: "needs tag information"
    skip_reason: "requires git tags, not yet supported by memfs"
`
	results, err := DryRun(writeTempFixture(t, yaml))
	if err != nil {
		t.Fatalf("DryRun returned an unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for _, r := range results {
		if !r.Skipped {
			t.Errorf("case %q should be skipped but was not", r.Name)
		}
		if r.SkipReason == "" {
			t.Errorf("case %q should have a SkipReason set", r.Name)
		}
		if r.Err != nil {
			t.Errorf("case %q should have no error, got: %v", r.Name, r.Err)
		}
	}
}

// TestDryRun_SkipReasonPreservedVerbatim confirms that the exact text written
// in skip_reason comes back in the Result.  The runner surfaces this text in
// CI output so that contributors know exactly what needs to be done to un-skip
// the case, so it must not be truncated or modified.
func TestDryRun_SkipReasonPreservedVerbatim(t *testing.T) {
	t.Parallel()

	wantReason := "requires git commit history, not yet supported by memfs"

	yaml := `
version: v1
rule_name: some-rule
test_cases:
  - name: "skipped case"
    skip_reason: "requires git commit history, not yet supported by memfs"
`
	results, err := DryRun(writeTempFixture(t, yaml))
	if err != nil {
		t.Fatalf("DryRun returned an error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].SkipReason != wantReason {
		t.Errorf("SkipReason = %q, want %q", results[0].SkipReason, wantReason)
	}
}
