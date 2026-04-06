# Minder Rule Testing Framework

A working prototype of the offline rule testing framework for Minder, built for the LFX Mentorship program (Summer 2026).

## The Problem

Right now, testing a Minder rule requires a live GitHub token and real network calls. You need to set up three separate files (rule-type YAML, entity YAML, profile YAML), grab a token, and hope the API does not rate limit you. This makes tests slow, flaky, and painful to write. As a result, only 11 out of 63+ rules have any test coverage today, and the 6 Data Source rules (OSV, Sonatype, OpenSSF Bestpractices, etc.) have zero coverage because they do not even make HTTP calls at ingestion time.

## What This Project Does

This framework lets anyone test a Minder rule by writing a single YAML fixture file and running `go test`. No internet required, no tokens, everything runs from memory.

You write something like this:

```yaml
version: v1
rule_name: secret_scanning
test_cases:
  - name: "Pass when secret scanning is enabled"
    expect: pass
    def:
      enabled: true
    entity:
      type: repository
      entity:
        owner: "testowner"
        name: "testrepo"
    mock_data:
      http_responses:
        "https://api.github.com/repos/testowner/testrepo":
          status_code: 200
          body: '{"security_and_analysis": {"secret_scanning": {"status": "enabled"}}}'
```

And the framework handles the rest. It builds offline mocks, validates everything, and (once integrated into Minder) runs the actual rule engine against your fixture data.

## How It Works

Minder has three kinds of rules, and each one talks to the outside world differently. This framework intercepts each of those paths with a mock layer so nothing ever hits the network:

**REST API rules** (like `branch_protection`, `secret_scanning`) make HTTP calls to the GitHub API. We replace the HTTP transport with a `MockRoundTripper` that returns whatever responses you put in the fixture. In Minder, this plugs in through `TestKit.WithHTTP()`.

**Git file rules** (like checking for `SECURITY.md` or `LICENSE`) clone a repository and read files from it. We replace the real filesystem with an in-memory `billy.Filesystem` populated from the fixture's `git_files` map. No actual clone happens.

**Data Source rules** (like `osv_vulnerabilities`, `sonatype_oss_index`) are the interesting part and the core of this project. These rules do NOT make HTTP calls at ingestion time. Instead, they register Rego built-in functions that fire inside the OPA evaluator during policy evaluation. The call path looks like:

```
Rego policy
  -> calls minder.datasource.osv.query(args)
  -> which is a rego.Function1 registered by buildFromDataSource()
  -> which calls DataSourceFuncDef.Call()
  -> which makes the actual REST call to the OSV API
```

We intercept at `DataSourceFuncDef.Call()` because that is the narrowest seam between the Rego engine and any real I/O. Our `MockDataSourceFuncDef` implements that interface and returns canned fixture data. The engine has no idea it is talking to a fake, and zero network calls happen.

In the fixture, data source mocks use the `"name.func"` key format:

```yaml
mock_data:
  data_source_responses:
    "osv.query":
      body: '{"vulns": [{"id": "GHSA-test-0001"}]}'
```

## Project Structure

```
minder_prototype/
├── main.go                          # CLI entry point
├── cmd/
│   ├── root.go                      # Root command setup (cobra)
│   └── test.go                      # test and dryrun subcommands
├── pkg/
│   └── testing/
│       ├── fixture.go               # YAML parser and schema validation
│       ├── fixture_test.go          # Parser tests (18 tests)
│       ├── mocks.go                 # HTTP, Git, and Data Source mocks
│       ├── mocks_test.go            # Mock tests (20 tests)
│       ├── runner.go                # DryRun validator + Evaluate entry point
│       └── runner_test.go           # DryRun/Evaluate tests (12 tests)
├── examples/
│   ├── rest_rule_test.yaml          # REST API rule fixture (secret_scanning)
│   ├── git_rule_test.yaml           # Git file rule fixture (osps-vm-05)
│   └── datasource_rule_test.yaml   # Data Source rule fixture (osv_vulnerabilities)
├── go.mod
├── implementation_plan.txt          # Detailed 12-week plan
├── Minder_proposal.md               # LFX mentorship proposal
├── project.txt                      # Program overview
├── whyme.txt                        # Background and motivation
└── README.md                        # You are here
```

## Running the Tests

```bash
cd minder_prototype

# Run all tests
go test ./... -v -count=1 -race

# Run just the fixture parser tests
go test ./pkg/testing/ -run TestParse -v

# Run just the mock tests
go test ./pkg/testing/ -run TestMock -v
go test ./pkg/testing/ -run TestBuild -v

# Run the DryRun/Evaluate tests
go test ./pkg/testing/ -run TestDryRun -v
go test ./pkg/testing/ -run TestEvaluate -v
```

All 50 tests pass with zero network calls.

## Using the CLI

```bash
# Build the CLI tool
go build -o minder-test

# Run full evaluation on a fixture
./minder-test test examples/rest_rule_test.yaml

# Run dry-run validation only (fast, no engine)
./minder-test dryrun examples/git_rule_test.yaml
```

The CLI currently runs DryRun validation (checks fixture structure, mock data integrity, URL validity). Full engine evaluation will be wired in once the framework is integrated into Minder's codebase and has access to `rtengine.NewRuleTypeEngine()`.

## How This Fits Into Minder

The framework extends Minder's existing TestKit (`pkg/testkit/v1/`) rather than replacing it. Here is what plugs in where:

| Rule Type | What Gets Mocked | Fixture Section | Minder Integration Point |
|-----------|------------------|-----------------|--------------------------|
| REST API | HTTP transport | `http_responses` | `TestKit.WithHTTP(mockRoundTripper)` |
| Git files | Repository filesystem | `git_files` | `TestKit.WithGitDir()` or `rte.WithCustomIngester()` |
| Data Source | Rego built-in functions | `data_source_responses` | `DataSourceRegistry` via `options.WithDataSources()` |

The existing `rules_test.go` in `minder-rules-and-profiles` already uses TestKit for REST and Git rules. This framework adds the missing Data Source support and wraps everything in a single fixture file format so non-Go developers can write tests too.

## The Fixture Format

A fixture has three sections: the rule name, and a list of test cases. Each test case provides:

- **name**: A human readable description of what this case tests
- **expect**: What should happen ("pass", "fail", or "error")
- **def**: Rule definition overrides (maps to `RuleTypeEngine.Eval()` def parameter)
- **params**: Rule parameters (maps to `RuleTypeEngine.Eval()` params parameter)
- **entity**: The entity to evaluate against (type + fields like owner/name)
- **mock_data**: The fake data for HTTP, Git, or Data Source providers

For cases that cannot be tested yet (like rules that need git commit history), you can use `skip_reason` to document why:

```yaml
test_cases:
  - name: "Needs commit history"
    skip_reason: "requires git commit history, memfs does not support this yet"
```

For cases where the engine itself should error (not just fail), use `expect: error` with `error_contains`:

```yaml
test_cases:
  - name: "Missing data source should error"
    expect: error
    error_contains: "data source not registered"
    entity:
      type: repository
      entity: {}
    mock_data: {}
```

## What Happens Next

This prototype demonstrates the fixture format, all three mock layers, and the validation pipeline. The next steps in the 12-week plan are:

1. Wire the mock layers into Minder's actual rule engine (`rtengine.NewRuleTypeEngine()`)
2. Add the `--fixture` flag to the existing CLI at `cmd/dev/app/rule_type/rttst.go`
3. Set up CI with two GitHub Actions jobs (fixture validation + full evaluation)
4. Migrate existing rules to use the new fixture format (starting with REST, then Data Source, then Git history)
5. Write the documentation (fixture format reference, migration guide, CLI docs)

The goal is to get from 24% rule coverage to 80%+ by the end of the mentorship, with every new rule getting at least one passing and one failing test case.

## Why YAML Fixtures Instead of Go Table Tests

The people adding rules to `minder-rules-and-profiles` are not always Go developers. A YAML file that anyone can read and modify without touching Go code means tests actually get written when new rules land. That is the only way the coverage number stays above zero six months from now.
