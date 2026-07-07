// Copyright (c) the go-ruby-simplecov/simplecov authors
//
// SPDX-License-Identifier: BSD-3-Clause

package simplecov

import (
	"regexp"
	"strings"
)

// Filter decides whether a source file matches a rule. A filter added via
// AddFilter excludes matching files from a result; a filter attached to a group
// selects the files that belong to that group. This mirrors SimpleCov::Filter.
type Filter interface {
	Matches(*SourceFile) bool
}

// StringFilter matches when its argument is a substring of the file's
// project-relative path, mirroring SimpleCov::StringFilter (which compares
// against project_filename, i.e. the path with the root stripped).
type StringFilter struct {
	Arg  string
	Root string
}

// Matches reports whether Arg occurs in the file's project filename.
func (f StringFilter) Matches(s *SourceFile) bool {
	return strings.Contains(s.ProjectFilename(f.Root), f.Arg)
}

// RegexFilter matches when its pattern matches the file's absolute filename,
// mirroring SimpleCov::RegexFilter.
type RegexFilter struct {
	Re *regexp.Regexp
}

// Matches reports whether the pattern matches the file's filename.
func (f RegexFilter) Matches(s *SourceFile) bool {
	return f.Re.MatchString(s.Filename)
}

// BlockFilter matches when its function returns true for the file, mirroring
// SimpleCov::BlockFilter (add_filter { |source_file| … }).
type BlockFilter struct {
	Fn func(*SourceFile) bool
}

// Matches delegates to the block.
func (f BlockFilter) Matches(s *SourceFile) bool { return f.Fn(s) }

// ArrayFilter matches when any of its member filters match, mirroring
// SimpleCov::ArrayFilter.
type ArrayFilter struct {
	Filters []Filter
}

// Matches reports whether any member filter matches.
func (f ArrayFilter) Matches(s *SourceFile) bool {
	for _, m := range f.Filters {
		if m.Matches(s) {
			return true
		}
	}
	return false
}
