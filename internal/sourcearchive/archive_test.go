package sourcearchive

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateArchivesSourceAndExcludesGitDirectory(t *testing.T) {
	t.Parallel()

	temporaryDirectory := t.TempDir()
	sourceDirectory := filepath.Join(temporaryDirectory, "source")
	archivePath := filepath.Join(temporaryDirectory, "archives", "source.tar.gz")

	writeTestFile(t, filepath.Join(sourceDirectory, "README.md"), "airport")
	writeTestFile(t, filepath.Join(sourceDirectory, "src", "main.go"), "package main")
	writeTestFile(t, filepath.Join(sourceDirectory, ".git", "config"), "secret metadata")

	if err := Create(sourceDirectory, archivePath); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	archivedFiles := readArchivedFiles(t, archivePath)

	if got := archivedFiles["README.md"]; got != "airport" {
		t.Errorf("README.md content = %q, want %q", got, "airport")
	}

	if got := archivedFiles["src/main.go"]; got != "package main" {
		t.Errorf("src/main.go content = %q, want %q", got, "package main")
	}

	for name := range archivedFiles {
		if name == ".git" || strings.HasPrefix(name, ".git/") {
			t.Errorf("archive unexpectedly contains Git metadata entry %q", name)
		}
	}
}

func TestCreateRemovesPartialArchiveOnFailure(t *testing.T) {
	t.Parallel()

	temporaryDirectory := t.TempDir()
	archivePath := filepath.Join(temporaryDirectory, "source.tar.gz")

	err := Create(filepath.Join(temporaryDirectory, "missing-source"), archivePath)
	if err == nil {
		t.Fatal("Create() error = nil, want an error")
	}

	_, statErr := os.Stat(archivePath)
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("archive still exists after failure: stat error = %v", statErr)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("create test directory: %v", err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}

func readArchivedFiles(t *testing.T, archivePath string) map[string]string {
	t.Helper()

	archiveFile, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer archiveFile.Close()

	gzipReader, err := gzip.NewReader(archiveFile)
	if err != nil {
		t.Fatalf("open gzip stream: %v", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	files := make(map[string]string)

	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("read tar entry: %v", err)
		}

		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			continue
		}

		content, err := io.ReadAll(tarReader)
		if err != nil {
			t.Fatalf("read tar entry %q: %v", header.Name, err)
		}

		files[header.Name] = string(content)
	}

	return files
}
