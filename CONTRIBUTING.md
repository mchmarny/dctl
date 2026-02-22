# Contributing to devpulse

Thank you for your interest in contributing to devpulse! We welcome contributions from developers of all backgrounds and experience levels.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [How to Contribute](#how-to-contribute)
- [Design Principles](#design-principles)
- [Pull Request Process](#pull-request-process)
- [Tips for Contributors](#tips-for-contributors)

## Code of Conduct

This project follows a commitment to fostering an open and welcoming environment. Please be respectful and professional in all interactions. See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) for details.

## Getting Started

Before contributing:

1. Read the [README.md](README.md) to understand the project
2. Check existing [issues](https://github.com/mchmarny/devpulse/issues) to avoid duplicates
3. Set up your development environment following [DEVELOPMENT.md](DEVELOPMENT.md)

## How to Contribute

### Reporting Bugs

- Use [GitHub Issues](https://github.com/mchmarny/devpulse/issues/new) to report bugs
- Describe the issue clearly with steps to reproduce
- Include system information (OS, Go version)
- Attach logs or screenshots if applicable
- Check if the issue already exists before creating a new one

### Suggesting Enhancements

- Open a [GitHub Issue](https://github.com/mchmarny/devpulse/issues/new) describing the feature
- Clearly describe the proposed feature and its use case
- Explain how it benefits the project and users

### Improving Documentation

- Fix typos, clarify instructions, or add examples
- Update README.md for user-facing changes
- Ensure code comments are accurate and helpful

### Contributing Code

- Fix bugs, add features, or improve performance
- Follow the development workflow in [DEVELOPMENT.md](DEVELOPMENT.md)
- Ensure all tests pass and code meets quality standards
- Write tests for new functionality

#### Go dependencies (vendor)

This project vendors Go dependencies. After changing `go.mod` or `go.sum`, run `make tidy` (which runs `go mod vendor`) and commit `go.mod`, `go.sum`, and the `vendor/` directory. CI will fail if `vendor/` is out of sync.

## Design Principles

These principles guide all design decisions in devpulse. When faced with trade-offs, these principles take precedence.

### Correctness Must Be Reproducible

Given the same inputs, the same version must always produce the same result.

**What:** No hidden state, no implicit defaults, no non-deterministic behavior.

**Why:** Reproducibility is a prerequisite for debugging, validation, and trust.

### Partial Failure Is the Steady State

Design for partitions, timeouts, and bounded retries.

**What:** GitHub API calls may fail, rate limits may be hit, network may be unreliable. Every external interaction must handle failure gracefully.

**Why:** The GitHub API is the primary external dependency. Users import large datasets that take many API calls. Resilience is not optional.

### Boring First

Default to proven, simple technologies.

**What:** SQLite for storage, `net/http` for the server, `log/slog` for logging. No frameworks, no ORMs, no unnecessary abstractions.

**Why:** Simplicity reduces bugs, makes debugging easier, and keeps the project accessible to new contributors.

### Trust Requires Verifiable Provenance

Every released artifact carries verifiable proof of origin and build process.

**What:** All releases include SBOM, Sigstore signatures, and GitHub attestations. Users can verify exactly which commit and workflow produced any binary.

**Why:** "Trust us" is not a security model.

## Pull Request Process

### Before Submitting

1. **Ensure all checks pass:**
   ```bash
   make qualify
   ```

2. **Update documentation if needed:**
   - README.md for user-facing changes
   - DEVELOPMENT.md for developer workflow changes

3. **Sign your commits:**
   ```bash
   git commit -S -m "feat: add network stats"
   ```

### Creating the Pull Request

1. Push your branch and open a PR against `main`
2. Provide a clear summary of changes
3. Reference related issues (e.g., "Fixes #123")

### Review Process

1. **Automated checks** run via GitHub Actions:
   - Go tests with race detector
   - golangci-lint
   - Vulnerability scan (grype)

2. **Maintainer review** covers:
   - Correctness and functionality
   - Code style and Go idioms
   - Test coverage and quality

3. **Address feedback** by pushing new commits

4. **Merge**: Once approved and CI passes, a maintainer will merge

### After Merging

```bash
git checkout main
git pull origin main
git branch -d your-branch
```

## Tips for Contributors

### First-Time Contributors

**Recommended starting points:**

1. Start with issues labeled `good first issue`
2. Read existing code in the package you're modifying before writing
3. Study the [Design Principles](#design-principles) section

**Good first contributions:**

- Documentation improvements (typos, clarifications)
- Adding test cases to existing tests
- Improving error messages with better context

### Code Style

- Follow existing patterns in the codebase
- Use `fmt.Errorf("context: %w", err)` for error wrapping
- Use `log/slog` for all logging (never `fmt.Println`)
- Write table-driven tests for multiple test cases
- Sentinel errors as package-level vars (e.g., `errDBNotInitialized`)

### Writing Good Commit Messages

```
Short summary (50 chars or less)

More detailed explanation if needed. Wrap at 72 characters.
Explain the problem being solved and why this approach was chosen.

- Use present tense ("Add feature" not "Added feature")
- Reference issues: "Fixes #123" or "Related to #456"
```

### Getting Help

- **GitHub Issues**: [Create an issue](https://github.com/mchmarny/devpulse/issues/new) with the "question" label
- **Existing Issues**: Search for similar questions first

## Additional Resources

- [DEVELOPMENT.md](DEVELOPMENT.md) - Development setup, architecture, and tooling
- [README.md](README.md) - Project overview and quick start

Thank you for contributing to devpulse!
