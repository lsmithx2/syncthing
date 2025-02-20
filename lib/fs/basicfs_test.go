// Copyright (C) 2017 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package fs

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/syncthing/syncthing/lib/rand"
)

func setup(t *testing.T) (*BasicFilesystem, string) {
	t.Helper()
	dir := t.TempDir()
	return newBasicFilesystem(dir), dir
}

func TestChmodFile(t *testing.T) {
	fs, dir := setup(t)
	path := filepath.Join(dir, "file")

	defer os.Chmod(path, 0666)

	fd, err := os.Create(path)
	if err != nil {
		t.Error(err)
	}
	fd.Close()

	if err := os.Chmod(path, 0666); err != nil {
		t.Error(err)
	}

	if stat, err := os.Stat(path); err != nil || stat.Mode()&os.ModePerm != 0666 {
		t.Errorf("wrong perm: %t %#o", err == nil, stat.Mode()&os.ModePerm)
	}

	if err := fs.Chmod("file", 0444); err != nil {
		t.Error(err)
	}

	if stat, err := os.Stat(path); err != nil || stat.Mode()&os.ModePerm != 0444 {
		t.Errorf("wrong perm: %t %#o", err == nil, stat.Mode()&os.ModePerm)
	}
}

func TestChownFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Not supported on Windows")
		return
	}
	if os.Getuid() != 0 {
		// We are not root. No expectation of being able to chown. Our tests
		// typically don't run with CAP_FOWNER.
		t.Skip("Test not possible")
		return
	}

	fs, dir := setup(t)
	path := filepath.Join(dir, "file")

	defer os.Chmod(path, 0666)

	fd, err := os.Create(path)
	if err != nil {
		t.Error("Unexpected error:", err)
	}
	fd.Close()

	_, err = fs.Lstat("file")
	if err != nil {
		t.Error("Unexpected error:", err)
	}

	newUID := 1000 + rand.Intn(30000)
	newGID := 1000 + rand.Intn(30000)

	if err := fs.Lchown("file", newUID, newGID); err != nil {
		t.Error("Unexpected error:", err)
	}

	info, err := fs.Lstat("file")
	if err != nil {
		t.Error("Unexpected error:", err)
	}
	if info.Owner() != newUID {
		t.Errorf("Incorrect owner, expected %d but got %d", newUID, info.Owner())
	}
	if info.Group() != newGID {
		t.Errorf("Incorrect group, expected %d but got %d", newGID, info.Group())
	}
}

func TestChmodDir(t *testing.T) {
	fs, dir := setup(t)
	path := filepath.Join(dir, "dir")

	mode := os.FileMode(0755)
	if runtime.GOOS == "windows" {
		mode = os.FileMode(0777)
	}

	defer os.Chmod(path, mode)

	if err := os.Mkdir(path, mode); err != nil {
		t.Error(err)
	}
	// On UNIX, Mkdir will subtract the umask, so force desired mode explicitly
	if err := os.Chmod(path, mode); err != nil {
		t.Error(err)
	}

	if stat, err := os.Stat(path); err != nil || stat.Mode()&os.ModePerm != mode {
		t.Errorf("wrong perm: %t %#o", err == nil, stat.Mode()&os.ModePerm)
	}

	if err := fs.Chmod("dir", 0555); err != nil {
		t.Error(err)
	}

	if stat, err := os.Stat(path); err != nil || stat.Mode()&os.ModePerm != 0555 {
		t.Errorf("wrong perm: %t %#o", err == nil, stat.Mode()&os.ModePerm)
	}
}

func TestChtimes(t *testing.T) {
	fs, dir := setup(t)
	path := filepath.Join(dir, "file")
	fd, err := os.Create(path)
	if err != nil {
		t.Error(err)
	}
	fd.Close()

	mtime := time.Now().Add(-time.Hour)

	fs.Chtimes("file", mtime, mtime)

	stat, err := os.Stat(path)
	if err != nil {
		t.Error(err)
	}

	diff := stat.ModTime().Sub(mtime)
	if diff > 3*time.Second || diff < -3*time.Second {
		t.Errorf("%s != %s", stat.Mode(), mtime)
	}
}

func TestCreate(t *testing.T) {
	fs, dir := setup(t)
	path := filepath.Join(dir, "file")

	if _, err := os.Stat(path); err == nil {
		t.Errorf("exists?")
	}

	fd, err := fs.Create("file")
	if err != nil {
		t.Error(err)
	}
	fd.Close()

	if _, err := os.Stat(path); err != nil {
		t.Error(err)
	}
}

func TestCreateSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows not supported")
	}

	fs, dir := setup(t)
	path := filepath.Join(dir, "file")

	if err := fs.CreateSymlink("blah", "file"); err != nil {
		t.Error(err)
	}

	if target, err := os.Readlink(path); err != nil || target != "blah" {
		t.Error("target", target, "err", err)
	}

	if err := os.Remove(path); err != nil {
		t.Error(err)
	}

	if err := fs.CreateSymlink(filepath.Join("..", "blah"), "file"); err != nil {
		t.Error(err)
	}

	if target, err := os.Readlink(path); err != nil || target != filepath.Join("..", "blah") {
		t.Error("target", target, "err", err)
	}
}

func TestDirNames(t *testing.T) {
	fs, dir := setup(t)

	// Case differences
	testCases := []string{
		"a",
		"bC",
	}
	sort.Strings(testCases)

	for _, sub := range testCases {
		if err := os.Mkdir(filepath.Join(dir, sub), 0777); err != nil {
			t.Error(err)
		}
	}

	if dirs, err := fs.DirNames("."); err != nil || len(dirs) != len(testCases) {
		t.Errorf("%s %s %s", err, dirs, testCases)
	} else {
		sort.Strings(dirs)
		for i := range dirs {
			if dirs[i] != testCases[i] {
				t.Errorf("%s != %s", dirs[i], testCases[i])
			}
		}
	}
}

func TestNames(t *testing.T) {
	// Tests that all names are without the root directory.
	fs, _ := setup(t)

	expected := "file"
	fd, err := fs.Create(expected)
	if err != nil {
		t.Error(err)
	}
	defer fd.Close()

	if fd.Name() != expected {
		t.Errorf("incorrect %s != %s", fd.Name(), expected)
	}
	if stat, err := fd.Stat(); err != nil || stat.Name() != expected {
		t.Errorf("incorrect %s != %s (%v)", stat.Name(), expected, err)
	}

	if err := fs.Mkdir("dir", 0777); err != nil {
		t.Error(err)
	}

	expected = filepath.Join("dir", "file")
	fd, err = fs.Create(expected)
	if err != nil {
		t.Error(err)
	}
	defer fd.Close()

	if fd.Name() != expected {
		t.Errorf("incorrect %s != %s", fd.Name(), expected)
	}

	// os.fd.Stat() returns just base, so do we.
	if stat, err := fd.Stat(); err != nil || stat.Name() != filepath.Base(expected) {
		t.Errorf("incorrect %s != %s (%v)", stat.Name(), filepath.Base(expected), err)
	}
}

func TestGlob(t *testing.T) {
	// Tests that all names are without the root directory.
	fs, _ := setup(t)

	for _, dirToCreate := range []string{
		filepath.Join("a", "test", "b"),
		filepath.Join("a", "best", "b"),
		filepath.Join("a", "best", "c"),
	} {
		if err := fs.MkdirAll(dirToCreate, 0777); err != nil {
			t.Error(err)
		}
	}

	testCases := []struct {
		pattern string
		matches []string
	}{
		{
			filepath.Join("a", "?est", "?"),
			[]string{
				filepath.Join("a", "test", "b"),
				filepath.Join("a", "best", "b"),
				filepath.Join("a", "best", "c"),
			},
		},
		{
			filepath.Join("a", "?est", "b"),
			[]string{
				filepath.Join("a", "test", "b"),
				filepath.Join("a", "best", "b"),
			},
		},
		{
			filepath.Join("a", "best", "?"),
			[]string{
				filepath.Join("a", "best", "b"),
				filepath.Join("a", "best", "c"),
			},
		},
	}

	for _, testCase := range testCases {
		results, err := fs.Glob(testCase.pattern)
		sort.Strings(results)
		sort.Strings(testCase.matches)
		if err != nil {
			t.Error(err)
		}
		if len(results) != len(testCase.matches) {
			t.Errorf("result count mismatch")
		}
		for i := range testCase.matches {
			if results[i] != testCase.matches[i] {
				t.Errorf("%s != %s", results[i], testCase.matches[i])
			}
		}
	}
}

func TestUsage(t *testing.T) {
	fs, _ := setup(t)
	usage, err := fs.Usage(".")
	if err != nil {
		if runtime.GOOS == "netbsd" || runtime.GOOS == "openbsd" || runtime.GOOS == "solaris" {
			t.Skip()
		}
		t.Errorf("Unexpected error: %s", err)
	}
	if usage.Free < 1 {
		t.Error("Disk is full?", usage.Free)
	}
}

func TestRooted(t *testing.T) {
	type testcase struct {
		root   string
		rel    string
		joined string
		ok     bool
	}
	cases := []testcase{
		// Valid cases
		{"foo", "bar", "foo/bar", true},
		{"foo", "/bar", "foo/bar", true},
		{"foo/", "bar", "foo/bar", true},
		{"foo/", "/bar", "foo/bar", true},
		{"baz/foo", "bar", "baz/foo/bar", true},
		{"baz/foo", "/bar", "baz/foo/bar", true},
		{"baz/foo/", "bar", "baz/foo/bar", true},
		{"baz/foo/", "/bar", "baz/foo/bar", true},
		{"foo", "bar/baz", "foo/bar/baz", true},
		{"foo", "/bar/baz", "foo/bar/baz", true},
		{"foo/", "bar/baz", "foo/bar/baz", true},
		{"foo/", "/bar/baz", "foo/bar/baz", true},
		{"baz/foo", "bar/baz", "baz/foo/bar/baz", true},
		{"baz/foo", "/bar/baz", "baz/foo/bar/baz", true},
		{"baz/foo/", "bar/baz", "baz/foo/bar/baz", true},
		{"baz/foo/", "/bar/baz", "baz/foo/bar/baz", true},

		// Not escape attempts, but oddly formatted relative paths.
		{"foo", "", "foo", true},
		{"foo", "/", "foo", true},
		{"foo", "/..", "foo", true},
		{"foo", "./bar", "foo/bar", true},
		{"foo/", "", "foo", true},
		{"foo/", "/", "foo", true},
		{"foo/", "/..", "foo", true},
		{"foo/", "./bar", "foo/bar", true},
		{"baz/foo", "./bar", "baz/foo/bar", true},
		{"foo", "./bar/baz", "foo/bar/baz", true},
		{"baz/foo", "./bar/baz", "baz/foo/bar/baz", true},
		{"baz/foo", "bar/../baz", "baz/foo/baz", true},
		{"baz/foo", "/bar/../baz", "baz/foo/baz", true},
		{"baz/foo", "./bar/../baz", "baz/foo/baz", true},

		// Results in an allowed path, but does it by probing. Disallowed.
		{"foo", "../foo", "", false},
		{"foo", "../foo/bar", "", false},
		{"baz/foo", "../foo/bar", "", false},
		{"baz/foo", "../../baz/foo/bar", "", false},
		{"baz/foo", "bar/../../foo/bar", "", false},
		{"baz/foo", "bar/../../../baz/foo/bar", "", false},

		// Escape attempts.
		{"foo", "..", "", false},
		{"foo", "../", "", false},
		{"foo", "../bar", "", false},
		{"foo", "../foobar", "", false},
		{"foo/", "../bar", "", false},
		{"foo/", "../foobar", "", false},
		{"baz/foo", "../bar", "", false},
		{"baz/foo", "../foobar", "", false},
		{"baz/foo/", "../bar", "", false},
		{"baz/foo/", "../foobar", "", false},
		{"baz/foo/", "bar/../../quux/baz", "", false},

		// Empty root is a misconfiguration.
		{"", "/foo", "", false},
		{"", "foo", "", false},
		{"", ".", "", false},
		{"", "..", "", false},
		{"", "/", "", false},
		{"", "", "", false},

		// Root=/ is valid, and things should be verified as usual.
		{"/", "foo", "/foo", true},
		{"/", "/foo", "/foo", true},
		{"/", "../foo", "", false},
		{"/", "..", "", false},
		{"/", "/", "/", true},
		{"/", "", "/", true},

		// special case for filesystems to be able to MkdirAll('.') for example
		{"/", ".", "/", true},
	}

	if runtime.GOOS == "windows" {
		extraCases := []testcase{
			{`c:\`, `foo`, `c:\foo`, true},
			{`\\?\c:\`, `foo`, `\\?\c:\foo`, true},
			{`c:\`, `\foo`, `c:\foo`, true},
			{`\\?\c:\`, `\foo`, `\\?\c:\foo`, true},
			{`c:\`, `\\foo`, ``, false},
			{`c:\`, ``, `c:\`, true},
			{`c:\`, `\`, `c:\`, true},
			{`\\?\c:\`, `\\foo`, ``, false},
			{`\\?\c:\`, ``, `\\?\c:\`, true},
			{`\\?\c:\`, `\`, `\\?\c:\`, true},
			{`\\?\c:\test`, `.`, `\\?\c:\test`, true},
			{`c:\test`, `.`, `c:\test`, true},
			{`\\?\c:\test`, `/`, `\\?\c:\test`, true},
			{`c:\test`, ``, `c:\test`, true},

			// makes no sense, but will be treated simply as a bad filename
			{`c:\foo`, `d:\bar`, `c:\foo\d:\bar`, true},

			// special case for filesystems to be able to MkdirAll('.') for example
			{`c:\`, `.`, `c:\`, true},
			{`\\?\c:\`, `.`, `\\?\c:\`, true},
		}

		for _, tc := range cases {
			// Add case where root is backslashed, rel is forward slashed
			extraCases = append(extraCases, testcase{
				root:   filepath.FromSlash(tc.root),
				rel:    tc.rel,
				joined: tc.joined,
				ok:     tc.ok,
			})
			// and the opposite
			extraCases = append(extraCases, testcase{
				root:   tc.root,
				rel:    filepath.FromSlash(tc.rel),
				joined: tc.joined,
				ok:     tc.ok,
			})
			// and both backslashed
			extraCases = append(extraCases, testcase{
				root:   filepath.FromSlash(tc.root),
				rel:    filepath.FromSlash(tc.rel),
				joined: tc.joined,
				ok:     tc.ok,
			})
		}

		cases = append(cases, extraCases...)
	}

	for _, tc := range cases {
		fs := BasicFilesystem{root: tc.root}
		res, err := fs.rooted(tc.rel)
		if tc.ok {
			if err != nil {
				t.Errorf("Unexpected error for rooted(%q, %q): %v", tc.root, tc.rel, err)
				continue
			}
			exp := filepath.FromSlash(tc.joined)
			if res != exp {
				t.Errorf("Unexpected result for rooted(%q, %q): %q != expected %q", tc.root, tc.rel, res, exp)
			}
		} else if err == nil {
			t.Errorf("Unexpected pass for rooted(%q, %q) => %q", tc.root, tc.rel, res)
			continue
		}
	}
}

func TestNewBasicFilesystem(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("non-windows root paths")
	}

	currentDir, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	testCases := []struct {
		input        string
		expectedRoot string
		expectedURI  string
	}{
		{"/foo/bar/baz", "/foo/bar/baz", "/foo/bar/baz"},
		{"/foo/bar/baz/", "/foo/bar/baz", "/foo/bar/baz"},
		{"", currentDir, currentDir},
		{".", currentDir, currentDir},
		{"/", "/", "/"},
	}

	for _, testCase := range testCases {
		fs := newBasicFilesystem(testCase.input)
		if fs.root != testCase.expectedRoot {
			t.Errorf("root %q != %q", fs.root, testCase.expectedRoot)
		}
		if fs.URI() != testCase.expectedURI {
			t.Errorf("uri %q != %q", fs.URI(), testCase.expectedURI)
		}
	}

	fs := newBasicFilesystem("relative/path")
	if fs.root == "relative/path" || !strings.HasPrefix(fs.root, string(PathSeparator)) {
		t.Errorf(`newBasicFilesystem("relative/path").root == %q, expected absolutification`, fs.root)
	}
}

func TestRel(t *testing.T) {
	testCases := []struct {
		root        string
		abs         string
		expectedRel string
	}{
		{"/", "/", ""},
		{"/", "/test", "test"},
		{"/", "/Test", "Test"},
		{"/Test", "/Test/test", "test"},
	}
	if runtime.GOOS == "windows" {
		for i := range testCases {
			testCases[i].root = filepath.FromSlash(testCases[i].root)
			testCases[i].abs = filepath.FromSlash(testCases[i].abs)
			testCases[i].expectedRel = filepath.FromSlash(testCases[i].expectedRel)
		}
	}

	for _, tc := range testCases {
		if res := rel(tc.abs, tc.root); res != tc.expectedRel {
			t.Errorf(`rel("%v", "%v") == "%v", expected "%v"`, tc.abs, tc.root, res, tc.expectedRel)
		}
	}
}

func TestBasicWalkSkipSymlink(t *testing.T) {
	_, dir := setup(t)
	testWalkSkipSymlink(t, FilesystemTypeBasic, dir)
}

func TestWalkTraverseDirJunct(t *testing.T) {
	_, dir := setup(t)
	testWalkTraverseDirJunct(t, FilesystemTypeBasic, dir)
}

func TestWalkInfiniteRecursion(t *testing.T) {
	_, dir := setup(t)
	testWalkInfiniteRecursion(t, FilesystemTypeBasic, dir)
}
