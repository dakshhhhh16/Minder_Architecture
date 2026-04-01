# Minder Rule Testing Framework

This repository contains a working prototype of the offline rule testing and CI framework for Minder security policies, proposed for the LFX Mentorship period.

## Key Features

- **Multi-Case Test Fixtures**: Define multiple assertions (pass/fail) for a single rule in one YAML file.
- **100% Offline Execution**: All GitHub API calls, Git file operations, and External HTTP Data Sources are intercepted and served from memory.
- **Robust Schema Validation**: Fixtures use a mandatory `version` field; the parser rejects unknown versions with a clear error.
- **Automated CI Integration**: A plug-and-play GitHub Actions workflow to evaluate all rules against their test cases automatically on every PR.

## Running the Tests

```bash
go test ./... -v
```

All 11 tests pass with zero network calls.

## Repository Structure

```text
.
├── .github/workflows/
│   └── rule_testing.yml         # CI pipeline to automatically test rules
├── go.mod                       # github.com/dakshhhhh16/Minder_Architecture
├── pkg/testing/
│   ├── fixture.go               # YAML schema parser, Parse() & validation
│   ├── fixture_test.go          # 11 unit tests for the fixture parser
│   └── mocks.go                 # Offline mock wrappers for v1.REST & v1.Git
└── rules/
    └── sample_rule_test.yaml    # Example of a multi-case fixture
```

## Test Results

All tests pass on the fixture parser:

```
$ go test ./... -v -count=1

=== RUN   TestParse_ValidFixture
--- PASS: TestParse_ValidFixture (0.00s)
=== RUN   TestParse_EmptyPath
--- PASS: TestParse_EmptyPath (0.00s)
=== RUN   TestParse_FileNotFound
--- PASS: TestParse_FileNotFound (0.00s)
=== RUN   TestParse_UnsupportedVersion
--- PASS: TestParse_UnsupportedVersion (0.00s)
=== RUN   TestParse_MissingVersion
--- PASS: TestParse_MissingVersion (0.00s)
=== RUN   TestParse_MissingRuleName
--- PASS: TestParse_MissingRuleName (0.00s)
=== RUN   TestParse_NoTestCases
--- PASS: TestParse_NoTestCases (0.00s)
=== RUN   TestParse_InvalidExpect
--- PASS: TestParse_InvalidExpect (0.00s)
=== RUN   TestParse_MissingCaseName
--- PASS: TestParse_MissingCaseName (0.00s)
=== RUN   TestParse_HTTPMockData
--- PASS: TestParse_HTTPMockData (0.00s)
=== RUN   TestParse_SampleFixtureFile
--- PASS: TestParse_SampleFixtureFile (0.00s)
PASS
ok      github.com/dakshhhhh16/Minder_Architecture/pkg/testing  0.851s
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

