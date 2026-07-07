// Copyright (c) the go-ruby-simplecov/simplecov authors
//
// SPDX-License-Identifier: BSD-3-Clause

package simplecov

import (
	"sort"
	"strings"
	"time"
)

// mergeHit combines two lines' coverage. Two non-coverable lines stay
// non-coverable; a non-coverable line yields the other; two coverable lines sum
// their counts. This matches SimpleCov's coverage merge.
func mergeHit(a, b Hit) Hit {
	switch {
	case !a.Valid && !b.Valid:
		return Hit{}
	case !a.Valid:
		return b
	case !b.Valid:
		return a
	default:
		return Hit{Valid: true, Count: a.Count + b.Count}
	}
}

// mergeLines merges two line arrays element-wise, tolerating differing lengths
// (a missing element is treated as non-coverable).
func mergeLines(a, b []Hit) []Hit {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	if n == 0 {
		return nil
	}
	out := make([]Hit, n)
	for i := range out {
		var ha, hb Hit
		if i < len(a) {
			ha = a[i]
		}
		if i < len(b) {
			hb = b[i]
		}
		out[i] = mergeHit(ha, hb)
	}
	return out
}

// MergeCoverage merges two files' coverage (lines and branches).
func MergeCoverage(a, b FileCoverage) FileCoverage {
	return FileCoverage{
		Lines:    mergeLines(a.Lines, b.Lines),
		Branches: mergeBranches(a.Branches, b.Branches),
	}
}

// MergeResults combines several results into one, summing coverage per file
// across the inputs. The merged command name is the sorted, de-duplicated set of
// input command names joined by ", " and the timestamp is the most recent input.
// Merging an empty slice yields an empty result.
func MergeResults(results []*Result) *Result {
	merged := make(map[string]FileCoverage)
	nameSet := make(map[string]struct{})
	var latest time.Time
	for _, r := range results {
		nameSet[r.CommandName] = struct{}{}
		if r.CreatedAt.After(latest) {
			latest = r.CreatedAt
		}
		for _, sf := range r.Files.files {
			fc := FileCoverage{Lines: sf.Lines, Branches: sf.Branches}
			if existing, ok := merged[sf.Filename]; ok {
				merged[sf.Filename] = MergeCoverage(existing, fc)
			} else {
				merged[sf.Filename] = fc
			}
		}
	}
	names := make([]string, 0, len(nameSet))
	for name := range nameSet {
		names = append(names, name)
	}
	sort.Strings(names)
	return NewResult(strings.Join(names, ", "), merged, latest)
}

// MergeStored merges the fresh commands in a resultset into a single result,
// dropping any command whose timestamp is older than the merge timeout relative
// to the injected clock — mirroring SimpleCov::ResultMerger's stale-result
// pruning. Commands are merged in command-name order for determinism.
func (sc *SimpleCov) MergeStored(rs Resultset) *Result {
	now := sc.clock()
	names := make([]string, 0, len(rs))
	for name := range rs {
		names = append(names, name)
	}
	sort.Strings(names)
	var kept []*Result
	for _, name := range names {
		cr := rs[name]
		if now.Sub(time.Unix(cr.Timestamp, 0)) < sc.MergeTimeout {
			kept = append(kept, resultFromCommand(name, cr))
		}
	}
	return MergeResults(kept)
}
