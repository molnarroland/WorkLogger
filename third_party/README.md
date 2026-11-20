# third_party

## spartan-native-host-wails

The Go half of `@spartan-native/host-wails` 0.2.0 (module `github.com/spartan-native/host-wails`). The npm package
(installed from the registry like the other `@spartan-native/*` packages) only ships the TypeScript adapter (`wailsHost()`), which calls
this service's bound methods by name. The Go module isn't on the Go proxy yet, so it is vendored here and wired up
with a `replace` in `go.mod`.

- Upstream: <https://github.com/Spartan-Native/host-wails>, tag `0.2.0`, commit
  `be832a2359c1947cb3b6d64469a7ba17325abc6e`.
- Copied unchanged: the Go files, `internal/` and `testdata/` (checked file by file against the tag).
- Licence: MIT, by the same author as WorkLogger. The upstream repository declares it in its npm package but has no
  licence file at that tag, so `LICENSE` here is the Spartan Native project's MIT licence.

When the Go module is on the Go proxy (it needs a `v0.2.0` tag; upstream is tagged `0.2.0`), depend on it instead and
remove this copy and the `replace`.
