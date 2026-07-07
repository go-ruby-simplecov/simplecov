// Copyright (c) the go-ruby-simplecov/simplecov authors
//
// SPDX-License-Identifier: BSD-3-Clause

package simplecov

import (
	"fmt"
	"strings"
)

// Formatter renders a result to text. It is the seam SimpleCov's formatter
// registry fills; the HTML formatter is out of scope, but any consumer can
// implement this interface.
type Formatter interface {
	Format(*Result) string
}

// SimpleFormatter reproduces SimpleCov::Formatter::SimpleFormatter: for each
// group it prints the group name, a rule, then one line per file with its
// coverage percentage. As in SimpleCov, a result with no configured groups
// produces no output.
type SimpleFormatter struct{}

// Format renders the result.
func (SimpleFormatter) Format(r *Result) string {
	var b strings.Builder
	for _, g := range r.Groups() {
		fmt.Fprintf(&b, "Group: %s\n", g.Name)
		b.WriteString(strings.Repeat("=", 40))
		b.WriteByte('\n')
		for _, f := range g.Files.files {
			fmt.Fprintf(&b, "%s (coverage: %.2f%%)\n", f.Filename, f.CoveredPercent())
		}
	}
	return b.String()
}
