# Contributing to apibind

Thank you for your interest in contributing!
apibind is a small, focused library — contributions that keep it simple and well-tested are especially welcome.

## Prerequisites

- Go 1.23.0 or later
- No external dependencies (this library has none)

## Getting Started

```sh
git clone https://github.com/yuma-seno/apibind.git
cd apibind
go test ./... -race
```

All tests should pass before and after your change.

## Branch Strategy

This project uses **GitHub Flow**:

| Branch | Purpose |
|---|---|
| `main` | Always release-ready. Direct pushes are restricted. |
| `feat/<short-description>` | New features (e.g. `feat/sse-support`) |
| `fix/<short-description>` | Bug fixes (e.g. `fix/patch-body`) |
| `docs/<short-description>` | Documentation only |
| `chore/<short-description>` | Tooling, CI, dependencies |

Steps:
1. Fork the repository and create a branch from `main`.
2. Make your changes with tests.
3. Open a pull request against `main`.
4. After review and CI pass, it will be squash-merged.

## Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>: <short summary>
```

Types: `feat`, `fix`, `docs`, `test`, `ci`, `chore`, `refactor`

Examples:
```
feat: add context.Context to Call()
fix: support PATCH method in client and handler
test: add comprehensive test suite
```

## Pull Request Guidelines

- Keep PRs small and focused on a single concern.
- Include tests for every bug fix and new feature.
- Update documentation (godoc comments, README) as needed.
- All CI checks must pass.

## Code Style

- Run `gofmt` and `go vet` before committing.
- Public API changes require godoc comments.
- Avoid adding external dependencies — the library intentionally has none.

## Testing

```sh
# Run all tests with race detector
go test ./... -race

# Run a specific test
go test -run TestCall_PATCH_Body -v
```

## Releases

Releases follow [Semantic Versioning](https://semver.org/):

- `v0.x.y` — initial development, breaking changes are allowed between minor versions
- `v1.x.y` and beyond — breaking changes require a major version bump

Maintainers create release tags on `main` after merging.

## Questions

Open a [GitHub Issue](https://github.com/yuma-seno/apibind/issues) for questions, bug reports, or feature proposals.
