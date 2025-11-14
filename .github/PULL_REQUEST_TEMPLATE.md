## Description

Brief description of what this PR does.

Fixes #(issue number)

## Type of Change

Select the type that best describes this PR (follows [Conventional Commits](https://www.conventionalcommits.org/)):

- [ ] `feat`: New feature
- [ ] `fix`: Bug fix
- [ ] `docs`: Documentation changes
- [ ] `style`: Code style changes (formatting, etc.)
- [ ] `refactor`: Code refactoring (neither fixes a bug nor adds a feature)
- [ ] `test`: Adding or updating tests
- [ ] `perf`: Performance improvements
- [ ] `chore`: Maintenance tasks, dependencies
- [ ] `ci`: CI/CD changes
- [ ] `build`: Build system changes (Makefile, Docker, etc.)

## Changes Made

- Change 1
- Change 2
- Change 3

## Quality Checks

All quality checks must pass before merging:

- [ ] `make check` - All quality checks pass (fmt, vet, lint, test)
- [ ] `make security` - Security scan passes (gosec)
- [ ] `make vuln-check` - Vulnerability check passes (govulncheck)
- [ ] `make lint-markdown` - Markdown linting passes (if docs changed)

## Testing

- [ ] Unit tests added/updated for new functionality
- [ ] Test coverage maintained/improved (business logic: 80%+, critical paths: 95%+)
- [ ] `make test` passes locally
- [ ] `make test-coverage` reviewed (if applicable)
- [ ] Integration tests added/updated (if applicable)

## Code Quality

- [ ] Code follows [Effective Go](https://golang.org/doc/effective_go.html) guidelines
- [ ] Exported functions have proper documentation comments
- [ ] Error handling uses wrapped errors with context (`fmt.Errorf(...: %w)`)
- [ ] Functions kept under ~50 lines where possible
- [ ] Pre-commit hooks pass (formatting, vet, lint, tests)

## Commit Messages

- [ ] Commit messages follow Conventional Commits format: `type(scope): subject`
- [ ] Commit messages are clear and descriptive

## Documentation

- [ ] README.md updated (if user-facing changes)
- [ ] Relevant documentation updated (ARCHITECTURE.md, METRICS.md, etc.)
- [ ] Code comments added/updated for complex logic
- [ ] Configuration examples updated (if config changes)

## Additional Notes

Any additional information or context about the PR.

---

**For Reviewers:**

- [ ] Code quality meets project standards
- [ ] Tests adequately cover changes
- [ ] Documentation is clear and complete
- [ ] No security concerns identified
