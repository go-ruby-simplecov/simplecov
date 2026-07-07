// Copyright (c) the go-ruby-simplecov/simplecov authors
//
// SPDX-License-Identifier: BSD-3-Clause

package simplecov

// FileList is SimpleCov::FileList: an ordered collection of SourceFile with
// aggregate coverage metrics.
type FileList struct {
	files []*SourceFile
}

// NewFileList builds a FileList over the given source files (order preserved).
func NewFileList(files ...*SourceFile) *FileList {
	return &FileList{files: files}
}

// Len is the number of files.
func (fl *FileList) Len() int { return len(fl.files) }

// Files returns the underlying source files (do not mutate).
func (fl *FileList) Files() []*SourceFile { return fl.files }

// CoveredLinesCount is the total covered coverable lines across all files.
func (fl *FileList) CoveredLinesCount() int {
	n := 0
	for _, f := range fl.files {
		n += f.CoveredLinesCount()
	}
	return n
}

// MissedLinesCount is the total missed coverable lines across all files.
func (fl *FileList) MissedLinesCount() int {
	n := 0
	for _, f := range fl.files {
		n += f.MissedLinesCount()
	}
	return n
}

// NeverLinesCount is the total non-coverable lines across all files.
func (fl *FileList) NeverLinesCount() int {
	n := 0
	for _, f := range fl.files {
		n += f.NeverLinesCount()
	}
	return n
}

// LinesOfCode is the total coverable lines across all files.
func (fl *FileList) LinesOfCode() int {
	n := 0
	for _, f := range fl.files {
		n += f.RelevantLines()
	}
	return n
}

// CoveredPercent is total covered / total coverable * 100. An empty list, or one
// with no coverable lines, is 100%.
func (fl *FileList) CoveredPercent() float64 {
	loc := fl.LinesOfCode()
	if loc == 0 {
		return 100.0
	}
	return float64(fl.CoveredLinesCount()) * 100.0 / float64(loc)
}

// CoveredStrength is the average hit count over all coverable lines, rounded to
// one decimal. Zero for an empty list or one with no coverable lines.
func (fl *FileList) CoveredStrength() float64 {
	loc := fl.LinesOfCode()
	if loc == 0 {
		return 0.0
	}
	strength := 0
	for _, f := range fl.files {
		strength += f.LinesStrength()
	}
	return roundFloat(float64(strength)/float64(loc), 1)
}

// CoveredPercentages is each file's CoveredPercent, in file order.
func (fl *FileList) CoveredPercentages() []float64 {
	out := make([]float64, len(fl.files))
	for i, f := range fl.files {
		out[i] = f.CoveredPercent()
	}
	return out
}

// LeastCoveredFile is the file with the lowest CoveredPercent, or nil if empty.
func (fl *FileList) LeastCoveredFile() *SourceFile {
	if len(fl.files) == 0 {
		return nil
	}
	least := fl.files[0]
	for _, f := range fl.files[1:] {
		if f.CoveredPercent() < least.CoveredPercent() {
			least = f
		}
	}
	return least
}

// TotalBranches is the total branch count across all files.
func (fl *FileList) TotalBranches() int {
	n := 0
	for _, f := range fl.files {
		n += f.TotalBranches()
	}
	return n
}

// CoveredBranches is the total executed branches across all files.
func (fl *FileList) CoveredBranches() int {
	n := 0
	for _, f := range fl.files {
		n += f.CoveredBranches()
	}
	return n
}

// MissedBranches is the total never-executed branches across all files.
func (fl *FileList) MissedBranches() int { return fl.TotalBranches() - fl.CoveredBranches() }

// BranchesCoveredPercent is total covered / total branches * 100 (100 with no
// branches).
func (fl *FileList) BranchesCoveredPercent() float64 {
	total := fl.TotalBranches()
	if total == 0 {
		return 100.0
	}
	return float64(fl.CoveredBranches()) * 100.0 / float64(total)
}
