package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestMultiFS(t *testing.T) {
	base := t.TempDir()
	write := func(p string) {
		full := filepath.Join(base, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a/Music/song.mp3")
	write("a/Music/album/track.flac")
	write("b/Music/other.mp3") // same base name -> "Music (2)"
	write("c/Movies/film.mkv")

	m := newMultiFS([]string{
		filepath.Join(base, "a/Music"),
		filepath.Join(base, "b/Music"),
		filepath.Join(base, "c/Movies"),
	})
	if err := fstest.TestFS(m,
		"Music/song.mp3",
		"Music/album/track.flac",
		"Music (2)/other.mp3",
		"Movies/film.mkv",
	); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(m, "Music/../../secret"); err == nil {
		t.Fatal("path escape must be rejected")
	}
}
