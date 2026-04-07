# Minder Rule Testing Framework

A working prototype for offline rule testing in [Minder](https://github.com/mindersec/minder), built as part of my LFX Mentorship application (Summer 2026).

## What does this prototype do?

It lets you test any Minder rule by writing one YAML file. No GitHub token, no internet, everything runs from memory.

Here's what a test looks like for a REST rule:

```yaml
version: v1
rule_name: secret_scanning
test_cases:
  - name: "Pass when secret scanning is enabled"
    expect: pass
    def: { enabled: true }
    entity:
      type: repository
      entity: { owner: "testowner", name: "testrepo" }
    mock_data:
      http_responses:
        "https://api.github.com/repos/testowner/testrepo":
          status_code: 200
          body: '{"security_and_analysis": {"secret_scanning": {"status": "enabled"}}}'
```

And here's one for a Data Source rule (the part that doesn't exist in Minder today):

```yaml
version: v1
rule_name: osv_vulnerabilities
test_cases:
  - name: "Fail when a vulnerable dependency is found"
    expect: fail
    mock_data:
      data_source_responses:
        "osv.query":
          body: '{"vulns": [{"id": "GHSA-test-0001"}]}'
```

The framework parses the fixture, builds the right mock for each rule type, and validates everything. Once integrated into Minder, it will run the actual rule engine against your fixture data.

## How the mocking works

Minder rules talk to the outside world in three different ways. Each one needs a different mock:

**REST rules** (like `branch_protection`, `secret_scanning`) make HTTP calls to the GitHub API. The mock replaces the HTTP transport with a `MockRoundTripper` that returns whatever you put in the fixture under `http_responses`.

**Git rules** (like checking for `SECURITY.md` or `LICENSE`) clone a repo and read files. The mock replaces the repo filesystem with an in-memory `billy.Filesystem` built from the fixture's `git_files` map.

**Data Source rules** (like `osv_vulnerabilities`, `sonatype_oss_index`) are the tricky ones. During evaluation, the Rego policy calls something like `minder.datasource.osv.query(args)`. That call goes through `DataSourceFuncDef.Call()` in Minder's data source layer. The mock implements that exact interface and returns canned data. The engine can't tell the difference, and nothing ever hits the network.

The mock mirrors all four methods from the real `DataSourceFuncDef` interface:

```go
type MockDataSourceFuncDef struct{ Response any }

func (m *MockDataSourceFuncDef) Call(_ context.Context, _ any, _ any) (any, error) {
    return m.Response, nil
}
func (m *MockDataSourceFuncDef) ValidateArgs(_ any) error    { return nil }
func (m *MockDataSourceFuncDef) ValidateUpdate(_ any) error  { return nil }
func (m *MockDataSourceFuncDef) GetArgsSchema() any          { return nil }
```

In Minder, these get registered in a `DataSourceRegistry` and injected into the engine through `options.WithDataSources()`. The Rego evaluator picks them up and uses them as built-in functions during policy evaluation.

## Project structure

```
minder_prototype/
├── main.go                       # entry point
├── cmd/
│   ├── root.go                   # cobra root command
│   └── test.go                   # test + dryrun subcommands
├── pkg/testing/
│   ├── fixture.go                # YAML parser + validation
│   ├── fixture_test.go           # 18 tests
│   ├── mocks.go                  # HTTP, Git, Data Source mocks
│   ├── mocks_test.go             # 20 tests
│   ├── runner.go                 # DryRun + Evaluate
│   └── runner_test.go            # 12 tests
└── examples/
    ├── rest_rule_test.yaml       # secret_scanning fixture
    ├── git_rule_test.yaml        # osps-vm-05 fixture
    └── datasource_rule_test.yaml # osv_vulnerabilities fixture
```

## Running the tests

```bash
# all tests (50 tests, zero network calls)
go test ./... -v -count=1 -race

# just the parser
go test ./pkg/testing/ -run TestParse -v

# just the mocks
go test ./pkg/testing/ -run TestMock -v

# just the runner
go test ./pkg/testing/ -run TestDryRun -v
```

## Using the CLI

```bash
go build -o minder-test

# validate a fixture
./minder-test dryrun examples/rest_rule_test.yaml

# run tests (currently runs DryRun, full engine comes after Minder integration)
./minder-test test examples/datasource_rule_test.yaml
```

## What the fixture format supports

Each fixture tests one rule with multiple cases. A test case has:

- **name**: what the case is testing
- **expect**: `pass`, `fail`, or `error`
- **def / params**: rule definition and parameter overrides
- **entity**: the thing being evaluated (usually a repository with owner + name)
- **mock_data**: fake responses for HTTP, Git files, or Data Sources

You can skip cases that can't be tested yet:

```yaml
- name: "Needs commit history"
  skip_reason: "memfs doesn't support git log yet"
```

Or test that the engine itself errors:

```yaml
- name: "Missing data source"
  expect: error
  error_contains: "data source not registered"
```

## How this fits into Minder

This extends Minder's existing TestKit, it doesn't replace it.

| Rule type | Mock | Fixture field | Minder hook |
|-----------|------|---------------|-------------|
| REST | `MockRoundTripper` | `http_responses` | `TestKit.WithHTTP()` |
| Git | `NewMockBillyFS` | `git_files` | `TestKit.WithGitDir()` |
| Data Source | `MockDataSourceFuncDef` | `data_source_responses` | `options.WithDataSources()` |

## What's next

This prototype proves the design works. The remaining work (covered in the 12-week plan):

1. Wire the mocks into Minder's actual rule engine
2. Add `--fixture` flag to the existing CLI test command
3. Set up CI with fixture validation + full evaluation
4. Write fixtures for the 62 untested rules (starting with the 12 data source rules)
5. Documentation
