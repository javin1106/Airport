package sourcearchive

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractRoundTrip(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := filepath.Join(root, "source")
	archive := filepath.Join(root, "source.tar.gz")
	destination := filepath.Join(root, "extracted")
	writeTestFile(t, filepath.Join(source, "src", "index.html"), "<h1>Airport</h1>")

	if err := Create(source, archive); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := Extract(archive, destination); err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	content, err := os.ReadFile(filepath.Join(destination, "src", "index.html"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(content) != "<h1>Airport</h1>" {
		t.Fatalf("extracted content = %q", content)
	}
}

func TestExtractRejectsTraversal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	archive := filepath.Join(root, "unsafe.tar.gz")
	destination := filepath.Join(root, "extracted")
	writeArchiveEntry(t, archive, &tar.Header{
		Name:     "../outside.txt",
		Mode:     0644,
		Size:     1,
		Typeflag: tar.TypeReg,
	}, []byte("x"))

	if err := Extract(archive, destination); err == nil {
		t.Fatal("Extract() error = nil, want unsafe path error")
	}
	if _, err := os.Stat(filepath.Join(root, "outside.txt")); !os.IsNotExist(err) {
		t.Fatalf("outside file exists or stat failed: %v", err)
	}
}

func TestExtractRejectsSymlink(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	archive := filepath.Join(root, "symlink.tar.gz")
	writeArchiveEntry(t, archive, &tar.Header{
		Name:     "shortcut",
		Linkname: "../outside",
		Typeflag: tar.TypeSymlink,
	}, nil)

	if err := Extract(archive, filepath.Join(root, "extracted")); err == nil {
		t.Fatal("Extract() error = nil, want unsupported entry error")
	}
}

func writeArchiveEntry(t *testing.T, archivePath string, header *tar.Header, content []byte) {
	t.Helper()
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
