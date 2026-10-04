package worker

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/javin1106/airport/internal/sourcearchive"
)

type artifactStore interface {
	DownloadArchive(ctx context.Context, deploymentID, destination string) error
	UploadDirectory(ctx context.Context, deploymentID, directory string) (int, error)
}

type Builder struct {
	store artifactStore
}

func NewBuilder(store artifactStore) *Builder {
	return &Builder{store: store}
}

func (builder *Builder) Build(ctx context.Context, deploymentID string) (int, error) {
	if len(deploymentID) != 16 {
		return 0, fmt.Errorf("invalid deployment ID %q", deploymentID)
	}
	if _, err := hex.DecodeString(deploymentID); err != nil {
		return 0, fmt.Errorf("invalid deployment ID %q: %w", deploymentID, err)
	}

	workDir, err := os.MkdirTemp("", "airport-build-"+deploymentID+"-")
	if err != nil {
		return 0, fmt.Errorf("create build directory: %w", err)
	}
	defer os.RemoveAll(workDir)

	archivePath := filepath.Join(workDir, "source.tar.gz")
	if err := builder.store.DownloadArchive(ctx, deploymentID, archivePath); err != nil {
		return 0, err
	}

	sourceDir := filepath.Join(workDir, "source")
	if err := sourcearchive.Extract(archivePath, sourceDir); err != nil {
		return 0, fmt.Errorf("extract source archive: %w", err)
	}

	if _, err := os.Stat(filepath.Join(sourceDir, "package.json")); err != nil {
		return 0, fmt.Errorf("build requires package.json at repository root: %w", err)
	}

	installArgs := []string{"install"}
	if _, err := os.Stat(filepath.Join(sourceDir, "package-lock.json")); err == nil {
		installArgs = []string{"ci"}
	} else if !os.IsNotExist(err) {
		return 0, fmt.Errorf("check package lock: %w", err)
	}
	if err := runNPM(ctx, sourceDir, installArgs...); err != nil {
		return 0, err
	}
	if err := runNPM(ctx, sourceDir, "run", "build"); err != nil {
		return 0, err
	}

	distDir := filepath.Join(sourceDir, "dist")
	info, err := os.Stat(distDir)
	if err != nil {
		return 0, fmt.Errorf("find dist directory: %w", err)
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("dist is not a directory")
	}

	count, err := builder.store.UploadDirectory(ctx, deploymentID, distDir)
	if err != nil {
		return count, err
	}
	return count, nil
}

func runNPM(ctx context.Context, directory string, args ...string) error {
	command := exec.CommandContext(ctx, "npm", args...)
	command.Dir = directory
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("npm %v: %w", args, err)
	}
	return nil
}
