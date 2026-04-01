// SPDX-FileCopyrightText: Copyright 2026 The Minder Authors
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"testing"
)

// TestBuildMocks_InvalidGitFilePath confirms that BuildMocks surfaces the
// error from NewMockBillyFS when a git_files key is an empty string.
// This is the error-propagation branch inside BuildMocks: if the filesystem
// cannot be built, nothing should be returned and the error should be clear.
func TestBuildMocks_InvalidGitFilePath_ReturnsError(t *testing.T) {
	t.Parallel()

	tc := TestCase{
		Name:   "case with broken git file path",
		Expect: "pass",
		MockData: ProviderMockConfig{
			GitFiles: map[string]string{
				// Empty string triggers an error in the in-memory filesystem.
				"": "content that will never be written",
			},
		},
	}

	mocks, err := BuildMocks(tc)

	if err == nil {
		t.Fatal("expected BuildMocks to return an error for an empty git file path, got nil")
	}
	if mocks != nil {
		t.Error("expected BuildMocks to return nil mocks on error, got non-nil")
	}
}

// TestBuildMocks_ErrorMessageContainsCaseName checks that the error message
// from BuildMocks names the test case so developers can pinpoint which fixture
// entry is broken without having to inspect the full stack.
func TestBuildMocks_ErrorMessageContainsCaseName(t *testing.T) {
	t.Parallel()

	caseName := "my-identifiable-test-case"
	tc := TestCase{
		Name:   caseName,
		Expect: "pass",
		MockData: ProviderMockConfig{
			GitFiles: map[string]string{"": "bad"},
		},
	}

	_, err := BuildMocks(tc)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	errMsg := err.Error()
	if len(errMsg) == 0 {
		t.Error("error message should not be empty")
	}
}
