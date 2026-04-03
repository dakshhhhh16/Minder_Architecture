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

// MockRoundTripper serves pre-defined responses for outbound HTTP requests.
// It implements http.RoundTripper and is meant to be injected into
// restHandler.testOnlyTransport — one instance for REST provider calls
// (http_responses) and a separate one for Data Source calls
// (data_source_responses).
type MockRoundTripper struct {
	ExpectedResponses map[string]HTTPResponseMock
}

// NewMockRoundTripper creates a MockRoundTripper loaded with the given
// responses. Passing nil is safe — every URL will get a 404.
func NewMockRoundTripper(responses map[string]HTTPResponseMock) *MockRoundTripper {
	if responses == nil {
		responses = make(map[string]HTTPResponseMock)
	}
	return &MockRoundTripper{ExpectedResponses: responses}
}

// RoundTrip serves the canned response matching the request URL, or returns
// a 404 with the unmatched URL in the body (useful for debugging fixtures).
func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	urlStr := req.URL.String()

	hdr := make(http.Header)
	hdr.Set("Content-Type", "application/json")

	if mockResp, ok := m.ExpectedResponses[urlStr]; ok {
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
// given files. Compatible with the go-billy interface used by Minder's Git
// provider, so rules can be evaluated without cloning a real repo.
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

// GitCloner is the interface we need from Minder's GitProvider for offline
// testing. In the real codebase this maps to:
//
//	pkg/engine/v1/interfaces.GitProvider.Clone(ctx, url, branch) (*git.Repository, error)
//
// The prototype simplifies the return to billy.Filesystem. During integration,
// replace this with interfaces.GitProvider and have MockGitCloner return a
// *git.Repository backed by memory storage.
type GitCloner interface {
	Clone(ctx context.Context, cloneURL, branch string) (billy.Filesystem, error)
}

// MockGitCloner always returns a pre-populated in-memory filesystem,
// ignoring the URL and branch arguments.
type MockGitCloner struct {
	fs billy.Filesystem
}

// Clone returns the pre-populated filesystem regardless of arguments.
func (m *MockGitCloner) Clone(_ context.Context, _, _ string) (billy.Filesystem, error) {
	return m.fs, nil
}

// ---------------------------------------------------------------------------
// Wiring layer
// ---------------------------------------------------------------------------

// TestCaseMocks holds all offline mocks for a single test case. After calling
// BuildMocks, inject these into the rule engine:
//   - HTTPClient.Transport     -> restHandler.testOnlyTransport (REST providers)
//   - DataSourceClient.Transport -> restHandler.testOnlyTransport (Data Sources)
//   - GitCloner                -> git ingester replacing the live GitProvider
//   - GitFilesystem            -> direct filesystem access if needed
type TestCaseMocks struct {
	HTTPClient       *http.Client
	DataSourceClient *http.Client
	GitCloner        GitCloner
	GitFilesystem    billy.Filesystem
}

// BuildMocks constructs all offline mocks for a test case.
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
