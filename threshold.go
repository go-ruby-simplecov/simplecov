// Copyright (c) the go-ruby-simplecov/simplecov authors
//
// SPDX-License-Identifier: BSD-3-Clause

package simplecov

import "sort"

// Criterion names a coverage dimension SimpleCov can gate on.
type Criterion string

const (
	// Line gates on line coverage.
	Line Criterion = "line"
	// Branch gates on branch coverage.
	Branch Criterion = "branch"
)

// ExitCode mirrors SimpleCov::ExitCodes: the process exit status a coverage run
// yields.
type ExitCode int

const (
	// Success — all thresholds met.
	Success ExitCode = 0
	// Exception — an error occurred (reserved; parity with SimpleCov).
	Exception ExitCode = 1
	// MinimumCoverage — an overall or per-file minimum was not met.
	MinimumCoverage ExitCode = 2
	// MaximumCoverageDrop — coverage dropped more than allowed vs the last run.
	MaximumCoverageDrop ExitCode = 3
)

// ViolationKind classifies a threshold breach.
type ViolationKind int

const (
	// BelowMinimum — overall coverage below minimum_coverage.
	BelowMinimum ViolationKind = iota
	// BelowMinimumByFile — a file below minimum_coverage_by_file.
	BelowMinimumByFile
	// ExceededDrop — coverage dropped more than maximum_coverage_drop.
	ExceededDrop
)

// Violation records a single failed threshold check.
type Violation struct {
	Kind      ViolationKind
	Criterion Criterion
	File      string // set only for BelowMinimumByFile
	Actual    float64
	Expected  float64
}

// Check holds the configured coverage thresholds. Each map is keyed by criterion
// (Line/Branch); an absent criterion means "no gate on that dimension".
type Check struct {
	MinimumCoverage       map[Criterion]float64
	MinimumCoverageByFile map[Criterion]float64
	MaximumCoverageDrop   map[Criterion]float64
}

// Run evaluates the thresholds against result (and, for drop checks, the previous
// run — pass nil when there is none). It returns the exit code SimpleCov would
// use and every violation found, sorted deterministically. A minimum-coverage
// breach outranks a drop breach.
func (c Check) Run(result, previous *Result) (ExitCode, []Violation) {
	var vs []Violation

	for crit, min := range c.MinimumCoverage {
		if got := result.CoveredPercentFor(crit); got < min {
			vs = append(vs, Violation{Kind: BelowMinimum, Criterion: crit, Actual: got, Expected: min})
		}
	}
	for crit, min := range c.MinimumCoverageByFile {
		for _, sf := range result.Files.files {
			if got := sourceFilePercentFor(sf, crit); got < min {
				vs = append(vs, Violation{Kind: BelowMinimumByFile, Criterion: crit, File: sf.Filename, Actual: got, Expected: min})
			}
		}
	}
	if previous != nil {
		for crit, max := range c.MaximumCoverageDrop {
			drop := previous.CoveredPercentFor(crit) - result.CoveredPercentFor(crit)
			if drop > max {
				vs = append(vs, Violation{Kind: ExceededDrop, Criterion: crit, Actual: drop, Expected: max})
			}
		}
	}

	sort.Slice(vs, func(i, j int) bool {
		if vs[i].Kind != vs[j].Kind {
			return vs[i].Kind < vs[j].Kind
		}
		if vs[i].Criterion != vs[j].Criterion {
			return vs[i].Criterion < vs[j].Criterion
		}
		return vs[i].File < vs[j].File
	})

	code := Success
	for _, v := range vs {
		if v.Kind == BelowMinimum || v.Kind == BelowMinimumByFile {
			return MinimumCoverage, vs
		}
		if v.Kind == ExceededDrop {
			code = MaximumCoverageDrop
		}
	}
	return code, vs
}

func sourceFilePercentFor(s *SourceFile, c Criterion) float64 {
	if c == Branch {
		return s.Branches.CoveredPercent()
	}
	return s.CoveredPercent()
}
