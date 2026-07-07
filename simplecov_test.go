// Copyright (c) the go-ruby-simplecov/simplecov authors
//
// SPDX-License-Identifier: BSD-3-Clause

package simplecov

import (
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

// --- helpers ---------------------------------------------------------------

func approx(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// fakeFS is an in-memory FS with injectable read/write errors.
type fakeFS struct {
	files    map[string][]byte
	readErr  map[string]error
	writeErr error
	written  map[string][]byte
}

func (f *fakeFS) ReadFile(name string) ([]byte, error) {
	if f.readErr != nil {
		if e, ok := f.readErr[name]; ok {
			return nil, e
		}
	}
	if d, ok := f.files[name]; ok {
		return d, nil
	}
	return nil, fs.ErrNotExist
}

func (f *fakeFS) WriteFile(name string, data []byte, _ fs.FileMode) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	if f.written == nil {
		f.written = map[string][]byte{}
	}
	f.written[name] = append([]byte(nil), data...)
	return nil
}

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

// --- Hit -------------------------------------------------------------------

func TestHit(t *testing.T) {
	if h := Coverable(3); !h.Valid || h.Count != 3 || !h.Covered() || h.Missed() {
		t.Fatalf("Coverable(3) wrong: %+v", h)
	}
	if h := Coverable(0); !h.Missed() || h.Covered() {
		t.Fatalf("Coverable(0) should be missed: %+v", h)
	}
	if h := Uncoverable(); h.Valid || h.Covered() || h.Missed() {
		t.Fatalf("Uncoverable wrong: %+v", h)
	}
}

func TestHitMarshalJSON(t *testing.T) {
	b, err := Coverable(5).MarshalJSON()
	if err != nil || string(b) != "5" {
		t.Fatalf("covered marshal: %q %v", b, err)
	}
	b, err = Uncoverable().MarshalJSON()
	if err != nil || string(b) != "null" {
		t.Fatalf("null marshal: %q %v", b, err)
	}
}

func TestHitUnmarshalJSON(t *testing.T) {
	var h Hit
	if err := h.UnmarshalJSON([]byte("null")); err != nil || h.Valid {
		t.Fatalf("null: %+v %v", h, err)
	}
	if err := h.UnmarshalJSON([]byte(" 7 ")); err != nil || !h.Valid || h.Count != 7 {
		t.Fatalf("int: %+v %v", h, err)
	}
	if err := h.UnmarshalJSON([]byte("1.5")); err == nil {
		t.Fatalf("expected error on non-integer")
	}
}

// --- Branches --------------------------------------------------------------

func sampleBranches() Branches {
	return Branches{
		"[:if, 0, 1, 0, 1, 10]": {"[:then, 1, …]": 3, "[:else, 2, …]": 0},
		"[:if, 3, 4, 0, 4, 10]": {"[:then, 4, …]": 1},
	}
}

func TestBranches(t *testing.T) {
	b := sampleBranches()
	if b.Total() != 3 {
		t.Fatalf("total=%d", b.Total())
	}
	if b.Covered() != 2 {
		t.Fatalf("covered=%d", b.Covered())
	}
	if b.Missed() != 1 {
		t.Fatalf("missed=%d", b.Missed())
	}
	approx(t, b.CoveredPercent(), 2.0/3.0*100)
	approx(t, Branches(nil).CoveredPercent(), 100.0)
}

func TestMergeBranches(t *testing.T) {
	if got := mergeBranches(nil, nil); got != nil {
		t.Fatalf("both empty should be nil, got %v", got)
	}
	a := Branches{"c": {"x": 1}}
	b := Branches{"c": {"x": 2, "y": 5}, "d": {"z": 4}}
	got := mergeBranches(a, b)
	want := Branches{"c": {"x": 3, "y": 5}, "d": {"z": 4}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merge got %v want %v", got, want)
	}
	// one side nil branch present
	if got := mergeBranches(a, nil); !reflect.DeepEqual(got, Branches{"c": {"x": 1}}) {
		t.Fatalf("merge with nil: %v", got)
	}
}

// --- SourceFile ------------------------------------------------------------

func sampleFile() *SourceFile {
	return &SourceFile{
		Filename: "/proj/app/models/user.rb",
		Lines:    []Hit{Uncoverable(), Coverable(1), Coverable(0), Coverable(2)},
		Branches: sampleBranches(),
	}
}

func TestSourceFileLineMetrics(t *testing.T) {
	s := sampleFile()
	if s.CoveredLinesCount() != 2 || s.MissedLinesCount() != 1 || s.NeverLinesCount() != 1 {
		t.Fatalf("counts wrong: %d %d %d", s.CoveredLinesCount(), s.MissedLinesCount(), s.NeverLinesCount())
	}
	if s.RelevantLines() != 3 || s.LinesOfCode() != 3 {
		t.Fatalf("relevant=%d loc=%d", s.RelevantLines(), s.LinesOfCode())
	}
	if s.LinesStrength() != 3 {
		t.Fatalf("strength sum=%d", s.LinesStrength())
	}
	approx(t, s.CoveredPercent(), 2.0/3.0*100)
	approx(t, s.CoveredStrength(), 1.0)
}

func TestSourceFileNoLines(t *testing.T) {
	s := &SourceFile{Lines: []Hit{Uncoverable(), Uncoverable()}}
	approx(t, s.CoveredPercent(), 100.0)
	approx(t, s.CoveredStrength(), 0.0)
}

func TestSourceFileBranchDelegation(t *testing.T) {
	s := sampleFile()
	if s.TotalBranches() != 3 || s.CoveredBranches() != 2 || s.MissedBranches() != 1 {
		t.Fatalf("branch counts wrong")
	}
	approx(t, s.BranchesCoveredPercent(), 2.0/3.0*100)
}

func TestProjectFilename(t *testing.T) {
	s := &SourceFile{Filename: "/proj/app/x.rb"}
	if got := s.ProjectFilename(""); got != "/proj/app/x.rb" {
		t.Fatalf("empty root: %q", got)
	}
	if got := s.ProjectFilename("/proj"); got != "/app/x.rb" {
		t.Fatalf("prefix: %q", got)
	}
	if got := s.ProjectFilename("/other"); got != "/proj/app/x.rb" {
		t.Fatalf("no prefix: %q", got)
	}
}

// --- FileList --------------------------------------------------------------

func sampleFileList() *FileList {
	a := &SourceFile{Filename: "/a.rb", Lines: []Hit{Coverable(1), Coverable(0)}, Branches: Branches{"c": {"x": 1, "y": 0}}}
	b := &SourceFile{Filename: "/b.rb", Lines: []Hit{Coverable(2), Coverable(2), Uncoverable()}}
	return NewFileList(a, b)
}

func TestFileListAggregates(t *testing.T) {
	fl := sampleFileList()
	if fl.Len() != 2 {
		t.Fatalf("len=%d", fl.Len())
	}
	if len(fl.Files()) != 2 {
		t.Fatalf("files len")
	}
	if fl.CoveredLinesCount() != 3 || fl.MissedLinesCount() != 1 || fl.NeverLinesCount() != 1 {
		t.Fatalf("line counts: %d %d %d", fl.CoveredLinesCount(), fl.MissedLinesCount(), fl.NeverLinesCount())
	}
	if fl.LinesOfCode() != 4 {
		t.Fatalf("loc=%d", fl.LinesOfCode())
	}
	approx(t, fl.CoveredPercent(), 75.0)
	approx(t, fl.CoveredStrength(), roundFloat(5.0/4.0, 1))
	if got := fl.CoveredPercentages(); len(got) != 2 {
		t.Fatalf("percentages len %d", len(got))
	}
	if fl.TotalBranches() != 2 || fl.CoveredBranches() != 1 || fl.MissedBranches() != 1 {
		t.Fatalf("branch aggregates wrong")
	}
	approx(t, fl.BranchesCoveredPercent(), 50.0)
}

func TestFileListEmpty(t *testing.T) {
	fl := NewFileList()
	approx(t, fl.CoveredPercent(), 100.0)
	approx(t, fl.CoveredStrength(), 0.0)
	approx(t, fl.BranchesCoveredPercent(), 100.0)
	if fl.LeastCoveredFile() != nil {
		t.Fatalf("empty least should be nil")
	}
}

func TestLeastCoveredFile(t *testing.T) {
	high := &SourceFile{Filename: "/h.rb", Lines: []Hit{Coverable(1), Coverable(1)}} // 100%
	low := &SourceFile{Filename: "/l.rb", Lines: []Hit{Coverable(1), Coverable(0)}}  // 50%
	// low is second -> exercises the "later file lower" branch
	if got := NewFileList(high, low).LeastCoveredFile(); got != low {
		t.Fatalf("expected low, got %v", got.Filename)
	}
	// low is first -> exercises the "keep current least" branch
	if got := NewFileList(low, high).LeastCoveredFile(); got != low {
		t.Fatalf("expected low kept, got %v", got.Filename)
	}
}

// --- Filters ---------------------------------------------------------------

func TestFilters(t *testing.T) {
	s := &SourceFile{Filename: "/proj/spec/user_spec.rb"}

	sf := StringFilter{Arg: "/spec/", Root: "/proj"}
	if !sf.Matches(s) {
		t.Fatalf("string filter should match")
	}
	if (StringFilter{Arg: "/nope/"}).Matches(s) {
		t.Fatalf("string filter should not match")
	}

	rf := RegexFilter{Re: regexp.MustCompile(`_spec\.rb$`)}
	if !rf.Matches(s) {
		t.Fatalf("regex should match")
	}
	if (RegexFilter{Re: regexp.MustCompile(`^/nope`)}).Matches(s) {
		t.Fatalf("regex should not match")
	}

	bf := BlockFilter{Fn: func(x *SourceFile) bool { return strings.HasSuffix(x.Filename, ".rb") }}
	if !bf.Matches(s) {
		t.Fatalf("block should match")
	}

	af := ArrayFilter{Filters: []Filter{rf, sf}}
	if !af.Matches(s) {
		t.Fatalf("array should match (any)")
	}
	if (ArrayFilter{Filters: []Filter{RegexFilter{Re: regexp.MustCompile(`zzz`)}}}).Matches(s) {
		t.Fatalf("array should not match")
	}
	if (ArrayFilter{}).Matches(s) {
		t.Fatalf("empty array should not match")
	}
}

// --- Result ----------------------------------------------------------------

func TestNewResultOrderingAndMetrics(t *testing.T) {
	cov := map[string]FileCoverage{
		"/b.rb": {Lines: []Hit{Coverable(1)}},
		"/a.rb": {Lines: []Hit{Coverable(0)}},
	}
	now := time.Unix(1000, 0)
	r := NewResult("RSpec", cov, now)
	if r.CommandName != "RSpec" || !r.CreatedAt.Equal(now) {
		t.Fatalf("result meta wrong")
	}
	files := r.Files.Files()
	if files[0].Filename != "/a.rb" || files[1].Filename != "/b.rb" {
		t.Fatalf("files not sorted: %v %v", files[0].Filename, files[1].Filename)
	}
	if r.Groups() != nil {
		t.Fatalf("no groups expected")
	}
	approx(t, r.CoveredPercent(), 50.0)
	approx(t, r.CoveredStrength(), 0.5)
	if r.CoveredLines() != 1 || r.MissedLines() != 1 || r.TotalLines() != 2 {
		t.Fatalf("line totals wrong")
	}
	approx(t, r.BranchesCoveredPercent(), 100.0)
	if r.LeastCoveredFile().Filename != "/a.rb" {
		t.Fatalf("least covered wrong")
	}
	approx(t, r.CoveredPercentFor(Line), 50.0)
	approx(t, r.CoveredPercentFor(Branch), 100.0)
}

// --- Resultset -------------------------------------------------------------

func TestResultsetRoundTrip(t *testing.T) {
	data := []byte(`{
      "RSpec": {
        "coverage": {
          "/a.rb": {"lines": [null, 1, 0], "branches": {"[:if,0]": {"[:then,1]": 2, "[:else,2]": 0}}}
        },
        "timestamp": 1600000000
      }
    }`)
	rs, err := ParseResultset(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	cr := rs["RSpec"]
	if cr.Timestamp != 1600000000 {
		t.Fatalf("timestamp=%d", cr.Timestamp)
	}
	fc := cr.Coverage["/a.rb"]
	if len(fc.Lines) != 3 || fc.Lines[0].Valid || fc.Lines[1].Count != 1 {
		t.Fatalf("lines wrong: %+v", fc.Lines)
	}
	if fc.Branches.Total() != 2 || fc.Branches.Covered() != 1 {
		t.Fatalf("branches wrong")
	}

	out, err := rs.JSON()
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	rs2, err := ParseResultset(out)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if !reflect.DeepEqual(rs, rs2) {
		t.Fatalf("round-trip mismatch:\n%v\n%v", rs, rs2)
	}
}

func TestResultsetLegacyArrayForm(t *testing.T) {
	rs, err := ParseResultset([]byte(`{"C": {"coverage": {"/a.rb": [null, 2, 0]}, "timestamp": 5}}`))
	if err != nil {
		t.Fatalf("parse legacy: %v", err)
	}
	fc := rs["C"].Coverage["/a.rb"]
	if len(fc.Lines) != 3 || fc.Lines[1].Count != 2 || fc.Branches != nil {
		t.Fatalf("legacy parse wrong: %+v", fc)
	}
}

func TestResultsetParseErrors(t *testing.T) {
	if _, err := ParseResultset([]byte("{not json")); err == nil {
		t.Fatalf("expected parse error")
	}
	// nested unmarshal error path in FileCoverage.UnmarshalJSON
	if _, err := ParseResultset([]byte(`{"C":{"coverage":{"/a.rb":{"lines":123}},"timestamp":1}}`)); err == nil {
		t.Fatalf("expected nested lines error")
	}
}

func TestFileCoverageMarshalBranchesOmitted(t *testing.T) {
	b, err := (FileCoverage{Lines: []Hit{Coverable(1)}}).MarshalJSON()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "branches") {
		t.Fatalf("branches should be omitted: %s", b)
	}
}

func TestResultsetMarshalErrorSeam(t *testing.T) {
	orig := jsonMarshal
	jsonMarshal = func(any) ([]byte, error) { return nil, errors.New("boom") }
	defer func() { jsonMarshal = orig }()

	if _, err := (Resultset{}).JSON(); err == nil {
		t.Fatalf("expected JSON error via seam")
	}
	if _, err := (FileCoverage{}).MarshalJSON(); err == nil {
		t.Fatalf("expected FileCoverage marshal error via seam")
	}
}

func TestResultsetResultsAndToResultset(t *testing.T) {
	rs := Resultset{
		"B": {Coverage: map[string]FileCoverage{"/x.rb": {Lines: []Hit{Coverable(1)}}}, Timestamp: 20},
		"A": {Coverage: map[string]FileCoverage{"/y.rb": {Lines: []Hit{Coverable(0)}}}, Timestamp: 10},
	}
	results := rs.Results()
	if len(results) != 2 || results[0].CommandName != "A" || results[1].CommandName != "B" {
		t.Fatalf("results order wrong")
	}
	if !results[0].CreatedAt.Equal(time.Unix(10, 0)) {
		t.Fatalf("timestamp not restored")
	}

	back := results[1].ToResultset()
	if _, ok := back["B"]; !ok {
		t.Fatalf("ToResultset missing command")
	}
	if back["B"].Timestamp != 20 {
		t.Fatalf("ToResultset timestamp wrong")
	}
}

// --- Merge -----------------------------------------------------------------

func TestMergeHitAndLines(t *testing.T) {
	if got := mergeHit(Uncoverable(), Uncoverable()); got.Valid {
		t.Fatalf("both null should stay null")
	}
	if got := mergeHit(Uncoverable(), Coverable(3)); got != Coverable(3) {
		t.Fatalf("null+n wrong: %+v", got)
	}
	if got := mergeHit(Coverable(2), Uncoverable()); got != Coverable(2) {
		t.Fatalf("n+null wrong: %+v", got)
	}
	if got := mergeHit(Coverable(2), Coverable(3)); got != Coverable(5) {
		t.Fatalf("sum wrong: %+v", got)
	}

	if got := mergeLines(nil, nil); got != nil {
		t.Fatalf("empty merge should be nil")
	}
	got := mergeLines([]Hit{Coverable(1), Coverable(1)}, []Hit{Coverable(2)})
	if len(got) != 2 || got[0] != Coverable(3) || got[1] != Coverable(1) {
		t.Fatalf("a longer: %+v", got)
	}
	got = mergeLines([]Hit{Coverable(1)}, []Hit{Coverable(2), Coverable(4)})
	if len(got) != 2 || got[0] != Coverable(3) || got[1] != Coverable(4) {
		t.Fatalf("b longer: %+v", got)
	}
}

func TestMergeCoverageAndResults(t *testing.T) {
	if got := MergeResults(nil); got.Files.Len() != 0 || got.CommandName != "" {
		t.Fatalf("empty merge wrong: %+v", got)
	}

	r1 := NewResult("Unit", map[string]FileCoverage{
		"/a.rb": {Lines: []Hit{Coverable(1), Coverable(0)}},
	}, time.Unix(100, 0))
	r2 := NewResult("Cucumber", map[string]FileCoverage{
		"/a.rb": {Lines: []Hit{Coverable(1), Coverable(1)}},
		"/b.rb": {Lines: []Hit{Coverable(5)}},
	}, time.Unix(200, 0))

	m := MergeResults([]*Result{r1, r2})
	if m.CommandName != "Cucumber, Unit" {
		t.Fatalf("merged name: %q", m.CommandName)
	}
	if !m.CreatedAt.Equal(time.Unix(200, 0)) {
		t.Fatalf("latest timestamp wrong")
	}
	files := m.Files.Files()
	if files[0].Filename != "/a.rb" || files[0].Lines[0] != Coverable(2) || files[0].Lines[1] != Coverable(1) {
		t.Fatalf("merged /a.rb wrong: %+v", files[0].Lines)
	}
	if MergeCoverage(FileCoverage{}, FileCoverage{}).Lines != nil {
		t.Fatalf("empty coverage merge")
	}
}

func TestMergeStoredStaleTimeout(t *testing.T) {
	now := time.Unix(10000, 0)
	sc := New(WithClock(fixedClock(now)), WithMergeTimeout(600*time.Second))
	rs := Resultset{
		"Stale": {Coverage: map[string]FileCoverage{"/old.rb": {Lines: []Hit{Coverable(1)}}}, Timestamp: now.Add(-1000 * time.Second).Unix()},
		"Fresh": {Coverage: map[string]FileCoverage{"/new.rb": {Lines: []Hit{Coverable(1)}}}, Timestamp: now.Add(-100 * time.Second).Unix()},
	}
	m := sc.MergeStored(rs)
	if m.CommandName != "Fresh" {
		t.Fatalf("stale not pruned, name=%q", m.CommandName)
	}
	if m.Files.Len() != 1 || m.Files.Files()[0].Filename != "/new.rb" {
		t.Fatalf("wrong file survived")
	}
}

// --- Thresholds ------------------------------------------------------------

func result50() *Result {
	return NewResult("X", map[string]FileCoverage{
		"/a.rb": {Lines: []Hit{Coverable(1), Coverable(0)}, Branches: Branches{"c": {"x": 1, "y": 0}}},
	}, time.Unix(1, 0))
}

func TestCheckMinimumCoverage(t *testing.T) {
	c := Check{MinimumCoverage: map[Criterion]float64{Line: 90}}
	code, vs := c.Run(result50(), nil)
	if code != MinimumCoverage || len(vs) != 1 || vs[0].Kind != BelowMinimum {
		t.Fatalf("expected min violation, got %v %+v", code, vs)
	}
	// met
	c = Check{MinimumCoverage: map[Criterion]float64{Line: 40}}
	if code, vs := c.Run(result50(), nil); code != Success || len(vs) != 0 {
		t.Fatalf("expected success, got %v %+v", code, vs)
	}
}

func TestCheckMinimumByFileLineAndBranch(t *testing.T) {
	c := Check{MinimumCoverageByFile: map[Criterion]float64{Line: 90, Branch: 90}}
	code, vs := c.Run(result50(), nil)
	if code != MinimumCoverage || len(vs) != 2 {
		t.Fatalf("expected 2 by-file violations: %v %+v", code, vs)
	}
	// branch first after sort by criterion string ("branch" < "line")
	if vs[0].Criterion != Branch || vs[1].Criterion != Line {
		t.Fatalf("sort order wrong: %+v", vs)
	}
	// met
	c = Check{MinimumCoverageByFile: map[Criterion]float64{Line: 10}}
	if code, _ := c.Run(result50(), nil); code != Success {
		t.Fatalf("by-file should pass")
	}
}

func TestCheckDrop(t *testing.T) {
	prev := NewResult("X", map[string]FileCoverage{"/a.rb": {Lines: []Hit{Coverable(1), Coverable(1)}}}, time.Unix(1, 0)) // 100%
	cur := result50()                                                                                                     // 50%

	// previous nil -> drop check skipped
	c := Check{MaximumCoverageDrop: map[Criterion]float64{Line: 5}}
	if code, vs := c.Run(cur, nil); code != Success || len(vs) != 0 {
		t.Fatalf("nil previous should skip: %v %+v", code, vs)
	}
	// drop of 50 > 5 -> violation
	code, vs := c.Run(cur, prev)
	if code != MaximumCoverageDrop || len(vs) != 1 || vs[0].Kind != ExceededDrop {
		t.Fatalf("expected drop violation: %v %+v", code, vs)
	}
	// within allowance
	c = Check{MaximumCoverageDrop: map[Criterion]float64{Line: 60}}
	if code, _ := c.Run(cur, prev); code != Success {
		t.Fatalf("drop within allowance should pass")
	}
}

func TestCheckPrecedenceMinOverDrop(t *testing.T) {
	prev := NewResult("X", map[string]FileCoverage{"/a.rb": {Lines: []Hit{Coverable(1), Coverable(1)}}}, time.Unix(1, 0))
	c := Check{
		MinimumCoverage:     map[Criterion]float64{Line: 90},
		MaximumCoverageDrop: map[Criterion]float64{Line: 5},
	}
	code, vs := c.Run(result50(), prev)
	if code != MinimumCoverage {
		t.Fatalf("min should outrank drop, got %v", code)
	}
	if len(vs) != 2 {
		t.Fatalf("expected both violations recorded: %+v", vs)
	}
}

func TestCheckByFileSortByFilename(t *testing.T) {
	// Two files failing the same criterion exercises the comparator's
	// filename tie-breaker.
	r := NewResult("X", map[string]FileCoverage{
		"/z.rb": {Lines: []Hit{Coverable(1), Coverable(0)}},
		"/a.rb": {Lines: []Hit{Coverable(1), Coverable(0)}},
	}, time.Unix(1, 0))
	c := Check{MinimumCoverageByFile: map[Criterion]float64{Line: 90}}
	code, vs := c.Run(r, nil)
	if code != MinimumCoverage || len(vs) != 2 {
		t.Fatalf("expected 2 violations: %v %+v", code, vs)
	}
	if vs[0].File != "/a.rb" || vs[1].File != "/z.rb" {
		t.Fatalf("violations not sorted by filename: %+v", vs)
	}
}

func TestSourceFilePercentFor(t *testing.T) {
	s := result50().Files.Files()[0]
	approx(t, sourceFilePercentFor(s, Line), 50.0)
	approx(t, sourceFilePercentFor(s, Branch), 50.0)
}

// --- Formatter -------------------------------------------------------------

func TestSimpleFormatter(t *testing.T) {
	sc := New(WithClock(fixedClock(time.Unix(1, 0))))
	sc.AddStringGroup("Models", "/a.rb")
	r := sc.NewResult("X", map[string]FileCoverage{
		"/a.rb": {Lines: []Hit{Coverable(1), Coverable(0)}},
	})
	out := SimpleFormatter{}.Format(r)
	if !strings.Contains(out, "Group: Models") || !strings.Contains(out, "/a.rb (coverage: 50.00%)") {
		t.Fatalf("formatter output wrong:\n%s", out)
	}
	// no groups -> empty output
	rNoGroup := New(WithClock(fixedClock(time.Unix(1, 0)))).NewResult("X", map[string]FileCoverage{
		"/a.rb": {Lines: []Hit{Coverable(1)}},
	})
	if got := (SimpleFormatter{}).Format(rNoGroup); got != "" {
		t.Fatalf("no-group output should be empty: %q", got)
	}
}

// --- FS seam ---------------------------------------------------------------

func TestOSFS(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.txt")
	if err := (OSFS{}).WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, err := (OSFS{}).ReadFile(p)
	if err != nil || string(data) != "hello" {
		t.Fatalf("read: %q %v", data, err)
	}
	if _, err := (OSFS{}).ReadFile(filepath.Join(dir, "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected not-exist, got %v", err)
	}
}

func TestSplitSourceLines(t *testing.T) {
	if got := splitSourceLines(nil); got != nil {
		t.Fatalf("empty should be nil: %v", got)
	}
	if got := splitSourceLines([]byte("a\nb\n")); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("trailing nl: %v", got)
	}
	if got := splitSourceLines([]byte("only")); !reflect.DeepEqual(got, []string{"only"}) {
		t.Fatalf("no nl: %v", got)
	}
}

// --- SimpleCov config ------------------------------------------------------

func TestNewDefaultsAndOptions(t *testing.T) {
	sc := New()
	if sc.MergeTimeout != DefaultMergeTimeout || sc.CommandName != "Unnamed" {
		t.Fatalf("defaults wrong")
	}
	if _, ok := sc.fs.(OSFS); !ok {
		t.Fatalf("default fs should be OSFS")
	}
	if sc.clock == nil {
		t.Fatalf("clock nil")
	}

	ff := &fakeFS{}
	clk := fixedClock(time.Unix(42, 0))
	sc = New(WithFS(ff), WithClock(clk), WithRoot("/proj"), WithCommandName("Suite"), WithMergeTimeout(time.Minute))
	if sc.fs != ff || sc.Root != "/proj" || sc.CommandName != "Suite" || sc.MergeTimeout != time.Minute {
		t.Fatalf("options not applied: %+v", sc)
	}
	if !sc.clock().Equal(time.Unix(42, 0)) {
		t.Fatalf("clock option not applied")
	}
}

func TestAddFiltersAndFiltered(t *testing.T) {
	sc := New(WithRoot("/proj"), WithClock(fixedClock(time.Unix(1, 0))))
	sc.AddStringFilter("/spec/")
	sc.AddRegexpFilter(regexp.MustCompile(`vendor`))
	sc.AddBlockFilter(func(s *SourceFile) bool { return strings.HasSuffix(s.Filename, "_gen.rb") })
	sc.AddFilter(RegexFilter{Re: regexp.MustCompile(`never-matches-xyz`)})

	fl := NewFileList(
		&SourceFile{Filename: "/proj/app/user.rb", Lines: []Hit{Coverable(1)}},
		&SourceFile{Filename: "/proj/spec/user_spec.rb", Lines: []Hit{Coverable(1)}},
		&SourceFile{Filename: "/proj/vendor/x.rb", Lines: []Hit{Coverable(1)}},
		&SourceFile{Filename: "/proj/app/thing_gen.rb", Lines: []Hit{Coverable(1)}},
	)
	kept := sc.Filtered(fl)
	if kept.Len() != 1 || kept.Files()[0].Filename != "/proj/app/user.rb" {
		t.Fatalf("filtered wrong: %d %v", kept.Len(), kept.Files())
	}
}

func TestGroupedWithAndWithoutUngrouped(t *testing.T) {
	sc := New(WithRoot("/proj"))
	sc.AddStringGroup("Models", "/models/")
	sc.AddGroup("Controllers", StringFilter{Arg: "/controllers/", Root: "/proj"})

	fl := NewFileList(
		&SourceFile{Filename: "/proj/app/models/user.rb"},
		&SourceFile{Filename: "/proj/app/controllers/home.rb"},
		&SourceFile{Filename: "/proj/lib/util.rb"},
	)
	g := sc.Grouped(fl)
	if len(g) != 3 || g[0].Name != "Models" || g[1].Name != "Controllers" || g[2].Name != "Ungrouped" {
		t.Fatalf("group names/order wrong: %+v", g)
	}
	if g[2].Files.Len() != 1 {
		t.Fatalf("ungrouped count wrong")
	}

	// all files claimed -> no Ungrouped bucket
	fl2 := NewFileList(&SourceFile{Filename: "/proj/app/models/user.rb"})
	g2 := sc.Grouped(fl2)
	if len(g2) != 2 {
		t.Fatalf("expected no ungrouped bucket: %+v", g2)
	}

	// no groups configured -> nil
	if got := New().Grouped(fl); got != nil {
		t.Fatalf("no groups should be nil")
	}
}

func TestSimpleCovNewResult(t *testing.T) {
	sc := New(WithClock(fixedClock(time.Unix(77, 0))), WithCommandName("Default"))
	sc.AddStringFilter("/spec/")
	sc.AddStringGroup("App", "/app/")
	r := sc.NewResult("", map[string]FileCoverage{
		"/app/user.rb":       {Lines: []Hit{Coverable(1)}},
		"/spec/user_spec.rb": {Lines: []Hit{Coverable(1)}},
	})
	if r.CommandName != "Default" {
		t.Fatalf("empty command should fall back: %q", r.CommandName)
	}
	if !r.CreatedAt.Equal(time.Unix(77, 0)) {
		t.Fatalf("clock not used")
	}
	if r.Files.Len() != 1 || r.Files.Files()[0].Filename != "/app/user.rb" {
		t.Fatalf("filter not applied in NewResult")
	}
	if len(r.Groups()) != 1 || r.Groups()[0].Name != "App" {
		t.Fatalf("groups not applied: %+v", r.Groups())
	}

	// explicit command name path
	r2 := sc.NewResult("Explicit", map[string]FileCoverage{"/app/a.rb": {Lines: []Hit{Coverable(1)}}})
	if r2.CommandName != "Explicit" {
		t.Fatalf("explicit name wrong: %q", r2.CommandName)
	}
}

func TestThresholdSettersAndRunChecks(t *testing.T) {
	sc := New(WithClock(fixedClock(time.Unix(1, 0))))
	sc.MinimumCoverage(Line, 90)
	sc.MinimumCoverage(Line, 95) // second write exercises non-nil-map branch
	sc.MinimumCoverageByFile(Line, 80)
	sc.MinimumCoverageByFile(Branch, 80)
	sc.MaximumCoverageDrop(Line, 3)
	sc.MaximumCoverageDrop(Branch, 3)

	chk := sc.Check()
	if chk.MinimumCoverage[Line] != 95 || chk.MinimumCoverageByFile[Branch] != 80 || chk.MaximumCoverageDrop[Line] != 3 {
		t.Fatalf("setters wrong: %+v", chk)
	}

	code, vs := sc.RunChecks(result50(), nil)
	if code != MinimumCoverage || len(vs) == 0 {
		t.Fatalf("RunChecks wrong: %v %+v", code, vs)
	}
}

func TestRefuseCoverageDrop(t *testing.T) {
	sc := New()
	sc.RefuseCoverageDrop() // default Line
	if sc.Check().MaximumCoverageDrop[Line] != 0 {
		t.Fatalf("default refuse should set Line=0")
	}
	sc.RefuseCoverageDrop(Branch)
	if sc.Check().MaximumCoverageDrop[Branch] != 0 {
		t.Fatalf("explicit refuse should set Branch=0")
	}
}

func TestLoadSource(t *testing.T) {
	ff := &fakeFS{
		files:   map[string][]byte{"/a.rb": []byte("line1\nline2\n")},
		readErr: map[string]error{"/bad.rb": errors.New("io error")},
	}
	sc := New(WithFS(ff))

	sf := &SourceFile{Filename: "/a.rb"}
	if err := sc.LoadSource(sf); err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(sf.Src, []string{"line1", "line2"}) {
		t.Fatalf("src wrong: %v", sf.Src)
	}
	if err := sc.LoadSource(&SourceFile{Filename: "/bad.rb"}); err == nil {
		t.Fatalf("expected read error")
	}
}

func TestLoadResultset(t *testing.T) {
	valid := []byte(`{"C":{"coverage":{"/a.rb":{"lines":[1]}},"timestamp":5}}`)
	ff := &fakeFS{
		files: map[string][]byte{
			"good.json": valid,
			"bad.json":  []byte("{oops"),
		},
		readErr: map[string]error{"boom.json": errors.New("disk error")},
	}
	sc := New(WithFS(ff))

	rs, err := sc.LoadResultset("good.json")
	if err != nil || rs["C"].Timestamp != 5 {
		t.Fatalf("good load: %v %+v", err, rs)
	}
	// missing file -> empty, no error
	rs, err = sc.LoadResultset("absent.json")
	if err != nil || len(rs) != 0 {
		t.Fatalf("missing should be empty: %v %+v", err, rs)
	}
	// other read error -> propagated
	if _, err := sc.LoadResultset("boom.json"); err == nil {
		t.Fatalf("expected read error")
	}
	// parse error -> propagated
	if _, err := sc.LoadResultset("bad.json"); err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestStoreResultset(t *testing.T) {
	ff := &fakeFS{}
	sc := New(WithFS(ff))
	rs := Resultset{"C": {Coverage: map[string]FileCoverage{"/a.rb": {Lines: []Hit{Coverable(1)}}}, Timestamp: 9}}
	if err := sc.StoreResultset(".resultset.json", rs); err != nil {
		t.Fatalf("store: %v", err)
	}
	if _, ok := ff.written[".resultset.json"]; !ok {
		t.Fatalf("nothing written")
	}

	// write error propagated
	ff.writeErr = errors.New("readonly")
	if err := sc.StoreResultset(".resultset.json", rs); err == nil {
		t.Fatalf("expected write error")
	}

	// JSON encode error via seam
	ff.writeErr = nil
	orig := jsonMarshal
	jsonMarshal = func(any) ([]byte, error) { return nil, errors.New("boom") }
	defer func() { jsonMarshal = orig }()
	if err := sc.StoreResultset(".resultset.json", rs); err == nil {
		t.Fatalf("expected JSON encode error")
	}
}

func TestExitCodeConstants(t *testing.T) {
	// Exception is reserved for parity; assert its value so it is referenced.
	if Success != 0 || Exception != 1 || MinimumCoverage != 2 || MaximumCoverageDrop != 3 {
		t.Fatalf("exit code values drifted")
	}
}
