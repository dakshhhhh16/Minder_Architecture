package testing

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
)

// Result captures what happened when we ran (or tried to run) a test case.
type Result struct {
	Name       string
	Skipped    bool
	SkipReason string
	Err        error
}

// DryRun validates a fixture without actually running the rule engine.
// It parses the YAML, builds mocks for every non-skipped case, and checks
// that all mock data is well formed. This is the fast check that CI runs
// on every pull request before the full evaluation suite.
//
// It catches things like:
//   - Bad YAML structure
//   - Relative URLs in http_responses (must be absolute)
//   - Malformed data source keys (must be "name.func")
//   - Git files that failed to write to the in-memory filesystem
func DryRun(fixturePath string) ([]Result, error) {
	f, err := Parse(fixturePath)
	if err != nil {
		return nil, fmt.Errorf("parsing fixture: %w", err)
	}

	results := make([]Result, 0, len(f.TestCases))

	for _, tc := range f.TestCases {
		// Skipped cases are just recorded, no validation needed.
		if tc.SkipReason != "" {
			results = append(results, Result{
				Name:       tc.Name,
				Skipped:    true,
				SkipReason: tc.SkipReason,
			})
			continue
		}

		// Try building all the mocks. If anything goes wrong with the
		// mock data (bad JSON in data source responses, etc.) we catch
		// it here.
		mocks, err := BuildMocks(tc)
		if err != nil {
			results = append(results, Result{Name: tc.Name, Err: err})
			continue
		}

		// Make sure every git file in the fixture actually made it into
		// the in-memory filesystem.
		if err := verifyGitFiles(mocks.GitFilesystem, tc.MockData.GitFiles); err != nil {
			results = append(results, Result{Name: tc.Name, Err: err})
			continue
		}

		// Check that all HTTP URLs are absolute. A relative URL would
		// never match a real API call from the rule engine.
		if err := verifyURLs(tc.MockData.HTTPResponses); err != nil {
			results = append(results, Result{Name: tc.Name, Err: err})
			continue
		}

		// Everything looks good for this case.
		results = append(results, Result{Name: tc.Name})
	}

	return results, nil
}

// Evaluate is the full rule evaluation pipeline. In the Minder codebase
// this will:
//
//  1. Resolve the rule type YAML by walking rule-types/ to find <rule_name>.yaml
//  2. Parse it into *minderv1.RuleType via minderv1.ParseResource()
//  3. For each non-skipped test case:
//     a. Build TestKit with HTTP mock -> tkv1.WithHTTP()
//     b. Build DataSourceRegistry from data_source_responses
//     c. Create engine -> rtengine.NewRuleTypeEngine(ctx, rt, tk, options.WithDataSources(dsReg))
//     d. For git rules, override ingester with memfs -> rte.WithCustomIngester(tk)
//     e. Validate rule definition -> rte.GetRuleInstanceValidator().ValidateRuleDefAgainstSchema(tc.Def)
//     f. Evaluate -> rte.Eval(ctx, entity, tc.Def, tc.Params, NewVoidResultSink())
//     g. Check: "pass" means err == nil, "fail" means err != nil,
//     "error" means err != nil and err.Error() contains tc.ErrorContains
//
// In this prototype, we demonstrate step 1 (resolving the rule type path
// on the filesystem) and then fall through to DryRun for the rest.
func Evaluate(fixturePath string) ([]Result, error) {
	f, err := Parse(fixturePath)
	if err != nil {
		return nil, fmt.Errorf("parsing fixture: %w", err)
	}

	// Step 1: Try to find the rule type YAML on disk.
	// This is pure filesystem work, no Minder imports needed.
	ruleTypePath, err := ResolveRuleTypePath(filepath.Dir(fixturePath), f.RuleName)
	if err != nil {
		// In the prototype, a missing rule type file is not an error.
		// The DryRun still validates everything else.
		_ = ruleTypePath
	}

	// Steps 2 through 7 need Minder internals (rtengine, minderv1, etc).
	// For now we fall through to DryRun.
	return DryRun(fixturePath)
}

// verifyGitFiles opens each file in the mock filesystem to make sure it
// was written correctly during NewMockBillyFS.
func verifyGitFiles(fs billy.Filesystem, files map[string]string) error {
	for path := range files {
		f, err := fs.Open(path)
		if err != nil {
			return fmt.Errorf("git_files: cannot open %q in mock filesystem: %w", path, err)
		}
		_ = f.Close()
	}
	return nil
}

// verifyURLs checks that every URL in the HTTP responses map is absolute
// (has a scheme and host). Relative URLs would never match an actual API
// call from the rule engine.
func verifyURLs(responses map[string]HTTPResponseMock) error {
	for rawURL := range responses {
		u, err := url.Parse(rawURL)
		if err != nil {
			return fmt.Errorf("invalid URL %q: %w", rawURL, err)
		}
		if u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("invalid URL %q: must be absolute with scheme and host", rawURL)
		}
	}
	return nil
}

// ResolveRuleTypePath searches for rule-types/<provider>/<ruleName>.yaml
// starting from baseDir and walking up to 3 parent directories. This is
// the first step of the Evaluate pipeline and does not need any Minder
// imports.
//
// In the full Minder integration, the found file gets parsed by
// minderv1.ParseResource() into a *minderv1.RuleType.
func ResolveRuleTypePath(baseDir, ruleName string) (string, error) {
	dir := baseDir
	for range 3 {
		ruleTypesDir := filepath.Join(dir, "rule-types")
		if info, err := os.Stat(ruleTypesDir); err == nil && info.IsDir() {
			var found string
			_ = filepath.WalkDir(ruleTypesDir, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				base := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
				if base == ruleName {
					found = path
					return filepath.SkipAll
				}
				return nil
			})
			if found != "" {
				return found, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("rule type %q not found under rule-types/", ruleName)
}
