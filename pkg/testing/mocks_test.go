package testing

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"
)

// ---- MockRoundTripper tests ----

func mustParseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parsing URL %q: %v", rawURL, err)
	}
	return u
}

func TestMockRoundTripper_MatchingURL(t *testing.T) {
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

func TestMockRoundTripper_NonMatchingURL_Returns404(t *testing.T) {
	t.Parallel()
	rt := NewMockRoundTripper(nil)

	req := &http.Request{URL: mustParseURL(t, "https://api.github.com/repos/missing")}
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestMockRoundTripper_NilResponses_InitializesMap(t *testing.T) {
	t.Parallel()
	rt := NewMockRoundTripper(nil)
	if rt.Responses == nil {
		t.Error("Responses should be initialised to non-nil map")
	}
}

func TestMockRoundTripper_MultipleURLs(t *testing.T) {
	t.Parallel()
	rt := NewMockRoundTripper(map[string]HTTPResponseMock{
		"https://api.github.com/repos/o/r":                      {StatusCode: 200, Body: `{}`},
		"https://api.github.com/repos/o/r/vulnerability-alerts": {StatusCode: 404, Body: `{"message":"Not Found"}`},
	})

	cases := []struct {
		url        string
		wantStatus int
	}{
		{"https://api.github.com/repos/o/r", 200},
		{"https://api.github.com/repos/o/r/vulnerability-alerts", 404},
		{"https://api.github.com/repos/o/r/unregistered", http.StatusNotFound},
	}

	for _, tc := range cases {
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

// ---- Git filesystem mock tests ----

func TestNewMockBillyFS_SingleFile(t *testing.T) {
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
		"LICENSE":     "MIT License",
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

func TestNewMockBillyFS_NonexistentFile_ReturnsError(t *testing.T) {
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

// ---- Data source mock tests ----

func TestBuildDataSourceMocks_SingleSource(t *testing.T) {
	t.Parallel()
	mocks, err := BuildDataSourceMocks(map[string]DataSourceResponseMock{
		"osv.query": {Body: `{"vulns": [{"id": "GHSA-0001"}]}`},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mocks) != 1 {
		t.Fatalf("len(mocks) = %d, want 1", len(mocks))
	}

	osv, ok := mocks["osv"]
	if !ok {
		t.Fatal("expected mocks[\"osv\"] to exist")
	}
	queryFunc, ok := osv.Funcs["query"]
	if !ok {
		t.Fatal("expected osv.Funcs[\"query\"] to exist")
	}

	result, err := queryFunc.Call(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("Call: unexpected error: %v", err)
	}
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	if _, hasVulns := resultMap["vulns"]; !hasVulns {
		t.Error("expected 'vulns' key in result")
	}
}

func TestBuildDataSourceMocks_MultipleFunctions(t *testing.T) {
	t.Parallel()
	mocks, err := BuildDataSourceMocks(map[string]DataSourceResponseMock{
		"osv.query":       {Body: `{"vulns": []}`},
		"osv.get_by_id":   {Body: `{"id": "GHSA-1234"}`},
		"sonatype.lookup": {Body: `{"components": []}`},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mocks) != 2 {
		t.Fatalf("len(mocks) = %d, want 2 (osv, sonatype)", len(mocks))
	}
	if len(mocks["osv"].Funcs) != 2 {
		t.Errorf("osv funcs = %d, want 2", len(mocks["osv"].Funcs))
	}
	if len(mocks["sonatype"].Funcs) != 1 {
		t.Errorf("sonatype funcs = %d, want 1", len(mocks["sonatype"].Funcs))
	}
}

func TestBuildDataSourceMocks_BadJSON_ReturnsError(t *testing.T) {
	t.Parallel()
	_, err := BuildDataSourceMocks(map[string]DataSourceResponseMock{
		"osv.query": {Body: `{not valid json`},
	})
	if err == nil {
		t.Error("expected error for bad JSON, got nil")
	}
}

func TestBuildDataSourceMocks_EmptyMap(t *testing.T) {
	t.Parallel()
	mocks, err := BuildDataSourceMocks(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mocks) != 0 {
		t.Errorf("expected empty map, got %d entries", len(mocks))
	}
}

// ---- MockGitCloner tests ----

func TestMockGitCloner_ReturnsPrepopulatedFS(t *testing.T) {
	t.Parallel()
	fs, err := NewMockBillyFS(map[string]string{"SECURITY.md": "report here"})
	if err != nil {
		t.Fatalf("building memfs: %v", err)
	}
	cloner := &MockGitCloner{fs: fs}

	got, err := cloner.Clone(context.Background(), "https://github.com/owner/repo", "main")
	if err != nil {
		t.Fatalf("Clone returned error: %v", err)
	}
	if got == nil {
		t.Fatal("Clone returned nil filesystem")
	}

	f, err := got.Open("SECURITY.md")
	if err != nil {
		t.Fatalf("opening SECURITY.md: %v", err)
	}
	defer f.Close()
	content, _ := io.ReadAll(f)
	if string(content) != "report here" {
		t.Errorf("content = %q, want %q", string(content), "report here")
	}
}

func TestMockGitCloner_IgnoresURLAndBranch(t *testing.T) {
	t.Parallel()
	fs, _ := NewMockBillyFS(map[string]string{"file.txt": "data"})
	cloner := &MockGitCloner{fs: fs}

	// Different URLs and branches should all return the same filesystem.
	for _, url := range []string{
		"https://github.com/a/b",
		"https://github.com/x/y",
	} {
		got, err := cloner.Clone(context.Background(), url, "develop")
		if err != nil {
			t.Fatalf("Clone(%s): %v", url, err)
		}
		if got != fs {
			t.Errorf("Clone(%s) returned different filesystem", url)
		}
	}
}

// ---- BuildMocks integration ----

func TestBuildMocks_WiresAllProviders(t *testing.T) {
	t.Parallel()
	tc := TestCase{
		Name:   "full wiring test",
		Expect: "pass",
		Entity: EntityConfig{Type: "repository", Entity: map[string]any{"owner": "o", "name": "r"}},
		MockData: ProviderMockConfig{
			HTTPResponses: map[string]HTTPResponseMock{
				"https://api.github.com/repos/o/r": {StatusCode: 200, Body: `{}`},
			},
			GitFiles: map[string]string{
				"SECURITY.md": "report here",
			},
			DataSourceResponses: map[string]DataSourceResponseMock{
				"osv.query": {Body: `{"vulns": []}`},
			},
		},
	}

	mocks, err := BuildMocks(tc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that the HTTP client works.
	req, _ := http.NewRequest(http.MethodGet, "https://api.github.com/repos/o/r", nil)
	resp, err := mocks.HTTPClient.Do(req)
	if err != nil {
		t.Fatalf("http client: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("http status = %d, want 200", resp.StatusCode)
	}

	// Check that the git filesystem has the file.
	f, err := mocks.GitFilesystem.Open("SECURITY.md")
	if err != nil {
		t.Fatalf("opening SECURITY.md: %v", err)
	}
	defer f.Close()
	content, _ := io.ReadAll(f)
	if string(content) != "report here" {
		t.Errorf("git file content = %q, want %q", string(content), "report here")
	}

	// Check that data source mocks were built.
	if len(mocks.DataSources) != 1 {
		t.Errorf("DataSources length = %d, want 1", len(mocks.DataSources))
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
		t.Error("HTTPClient and GitFilesystem must be non-nil even with empty mock data")
	}
	if len(mocks.DataSources) != 0 {
		t.Errorf("DataSources should be empty, got %d entries", len(mocks.DataSources))
	}
}
