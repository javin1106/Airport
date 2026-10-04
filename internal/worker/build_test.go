package worker

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/javin1106/airport/internal/sourcearchive"
)

type fakeStore struct {
	archivePath string
	uploaded    string
}

func (store *fakeStore) DownloadArchive(_ context.Context, _ string, destination string) error {
	content, err := os.ReadFile(store.archivePath)
	if err != nil {
		return err
	}
	return os.WriteFile(destination, content, 0600)
}

func (store *fakeStore) UploadDirectory(_ context.Context, _ string, directory string) (int, error) {
	content, err := os.ReadFile(filepath.Join(directory, "index.html"))
	if err != nil {
		return 0, err
	}
	store.uploaded = string(content)
	return 1, nil
}

func TestBuildRunsNPMAndUploadsDist(t *testing.T) {
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm is not installed")
	}
	t.Setenv("npm_config_offline", "true")
	t.Setenv("npm_config_audit", "false")
	t.Setenv("npm_config_fund", "false")

	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "package.json"), []byte(`{"name":"airport-worker-test","version":"1.0.0","scripts":{"build":"node build.js"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "build.js"), []byte(`const fs = require('fs'); fs.mkdirSync('dist'); fs.writeFileSync('dist/index.html', 'built');`), 0644); err != nil {
		t.Fatal(err)
	}

	archivePath := filepath.Join(root, "source.tar.gz")
	if err := sourcearchive.Create(source, archivePath); err != nil {
		t.Fatal(err)
	}
	store := &fakeStore{archivePath: archivePath}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	count, err := NewBuilder(store).Build(ctx, "0123456789abcdef")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if count != 1 || store.uploaded != "built" {
		t.Fatalf("Build() uploaded %d files with content %q", count, store.uploaded)
	}
}
