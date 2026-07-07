// Copyright (c) the go-ruby-simplecov/simplecov authors
//
// SPDX-License-Identifier: BSD-3-Clause

package simplecov

import (
	"io/fs"
	"os"
	"strings"
)

// FS is the filesystem seam: resultset load/store and source loading go through
// it, so tests can inject a deterministic, in-memory filesystem and exercise
// error branches. The default implementation ([OSFS]) is os-backed.
type FS interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
}

// OSFS is the production FS, backed by the os package.
type OSFS struct{}

// ReadFile reads a file from disk.
func (OSFS) ReadFile(name string) ([]byte, error) { return os.ReadFile(name) }

// WriteFile writes a file to disk.
func (OSFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(name, data, perm)
}

// splitSourceLines splits file contents into lines, dropping a single trailing
// newline so a file of N text lines yields N entries.
func splitSourceLines(data []byte) []string {
	s := strings.TrimSuffix(string(data), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
