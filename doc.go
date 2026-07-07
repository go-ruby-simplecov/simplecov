// Copyright (c) the go-ruby-simplecov/simplecov authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package simplecov is a pure-Go (CGO-free) reimplementation of the
// deterministic core of Ruby's [SimpleCov] coverage gem — the result engine.
//
// SimpleCov splits into two halves: a collector that instruments the running
// program and records, per file, how many times each line (and branch) was hit;
// and a result engine that models that raw hit data, merges it across runs,
// filters and groups the files, formats a summary, and enforces coverage
// thresholds. Everything in the second half is deterministic and needs no Ruby
// interpreter, so it lives here as pure Go. The collector is the host's job: in
// [go-embedded-ruby]/rbgo the VM already tracks execution, so it feeds this
// package the per-file line-hit arrays and this package does the rest.
//
// # The coverage seam
//
// The unit of input is a [FileCoverage]: a slice of [Hit] (one per source line;
// a [Hit] is either an integer hit count or "not coverable" — null in the
// resultset) and, optionally, [Branches]. rbgo builds a map[filename]FileCoverage
// from its VM coverage tables and hands it to [SimpleCov.NewResult]; nothing in
// this package instruments code or opens Ruby.
//
// # Value model
//
//   - [Hit]        — one line's coverage: coverable+count, or null.
//   - [Branches]   — nested condition→branch→count map (branch coverage).
//   - [SourceFile] — one file: covered/missed/never lines, percent, strength.
//   - [FileList]   — a collection of SourceFile with aggregate metrics.
//   - [Result]     — a named, timestamped FileList, filtered and grouped.
//   - [Resultset]  — the .resultset.json shape: command → coverage + timestamp.
//
// # Ruby surface it stands in for
//
//	SimpleCov.start                          -> New(opts...)
//	SimpleCov.add_filter "…" / %r{…} / {…}   -> AddStringFilter / AddRegexpFilter / AddBlockFilter
//	SimpleCov.add_group "Models", "app/…"    -> AddGroup
//	SimpleCov.minimum_coverage 90            -> MinimumCoverage
//	SimpleCov.minimum_coverage_by_file 80    -> MinimumCoverageByFile
//	SimpleCov.maximum_coverage_drop 5        -> MaximumCoverageDrop
//	SimpleCov.refuse_coverage_drop           -> RefuseCoverageDrop
//	SimpleCov::Result / SourceFile / FileList-> Result / SourceFile / FileList
//	.resultset.json (de)serialise            -> ParseResultset / Resultset.JSON
//	SimpleCov::ResultMerger                  -> MergeResults / (*SimpleCov).MergeStored
//	SimpleCov::Formatter::SimpleFormatter    -> SimpleFormatter
//
// # Seams
//
// The filesystem ([FS]) and the clock (a func() time.Time) are injectable, so
// resultset load/store, source loading, timestamps and the merge staleness
// window are all deterministic under test. The default [FS] is os-backed and the
// default clock is time.Now.
//
// [SimpleCov]: https://github.com/simplecov-ruby/simplecov
// [go-embedded-ruby]: https://github.com/go-embedded-ruby/ruby
package simplecov
