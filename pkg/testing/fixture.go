package testing

// Fixture defines the top-level schema for a multi-case rule test.
type Fixture struct {
	Version   string     `yaml:"version" validate:"required,eq=v1"`
	RuleName  string     `yaml:"rule_name" validate:"required"`
	TestCases []TestCase `yaml:"test_cases" validate:"required,dive,required"`
}

// TestCase outlines a specific evaluation branch and the expected result.
type TestCase struct {
	Name     string             `yaml:"name" validate:"required"`
	Expect   string             `yaml:"expect" validate:"required,oneof=pass fail"`
	MockData ProviderMockConfig `yaml:"mock_data" validate:"required"`
}

// ProviderMockConfig holds mocked data for REST APIs, Git FS, or Data Sources.
type ProviderMockConfig struct {
	HTTPResponses map[string]HTTPResponseMock `yaml:"http_responses,omitempty"`
	GitFiles      map[string]string           `yaml:"git_files,omitempty"`
}

type HTTPResponseMock struct {
	StatusCode int    `yaml:"status_code"`
	Body       string `yaml:"body"`
}
