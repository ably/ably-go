package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGeneratedFilesAreUpToDate fails when api_gen.go no longer matches what
// the generator produces, which is what happens when the API of internal/ably
// changes and nobody runs `go generate ./device/... ./server/...`.
func TestGeneratedFilesAreUpToDate(t *testing.T) {
	repo, err := repoRoot("")
	if err != nil {
		t.Fatal(err)
	}
	for name, target := range targets {
		t.Run(name, func(t *testing.T) {
			want, err := generate(repo, target)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(repo, target.dir, "api_gen.go")
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(want) {
				t.Errorf("%s is out of date; run `go generate ./device/... ./server/...`", target.dir+"/api_gen.go")
			}
		})
	}
}
