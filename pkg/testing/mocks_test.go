// SPDX-FileCopyrightText: Copyright 2026 The Minder Authors
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"io"
	"net/http"
	"net/url"
	"testing"
)

// ---------- MockRoundTripper ----------

func TestMockRoundTripper_HitURL(t *testing.T) {
	t.Parallel()
	rt := NewMockRoundTripper(map[string]HTTPResponseMock{
		"https://api.github.com/repos/o/r": {StatusCode: 200, Body: `{"ok":true}`},
	})

	req := &http.Request{URL: mustParseURL(t, "https://api.github.com/repos/o/r")}
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"ok":true}` {
		t.Errorf("body = %q, want %q", string(body), `{"ok":true}`)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want \"application/json\"", ct)
	}
}

func TestMockRoundTripper_MissURL_Returns404(t *testing.T) {
	t.Parallel()
	rt := NewMockRoundTripper(nil) // empty map

	req := &http.Request{URL: mustParseURL(t, "https://api.github.com/repos/missing")}
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want \"application/json\"", ct)
	}
}

func TestMockRoundTripper_NilResponses_Safe(t *testing.T) {
	t.Parallel()
	// Constructing with nil must not panic on lookup.
	rt := NewMockRoundTripper(nil)
	if rt.ExpectedResponses == nil {
		t.Error("ExpectedResponses should be initialised to non-nil map")
	}
}

func TestMockRoundTripper_MultipleURLs(t *testing.T) {
	t.Parallel()
	rt := NewMockRoundTripper(map[string]HTTPResponseMock{
		"https://api.github.com/repos/o/r":                      {StatusCode: 200, Body: `{}`},
		"https://api.github.com/repos/o/r/vulnerability-alerts": {StatusCode: 404, Body: `{"message":"Not Found"}`},
	})

	for _, tc := range []struct {
		url        string
		wantStatus int
	}{
		{"https://api.github.com/repos/o/r", 200},
		{"https://api.github.com/repos/o/r/vulnerability-alerts", 404},
		{"https://api.github.com/repos/o/r/unregistered", http.StatusNotFound},
	} {
		req := &http.Request{URL: mustParseURL(t, tc.url)}
		resp, err := rt.RoundTrip(req)
		if err != nil {
			t.Fatalf("url %s: unexpected error: %v", tc.url, err)
		}
		if resp.StatusCode != tc.wantStatus {
			t.Errorf("url %s: status = %d, want %d", tc.url, resp.StatusCode, tc.wantStatus)
		}
	}
}

// ---------- NewMockBillyFS ----------

func TestNewMockBillyFS_FileExists(t *testing.T) {
	t.Parallel()
	fs, err := NewMockBillyFS(map[string]string{
		"SECURITY.md": "report vulns here",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f, err := fs.Open("SECURITY.md")
	if err != nil {
		t.Fatalf("opening SECURITY.md: %v", err)
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("reading SECURITY.md: %v", err)
	}
	if string(content) != "report vulns here" {
		t.Errorf("content = %q, want %q", string(content), "report vulns here")
	}
}

func TestNewMockBillyFS_EmptyMap(t *testing.T) {
	t.Parallel()
	fs, err := NewMockBillyFS(nil)
	if err != nil {
		t.Fatalf("unexpected error for nil map: %v", err)
	}
	if fs == nil {
		t.Error("expected non-nil filesystem")
	}
}

func TestNewMockBillyFS_MultipleFiles(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"SECURITY.md": "security policy",
		"README.md":   "readme content",
	}
	fs, err := NewMockBillyFS(files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for name, wantContent := range files {
		f, err := fs.Open(name)
		if err != nil {
			t.Fatalf("opening %s: %v", name, err)
		}
		got, _ := io.ReadAll(f)
		f.Close()
		if string(got) != wantContent {
			t.Errorf("%s: content = %q, want %q", name, string(got), wantContent)
		}
	}
}

func TestNewMockBillyFS_MissingFile_ReturnsError(t *testing.T) {
	t.Parallel()
	fs, err := NewMockBillyFS(map[string]string{"README.md": "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = fs.Open("SECURITY.md")
	if err == nil {
		t.Error("expected error opening non-existent file, got nil")
	}
}

// ---------- BuildMocks ----------

func TestBuildMocks_WiresHTTPAndGit(t *testing.T) {
	t.Parallel()
	tc := TestCase{
		Name:   "wiring test",
		Expect: "pass",
		MockData: ProviderMockConfig{
			HTTPResponses: map[string]HTTPResponseMock{
				"https://api.github.com/repos/o/r": {StatusCode: 200, Body: `{}`},
			},
			GitFiles: map[string]string{
				"SECURITY.md": "report here",
			},
		},
	}

	mocks, err := BuildMocks(tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mocks.HTTPClient == nil {
		t.Error("HTTPClient must not be nil")
	}
	if mocks.GitFilesystem == nil {
		t.Error("GitFilesystem must not be nil")
	}

	// Verify the HTTP client is correctly wired.
	req, _ := http.NewRequest(http.MethodGet, "https://api.github.com/repos/o/r", nil)
	resp, err := mocks.HTTPClient.Do(req)
	if err != nil {
		t.Fatalf("http client do: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("http status = %d, want 200", resp.StatusCode)
	}

	// Verify the git filesystem is correctly wired.
	f, err := mocks.GitFilesystem.Open("SECURITY.md")
	if err != nil {
		t.Fatalf("opening SECURITY.md from git fs: %v", err)
	}
	defer f.Close()
	content, _ := io.ReadAll(f)
	if string(content) != "report here" {
		t.Errorf("git file content = %q, want %q", string(content), "report here")
	}
}

func TestBuildMocks_EmptyMockData(t *testing.T) {
	t.Parallel()
	tc := TestCase{Name: "empty", Expect: "fail", MockData: ProviderMockConfig{}}
	mocks, err := BuildMocks(tc)
	if err != nil {
		t.Fatalf("unexpected error for empty mock data: %v", err)
	}
	if mocks.HTTPClient == nil || mocks.GitFilesystem == nil {
		t.Error("both fields must be non-nil even when mock data is empty")
	}
}

// ---------- MockGitCloner ----------

func TestMockGitCloner_Clone_ReturnsPrepopulatedFS(t *testing.T) {
	t.Parallel()
	fs, err := NewMockBillyFS(map[string]string{"SECURITY.md": "report here"})
	if err != nil {
		t.Fatalf("building memfs: %v", err)
	}
	cloner := &MockGitCloner{fs: fs}

	got, err := cloner.Clone(t.Context(), "https://github.com/owner/repo", "main")
	if err != nil {
		t.Fatalf("Clone returned error: %v", err)
	}
	if got == nil {
		t.Fatal("Clone returned nil filesystem")
	}

	f, err := got.Open("SECURITY.md")
	if err != nil {
		t.Fatalf("opening SECURITY.md from cloned fs: %v", err)
	}
	defer f.Close()
	content, _ := io.ReadAll(f)
	if string(content) != "report here" {
		t.Errorf("content = %q, want %q", string(content), "report here")
	}
}

func TestMockGitCloner_Clone_IgnoresURLAndBranch(t *testing.T) {
	t.Parallel()
	fs, _ := NewMockBillyFS(map[string]string{"README.md": "hello"})
	cloner := &MockGitCloner{fs: fs}

	// Different URLs and branches must all return the same pre-populated FS.
	for _, args := range []struct{ url, branch string }{
		{"https://github.com/a/b", "main"},
		{"https://github.com/c/d", "develop"},
		{"", ""},
	} {
		got, err := cloner.Clone(t.Context(), args.url, args.branch)
		if err != nil {
			t.Fatalf("Clone(%q, %q): %v", args.url, args.branch, err)
		}
		if got != fs {
			t.Errorf("Clone(%q, %q): returned different filesystem", args.url, args.branch)
		}
	}
}

// ---------- BuildMocks — DataSourceClient ----------

func TestBuildMocks_DataSourceClientIsIndependent(t *testing.T) {
	t.Parallel()
	tc := TestCase{
		Name:   "ds independence",
		Expect: "pass",
		MockData: ProviderMockConfig{
			HTTPResponses: map[string]HTTPResponseMock{
				"https://api.github.com/repos/o/r": {StatusCode: 200, Body: `{"http":true}`},
			},
			DataSourceResponses: map[string]HTTPResponseMock{
				"https://ds.example.com/data": {StatusCode: 200, Body: `{"ds":true}`},
			},
		},
	}

	mocks, err := BuildMocks(tc)
	if err != nil {
		t.Fatalf("BuildMocks: %v", err)
	}

	// HTTPClient must serve http_responses but 404 for data source URLs.
	req, _ := http.NewRequest(http.MethodGet, "https://ds.example.com/data", nil)
	resp, _ := mocks.HTTPClient.Do(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("HTTPClient served DS URL: status = %d, want 404", resp.StatusCode)
	}

	// DataSourceClient must serve data_source_responses but 404 for HTTP provider URLs.
	req2, _ := http.NewRequest(http.MethodGet, "https://api.github.com/repos/o/r", nil)
	resp2, _ := mocks.DataSourceClient.Do(req2)
	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("DataSourceClient served HTTP provider URL: status = %d, want 404", resp2.StatusCode)
	}

	// DataSourceClient must serve its own URL correctly.
	req3, _ := http.NewRequest(http.MethodGet, "https://ds.example.com/data", nil)
	resp3, _ := mocks.DataSourceClient.Do(req3)
	if resp3.StatusCode != 200 {
		t.Errorf("DataSourceClient: status = %d, want 200", resp3.StatusCode)
	}
}

// ---------- helpers ----------

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parsing URL %q: %v", raw, err)
	}
	return u
}
