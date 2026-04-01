// SPDX-FileCopyrightText: Copyright 2026 The Minder Authors
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"fmt"
	"net/url"

	"github.com/go-git/go-billy/v5"
)

// Result captures the outcome of one test case during a DryRun.
type Result struct {
	// Name is the test case name from the fixture.
	Name string

	// Skipped is true when the case has a non-empty skip_reason.
	Skipped bool

	// SkipReason is the verbatim skip_reason value from the fixture.
	SkipReason string

	// Err is non-nil when mock construction or verification failed.
	// A nil Err with Skipped == false means the case is ready for engine injection.
	Err error
}

// DryRun parses the fixture at fixturePath, builds offline mocks for every
// non-skipped test case, and verifies that:
//   - all declared git files are readable in the constructed filesystem, and
//   - all declared HTTP and data-source URLs are syntactically valid.
//
// It does NOT invoke the Minder rule engine.  That integration happens in
// cmd/dev/app/rule_type/rttst.go once the --fixture flag is added.  DryRun
// is suitable as a standalone CI step to catch malformed fixtures before
// mindev integration is complete.
//
// Returns one Result per test case, or an error if the fixture itself cannot
// be parsed.
func DryRun(fixturePath string) ([]Result, error) {
	f, err := Parse(fixturePath)
	if err != nil {
		return nil, fmt.Errorf("parsing fixture: %w", err)
	}

	results := make([]Result, 0, len(f.TestCases))
	for _, tc := range f.TestCases {
		if tc.SkipReason != "" {
			results = append(results, Result{
				Name:       tc.Name,
				Skipped:    true,
				SkipReason: tc.SkipReason,
			})
			continue
		}

		mocks, err := BuildMocks(tc)
		if err != nil {
			results = append(results, Result{Name: tc.Name, Err: err})
			continue
		}

		if err := verifyGitFiles(mocks.GitFilesystem, tc.MockData.GitFiles); err != nil {
			results = append(results, Result{Name: tc.Name, Err: err})
			continue
		}

		if err := verifyURLs(tc.MockData.HTTPResponses, tc.MockData.DataSourceResponses); err != nil {
			results = append(results, Result{Name: tc.Name, Err: err})
			continue
		}

		results = append(results, Result{Name: tc.Name})
	}

	return results, nil
}

// verifyGitFiles opens each declared path in the built filesystem to confirm
// it was written correctly.  This catches mismatches between the fixture YAML
// and the in-memory filesystem construction early, before rule engine injection.
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

// verifyURLs checks that every URL key in the provided response maps is a
// syntactically valid absolute URL.  This catches typos in fixture files that
// would silently cause the mock to return 404 for every request.
func verifyURLs(maps ...map[string]HTTPResponseMock) error {
	for _, m := range maps {
		for rawURL := range m {
			u, err := url.Parse(rawURL)
			if err != nil {
				return fmt.Errorf("invalid URL in mock data %q: %w", rawURL, err)
			}
			if u.Scheme == "" || u.Host == "" {
				return fmt.Errorf("invalid URL in mock data %q: must be an absolute URL with scheme and host", rawURL)
			}
		}
	}
	return nil
}
