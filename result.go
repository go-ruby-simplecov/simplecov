// Copyright (c) the go-ruby-simplecov/simplecov authors
//
// SPDX-License-Identifier: BSD-3-Clause

package simplecov

import (
	"sort"
	"time"
)

// FileCoverage is the raw per-file coverage the host (rbgo's VM) feeds in: one
// Hit per source line and, optionally, branch coverage. It is also the on-disk
// shape inside a resultset's "coverage" map.
type FileCoverage struct {
	Lines    []Hit
	Branches Branches
}

// NamedFileList is a group name paired with its files, as produced by grouping.
type NamedFileList struct {
	Name  string
	Files *FileList
}

// Result is SimpleCov::Result: a named, timestamped, filtered and grouped view
// over a set of source files.
type Result struct {
	Files       *FileList
	CommandName string
	CreatedAt   time.Time
	groups      []NamedFileList
}

// fileListFromCoverage builds source files from raw coverage, ordered by
// filename for deterministic output.
func fileListFromCoverage(cov map[string]FileCoverage) *FileList {
	names := make([]string, 0, len(cov))
	for name := range cov {
		names = append(names, name)
	}
	sort.Strings(names)
	files := make([]*SourceFile, 0, len(names))
	for _, name := range names {
		fc := cov[name]
		files = append(files, &SourceFile{
			Filename: name,
			Lines:    fc.Lines,
			Branches: fc.Branches,
		})
	}
	return NewFileList(files...)
}

// NewResult builds an unfiltered, ungrouped result directly from raw coverage,
// stamped at t. Use (*SimpleCov).NewResult to also apply filters and groups.
func NewResult(commandName string, cov map[string]FileCoverage, t time.Time) *Result {
	return &Result{
		Files:       fileListFromCoverage(cov),
		CommandName: commandName,
		CreatedAt:   t,
	}
}

// Groups returns the result's named groups in definition order (empty when no
// groups were configured), matching SimpleCov::Result#groups.
func (r *Result) Groups() []NamedFileList { return r.groups }

// CoveredPercent is the result's overall line coverage percentage.
func (r *Result) CoveredPercent() float64 { return r.Files.CoveredPercent() }

// CoveredStrength is the result's overall average hit count per coverable line.
func (r *Result) CoveredStrength() float64 { return r.Files.CoveredStrength() }

// CoveredLines is the total covered coverable lines.
func (r *Result) CoveredLines() int { return r.Files.CoveredLinesCount() }

// MissedLines is the total missed coverable lines.
func (r *Result) MissedLines() int { return r.Files.MissedLinesCount() }

// TotalLines is the total coverable lines (covered + missed).
func (r *Result) TotalLines() int { return r.Files.LinesOfCode() }

// BranchesCoveredPercent is the result's overall branch coverage percentage.
func (r *Result) BranchesCoveredPercent() float64 { return r.Files.BranchesCoveredPercent() }

// LeastCoveredFile is the least-covered file, or nil when there are none.
func (r *Result) LeastCoveredFile() *SourceFile { return r.Files.LeastCoveredFile() }

// CoveredPercentFor returns the result's coverage percentage for the criterion.
func (r *Result) CoveredPercentFor(c Criterion) float64 {
	if c == Branch {
		return r.Files.BranchesCoveredPercent()
	}
	return r.Files.CoveredPercent()
}
