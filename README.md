<p align="center"><img src="https://go-ruby-simplecov.github.io/logo.png" alt="go-ruby-simplecov/simplecov" width="720"></p>

# simplecov — go-ruby-simplecov

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-simplecov.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of the deterministic core of Ruby's
[SimpleCov](https://github.com/simplecov-ruby/simplecov) coverage gem — the
result engine.** It models raw per-file line- and branch-hit data, merges it
across runs, filters and groups the files, formats a summary, (de)serialises the
`.resultset.json` format, and enforces the coverage thresholds — **without any
Ruby runtime**.

It is the SimpleCov result engine for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby)/rbgo, but a
**standalone, reusable** module.

> **What it is — and isn't.** SimpleCov splits in two: a *collector* that
> instruments the running program and records how many times each line and branch
> executed, and a *result engine* that models that raw data and does everything
> else. The result engine is deterministic and needs **no interpreter**, so it
> lives here as pure Go: covered/missed/percent (line + branch), filtering,
> grouping, merging with a staleness window, the `.resultset.json` (de)serialiser,
> a text formatter, and the minimum-coverage / coverage-drop gates. The
> **collector is the host's job** — in rbgo the VM already tracks execution, so it
> hands this package a `map[filename]FileCoverage` and this package does the rest.

## The coverage seam

The unit of input is a `FileCoverage`: one `Hit` per source line (a coverable hit
count, or *null* = not coverable) plus optional `Branches`. rbgo builds the map
from its VM coverage tables; nothing here instruments code or opens Ruby.

```go
sc := simplecov.New(
    simplecov.WithRoot("/proj"),
    simplecov.WithClock(time.Now),
)
sc.AddStringFilter("/spec/")                 // add_filter "/spec/"
sc.AddStringGroup("Models", "/app/models/")  // add_group "Models", "app/models"
sc.MinimumCoverage(simplecov.Line, 90)       // minimum_coverage 90

result := sc.NewResult("RSpec", map[string]simplecov.FileCoverage{
    "/proj/app/models/user.rb": {
        Lines: []simplecov.Hit{
            simplecov.Uncoverable(), // null    — e.g. a comment
            simplecov.Coverable(1),  // hit once
            simplecov.Coverable(0),  // missed
        },
    },
})

fmt.Println(result.CoveredPercent())          // 50
code, violations := sc.RunChecks(result, nil) // code == simplecov.MinimumCoverage
```

## Features

Faithful port of SimpleCov's result engine:

- **Value model** — `Hit` (coverable count or null), `Branches`
  (condition→branch→count), `SourceFile` (covered/missed/never lines, percent,
  strength, branch metrics), `FileList` (aggregates, `LeastCoveredFile`,
  `CoveredPercentages`), and `Result` (named, timestamped, filtered, grouped).
- **Filters** — `StringFilter` (project-relative substring), `RegexFilter`,
  `BlockFilter`, `ArrayFilter` (any-of); `AddFilter` excludes matching files.
- **Groups** — `AddGroup("Models", filter)` partitions the files in definition
  order, with an automatic `"Ungrouped"` bucket.
- **Merging** — `MergeResults` sums coverage across runs (element-wise lines +
  branch counts, combined command name); `MergeStored` prunes results older than
  the `MergeTimeout` window before merging, against the injected clock.
- **`.resultset.json`** — `ParseResultset` / `Resultset.JSON` round-trip
  SimpleCov's `{"Command": {"coverage": {file: {"lines":[…], "branches":{…}}},
  "timestamp": N}}` format (and the legacy bare-array line form on read).
- **Thresholds** — `MinimumCoverage`, `MinimumCoverageByFile`,
  `MaximumCoverageDrop`, `RefuseCoverageDrop` (line + branch), returning a
  SimpleCov `ExitCode` and the list of `Violation`s.
- **Formatter** — a `Formatter` interface seam with `SimpleFormatter`
  (SimpleCov's console summary); the HTML formatter is out of scope.
- **Seams** — the filesystem (`FS`) and clock are injectable, so resultset
  load/store, source loading, timestamps, and the merge window are deterministic.

CGO-free, dependency-free (stdlib only), **100% test coverage**, `gofmt` +
`go vet` clean, and green across the six 64-bit Go targets (amd64, arm64,
riscv64, loong64, ppc64le, **s390x** — big-endian) plus `js/wasm` and
`wasip1/wasm`.

## Install

```sh
go get github.com/go-ruby-simplecov/simplecov
```

## Ruby surface it stands in for

| SimpleCov (Ruby)                          | this package                                  |
| ----------------------------------------- | --------------------------------------------- |
| `SimpleCov.start`                         | `New(opts…)`                                  |
| `add_filter "…"` / `%r{…}` / `{ … }`      | `AddStringFilter` / `AddRegexpFilter` / `AddBlockFilter` |
| `add_group "Models", "app/models"`        | `AddGroup` / `AddStringGroup`                 |
| `minimum_coverage 90`                     | `MinimumCoverage(Line, 90)`                   |
| `minimum_coverage_by_file 80`             | `MinimumCoverageByFile(Line, 80)`             |
| `maximum_coverage_drop 5`                 | `MaximumCoverageDrop(Line, 5)`                |
| `refuse_coverage_drop`                    | `RefuseCoverageDrop()`                        |
| `SimpleCov::Result` / `SourceFile` / `FileList` | `Result` / `SourceFile` / `FileList`    |
| `.resultset.json` (de)serialise           | `ParseResultset` / `Resultset.JSON`           |
| `SimpleCov::ResultMerger`                 | `MergeResults` / `(*SimpleCov).MergeStored`   |
| `SimpleCov::Formatter::SimpleFormatter`   | `SimpleFormatter`                             |

## Tests & coverage

The suite is deterministic: an in-memory `FS` and a fixed clock drive resultset
load/store, source loading, timestamps, and the merge staleness window, so every
branch — filter/group edge cases, merge stale-timeout, threshold pass/fail,
resultset round-trip, and FS-error paths — holds coverage at **100%** on every OS
and arch lane.

```sh
COVERPKG=$(go list ./... | paste -sd, -)
go test -race -coverpkg="$COVERPKG" -coverprofile=cover.out ./...
go tool cover -func=cover.out | tail -1   # 100.0%
```

## WebAssembly

Being pure Go (CGO=0), this library also compiles to **WebAssembly** — both
`GOOS=js GOARCH=wasm` (browser / Node.js) and `GOOS=wasip1 GOARCH=wasm` (WASI).
CI builds both targets on every push, alongside the six 64-bit native/qemu arches.

```sh
GOOS=js     GOARCH=wasm go build ./...   # browser / Node
GOOS=wasip1 GOARCH=wasm go build ./...   # WASI (wasmtime, wasmer, wasmedge, …)
```

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-simplecov/simplecov authors.
