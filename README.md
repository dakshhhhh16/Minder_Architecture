# Minder Rule Testing Framework (LFX Mentorship 2026)

This repository contains a sample implementation of the offline rule testing and CI framework for Minder security policies, proposed for the LFX Mentorship period.

## Key Features

- **Multi-Case Test Fixtures**: Define multiple assertions (pass/fail) for a single rule in one YAML file.
- **100% Offline Execution**: All GitHub API calls, Git file operations, and External HTTP Data Sources are intercepted and served from memory.
- **Robust Schema Validation**: Fixtures use a mandatory `version` field validated via `go-playground/validator`.
- **Automated CI Integration**: A plug-and-play GitHub Actions workflow to evaluate all rules against their test cases automatically on every PR.

## Projected Structure

During the LFX term, these components will be integrated directly into the `minder` and `minder-rules-and-profiles` repositories:

```text
.
├── .github/workflows/
│   └── rule_testing.yml         # CI pipeline to automatically test rules
├── pkg/testing/
│   ├── fixture.go               # YAML schema parser & test loop logic
│   └── mocks.go                 # Offline mock wrappers for v1.REST & v1.Git
└── rules/
    └── sample_rule_test.yaml    # Example of a multi-case fixture
```

## Upcoming LFX Term Objectives
1. Integrate the `Fixture Parser` into `cmd/dev/app/rule_type/rttst.go`.
2. Implement the `Mocking Layer` utilizing `http.RoundTripper` and `go-billy`/`go-git`.
3. Add the `-t` flag to evaluate all test cases in batch.
4. Migrate the top 20-25 priority community rules to use this new framework.

## Local Execution (Planned)

Once integrated, fixtures can be executed locally using:

```bash
mindev ruletype test -t rules/sample_rule_test.yaml
```
