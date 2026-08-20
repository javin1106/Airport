package gitrepo

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Clone(ctx context.Context, repoURL, destination string) error {
	parsedURL, err := url.ParseRequestURI(repoURL)
	if err != nil {
		return fmt.Errorf("invalid repository URL: %w", err)
	}

	if parsedURL.Scheme != "https" || !strings.EqualFold(parsedURL.Hostname(), "github.com") {
		return fmt.Errorf("only HTTPS github repositories are supported")
	}

	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	command := exec.CommandContext(
		ctx,
		"git",
		"clone",
		"--depth",
		"1",
		"--",
		repoURL,
		destination,
	)

	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"git clone failed: %w: %s",
			err,
			strings.TrimSpace(string(output)),
		)
	}

	return nil
}
