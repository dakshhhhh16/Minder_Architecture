// SPDX-FileCopyrightText: Copyright 2026 The Minder Authors
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"io"
	"net/http"
	"testing"
)

// TestBuildMocks_AllThreeMockTypes exercises a fixture test case that uses
// all three ingestion paths at once: git_files, http_responses, and
// data_source_responses.  This is the realistic scenario for a complex rule
// that reads a file from the repository AND calls a REST API AND reads from a
// declared data source.
func TestBuildMocks_AllThreeMockTypes(t *testing.T) {
	t.Parallel()

	tc := TestCase{
		Name:   "all three mock types",
		Expect: "pass",
		MockData: ProviderMockConfig{
			GitFiles: map[string]string{
				"SECURITY.md": "Please report vulnerabilities to security@example.com",
			},
			HTTPResponses: map[string]HTTPResponseMock{
				"https://api.github.com/repos/owner/repo/vulnerability-alerts": {
					StatusCode: 200,
					Body:       `{"enabled": true}`,
				},
			},
			DataSourceResponses: map[string]HTTPResponseMock{
				"https://ds.example.com/org-policy": {
					StatusCode: 200,
					Body:       `{"require_security_md": true}`,
				},
			},
		},
	}

	mocks, err := BuildMocks(tc)
	if err != nil {
		t.Fatalf("BuildMocks returned unexpected error: %v", err)
	}

	// Git filesystem contains the declared file.
	f, err := mocks.GitFilesystem.Open("SECURITY.md")
	if err != nil {
		t.Fatalf("opening SECURITY.md: %v", err)
	}
	content, _ := io.ReadAll(f)
	f.Close()
	if string(content) != "Please report vulnerabilities to security@example.com" {
		t.Errorf("git file content = %q", string(content))
	}

	// HTTP client serves the vulnerability alerts response.
	req, _ := http.NewRequest(http.MethodGet,
		"https://api.github.com/repos/owner/repo/vulnerability-alerts", nil)
	resp, err := mocks.HTTPClient.Do(req)
	if err != nil {
		t.Fatalf("HTTP client request: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("http status = %d, want 200", resp.StatusCode)
	}

	// Data source client serves the org-policy response.
	req2, _ := http.NewRequest(http.MethodGet, "https://ds.example.com/org-policy", nil)
	resp2, err := mocks.DataSourceClient.Do(req2)
	if err != nil {
		t.Fatalf("DataSource client request: %v", err)
	}
	if resp2.StatusCode != 200 {
		t.Errorf("data source status = %d, want 200", resp2.StatusCode)
	}

	// Data source client does NOT accidentally serve HTTP provider URLs.
	req3, _ := http.NewRequest(http.MethodGet,
		"https://api.github.com/repos/owner/repo/vulnerability-alerts", nil)
	resp3, err := mocks.DataSourceClient.Do(req3)
	if err != nil {
		t.Fatalf("DataSource client unexpected error: %v", err)
	}
	if resp3.StatusCode == 200 {
		t.Error("data source client should not serve HTTP provider URLs")
	}
}

// TestDryRun_AllThreeMockTypes runs DryRun on a fixture that uses all three
// ingestion types, ensuring the full validation pipeline handles them.
func TestDryRun_AllThreeMockTypes_PassesValidation(t *testing.T) {
	t.Parallel()

	yaml := `
version: v1
rule_name: complex-rule
test_cases:
  - name: "all three mocks pass"
    expect: pass
    mock_data:
      git_files:
        "SECURITY.md": "security policy content"
      http_responses:
        "https://api.github.com/repos/owner/repo/vulnerability-alerts":
          status_code: 200
          body: '{"enabled": true}'
      data_source_responses:
        "https://ds.example.com/org-policy":
          status_code: 200
          body: '{"require_security_md": true}'
`
	results, err := DryRun(writeTempFixture(t, yaml))
	if err != nil {
		t.Fatalf("DryRun returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err != nil {
		t.Errorf("expected no error, got: %v", results[0].Err)
	}
	if results[0].Skipped {
		t.Error("case should not be skipped")
	}
}
