package testing

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// SupportedVersion is the only fixture schema version accepted by the parser.
const SupportedVersion = "v1"

// Sentinel errors returned by Parse.
var (
	ErrEmptyPath        = errors.New("fixture path must not be empty")
	ErrUnsupportedVer   = errors.New("unsupported fixture version")
	ErrMissingRuleName  = errors.New("rule_name is required")
	ErrNoTestCases      = errors.New("at least one test_case is required")
	ErrInvalidExpect    = errors.New("expect must be \"pass\" or \"fail\"")
	ErrMissingCaseName  = errors.New("test case name is required")
)

// Fixture defines the top-level schema for a multi-case rule test.
type Fixture struct {
	Version   string     `yaml:"version"`
	RuleName  string     `yaml:"rule_name"`
	TestCases []TestCase `yaml:"test_cases"`
}

// TestCase outlines a specific evaluation branch and the expected result.
type TestCase struct {
	Name     string             `yaml:"name"`
	Expect   string             `yaml:"expect"`
	MockData ProviderMockConfig `yaml:"mock_data"`
}

// ProviderMockConfig holds mocked data for REST APIs, Git FS, or Data Sources.
type ProviderMockConfig struct {
	HTTPResponses map[string]HTTPResponseMock `yaml:"http_responses,omitempty"`
	GitFiles      map[string]string           `yaml:"git_files,omitempty"`
}

// HTTPResponseMock represents a single canned HTTP response.
type HTTPResponseMock struct {
	StatusCode int    `yaml:"status_code"`
	Body       string `yaml:"body"`
}

// Parse reads a YAML fixture file from disk, deserialises it, and runs
// structural validation.  It returns a ready-to-use Fixture or an error
// describing the first validation failure.
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

// validate checks every invariant the schema requires.
func (f *Fixture) validate() error {
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
		if tc.Expect != "pass" && tc.Expect != "fail" {
			return fmt.Errorf("test_cases[%d] %q: %w: got %q", i, tc.Name, ErrInvalidExpect, tc.Expect)
		}
	}
	return nil
}
