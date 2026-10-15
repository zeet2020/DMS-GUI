package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// multiFS presents several media folders as one fs.FS for dms's Server.FS:
// the root lists one virtual directory per folder (named after the folder,
// with " (2)" etc. on name clashes) and every path below it resolves into
// that folder via os.DirFS, which already rejects ".." escapes.
type multiFS struct {
	names []string
	roots map[string]fs.FS
}

func newMultiFS(dirs []string) *multiFS {
	m := &multiFS{roots: map[string]fs.FS{}}
	for _, dir := range dirs {
		base := filepath.Base(filepath.Clean(dir))
		name := base
		for i := 2; m.roots[name] != nil; i++ {
			name = fmt.Sprintf("%s (%d)", base, i)
		}
		m.names = append(m.names, name)
		m.roots[name] = os.DirFS(dir)
	}
	return m
}

// split maps a virtual path to its folder's FS and the path inside it.
func (m *multiFS) split(op, name string) (fs.FS, string, error) {
	if !fs.ValidPath(name) {
		return nil, "", &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}
	top, rest, _ := strings.Cut(name, "/")
	root := m.roots[top]
	if root == nil {
		return nil, "", &fs.PathError{Op: op, Path: name, Err: fs.ErrNotExist}
	}
	if rest == "" {
		rest = "."
	}
	return root, rest, nil
}

func (m *multiFS) Open(name string) (fs.File, error) {
	if name == "." {
		entries, err := m.ReadDir(".")
		if err != nil {
			return nil, err
		}
		return &multiRoot{entries: entries}, nil
	}
	root, rest, err := m.split("open", name)
	if err != nil {
		return nil, err
	}
	f, err := root.Open(rest)
	if err != nil {
		return nil, err
	}
	if rest == "." {
		// os.DirFS reports the folder as "."; show the virtual name instead.
		return renamedFile{f, name}, nil
	}
	return f, nil
}

func (m *multiFS) Stat(name string) (fs.FileInfo, error) {
	if name == "." {
		return rootInfo{}, nil
	}
	root, rest, err := m.split("stat", name)
	if err != nil {
		return nil, err
	}
	fi, err := fs.Stat(root, rest)
	if err != nil {
		return nil, err
	}
	if rest == "." {
		fi = renamedInfo{fi, name}
	}
	return fi, nil
}

func (m *multiFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name != "." {
		root, rest, err := m.split("readdir", name)
		if err != nil {
			return nil, err
		}
		return fs.ReadDir(root, rest)
	}
	// A folder that has gone missing is skipped rather than failing the
	// whole root listing.
	var entries []fs.DirEntry
	for _, n := range m.names {
		if fi, err := m.Stat(n); err == nil {
			entries = append(entries, fs.FileInfoToDirEntry(fi))
		}
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return entries, nil
}

type renamedInfo struct {
	fs.FileInfo
	name string
}

func (r renamedInfo) Name() string { return r.name }

type renamedFile struct {
	fs.File
	name string
}

func (r renamedFile) Stat() (fs.FileInfo, error) {
	fi, err := r.File.Stat()
	if err != nil {
		return nil, err
	}
	return renamedInfo{fi, r.name}, nil
}

func (r renamedFile) ReadDir(n int) ([]fs.DirEntry, error) {
	d, ok := r.File.(fs.ReadDirFile)
	if !ok {
		return nil, &fs.PathError{Op: "readdir", Path: r.name, Err: errors.New("not a directory")}
	}
	return d.ReadDir(n)
}

type rootInfo struct{}

func (rootInfo) Name() string       { return "." }
func (rootInfo) Size() int64        { return 0 }
func (rootInfo) Mode() fs.FileMode  { return fs.ModeDir | 0o555 }
func (rootInfo) ModTime() time.Time { return time.Time{} }
func (rootInfo) IsDir() bool        { return true }
func (rootInfo) Sys() any           { return nil }

// multiRoot is the open handle for the virtual root directory.
type multiRoot struct {
	entries []fs.DirEntry
	off     int
}

func (r *multiRoot) Stat() (fs.FileInfo, error) { return rootInfo{}, nil }
func (r *multiRoot) Close() error               { return nil }
func (r *multiRoot) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: ".", Err: errors.New("is a directory")}
}

func (r *multiRoot) ReadDir(n int) ([]fs.DirEntry, error) {
	rest := r.entries[r.off:]
	if n <= 0 {
		r.off = len(r.entries)
		return rest, nil
	}
	if len(rest) == 0 {
		return nil, io.EOF
	}
	if n > len(rest) {
		n = len(rest)
	}
	r.off += n
	return rest[:n], nil
}
