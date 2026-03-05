## [0.7.3] - 2026-03-05

### ⚙️ Miscellaneous Tasks

- Replace file-based versioning with git-tag bump workflow
## [0.7.2] - 2026-03-05

### 🚀 Features

- Add query for lowest-reputation usernames
- Add ImportDeepReputation for bulk deep scoring
- Add --deep flag for bulk deep reputation scoring during import

### ⚙️ Miscellaneous Tasks

- Remove plans
## [0.7.1] - 2026-03-02

### 🚀 Features

- Integrate reputer v0.5.0 with v3 scoring model and PR action
- Update reputation display for v3 model and exclude bots

### 💼 Other

- *(deps)* Bump the all-actions group with 2 updates (#86)
- *(deps)* Bump anchore/sbom-action (#85)
- *(deps)* Bump actions/setup-go (#84)

### 🧪 Testing

- Verify reputation action fires on PR (#87)
## [0.7.0] - 2026-02-25

### 🚀 Features

- Add release_asset migration
- Add SQL constants and types for release downloads
- Import release asset download counts
- Add release downloads query
- Add release downloads API endpoint
- Add release downloads dashboard panel
- Add GetReleaseDownloadsByTag query
- Add release downloads by tag API endpoint
- Add release downloads by tag dashboard panel

### 🚜 Refactor

- Use command name as log prefix instead of static [devpulse]
- Remove redundant context from log messages

### 📚 Documentation

- Add design for per-tag release downloads panel
- Add implementation plan for per-tag release downloads

### 🧪 Testing

- Add tests for GetReleaseDownloadsByTag

### ⚙️ Miscellaneous Tasks

- Add .worktrees to gitignore
## [0.6.8] - 2026-02-23

### 🚀 Features

- Add forks & activity panel, fix velocity filters, add indexes
## [0.6.7] - 2026-02-23

### 🚀 Features

- Support GITHUB_TOKEN env var and --debug on all commands
## [0.6.6] - 2026-02-23

### 🚀 Features

- Add --public flag to auth command for public-only repo access
## [0.6.5] - 2026-02-22

### ⚙️ Miscellaneous Tasks

- Clean up github actions, add image build using ko
## [0.6.4] - 2026-02-22

### ⚙️ Miscellaneous Tasks

- Readme
- Rename workflows
- Rename workflows
## [0.6.3] - 2026-02-22

### 🚀 Features

- Port reputer CI/build patterns to devpulse

### 🐛 Bug Fixes

- Add id-token permission for codecov OIDC in caller workflows
- Yamllint CI failures (dependabot indent, vendor exclusion)

### ⚙️ Miscellaneous Tasks

- Clean up makefile
- Bump yamllint-github-action v2.1.1 to v3.0.0
## [0.6.2] - 2026-02-22

### 🚀 Features

- Rename project from dctl to devpulse

### ⚙️ Miscellaneous Tasks

- Remove CHANGELOG generation and update goreleaser config
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

### 💼 Other

- Release v0.6.0

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
