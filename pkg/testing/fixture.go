// SPDX-FileCopyrightText: Copyright 2026 The Minder Authors
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// SupportedVersion is the only fixture schema version the parser accepts.
const SupportedVersion = "v1"

// Sentinel errors returned by Parse.
var (
	ErrEmptyPath       = errors.New("fixture path must not be empty")
	ErrMissingVersion  = errors.New("fixture version is required")
	ErrUnsupportedVer  = errors.New("unsupported fixture version")
	ErrMissingRuleName = errors.New("rule_name is required")
	ErrNoTestCases     = errors.New("at least one test_case is required")
	ErrInvalidExpect   = errors.New("expect must be \"pass\" or \"fail\"")
	ErrMissingCaseName = errors.New("test case name is required")
)

// Fixture is the top-level schema for a multi-case rule test file.
type Fixture struct {
	Version   string     `yaml:"version"`
	RuleName  string     `yaml:"rule_name"`
	TestCases []TestCase `yaml:"test_cases"`
}

// TestCase describes one evaluation scenario and its expected outcome.
type TestCase struct {
	Name       string             `yaml:"name"`
	Expect     string             `yaml:"expect"`
	SkipReason string             `yaml:"skip_reason,omitempty"`
	MockData   ProviderMockConfig `yaml:"mock_data"`
}

// ProviderMockConfig holds mock data for REST APIs, Git files, or Data Sources.
type ProviderMockConfig struct {
	HTTPResponses       map[string]HTTPResponseMock `yaml:"http_responses,omitempty"`
	GitFiles            map[string]string           `yaml:"git_files,omitempty"`
	DataSourceResponses map[string]HTTPResponseMock `yaml:"data_source_responses,omitempty"`
}

// HTTPResponseMock is a canned HTTP response used by the mock transport.
type HTTPResponseMock struct {
	StatusCode int    `yaml:"status_code"`
	Body       string `yaml:"body"`
}

// Parse reads a YAML fixture from disk and validates its structure.
// Returns a Fixture ready for use, or an error describing the first
// validation failure.
func Parse(path string) (*Fixture, error) {
	if path == "" {
		return nil, ErrEmptyPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading fixture %s: %w", path, err)
	}

	var f Fixture
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("unmarshalling fixture: %w", err)
	}

	if err := f.validate(); err != nil {
		return nil, err
	}

	return &f, nil
}

// validate checks every required invariant.
func (f *Fixture) validate() error {
	if f.Version == "" {
		return ErrMissingVersion
	}
	if f.Version != SupportedVersion {
		return fmt.Errorf("%w: got %q, want %q", ErrUnsupportedVer, f.Version, SupportedVersion)
	}
	if f.RuleName == "" {
		return ErrMissingRuleName
	}
	if len(f.TestCases) == 0 {
		return ErrNoTestCases
	}
	for i, tc := range f.TestCases {
		if tc.Name == "" {
			return fmt.Errorf("test_cases[%d]: %w", i, ErrMissingCaseName)
		}
		// Skipped cases don't need an expect value.
		if tc.SkipReason != "" {
			continue
		}
		if tc.Expect != "pass" && tc.Expect != "fail" {
			return fmt.Errorf("test_cases[%d] %q: %w: got %q", i, tc.Name, ErrInvalidExpect, tc.Expect)
		}
	}
	return nil
}
