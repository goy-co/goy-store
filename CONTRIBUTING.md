# Contributing to Goy Store

Thank you for your interest in contributing to **Goy Store**! This document outlines everything you need to know to contribute.

---

## Legal Framework

### License

Goy Store is released under the **Goy Source Available License (GSAL) v2.0**. This is a source-available license — you can view, inspect, and audit the code, but you cannot deploy, modify, or redistribute it without a separate agreement with The Goy Company.

By contributing to this repository, you acknowledge that your contributions will be licensed under the GSAL.

### Contributor License Agreement (CLA)

All contributors must agree to the **[Contributor License Agreement (CLA)](CLA.md)** before their pull request can be merged. The CLA:

- Grants The Goy Company a broad license to use, modify, and relicense your contributions
- Includes a patent grant from you to The Goy Company
- Requires you to warrant that you have the right to make the contribution
- Allows The Goy Company to relicense your contributions under any license (GSAL, open-source, or proprietary)

**You do not need to sign the CLA upfront.** When you submit your first pull request, a bot will check whether you've agreed to the CLA and provide instructions if you haven't.

### What This Means for You

| You retain | Goy gains |
|---|---|
| Copyright in your contribution | Perpetual, worldwide, royalty-free license to use it |
| The right to use your own contribution in your own projects | Right to modify, adapt, and create derivative works |
| The right to contribute to other projects | Right to sublicense and relicense under any terms |

---

## How to Contribute

### 1. Before You Start

- **Check existing issues** to see if someone is already working on the same thing
- **Open an issue** for significant changes so we can discuss the approach before you invest time
- **Read the PRD** (`PRD.md`) and **ADRs** (`adr/`) to understand the project's direction and architectural decisions

### 2. Development Setup

```bash
# Clone the repository
git clone https://github.com/goy-co/goy-store.git
cd goy-store

# Set up git hooks (enforces Conventional Commits and formatting)
git config core.hooksPath .githooks
```

### 3. Making Changes

#### Conventional Commits

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification for all commit messages. This ensures structured changelogs and automated release notes generation.

**Format:**
```
<type>(<optional scope>): <description>

[optional body]

[optional footer(s)]
```

**Allowed Types:**

| Type | Description |
|---|---|
| `feat` | A new feature or capability |
| `fix` | A bug fix |
| `docs` | Documentation updates or additions |
| `style` | Formatting, missing semicolons, etc. (no functional code changes) |
| `refactor` | Code refactoring that neither fixes a bug nor adds a feature |
| `perf` | Performance improvements |
| `test` | Adding or correcting tests |
| `build` | Changes affecting build system or external dependencies |
| `ci` | Changes to CI/CD workflows and configuration scripts |
| `chore` | Maintenance tasks |
| `revert` | Reverting previous commits |

**Examples:**
- `feat(blob): implement S3/MinIO backend for BlobStore in Rust and Go`
- `fix(relational): handle null values in query scan`
- `docs(readme): add Prometheus metrics reference`

#### Git Hooks

The repository includes git hooks in `.githooks/`:

| Hook | Purpose |
|---|---|
| `commit-msg` | Validates Conventional Commits format |
| `pre-commit` | Runs `go fmt` and `cargo fmt -- --check` |

### 4. Testing

Before submitting a Pull Request, verify that all linters and tests pass:

```bash
# Start Docker services
make e2e-up

# Run all unit and E2E integration tests
make test-all

# Stop Docker services
make e2e-down
```

### 5. Submitting a Pull Request

1. **Fork the repository** and create a feature branch
2. **Make your changes** following the guidelines above
3. **Write tests** for new functionality
4. **Ensure all tests pass** (see Testing section)
5. **Open a Pull Request** against the `main` branch
6. **Describe your changes** clearly in the PR description — what problem does it solve, what approach did you take, and any trade-offs you made

### 6. Review Process

- A maintainer will review your PR and may request changes
- The CLA bot will verify you've agreed to the Contributor License Agreement
- Once approved and all checks pass, your PR will be merged

---

## Code of Conduct

### Our Standards

- Be respectful and constructive in all interactions
- Focus on the technical merits of contributions, not the person making them
- Accept constructive criticism gracefully
- Help others learn and grow

### Unacceptable Behavior

- Harassment, discrimination, or exclusionary language
- Trolling, insulting/derogatory comments, and personal attacks
- Public or private harassment
- Publishing others' private information without permission

### Enforcement

Violations of the code of conduct may result in removal from the project. If you witness or experience unacceptable behavior, contact us at legal@goycompany.com.

---

## Security

If you discover a security vulnerability, please see our **[Responsible Disclosure Policy](SECURITY.md)**. Do not open public issues for security vulnerabilities — report them directly to legal@goycompany.com.

---

## Questions?

- **Legal questions** (CLA, licensing): legal@goycompany.com
- **Security issues**: legal@goycompany.com (see [SECURITY.md](SECURITY.md))
- **General questions**: Open an issue in this repository

---

**The Goy Company**
legal@goycompany.com
