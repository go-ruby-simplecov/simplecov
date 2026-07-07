// Copyright (c) the go-ruby-simplecov/simplecov authors
//
// SPDX-License-Identifier: BSD-3-Clause

package simplecov

import (
	"strconv"
	"strings"
)

// Hit is one source line's coverage datum. SimpleCov stores each line as either
// an integer hit count (0 = executed zero times = missed, >0 = covered) or null,
// meaning the line is not coverable (blank, comment, structural). Valid==false
// models null; Valid==true carries Count.
type Hit struct {
	Valid bool
	Count int
}

// Coverable returns a coverable line with the given hit count (0 = missed).
func Coverable(count int) Hit { return Hit{Valid: true, Count: count} }

// Uncoverable returns a non-coverable line (null in the resultset).
func Uncoverable() Hit { return Hit{} }

// Covered reports whether the line is coverable and was hit at least once.
func (h Hit) Covered() bool { return h.Valid && h.Count > 0 }

// Missed reports whether the line is coverable and was never hit.
func (h Hit) Missed() bool { return h.Valid && h.Count == 0 }

// MarshalJSON encodes a coverable line as its integer count and a non-coverable
// line as JSON null, matching SimpleCov's resultset line arrays.
func (h Hit) MarshalJSON() ([]byte, error) {
	if !h.Valid {
		return []byte("null"), nil
	}
	return []byte(strconv.Itoa(h.Count)), nil
}

// UnmarshalJSON decodes a resultset line entry: null → non-coverable, an integer
// → coverable with that count.
func (h *Hit) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" {
		h.Valid = false
		h.Count = 0
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return err
	}
	h.Valid = true
	h.Count = n
	return nil
}
