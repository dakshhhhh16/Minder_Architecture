package testing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
)

// ---------------------------------------------------------------------------
// HTTP mocking
//
// REST based rules in Minder (like branch_protection, secret_scanning) make
// HTTP calls to the GitHub API at evaluation time. The existing TestKit
// intercepts these with a MockRoundTripper that replaces the real HTTP
// transport. Our fixture data gets loaded into this mock so the rule sees
// canned responses instead of hitting the network.
//
// In the Minder codebase this plugs in through:
//   tkv1.WithHTTP(mockRoundTripper)
// ---------------------------------------------------------------------------

// MockRoundTripper implements http.RoundTripper. It holds a map of URL to
// canned response and returns them when the rule engine makes HTTP calls.
// Any URL that is not in the map gets a 404.
type MockRoundTripper struct {
	Responses map[string]HTTPResponseMock
}

// NewMockRoundTripper builds a transport loaded with fixture responses.
// Passing nil is safe, every URL will just get a 404.
func NewMockRoundTripper(responses map[string]HTTPResponseMock) *MockRoundTripper {
	if responses == nil {
		responses = make(map[string]HTTPResponseMock)
	}
	return &MockRoundTripper{Responses: responses}
}

// RoundTrip looks up the request URL in our map and returns the canned
// response. If the URL is not registered, it returns a 404 with a helpful
// error message telling you which URL was requested so you can add it to
// your fixture.
func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	url := req.URL.String()

	header := make(http.Header)
	header.Set("Content-Type", "application/json")

	if mock, ok := m.Responses[url]; ok {
		return &http.Response{
			StatusCode: mock.StatusCode,
			Body:       io.NopCloser(bytes.NewBufferString(mock.Body)),
			Header:     header,
			Request:    req,
		}, nil
	}

	body := fmt.Sprintf(`{"error":"no mock data for this URL","url":%q}`, url)
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     header,
		Request:    req,
	}, nil
}

// ---------------------------------------------------------------------------
// Git filesystem mocking
//
// File based rules (like checking for SECURITY.md, LICENSE, etc.) use
// Minder's Git ingester which clones a repo and reads files from it. The
// existing TestKit handles this with osfs.New() in fakeGit(). We replace
// that with an in-memory billy.Filesystem populated from the fixture's
// git_files map, so no actual clone happens.
//
// In the Minder codebase this plugs in through:
//   tkv1.WithGitDir(memfsPath) or rte.WithCustomIngester(tk)
// ---------------------------------------------------------------------------

// NewMockBillyFS creates an in-memory filesystem with the files from
// the fixture. Each key is a file path and the value is the file content.
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

// GitCloner is the interface that Minder's Git provider uses for cloning.
// In production it calls git clone over the network. Our mock just returns
// the pre-built in-memory filesystem.
type GitCloner interface {
	Clone(ctx context.Context, cloneURL, branch string) (billy.Filesystem, error)
}

// MockGitCloner returns the same pre-populated filesystem every time,
// regardless of what URL or branch you ask for.
type MockGitCloner struct {
	fs billy.Filesystem
}

// Clone returns the in-memory filesystem. The URL and branch are ignored.
func (m *MockGitCloner) Clone(_ context.Context, _, _ string) (billy.Filesystem, error) {
	return m.fs, nil
}

// ---------------------------------------------------------------------------
// Data Source mocking (the core of this project)
//
// This is where things get interesting. Data Source rules (like OSV
// vulnerability checks, Sonatype scans, OpenSSF Bestpractices lookups)
// do NOT make HTTP calls at ingestion time. Instead, they register Rego
// built-in functions that fire inside the OPA evaluator during policy
// evaluation.
//
// The real call path looks like this:
//
//   Rego policy
//     -> calls minder.datasource.osv.query(args)
//     -> which is a rego.Function1 registered by buildFromDataSource()
//     -> which calls DataSourceFuncDef.Call()
//     -> which makes the actual REST call to the OSV API
//
// We intercept at DataSourceFuncDef.Call(). Our mock implements that
// interface and returns canned fixture data. The engine has no idea it
// is talking to a fake, and zero network calls happen.
//
// In the Minder codebase, the real interface lives at
// pkg/datasources/v1/datasources.go:
//
//   type DataSourceFuncDef interface {
//       ValidateArgs(obj any) error
//       ValidateUpdate(obj *structpb.Struct) error
//       Call(ctx context.Context, ingest *interfaces.Ingested, args any) (any, error)
//       GetArgsSchema() *structpb.Struct
//   }
//
// Our mock wires in through:
//
//   dsReg := v1datasources.NewDataSourceRegistry()
//   dsReg.RegisterDataSource("osv", mockOSV)
//   rte, _ := rtengine.NewRuleTypeEngine(ctx, rt, tk,
//       options.WithDataSources(dsReg))
// ---------------------------------------------------------------------------

// MockDataSourceFuncDef implements the DataSourceFuncDef interface from
// Minder's data source layer. It just returns whatever JSON was in the
// fixture, parsed into a Go value.
type MockDataSourceFuncDef struct {
	Response any // the parsed JSON from fixture body
}

// Call returns the fixture response. In the real interface this takes
// (ctx, *interfaces.Ingested, args), but since we cannot import Minder
// internals in the prototype, we simplify to (ctx, args).
func (m *MockDataSourceFuncDef) Call(_ context.Context, _ any) (any, error) {
	return m.Response, nil
}

// ValidateArgs always passes for test fixtures.
func (m *MockDataSourceFuncDef) ValidateArgs(_ any) error {
	return nil
}

// MockDataSource groups mock functions under one data source name.
// For example, the "osv" data source has a "query" function.
type MockDataSource struct {
	Name  string
	Funcs map[string]*MockDataSourceFuncDef
}

// GetFuncs returns the mock functions keyed by function name.
func (m *MockDataSource) GetFuncs() map[string]*MockDataSourceFuncDef {
	return m.Funcs
}

// BuildDataSourceMocks takes the fixture's data_source_responses map and
// turns it into MockDataSource objects grouped by data source name.
//
// For example, a fixture with:
//   data_source_responses:
//     "osv.query":
//       body: '{"vulns": []}'
//
// Produces a MockDataSource named "osv" with one function "query" that
// returns {"vulns": []}.
//
// During Minder integration, these get registered in a DataSourceRegistry:
//
//   reg := v1datasources.NewDataSourceRegistry()
//   for name, mockDS := range dsMocks {
//       reg.RegisterDataSource(name, mockDS)
//   }
//   rte, _ := rtengine.NewRuleTypeEngine(ctx, rt, tk, options.WithDataSources(reg))
func BuildDataSourceMocks(responses map[string]DataSourceResponseMock) (map[string]*MockDataSource, error) {
	mocks := make(map[string]*MockDataSource)

	for key, mock := range responses {
		dsName, funcName, err := splitDSKey(key)
		if err != nil {
			return nil, err
		}

		var parsed any
		if err := json.Unmarshal([]byte(mock.Body), &parsed); err != nil {
			return nil, fmt.Errorf("parsing data source response for %q: %w", key, err)
		}

		ds, ok := mocks[dsName]
		if !ok {
			ds = &MockDataSource{
				Name:  dsName,
				Funcs: make(map[string]*MockDataSourceFuncDef),
			}
			mocks[dsName] = ds
		}
		ds.Funcs[funcName] = &MockDataSourceFuncDef{Response: parsed}
	}

	return mocks, nil
}

// splitDSKey splits "name.func" into its two parts. The dot is required
// and both sides must be non-empty.
func splitDSKey(key string) (string, string, error) {
	for i, c := range key {
		if c == '.' && i > 0 && i < len(key)-1 {
			return key[:i], key[i+1:], nil
		}
	}
	return "", "", fmt.Errorf("invalid data source key %q: must be \"name.func\"", key)
}

// ---------------------------------------------------------------------------
// Wiring everything together
// ---------------------------------------------------------------------------

// TestCaseMocks is the bundle of all offline mocks for one test case.
// When integrating with Minder, these get plugged into:
//   - HTTPClient    -> TestKit's REST provider (replaces live HTTP calls)
//   - GitCloner     -> TestKit's Git ingester (replaces real git clone)
//   - GitFilesystem -> In-memory filesystem for file based rules
//   - DataSources   -> DataSourceRegistry injected via options.WithDataSources()
type TestCaseMocks struct {
	HTTPClient    *http.Client
	GitCloner     GitCloner
	GitFilesystem billy.Filesystem
	DataSources   map[string]*MockDataSource
}

// BuildMocks constructs all the offline mocks for a single test case.
// It reads the fixture's mock_data section and creates the appropriate
// mock for each type of provider.
func BuildMocks(tc TestCase) (*TestCaseMocks, error) {
	fs, err := NewMockBillyFS(tc.MockData.GitFiles)
	if err != nil {
		return nil, fmt.Errorf("building git filesystem for %q: %w", tc.Name, err)
	}

	dsMocks, err := BuildDataSourceMocks(tc.MockData.DataSourceResponses)
	if err != nil {
		return nil, fmt.Errorf("building data source mocks for %q: %w", tc.Name, err)
	}

	return &TestCaseMocks{
		HTTPClient:    &http.Client{Transport: NewMockRoundTripper(tc.MockData.HTTPResponses)},
		GitCloner:     &MockGitCloner{fs: fs},
		GitFilesystem: fs,
		DataSources:   dsMocks,
	}, nil
}
