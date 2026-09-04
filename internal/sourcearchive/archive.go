package sourcearchive

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func Create(sourceDir, archivePath string) error {
	if err := os.MkdirAll(filepath.Dir(archivePath), 0755); err != nil {
		return fmt.Errorf("create archive directory: %w", err)
	}

	archiveFile, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("create archive file: %w", err)
	}

	gzipWriter := gzip.NewWriter(archiveFile)
	tarWriter := tar.NewWriter(gzipWriter)

	completed := false

	defer func() {
		_ = tarWriter.Close()
		_ = gzipWriter.Close()
		_ = archiveFile.Close()

		if !completed {
			_ = os.Remove(archivePath)
		}
	}()

	err = filepath.WalkDir(
		sourceDir,
		func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			if entry.IsDir() && entry.Name() == ".git" {
				return filepath.SkipDir
			}

			relativePath, err := filepath.Rel(sourceDir, path)
			if err != nil {
				return err
			}

			if relativePath == "." {
				return nil
			}

			info, err := entry.Info()
			if err != nil {
				return err
			}

			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}

			header.Name = filepath.ToSlash(relativePath)

			if err := tarWriter.WriteHeader(header); err != nil {
				return err
			}

			if !info.Mode().IsRegular() {
				return nil
			}

			file, err := os.Open(path)
			if err != nil {
				return err
			}

			_, copyErr := io.Copy(tarWriter, file)
			closeErr := file.Close()

			if copyErr != nil {
				return copyErr
			}

			return closeErr
		},
	)
	if err != nil {
		return fmt.Errorf("archive source directory: %w", err)
	}

	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("close tar writer: %w", err)
	}

	if err := gzipWriter.Close(); err != nil {
		return fmt.Errorf("close gzip writer: %w", err)
	}

	if err := archiveFile.Close(); err != nil {
		return fmt.Errorf("close archive file: %w", err)
	}

	completed = true
	return nil
}
