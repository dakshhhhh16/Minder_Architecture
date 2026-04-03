// SPDX-FileCopyrightText: Copyright 2026 The Minder Authors
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"fmt"
	"net/url"

	"github.com/go-git/go-billy/v5"
)

// Result captures the outcome of one test case in a DryRun.
type Result struct {
	Name       string
	Skipped    bool
	SkipReason string
	Err        error
}

// DryRun parses a fixture, builds mocks for every non-skipped case, and
// checks that all declared git files are readable and all URLs are valid.
//
// It does NOT invoke the rule engine — that happens once the --fixture flag
// is wired into cmd/dev/app/rule_type/rttst.go. DryRun is useful as a
// standalone CI step to catch broken fixtures early.
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

// verifyGitFiles opens each file in the mock filesystem to confirm it was
// written correctly.
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

// verifyURLs checks that every URL key in the response maps is a valid
// absolute URL. Catches typos that would silently cause 404s.
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
