# Contributing to GitLab Reviewer Roulette Bot

Thank you for your interest in contributing! This document provides guidelines
and instructions for contributing to this project.

## Table of Contents

- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Code Quality Standards](#code-quality-standards)
- [Commit Message Guidelines](#commit-message-guidelines)
- [Pull Request Process](#pull-request-process)
- [Testing](#testing)
- [Code Review](#code-review)

## Getting Started

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:

   ```bash
   git clone https://github.com/aimd54/gitlab-reviewer-roulette.git
   cd gitlab-reviewer-roulette
   ```

3. **Add upstream remote**:

   ```bash
   git remote add upstream https://github.com/aimd54/gitlab-reviewer-roulette.git
   ```

4. **Create a feature branch**:

   ```bash
   git checkout -b feat/your-feature-name
   ```

## Development Setup

### Prerequisites

- Go 1.25+
- Docker & Docker Compose
- Make
- PostgreSQL client tools (`psql`)
- Git

### Initial Setup

1. **Install development tools**:

   ```bash
   make install-tools
   ```

   This installs:
   - golangci-lint (linter)
   - gosec (security scanner)
   - govulncheck (vulnerability checker)

2. **Install pre-commit hooks** (recommended):

   ```bash
   make pre-commit-install
   ```

   This installs Git hooks that run automatically before each commit:
   - Code formatting check
   - Go vet
   - Linting
   - Unit tests (fast mode)

3. **Setup local environment**:

   ```bash
   make setup-complete
   ```

   Or manually:

   ```bash
   make start       # Start Docker services
   make migrate     # Run database migrations
   make seed        # Seed test data (optional)
   ```

## Code Quality Standards

### Before Committing

**During development** (every commit):

- Pre-commit hooks run automatically (format, vet, lint, fast tests)

**Before creating a PR**:

```bash
make check
```

This comprehensive check includes:

- `make fmt` - Format code with gofmt
- `make vet` - Run go vet
- `make lint` - Run golangci-lint with 25+ linters
- `make lint-markdown` - Lint markdown documentation
- `make test` - Run full unit test suite
- `make security` - Security scan with gosec
- `make vuln-check` - Check for dependency vulnerabilities (govulncheck)

### Additional Quality Checks

```bash
make fmt-check        # Check formatting without modifying
make test-coverage    # Generate HTML coverage report
make test-short       # Run tests without race detector (faster)
make test-integration # Run integration tests (requires Docker)
```

### Code Style Guidelines

- **Formatting**: Use `gofmt` (enforced by pre-commit hooks)
- **Imports**: Use `goimports` for automatic import organization
- **Documentation**: All exported functions must have comments starting with the
  function name
- **Error Handling**: Always check errors; use `_ =` for intentionally ignored
  errors
- **Complexity**: Keep functions under ~50 lines when possible
- **Testing**: Maintain test coverage above 80% for business logic

### Go Best Practices

Follow [Effective Go](https://golang.org/doc/effective_go.html) and these
project-specific guidelines:

- **Package Structure**:
  - `internal/` - Private application code
  - `pkg/` - Reusable libraries
  - `cmd/` - Application entry points

- **Naming Conventions**:
  - Interfaces: `...er` suffix (e.g., `UserRepository`, `Selector`)
  - Structs: PascalCase (e.g., `SelectionRequest`)
  - Files: snake_case.go (e.g., `user_repository.go`)

- **Error Handling**:

  ```go
  // ✅ Good
  if err != nil {
      return fmt.Errorf("failed to select reviewer: %w", err)
  }

  // ❌ Bad
  if err != nil {
      return err  // Lost context
  }
  ```

- **Logging** (Zerolog):

  ```go
  log.Info().
      Str("username", user.Username).
      Int("active_reviews", count).
      Msg("Reviewer selected")
  ```

## Commit Message Guidelines

We follow [Conventional Commits](https://www.conventionalcommits.org/)
specification.

### Format

```shell
type(scope): subject

[optional body]

[optional footer]
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, missing semi-colons, etc.)
- `refactor`: Code refactoring (neither fixes a bug nor adds a feature)
- `test`: Adding or updating tests
- `chore`: Maintenance tasks, dependencies
- `perf`: Performance improvements
- `ci`: CI/CD changes
- `build`: Build system changes (Makefile, Docker, etc.)
- `revert`: Revert a previous commit

### Examples

```bash
# Good examples
feat(roulette): add expertise-based reviewer selection
fix(webhook): handle missing CODEOWNERS file gracefully
docs: update README with Docker setup instructions
refactor(cache): simplify Redis key generation logic
test(roulette): add tests for edge cases in selection
chore: upgrade dependencies to latest versions
perf(database): optimize query for active reviews
ci: add security scanning to pipeline
```

```bash
# Bad examples
❌ Added new feature           # Missing type
❌ fix: bug                    # Too vague
❌ FEAT: NEW FEATURE           # Wrong case
❌ fix(really-long-scope-name-that-exceeds-80-chars): ...  # Too long
```

### Commit Message Validation

The commit-msg hook validates your commit messages automatically. To bypass (not
recommended):

```bash
git commit --no-verify
```

## Pull Request Process

1. **Update your branch** with latest upstream:

   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Run all quality checks**:

   ```bash
   make check
   make security
   make vuln-check
   ```

3. **Push your branch**:

   ```bash
   git push origin feat/your-feature-name
   ```

4. **Create a Pull Request** on GitHub with:
   - Clear title following commit message conventions
   - Description explaining what and why
   - Reference to any related issues
   - Screenshots/examples if applicable
   - Fill out the PR template checklist

5. **Wait for CI checks** to pass:
   - All GitHub Actions workflows must pass (quality, test, build, security)
   - Check the "Actions" tab for detailed results
   - Fix any failing checks before requesting review

6. **Respond to feedback** promptly and professionally

7. **Squash commits** if requested before merging

### CI/CD Pipeline

All pull requests automatically run through GitHub Actions:

**Quality Checks:**

- Format check (`gofmt`)
- Go vet analysis
- golangci-lint (25+ linters)

**Tests:**

- Unit tests with coverage reporting
- Coverage uploaded to Codecov

**Build:**

- Binary compilation verification for all targets

**Security:**

- gosec security scanner
- govulncheck vulnerability scanner
- Dependency verification
- Secret scanning (TruffleHog)
- Trivy vulnerability scan

**Required for merge:**

- All CI jobs must pass (except vulnerability warnings)
- At least one approval from maintainer
- All conversations resolved
- Branch up to date with main

## Markdown Quality

Documentation quality is as important as code quality. All markdown files should pass linting checks.

### Linting Markdown

```bash
# Check all markdown files
make lint-markdown

# Auto-fix markdown issues (where possible)
make fix-markdown

# Run all quality checks (includes markdown)
make check
```

### Markdown Configuration

Markdown linting is configured in `.markdownlint.json`:

- **MD013** (line length): Disabled - technical docs need flexibility
- **MD033** (inline HTML): Disabled - needed for badges and tables
- **MD040** (code language): Disabled - ASCII diagrams don't need language specifiers
- **MD041** (first line H1): Disabled - too rigid for complex documents
- **MD036** (emphasis as heading): Disabled - false positives on bold numbered lists
- **MD060** (table style): Disabled - cosmetic formatting
- **MD024** (duplicate headings): Allowed in different sections (siblings_only)

### Best Practices

- Use consistent heading hierarchy (don't skip levels)
- Add blank lines around headings, lists, and code blocks
- Use fenced code blocks with language specifiers for syntax highlighting (when applicable)
- Keep lines reasonably short where possible (but don't sacrifice readability)
- Use tables for structured data
- Link to other documentation files for cross-references

## Testing

### Running Tests

```bash
make test              # Unit tests with race detector
make test-short        # Fast tests (no race detector)
make test-integration  # Integration tests (requires Docker)
make test-all          # All tests
make test-coverage     # Generate HTML coverage report
```

### Writing Tests

- **Unit Tests**: Place in same package as code (`*_test.go`)
- **Integration Tests**: Place in `test/integration/` with build tag:

  ```go
  //go:build integration
  ```

- **Test Naming**:

  ```go
  func TestFunctionName(t *testing.T) {}
  func TestFunctionName_EdgeCase(t *testing.T) {}
  ```

- **Table-Driven Tests**:

  ```go
  tests := []struct {
      name    string
      input   string
      want    string
      wantErr bool
  }{
      {name: "valid input", input: "foo", want: "bar", wantErr: false},
      {name: "invalid input", input: "", want: "", wantErr: true},
  }
  ```

### Test Coverage Goals

- Business logic (services): 80%+
- Critical paths (roulette algorithm): 95%+
- Infrastructure code: Best effort

## Code Review

### As an Author

- Keep changes focused and reasonably sized
- Add clear commit messages and PR descriptions
- Respond to feedback constructively
- Update based on comments before requesting re-review

### As a Reviewer

- Be respectful and constructive
- Focus on code quality, not personal preferences
- Suggest alternatives when requesting changes
- Approve when code meets standards, even if you'd write it differently

## Security

- **Never commit secrets** (tokens, passwords, API keys)
- Use environment variables for sensitive config
- Run `make security` to scan for security issues
- Run `make vuln-check` to check dependencies
- Report security issues privately to maintainers

## Questions?

- Check [README.md](README.md) for setup instructions
- Open an issue for questions or discussions

## License

By contributing, you agree that your contributions will be licensed under the
same license as the project.

---

Thank you for contributing! 🎉
