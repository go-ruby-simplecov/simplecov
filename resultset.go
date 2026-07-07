// Copyright (c) the go-ruby-simplecov/simplecov authors
//
// SPDX-License-Identifier: BSD-3-Clause

package simplecov

import (
	"bytes"
	"encoding/json"
	"sort"
	"time"
)

// Resultset is SimpleCov's .resultset.json shape: a map from command name to
// that command's coverage and timestamp, e.g.
//
//	{ "RSpec": { "coverage": { "/a.rb": {"lines":[null,1,0]} }, "timestamp": 172… } }
type Resultset map[string]CommandResult

// CommandResult is one command's entry in a resultset: the per-file coverage and
// the Unix timestamp at which it was recorded.
type CommandResult struct {
	Coverage  map[string]FileCoverage `json:"coverage"`
	Timestamp int64                   `json:"timestamp"`
}

// jsonMarshal is a seam so tests can force an encoding error; production uses the
// stdlib encoder.
var jsonMarshal = json.Marshal

// MarshalJSON emits a file's coverage in SimpleCov's current nested form,
// {"lines":[…]}, adding "branches" only when branch data is present.
func (fc FileCoverage) MarshalJSON() ([]byte, error) {
	out := struct {
		Lines    []Hit    `json:"lines"`
		Branches Branches `json:"branches,omitempty"`
	}{Lines: fc.Lines, Branches: fc.Branches}
	return jsonMarshal(out)
}

// UnmarshalJSON accepts both SimpleCov's current nested form ({"lines":[…]}) and
// the legacy bare-array form ([null,1,0]).
func (fc *FileCoverage) UnmarshalJSON(b []byte) error {
	trimmed := bytes.TrimSpace(b)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		return json.Unmarshal(trimmed, &fc.Lines)
	}
	var nested struct {
		Lines    []Hit    `json:"lines"`
		Branches Branches `json:"branches"`
	}
	if err := json.Unmarshal(b, &nested); err != nil {
		return err
	}
	fc.Lines = nested.Lines
	fc.Branches = nested.Branches
	return nil
}

// ParseResultset decodes .resultset.json bytes.
func ParseResultset(data []byte) (Resultset, error) {
	var rs Resultset
	if err := json.Unmarshal(data, &rs); err != nil {
		return nil, err
	}
	return rs, nil
}

// JSON encodes the resultset as .resultset.json bytes. Map keys are emitted in
// sorted order by the stdlib encoder, so output is deterministic.
func (rs Resultset) JSON() ([]byte, error) {
	return jsonMarshal(rs)
}

// Results turns a resultset into unfiltered results, one per command, ordered by
// command name for determinism.
func (rs Resultset) Results() []*Result {
	names := make([]string, 0, len(rs))
	for name := range rs {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]*Result, 0, len(names))
	for _, name := range names {
		out = append(out, resultFromCommand(name, rs[name]))
	}
	return out
}

// resultFromCommand builds a single unfiltered result from a command entry.
func resultFromCommand(name string, cr CommandResult) *Result {
	return NewResult(name, cr.Coverage, time.Unix(cr.Timestamp, 0))
}

// ToResultset serialises a result back into a single-command resultset, ready to
// merge into or replace an on-disk .resultset.json.
func (r *Result) ToResultset() Resultset {
	cov := make(map[string]FileCoverage, r.Files.Len())
	for _, sf := range r.Files.files {
		cov[sf.Filename] = FileCoverage{Lines: sf.Lines, Branches: sf.Branches}
	}
	return Resultset{r.CommandName: {Coverage: cov, Timestamp: r.CreatedAt.Unix()}}
}
