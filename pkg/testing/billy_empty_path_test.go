// SPDX-FileCopyrightText: Copyright 2026 The Minder Authors
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"testing"
)

// TestNewMockBillyFS_EmptyStringKey covers the fs.Create error branch inside
// NewMockBillyFS.  The go-billy in-memory filesystem treats an empty string
// path as the root directory and refuses to create a file there, so the
// function must propagate that error to the caller rather than silently
// succeeding with a broken filesystem.
func TestNewMockBillyFS_EmptyStringKey_ReturnsError(t *testing.T) {
	t.Parallel()

	// An empty string key is the easiest way to make the in-memory filesystem
	// return an error from Create without any special setup.
	_, err := NewMockBillyFS(map[string]string{
		"": "this path is empty and should fail",
	})

	if err == nil {
		t.Fatal("expected an error for empty string file path, got nil")
	}
}

// TestNewMockBillyFS_ValidAndInvalidKeys verifies that a map containing both
// a valid path and an empty-string path still returns an error.  The function
// should not partially succeed and return a half-populated filesystem.
func TestNewMockBillyFS_MixedValidAndEmptyKeys_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := NewMockBillyFS(map[string]string{
		"SECURITY.md": "valid content",
		"":            "empty key should cause failure",
	})

	if err == nil {
		t.Fatal("expected an error when one key is an empty string, got nil")
	}
}
