package testing

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// SupportedVersion is the only fixture schema version we accept right now.
const SupportedVersion = "v1"

// These are the errors you will see when a fixture file has something wrong
// with it. Each one maps to a specific validation check so you know exactly
// what to fix.
var (
	ErrEmptyPath          = errors.New("fixture path must not be empty")
	ErrMissingVersion     = errors.New("fixture version is required")
	ErrUnsupportedVer     = errors.New("unsupported fixture version")
	ErrMissingRuleName    = errors.New("rule_name is required")
	ErrNoTestCases        = errors.New("at least one test_case is required")
	ErrInvalidExpect      = errors.New("expect must be \"pass\", \"fail\", or \"error\"")
	ErrMissingCaseName    = errors.New("test case name is required")
	ErrMissingEntity      = errors.New("entity with type is required for non-skipped test cases")
	ErrInvalidDSKey       = errors.New("data_source_responses key must be \"name.func\" format")
	ErrMissingErrContains = errors.New("error_contains is required when expect is \"error\"")
)

// Fixture is the top level schema for a rule test file. One fixture tests
// one rule with multiple test cases. The rule type YAML gets auto-resolved
// from RuleName by walking the rule-types/ directory.
type Fixture struct {
	Version   string     `yaml:"version"`
	RuleName  string     `yaml:"rule_name"`
	TestCases []TestCase `yaml:"test_cases"`
}

// TestCase is a single scenario you want to test. You give it the rule
// definition overrides, params, the entity to evaluate against, and the
// mock data that the engine should use instead of making real API calls.
//
// Expect tells the runner what should happen:
//   - "pass" means the entity complies with the rule
//   - "fail" means the entity does not comply
//   - "error" means the engine itself should return an error (and
//     ErrorContains tells us what substring to look for in that error)
//
// SkipReason lets you document test cases that can not run yet, like
// rules that need git commit history which the memfs mock does not support.
type TestCase struct {
	Name          string             `yaml:"name"`
	Expect        string             `yaml:"expect"`
	ErrorContains string             `yaml:"error_contains,omitempty"`
	SkipReason    string             `yaml:"skip_reason,omitempty"`
	Def           map[string]any     `yaml:"def,omitempty"`
	Params        map[string]any     `yaml:"params,omitempty"`
	Entity        EntityConfig       `yaml:"entity,omitempty"`
	MockData      ProviderMockConfig `yaml:"mock_data"`
}

// EntityConfig tells the engine what kind of entity to evaluate the rule
// against. For Minder, type is usually "repository" and the entity map
// contains things like owner and name.
type EntityConfig struct {
	Type   string         `yaml:"type"`
	Entity map[string]any `yaml:"entity"`
}

// ProviderMockConfig holds all the fake data for one test case. Depending
// on the rule type, you will use one or more of these:
//   - HTTPResponses for REST API rules (like branch protection checks)
//   - GitFiles for file based rules (like checking for SECURITY.md)
//   - DataSourceResponses for Rego data source rules (like OSV queries)
type ProviderMockConfig struct {
	HTTPResponses       map[string]HTTPResponseMock       `yaml:"http_responses,omitempty"`
	GitFiles            map[string]string                 `yaml:"git_files,omitempty"`
	DataSourceResponses map[string]DataSourceResponseMock `yaml:"data_source_responses,omitempty"`
}

// HTTPResponseMock is a canned HTTP response. The key in the parent map
// is the full URL that the rule will request, and this struct says what
// status code and body to send back.
type HTTPResponseMock struct {
	StatusCode int    `yaml:"status_code"`
	Body       string `yaml:"body"`
}

// DataSourceResponseMock holds the canned response for a Rego data source
// function. The key in the parent map uses the format "name.func", for
// example "osv.query". During evaluation this gets wired into a
// MockDataSourceFuncDef and registered in the DataSourceRegistry.
type DataSourceResponseMock struct {
	Body string `yaml:"body"`
}

// Parse reads a YAML fixture file from disk, unmarshals it, and runs
// validation on the structure. If anything is wrong it tells you exactly
// what the problem is.
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

// validate walks through every field and makes sure the fixture is
// well formed before we try to do anything with it.
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

		// Skipped cases do not need expect or entity, they are just documented
		// placeholders for things we can not test yet.
		if tc.SkipReason != "" {
			continue
		}

		if tc.Expect != "pass" && tc.Expect != "fail" && tc.Expect != "error" {
			return fmt.Errorf("test_cases[%d] %q: %w: got %q", i, tc.Name, ErrInvalidExpect, tc.Expect)
		}

		// If you say expect: error, you need to also say what error message
		// to look for, otherwise we cannot tell "error" apart from "fail".
		if tc.Expect == "error" && tc.ErrorContains == "" {
			return fmt.Errorf("test_cases[%d] %q: %w", i, tc.Name, ErrMissingErrContains)
		}

		// Every non-skipped case needs an entity to evaluate against.
		if tc.Entity.Type == "" {
			return fmt.Errorf("test_cases[%d] %q: %w", i, tc.Name, ErrMissingEntity)
		}

		// Data source keys must use the "name.func" format so we know
		// which data source and which function to mock.
		for key := range tc.MockData.DataSourceResponses {
			parts := strings.SplitN(key, ".", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return fmt.Errorf("test_cases[%d] %q: %w: got %q", i, tc.Name, ErrInvalidDSKey, key)
			}
		}
	}

	return nil
}
