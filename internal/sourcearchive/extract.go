package sourcearchive

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxArchiveEntries = 20000
	maxExtractedSize  = 250 << 20
)

func Extract(archivePath, destination string) error {
	archiveFile, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open source archive: %w", err)
	}
	defer archiveFile.Close()

	gzipReader, err := gzip.NewReader(archiveFile)
	if err != nil {
		return fmt.Errorf("open compressed source archive: %w", err)
	}
	defer gzipReader.Close()

	if err := os.MkdirAll(destination, 0755); err != nil {
		return fmt.Errorf("create source directory: %w", err)
	}

	tarReader := tar.NewReader(gzipReader)
	var totalSize int64
	entries := 0
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read source archive: %w", err)
		}
		entries++
		if entries > maxArchiveEntries {
			return fmt.Errorf("source archive has too many entries")
		}

		if header.Name == "." || strings.Contains(header.Name, "\\") || !filepath.IsLocal(header.Name) {
			return fmt.Errorf("invalid source archive path %q", header.Name)
		}
		target := filepath.Join(destination, filepath.FromSlash(header.Name))

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("create source directory %q: %w", header.Name, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if header.Size < 0 || header.Size > maxExtractedSize-totalSize {
				return fmt.Errorf("source archive exceeds extraction size limit")
			}
			totalSize += header.Size
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("create parent directory for %q: %w", header.Name, err)
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, header.FileInfo().Mode().Perm()|0600)
			if err != nil {
				return fmt.Errorf("create source file %q: %w", header.Name, err)
			}
			_, copyErr := io.CopyN(file, tarReader, header.Size)
			closeErr := file.Close()
			if copyErr != nil {
				return fmt.Errorf("extract source file %q: %w", header.Name, copyErr)
			}
			if closeErr != nil {
				return fmt.Errorf("close source file %q: %w", header.Name, closeErr)
			}
		default:
			return fmt.Errorf("unsupported source archive entry %q", header.Name)
		}
	}
}
