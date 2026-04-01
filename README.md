# Minder Rule Testing Framework

This repository contains a working prototype of the offline rule testing and CI framework for Minder security policies, proposed for the LFX Mentorship period.

## Key Features

- **Multi-Case Test Fixtures**: Define multiple assertions (pass/fail) for a single rule in one YAML file.
- **100% Offline Execution**: All GitHub API calls, Git file operations, and External HTTP Data Sources are intercepted and served from memory.
- **Robust Schema Validation**: Fixtures use a mandatory `version` field; the parser rejects unknown versions with a clear error.
- **Automated CI Integration**: A plug-and-play GitHub Actions workflow to evaluate all rules against their test cases automatically on every PR.

## Running the Tests

```bash
go test ./... -v -count=1 -race
```

All 32 tests pass with zero network calls and the race detector enabled.

## Repository Structure

```text
.
├── .github/workflows/
│   └── rule_testing.yml         # CI: unit-tests job + validate-fixtures job
├── go.mod                       # github.com/dakshhhhh16/Minder_Architecture
├── pkg/testing/
│   ├── fixture.go               # YAML schema parser, Parse(), validation, SkipReason
│   ├── fixture_test.go          # 12 unit tests for the fixture parser (all parallel)
│   ├── mocks.go                 # MockRoundTripper, MockGitCloner, NewMockBillyFS,
│   │                            # GitCloner interface, TestCaseMocks, BuildMocks
│   ├── mocks_test.go            # 13 unit tests covering all mock types
│   ├── runner.go                # Result, DryRun, verifyGitFiles, verifyURLs
│   └── runner_test.go           # 7 unit tests for DryRun
└── rules/
    └── sample_rule_test.yaml    # Example of a multi-case fixture
```

## Integration Points (for Minder merge)

| Component | Injection point | Mock |
|-----------|----------------|------|
| REST providers | `restHandler.testOnlyTransport` | `MockRoundTripper` from `http_responses` |
| Declared data sources | `restHandler.testOnlyTransport` (separate instance) | `MockRoundTripper` from `data_source_responses` |
| Git ingester | `interfaces.GitProvider` passed to `git.New()` | `MockGitCloner` (returns `memfs` from `git_files`) |

The new CLI flag is `--fixture` (no short form — `-t` is already the token flag).

## Test Results

```
$ go test ./... -v -count=1 -race

--- PASS: TestParse_ValidFixture
--- PASS: TestParse_EmptyPath
--- PASS: TestParse_FileNotFound
--- PASS: TestParse_UnsupportedVersion
--- PASS: TestParse_MissingVersion
--- PASS: TestParse_MissingRuleName
--- PASS: TestParse_NoTestCases
--- PASS: TestParse_InvalidExpect
--- PASS: TestParse_MissingCaseName
--- PASS: TestParse_HTTPMockData
--- PASS: TestParse_SkipReason_RelaxesExpectValidation
--- PASS: TestParse_SampleFixtureFile
--- PASS: TestMockRoundTripper_HitURL
--- PASS: TestMockRoundTripper_MissURL_Returns404
--- PASS: TestMockRoundTripper_NilResponses_Safe
--- PASS: TestMockRoundTripper_MultipleURLs
--- PASS: TestNewMockBillyFS_FileExists
--- PASS: TestNewMockBillyFS_EmptyMap
--- PASS: TestNewMockBillyFS_MultipleFiles
--- PASS: TestNewMockBillyFS_MissingFile_ReturnsError
--- PASS: TestBuildMocks_WiresHTTPAndGit
--- PASS: TestBuildMocks_EmptyMockData
--- PASS: TestMockGitCloner_Clone_ReturnsPrepopulatedFS
--- PASS: TestMockGitCloner_Clone_IgnoresURLAndBranch
--- PASS: TestBuildMocks_DataSourceClientIsIndependent
--- PASS: TestDryRun_ValidFixture_AllPass
--- PASS: TestDryRun_SkippedCasesReported
--- PASS: TestDryRun_BadFixturePath_ReturnsError
--- PASS: TestDryRun_InvalidFixtureContent_ReturnsError
--- PASS: TestDryRun_InvalidURL_ReturnsResultError
--- PASS: TestDryRun_DataSourceURLValidated
--- PASS: TestDryRun_SampleFixtureFile
PASS
ok      github.com/dakshhhhh16/Minder_Architecture/pkg/testing
```

## Integration Roadmap
1. Integrate the `Fixture Parser` into `cmd/dev/app/rule_type/rttst.go`.
2. Implement the `Mocking Layer` utilizing `http.RoundTripper` and `go-billy`/`go-git`.
3. Add the `-t` flag to evaluate all test cases in batch.
4. Migrate the top 20-25 priority community rules to use this new framework.

## Local Execution

Once integrated into the Minder CLI, fixtures run with:

```bash
mindev ruletype test -t rules/sample_rule_test.yaml
```

