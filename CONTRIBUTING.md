# Contributing

Thanks for your interest in contributing to Bender.

## Development Setup

1. Install Go 1.25.4 or newer.
2. Clone the repository.
3. Run tests:

```bash
go test ./...
go test -race ./...
go vet ./...
```

## Pull Request Guidelines

- Keep PRs focused and small.
- Add or update tests for behavior changes.
- Keep public API changes documented in `README.md`.
- Avoid breaking API compatibility without clear justification.

## Commit Style

Recommended prefixes:

- `feat:` new behavior
- `fix:` bug fix
- `test:` tests only
- `docs:` documentation
- `chore:` maintenance

## Reporting Issues

When opening an issue, include:

- Go version
- minimal reproducible example
- expected vs actual behavior
- stack trace/logs when available
