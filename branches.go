// Copyright (c) the go-ruby-simplecov/simplecov authors
//
// SPDX-License-Identifier: BSD-3-Clause

package simplecov

// Branches models SimpleCov's branch-coverage table for one file: a mapping from
// a condition key (e.g. "[:if, 0, 3, 6, 3, 21]") to that condition's branches
// (e.g. "[:then, 1, …]" and "[:else, 2, …]"), each carrying its hit count. Every
// inner entry is one branch; a branch with count 0 is missed, >0 is covered.
type Branches map[string]map[string]int

// Total is the number of distinct branches across every condition.
func (b Branches) Total() int {
	n := 0
	for _, cond := range b {
		n += len(cond)
	}
	return n
}

// Covered is the number of branches executed at least once.
func (b Branches) Covered() int {
	n := 0
	for _, cond := range b {
		for _, hits := range cond {
			if hits > 0 {
				n++
			}
		}
	}
	return n
}

// Missed is the number of branches never executed.
func (b Branches) Missed() int { return b.Total() - b.Covered() }

// CoveredPercent is covered/total * 100. A file with no branches is 100%.
func (b Branches) CoveredPercent() float64 {
	total := b.Total()
	if total == 0 {
		return 100.0
	}
	return float64(b.Covered()) * 100.0 / float64(total)
}

// merge returns the element-wise sum of two branch tables. A nil operand is
// treated as empty; the result is nil only when both are empty.
func mergeBranches(a, b Branches) Branches {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	out := make(Branches)
	for _, src := range []Branches{a, b} {
		for cond, branches := range src {
			dst := out[cond]
			if dst == nil {
				dst = make(map[string]int)
				out[cond] = dst
			}
			for branch, hits := range branches {
				dst[branch] += hits
			}
		}
	}
	return out
}
