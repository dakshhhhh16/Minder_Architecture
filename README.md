# Minder Rule Testing Framework

A working prototype of the offline rule testing framework for Minder, built for the LFX Mentorship program (Summer 2026).

## What this does

Right now, testing a Minder rule means you need a live GitHub token and real network calls. This framework lets you test any rule by writing a YAML fixture file and running `go test` — no internet, no tokens, everything runs from memory.

## How it works

- **Fixture files** define test cases in YAML — each case says what mock data the rule should see and whether it should pass or fail.
- **Three mock layers** intercept the rule engine's calls:
  - HTTP mock (for REST API rules like branch protection checks)
  - Data Source mock (for Rego-evaluated rules like OSV vulnerability scans)
  - Git mock (for file-based rules like checking for SECURITY.md)
- **Dry-run validation** catches broken fixtures early without running the full engine.

## Running the tests

```bash
go test ./... -v -count=1 -race
```

All 32 tests pass with zero network calls.

## Project layout

```
.
├── .github/workflows/
│   └── rule_testing.yml         # CI workflow for tests + fixture validation
├── pkg/testing/
│   ├── fixture.go               # YAML parser and schema validation
│   ├── fixture_test.go          # Parser tests
│   ├── mocks.go                 # HTTP, Git, and Data Source mocks
│   ├── mocks_test.go            # Mock tests
│   ├── runner.go                # DryRun validator
│   └── runner_test.go           # DryRun tests
└── rules/
    └── sample_rule_test.yaml    # Example fixture (osps-vm-05)
```

## How this fits into Minder

| What | Where it plugs in | Mock used |
|------|-------------------|-----------|
| REST providers | `restHandler.testOnlyTransport` | `MockRoundTripper` from `http_responses` |
| Data Sources | `restHandler.testOnlyTransport` (separate instance) | `MockRoundTripper` from `data_source_responses` |
| Git ingester | `interfaces.GitProvider` via `git.New()` | `MockGitCloner` returns `memfs` from `git_files` |

The CLI flag will be `--fixture` (not `-t`, since that's already taken by the token flag).

## What a fixture looks like

```yaml
version: v1
rule_name: secret_scanning
test_cases:
  - name: "Enabled"
    expect: pass
    mock_data:
      http_responses:
        "https://api.github.com/repos/owner/repo":
          status_code: 200
          body: '{"security_and_analysis": {"secret_scanning": {"status": "enabled"}}}'
  - name: "Disabled"
    expect: fail
    mock_data:
      http_responses:
        "https://api.github.com/repos/owner/repo":
          status_code: 200
          body: '{"security_and_analysis": {"secret_scanning": {"status": "disabled"}}}'
```

## Next steps

1. Integrate the fixture parser into `cmd/dev/app/rule_type/rttst.go`
2. Wire the mocking layer into the rule engine via `http.RoundTripper` and `go-billy`
3. Add the `--fixture` flag to run test cases in batch
4. Migrate the top 20-25 community rules to use fixtures
