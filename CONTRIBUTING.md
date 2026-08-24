# Contributing to Goy Store

Thank you for contributing to **Goy Store**!

## Development Guidelines

### Conventional Commits

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification for all commit messages. This ensures structured changelogs and automated release notes generation.

#### Commit Format
```text
<type>(<optional scope>): <description>

[optional body]

[optional footer(s)]
```

#### Allowed Types
- `feat`: A new feature or capability
- `fix`: A bug fix
- `docs`: Documentation updates or additions
- `style`: Formatting, missing semicolons, etc. (no functional code changes)
- `refactor`: Code refactoring that neither fixes a bug nor adds a feature
- `perf`: Performance improvements
- `test`: Adding or correcting tests
- `build`: Changes affecting build system or external dependencies
- `ci`: Changes to CI/CD workflows and configuration scripts
- `chore`: Maintenance tasks
- `revert`: Reverting previous commits

#### Examples
- `feat(blob): implement S3/MinIO backend for BlobStore in Rust and Go`
- `fix(relational): handle null values in query scan`
- `docs(readme): add Prometheus metrics reference`

---

## Git Hooks Setup

To enforce Conventional Commits and automatic formatting checks before every commit:

```bash
git config core.hooksPath .githooks
```

Hooks included:
- `.githooks/commit-msg`: Validates Conventional Commits format.
- `.githooks/pre-commit`: Runs `go fmt` and `cargo fmt -- --check`.

---

## Testing & Quality Assurance

Before submitting a Pull Request, verify that all linters and tests pass:

```bash
# Start Docker services
make e2e-up

# Run all Unit and E2E integration tests
make test-all

# Stop Docker services
make e2e-down
```

---

## Automated Releases

Releases are automated via [`.github/workflows/release.yml`](.github/workflows/release.yml).

1. Push a semantic tag (e.g. `v0.1.0-alpha` or `v0.1.0`):
   ```bash
   git tag -a v0.1.0-alpha -m "release: goy-store v0.1.0-alpha"
   git push origin v0.1.0-alpha
   ```
2. The GitHub Actions Release workflow will:
   - Validate full unit and E2E tests against real Docker services.
   - Package Rust crate and verify Go module.
   - Generate structured release notes using `git-cliff`.
   - Publish the GitHub Release with pre-release tags appropriately marked.
