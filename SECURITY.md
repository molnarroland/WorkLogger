# Security

## Reporting a problem

Please report security problems privately, not in a public issue: open the repository's **Security** tab and
choose **Report a vulnerability** (GitHub's private vulnerability reporting). Include what you found, how to
reproduce it, and which version and operating system you used.

You can expect a first answer within a week. Fixes go into the next release, and the advisory is published once a
fixed version is out. Only the latest release gets security fixes.

## What WorkLogger protects

WorkLogger holds API tokens for Jira, Tempo and Google, and reads your git history, calendars, chats and HR
records. Everything stays on your machine; tokens are kept in the OS keyring, never in the database. The app's
trust boundaries, including why pages shown in its Google and HRmaster windows can't reach the app, are described
in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md#trust-boundaries).

Things that are in scope: anything that lets a web page, another program, or a crafted file (calendar feed, chat
export, git repository) read your data or tokens, or act through the app (send worklogs, change settings).

## Dependency audits

The CI runs `govulncheck` on every push. Release builds use the Go toolchain pinned in `go.mod` (`toolchain`),
which carries the standard library's security fixes.

Audit of 2026-11-13:

- **Go (`govulncheck`):** no vulnerabilities with Go 1.27.4. Go 1.27.1 had 12 known standard-library issues
  (`net/http`, `crypto/x509`, `crypto/tls`, `mime/multipart`), all fixed in 1.27.4, which `go.mod` now requires.
- **npm, what ships in the app (`npm audit --omit=dev`):** 0 vulnerabilities.
- **npm, development tools only (`npm audit`):** 20 advisories (18 moderate, 2 high), none in the app bundle:
  - `minimatch` (moderate, denial of service by crafted glob patterns), through ESLint, typescript-eslint, Nx and
    ts-morph. npm's suggested fixes are major-version downgrades of ESLint and typescript-eslint. These tools only
    match globs from this repository's own configuration.
  - `zod` (high, memory exhaustion validating oversized input), through `@spartan-ng/cli`, the component generator.
    No fixed version is available for its dependency range. It only validates the generator's own options.

  Recorded rather than forced: overriding them would put the tools on major versions they don't support. To be
  revisited when ESLint, Nx and `@spartan-ng/cli` move to fixed versions.
