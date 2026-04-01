// SPDX-FileCopyrightText: Copyright 2026 The Minder Authors
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParse_MalformedYAML covers the yaml.Unmarshal error branch inside Parse.
// The YAML parser rejects the triple-brace sequence because it is not valid
// YAML flow-mapping syntax, so Unmarshal returns an error before validation
// even runs.
func TestParse_MalformedYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")

	// Write bytes that look like a YAML file but are syntactically broken.
	// The unclosed flow mapping "{{{" triggers a parse error in yaml.v3.
	if err := os.WriteFile(path, []byte("{{{this is not valid yaml"), 0o644); err != nil {
		t.Fatalf("writing bad fixture file: %v", err)
	}

	_, err := Parse(path)
	if err == nil {
		t.Fatal("expected an error for malformed YAML, got nil")
	}
}

// TestParse_BinaryContent covers the same branch with non-text bytes.
// Rule fixture files should always be plain text; passing binary data is
// another way the YAML parser can fail.
func TestParse_BinaryContent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "binary.yaml")

	// Write raw bytes that cannot form valid YAML.
	if err := os.WriteFile(path, []byte{0x80, 0x81, 0x82, 0xff, 0xfe}, 0o644); err != nil {
		t.Fatalf("writing binary fixture file: %v", err)
	}

	_, err := Parse(path)
	if err == nil {
		t.Fatal("expected an error for binary content, got nil")
	}
}
