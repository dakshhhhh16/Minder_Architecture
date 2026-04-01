// SPDX-FileCopyrightText: Copyright 2026 The Minder Authors
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
)

// ---------------------------------------------------------------------------
// HTTP mocking
// ---------------------------------------------------------------------------

// MockRoundTripper intercepts outbound HTTP requests and returns pre-defined
// responses declared in a fixture file.  Use NewMockRoundTripper to construct
// one; do not initialise ExpectedResponses directly.
//
// Integration note: assign an instance to restHandler.testOnlyTransport in
// internal/datasources/rest/handler.go to mock both REST provider calls
// (http_responses) and declared data source calls (data_source_responses).
// Use separate instances for each so the two mock sets remain independent.
type MockRoundTripper struct {
	ExpectedResponses map[string]HTTPResponseMock
}

// NewMockRoundTripper returns a MockRoundTripper pre-loaded with responses.
// Passing nil is safe and results in a tripper that returns 404 for every URL.
func NewMockRoundTripper(responses map[string]HTTPResponseMock) *MockRoundTripper {
	if responses == nil {
		responses = make(map[string]HTTPResponseMock)
	}
	return &MockRoundTripper{ExpectedResponses: responses}
}

// RoundTrip implements http.RoundTripper.  It serves the canned response whose
// key matches the full request URL, or a 404 JSON body if no match is found.
// The unmatched URL is included in the 404 body to simplify fixture debugging.
func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	urlStr := req.URL.String()

	hdr := make(http.Header)
	hdr.Set("Content-Type", "application/json")

	if mockResp, exists := m.ExpectedResponses[urlStr]; exists {
		return &http.Response{
			StatusCode: mockResp.StatusCode,
			Body:       io.NopCloser(bytes.NewBufferString(mockResp.Body)),
			Header:     hdr,
			Request:    req,
		}, nil
	}

	body := fmt.Sprintf(`{"error":"mock data not found","url":%q}`, urlStr)
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     hdr,
		Request:    req,
	}, nil
}

// ---------------------------------------------------------------------------
// Git filesystem mocking
// ---------------------------------------------------------------------------

// NewMockBillyFS builds an in-memory billy.Filesystem pre-populated with the
// provided files.  Each map key is the file path; the value is its content.
// This is compatible with the go-git/go-billy interface used by Minder's Git
// provider, enabling rule evaluation without cloning a real repository.
func NewMockBillyFS(files map[string]string) (billy.Filesystem, error) {
	fs := memfs.New()
	for path, content := range files {
		f, err := fs.Create(path)
		if err != nil {
			return nil, fmt.Errorf("creating mock file %q: %w", path, err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("writing mock file %q: %w", path, err)
		}
		if err := f.Close(); err != nil {
			return nil, fmt.Errorf("closing mock file %q: %w", path, err)
		}
	}
	return fs, nil
}

// GitCloner is the subset of Minder's interfaces.GitProvider needed for
// offline testing.  In the main codebase this maps to:
//
//	pkg/engine/v1/interfaces.GitProvider.Clone(ctx, url, branch) (*git.Repository, error)
//
// The prototype simplifies the return type to billy.Filesystem so it can be
// tested without importing go-git.  During Minder integration, replace this
// interface with the real interfaces.GitProvider and update MockGitCloner.Clone
// to initialise a *git.Repository from memory storage backed by the memfs.
type GitCloner interface {
	// Clone returns a filesystem representing the repository at cloneURL on
	// the given branch.  Mock implementations ignore both arguments and return
	// the pre-populated in-memory filesystem.
	Clone(ctx context.Context, cloneURL, branch string) (billy.Filesystem, error)
}

// MockGitCloner implements GitCloner using a pre-populated in-memory
// filesystem.  It is constructed by BuildMocks; do not initialise directly.
type MockGitCloner struct {
	fs billy.Filesystem
}

// Clone ignores cloneURL and branch and returns the pre-populated filesystem,
// enabling fully offline rule evaluation for git_files-based test cases.
func (m *MockGitCloner) Clone(_ context.Context, _, _ string) (billy.Filesystem, error) {
	return m.fs, nil
}

// ---------------------------------------------------------------------------
// Unified mock set
// ---------------------------------------------------------------------------

// TestCaseMocks holds all offline mocks wired for a single test case.
// After calling BuildMocks, inject the fields into the rule engine components:
//   - HTTPClient.Transport     → restHandler.testOnlyTransport (REST providers)
//   - DataSourceClient.Transport → restHandler.testOnlyTransport (data sources)
//   - GitCloner                → git ingester in place of the live GitProvider
//   - GitFilesystem            → direct billy.Filesystem access if needed
type TestCaseMocks struct {
	// HTTPClient is pre-configured with MockRoundTripper for http_responses.
	HTTPClient *http.Client

	// DataSourceClient is pre-configured with MockRoundTripper for
	// data_source_responses.  Kept separate from HTTPClient so REST provider
	// mocks and declared data source mocks do not interfere.
	DataSourceClient *http.Client

	// GitCloner returns the pre-populated in-memory filesystem on Clone().
	// Assign it to the git ingester in place of the live GitProvider.
	GitCloner GitCloner

	// GitFilesystem is the underlying billy.Filesystem for direct access.
	GitFilesystem billy.Filesystem
}

// BuildMocks constructs all offline mocks for the given test case and returns
// them ready for injection into the rule engine.
func BuildMocks(tc TestCase) (*TestCaseMocks, error) {
	fs, err := NewMockBillyFS(tc.MockData.GitFiles)
	if err != nil {
		return nil, fmt.Errorf("building git filesystem for %q: %w", tc.Name, err)
	}
	return &TestCaseMocks{
		HTTPClient:       &http.Client{Transport: NewMockRoundTripper(tc.MockData.HTTPResponses)},
		DataSourceClient: &http.Client{Transport: NewMockRoundTripper(tc.MockData.DataSourceResponses)},
		GitCloner:        &MockGitCloner{fs: fs},
		GitFilesystem:    fs,
	}, nil
}
