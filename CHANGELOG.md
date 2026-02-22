## [0.6.0] - 2026-02-22

### 🚀 Features

- Add import progress logging and multi-repo flag support
- Enrich import JSON output with per-repo summary
- Simplify CLI by removing import subcommands
- Add --format flag for JSON/YAML output
- Add --format and --debug flags to all commands
- Dashboard overhaul and data integrity fixes
- Entity filtering, adjustable period, reset command, docs update

### 🐛 Bug Fixes

- Omit empty org and repos from import JSON output
- Filter bot accounts from reputation dashboard query
- Add format/debug flags to all query subcommands

### 🚜 Refactor

- Move substitute command to pkg/cli/sub.go

### 📚 Documentation

- Add usage examples to import command help
- Add usage examples to substitute and query commands

### ⚙️ Miscellaneous Tasks

- Add yaml struct tags for YAML output format
- Remove CHANGELOG.md and git-cliff reference
## [0.5.7] - 2026-02-22

### 🐛 Bug Fixes

- Bump golangci-lint to v2.10.1 for Go 1.26 support

### 💼 Other

- Release v0.5.7
## [0.5.6] - 2026-02-22

### 🐛 Bug Fixes

- Exclude vendor from CI format check

### 💼 Other

- Release v0.5.6

### ⚙️ Miscellaneous Tasks

- Update readme
## [0.5.5] - 2026-02-22

### 🚀 Features

- Add contributor reputation scoring with two-tier model

### 🐛 Bug Fixes

- Resolve bugs, remove dead code, reduce duplication

### 💼 Other

- Streamline CI workflows and reduce duplication
- Upgrade Go from 1.25 to 1.26
- Release v0.5.5

### 📚 Documentation

- Update verification command for cosign v3 and add CI/release to CLAUDE.md

### ⚙️ Miscellaneous Tasks

- Update images
## [0.5.4] - 2026-02-21

### 💼 Other

- Release v0.5.0
- Release v0.5.4
## [0.0.1] - 2022-05-15
