// Copyright (c) the go-ruby-simplecov/simplecov authors
//
// SPDX-License-Identifier: BSD-3-Clause

package simplecov

import (
	"errors"
	"io/fs"
	"regexp"
	"time"
)

// DefaultMergeTimeout is SimpleCov's default merge window: results older than
// this (relative to the clock) are pruned before merging.
const DefaultMergeTimeout = 600 * time.Second

// DefaultResultsetPath is SimpleCov's default resultset filename.
const DefaultResultsetPath = ".resultset.json"

// namedFilter pairs a group name with the filter selecting its files.
type namedFilter struct {
	name   string
	filter Filter
}

// SimpleCov is the configuration object and orchestrator — the Go stand-in for
// the Ruby SimpleCov module. Build one with New, configure filters, groups and
// thresholds, then turn raw coverage into a Result with NewResult. The FS and
// clock seams make every step deterministic.
type SimpleCov struct {
	CommandName  string
	Root         string
	MergeTimeout time.Duration

	filters []Filter
	groups  []namedFilter
	check   Check

	fs    FS
	clock func() time.Time
}

// Option configures a SimpleCov at construction.
type Option func(*SimpleCov)

// WithFS overrides the filesystem seam.
func WithFS(f FS) Option { return func(sc *SimpleCov) { sc.fs = f } }

// WithClock overrides the clock seam used for timestamps and merge staleness.
func WithClock(clock func() time.Time) Option { return func(sc *SimpleCov) { sc.clock = clock } }

// WithRoot sets the project root used to derive project-relative paths.
func WithRoot(root string) Option { return func(sc *SimpleCov) { sc.Root = root } }

// WithCommandName sets the default command name for results.
func WithCommandName(name string) Option { return func(sc *SimpleCov) { sc.CommandName = name } }

// WithMergeTimeout overrides the merge staleness window.
func WithMergeTimeout(d time.Duration) Option { return func(sc *SimpleCov) { sc.MergeTimeout = d } }

// New builds a SimpleCov with SimpleCov's defaults (os FS, time.Now clock,
// 600s merge timeout), then applies the options.
func New(opts ...Option) *SimpleCov {
	sc := &SimpleCov{
		CommandName:  "Unnamed",
		MergeTimeout: DefaultMergeTimeout,
		fs:           OSFS{},
		clock:        time.Now,
	}
	for _, o := range opts {
		o(sc)
	}
	return sc
}

// AddFilter registers a file-exclusion filter (SimpleCov.add_filter).
func (sc *SimpleCov) AddFilter(f Filter) { sc.filters = append(sc.filters, f) }

// AddStringFilter excludes files whose project path contains arg.
func (sc *SimpleCov) AddStringFilter(arg string) {
	sc.AddFilter(StringFilter{Arg: arg, Root: sc.Root})
}

// AddRegexpFilter excludes files whose filename matches re.
func (sc *SimpleCov) AddRegexpFilter(re *regexp.Regexp) { sc.AddFilter(RegexFilter{Re: re}) }

// AddBlockFilter excludes files for which fn returns true.
func (sc *SimpleCov) AddBlockFilter(fn func(*SourceFile) bool) { sc.AddFilter(BlockFilter{Fn: fn}) }

// AddGroup registers a named group selected by filter (SimpleCov.add_group).
func (sc *SimpleCov) AddGroup(name string, filter Filter) {
	sc.groups = append(sc.groups, namedFilter{name: name, filter: filter})
}

// AddStringGroup registers a group selecting files whose project path contains
// arg — the common SimpleCov.add_group "Name", "path" form.
func (sc *SimpleCov) AddStringGroup(name, arg string) {
	sc.AddGroup(name, StringFilter{Arg: arg, Root: sc.Root})
}

// MinimumCoverage sets the overall minimum for a criterion (minimum_coverage).
func (sc *SimpleCov) MinimumCoverage(c Criterion, pct float64) {
	if sc.check.MinimumCoverage == nil {
		sc.check.MinimumCoverage = map[Criterion]float64{}
	}
	sc.check.MinimumCoverage[c] = pct
}

// MinimumCoverageByFile sets the per-file minimum (minimum_coverage_by_file).
func (sc *SimpleCov) MinimumCoverageByFile(c Criterion, pct float64) {
	if sc.check.MinimumCoverageByFile == nil {
		sc.check.MinimumCoverageByFile = map[Criterion]float64{}
	}
	sc.check.MinimumCoverageByFile[c] = pct
}

// MaximumCoverageDrop sets the largest allowed drop vs the last run
// (maximum_coverage_drop).
func (sc *SimpleCov) MaximumCoverageDrop(c Criterion, pct float64) {
	if sc.check.MaximumCoverageDrop == nil {
		sc.check.MaximumCoverageDrop = map[Criterion]float64{}
	}
	sc.check.MaximumCoverageDrop[c] = pct
}

// RefuseCoverageDrop forbids any drop (drop threshold 0) for the given criteria,
// defaulting to Line when none are named (refuse_coverage_drop).
func (sc *SimpleCov) RefuseCoverageDrop(criteria ...Criterion) {
	if len(criteria) == 0 {
		criteria = []Criterion{Line}
	}
	for _, c := range criteria {
		sc.MaximumCoverageDrop(c, 0)
	}
}

// Check returns the configured thresholds.
func (sc *SimpleCov) Check() Check { return sc.check }

// RunChecks evaluates the configured thresholds (see Check.Run).
func (sc *SimpleCov) RunChecks(result, previous *Result) (ExitCode, []Violation) {
	return sc.check.Run(result, previous)
}

// Filtered returns a copy of fl with every file matching any exclusion filter
// removed (SimpleCov.filtered).
func (sc *SimpleCov) Filtered(fl *FileList) *FileList {
	var kept []*SourceFile
	for _, sf := range fl.files {
		excluded := false
		for _, f := range sc.filters {
			if f.Matches(sf) {
				excluded = true
				break
			}
		}
		if !excluded {
			kept = append(kept, sf)
		}
	}
	return NewFileList(kept...)
}

// Grouped partitions fl into the configured groups, in definition order,
// appending an "Ungrouped" bucket for files in no group. With no groups
// configured it returns nil, matching SimpleCov.grouped.
func (sc *SimpleCov) Grouped(fl *FileList) []NamedFileList {
	if len(sc.groups) == 0 {
		return nil
	}
	claimed := make(map[*SourceFile]bool)
	out := make([]NamedFileList, 0, len(sc.groups)+1)
	for _, g := range sc.groups {
		var sel []*SourceFile
		for _, sf := range fl.files {
			if g.filter.Matches(sf) {
				sel = append(sel, sf)
				claimed[sf] = true
			}
		}
		out = append(out, NamedFileList{Name: g.name, Files: NewFileList(sel...)})
	}
	var other []*SourceFile
	for _, sf := range fl.files {
		if !claimed[sf] {
			other = append(other, sf)
		}
	}
	if len(other) > 0 {
		out = append(out, NamedFileList{Name: "Ungrouped", Files: NewFileList(other...)})
	}
	return out
}

// NewResult builds a result from raw coverage: it constructs the source files,
// applies exclusion filters, groups the survivors, and stamps the result with
// the clock. An empty commandName falls back to the configured CommandName.
func (sc *SimpleCov) NewResult(commandName string, cov map[string]FileCoverage) *Result {
	if commandName == "" {
		commandName = sc.CommandName
	}
	fl := sc.Filtered(fileListFromCoverage(cov))
	return &Result{
		Files:       fl,
		CommandName: commandName,
		CreatedAt:   sc.clock(),
		groups:      sc.Grouped(fl),
	}
}

// LoadSource fills sf.Src by reading sf.Filename through the FS seam.
func (sc *SimpleCov) LoadSource(sf *SourceFile) error {
	data, err := sc.fs.ReadFile(sf.Filename)
	if err != nil {
		return err
	}
	sf.Src = splitSourceLines(data)
	return nil
}

// LoadResultset reads and parses a resultset file through the FS seam. A missing
// file is not an error: it yields an empty resultset, matching SimpleCov's
// treatment of a first run.
func (sc *SimpleCov) LoadResultset(path string) (Resultset, error) {
	data, err := sc.fs.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Resultset{}, nil
		}
		return nil, err
	}
	return ParseResultset(data)
}

// StoreResultset encodes rs and writes it to path through the FS seam.
func (sc *SimpleCov) StoreResultset(path string, rs Resultset) error {
	data, err := rs.JSON()
	if err != nil {
		return err
	}
	return sc.fs.WriteFile(path, data, 0o644)
}
