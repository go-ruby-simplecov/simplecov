// Copyright (c) the go-ruby-simplecov/simplecov authors
//
// SPDX-License-Identifier: BSD-3-Clause

package simplecov

import (
	"math"
	"strings"
)

// SourceFile is SimpleCov::SourceFile: one covered file's line and branch data,
// plus optional source text. Filename is absolute (as SimpleCov stores it); Src,
// when loaded through the FS seam, holds the file's lines for rendering.
type SourceFile struct {
	Filename string
	Lines    []Hit
	Branches Branches
	Src      []string
}

// CoveredLinesCount is the number of coverable lines hit at least once.
func (s *SourceFile) CoveredLinesCount() int {
	n := 0
	for _, h := range s.Lines {
		if h.Covered() {
			n++
		}
	}
	return n
}

// MissedLinesCount is the number of coverable lines never hit.
func (s *SourceFile) MissedLinesCount() int {
	n := 0
	for _, h := range s.Lines {
		if h.Missed() {
			n++
		}
	}
	return n
}

// NeverLinesCount is the number of non-coverable (null) lines.
func (s *SourceFile) NeverLinesCount() int {
	n := 0
	for _, h := range s.Lines {
		if !h.Valid {
			n++
		}
	}
	return n
}

// RelevantLines is the number of coverable lines (covered + missed); SimpleCov's
// lines_of_code.
func (s *SourceFile) RelevantLines() int {
	return s.CoveredLinesCount() + s.MissedLinesCount()
}

// LinesOfCode is an alias for RelevantLines, matching SimpleCov's terminology.
func (s *SourceFile) LinesOfCode() int { return s.RelevantLines() }

// LinesStrength is the sum of hit counts over coverable lines.
func (s *SourceFile) LinesStrength() int {
	sum := 0
	for _, h := range s.Lines {
		if h.Valid {
			sum += h.Count
		}
	}
	return sum
}

// CoveredPercent is covered/relevant * 100. A file with no coverable lines is
// reported as 100%, matching SimpleCov's no_lines? shortcut.
func (s *SourceFile) CoveredPercent() float64 {
	relevant := s.RelevantLines()
	if relevant == 0 {
		return 100.0
	}
	return float64(s.CoveredLinesCount()) * 100.0 / float64(relevant)
}

// CoveredStrength is the average hit count over coverable lines, rounded to one
// decimal (SimpleCov's covered_strength). Zero when there are no coverable lines.
func (s *SourceFile) CoveredStrength() float64 {
	relevant := s.RelevantLines()
	if relevant == 0 {
		return 0.0
	}
	return roundFloat(float64(s.LinesStrength())/float64(relevant), 1)
}

// TotalBranches is the file's branch count.
func (s *SourceFile) TotalBranches() int { return s.Branches.Total() }

// CoveredBranches is the number of executed branches.
func (s *SourceFile) CoveredBranches() int { return s.Branches.Covered() }

// MissedBranches is the number of never-executed branches.
func (s *SourceFile) MissedBranches() int { return s.Branches.Missed() }

// BranchesCoveredPercent is covered/total branches * 100 (100 with no branches).
func (s *SourceFile) BranchesCoveredPercent() float64 { return s.Branches.CoveredPercent() }

// ProjectFilename is Filename with a leading root prefix stripped, mirroring
// SimpleCov's project_filename used by string filters and groups.
func (s *SourceFile) ProjectFilename(root string) string {
	if root != "" && strings.HasPrefix(s.Filename, root) {
		return strings.TrimPrefix(s.Filename, root)
	}
	return s.Filename
}

func roundFloat(v float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(v*p) / p
}
