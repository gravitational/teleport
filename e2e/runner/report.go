/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/go-github/v84/github"
	"golang.org/x/oauth2"
)

type reportConfig struct {
	prNumber  int
	repo      string
	e2eDir    string
	tracePath string
}

func runReport(cfg *reportConfig) error {
	tmpDir, err := downloadArtifact(cfg.prNumber, cfg.repo, "playwright-report")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	showCmd := exec.Command("pnpm", "exec", "playwright", "show-report", tmpDir, "--port", "0")
	showCmd.Dir = cfg.e2eDir
	showCmd.Stdin = os.Stdin
	showCmd.Stdout = os.Stdout
	showCmd.Stderr = os.Stderr

	return showCmd.Run()
}

func runTestResults(cfg *reportConfig) error {
	tmpDir, err := downloadArtifact(cfg.prNumber, cfg.repo, "test-results")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	tracePath := filepath.Join(tmpDir, cfg.tracePath)
	slog.Info("opening trace", "path", tracePath)

	showCmd := exec.Command("pnpm", "exec", "playwright", "show-trace", tracePath)
	showCmd.Dir = cfg.e2eDir
	showCmd.Stdin = os.Stdin
	showCmd.Stdout = os.Stdout
	showCmd.Stderr = os.Stderr

	return showCmd.Run()
}

func ghClient(ctx context.Context) (*github.Client, error) {
	var stdout, stderr bytes.Buffer

	cmd := exec.Command("gh", "auth", "token")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("getting gh auth token: %s\nPlease login using \"gh auth login\"", stderr.String())
	}

	token := strings.TrimSpace(stdout.String())
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})

	return github.NewClient(oauth2.NewClient(ctx, ts)), nil
}

func downloadArtifact(prNumber int, repo, artifactName string) (string, error) {
	ctx := context.Background()
	client, err := ghClient(ctx)
	if err != nil {
		return "", err
	}

	ghRepo := "gravitational/" + repo
	owner, repoName, _ := strings.Cut(ghRepo, "/")

	pr, _, err := client.PullRequests.Get(ctx, owner, repoName, prNumber)
	if err != nil {
		return "", fmt.Errorf("getting PR #%d: %w", prNumber, err)
	}

	headSHA := pr.GetHead().GetSHA()
	slog.Debug("resolved PR head SHA", "sha", headSHA)

	opts := &github.ListArtifactsOptions{
		Name: github.Ptr(artifactName),
	}

	var target *github.Artifact
	for {
		artifacts, resp, err := client.Actions.ListArtifacts(ctx, owner, repoName, opts)
		if err != nil {
			return "", fmt.Errorf("listing artifacts: %w", err)
		}

		for _, a := range artifacts.Artifacts {
			if a.GetWorkflowRun().GetHeadSHA() == headSHA {
				target = a

				break
			}
		}

		if target != nil || resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	if target == nil {
		return "", fmt.Errorf("no artifact %q found for PR head SHA %q", artifactName, headSHA)
	}

	slog.Debug("found artifact", "id", target.GetID(), "run_id", target.GetWorkflowRun().GetID())

	url, _, err := client.Actions.DownloadArtifact(ctx, owner, repoName, target.GetID(), 3)
	if err != nil {
		return "", fmt.Errorf("getting artifact download URL: %w", err)
	}

	resp, err := http.Get(url.String())
	if err != nil {
		return "", fmt.Errorf("downloading artifact: %w", err)
	}
	defer resp.Body.Close()

	zipFile, err := os.CreateTemp("", artifactName+"-*.zip")
	if err != nil {
		return "", fmt.Errorf("creating temp zip file: %w", err)
	}
	defer os.Remove(zipFile.Name())
	defer zipFile.Close()

	if _, err := io.Copy(zipFile, resp.Body); err != nil {
		return "", fmt.Errorf("downloading artifact to disk: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", artifactName+"-*")
	if err != nil {
		return "", fmt.Errorf("creating temp directory: %w", err)
	}

	slog.Info("extracting artifact", "artifact", artifactName, "dir", tmpDir)

	zr, err := zip.OpenReader(zipFile.Name())
	if err != nil {
		return "", fmt.Errorf("opening artifact zip: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		dest := filepath.Join(tmpDir, f.Name)

		if !strings.HasPrefix(filepath.Clean(dest), filepath.Clean(tmpDir)+string(os.PathSeparator)) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(dest, 0o755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return "", fmt.Errorf("creating directory for %s: %w", f.Name, err)
		}

		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("opening zip entry %s: %w", f.Name, err)
		}

		out, err := os.Create(dest)
		if err != nil {
			rc.Close()
			return "", fmt.Errorf("creating file %s: %w", f.Name, err)
		}

		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()

		if err != nil {
			return "", fmt.Errorf("extracting %s: %w", f.Name, err)
		}
	}

	return tmpDir, nil
}

func ciPRNumber() int {
	ref := os.Getenv("GITHUB_REF")

	// refs/pull/<number>/merge
	parts := strings.SplitN(ref, "/", 4)
	if len(parts) >= 3 && parts[1] == "pull" {
		n, _ := strconv.Atoi(parts[2])

		return n
	}

	return 0
}

func detectRepo(e2eDir string) string {
	if filepath.Base(filepath.Dir(e2eDir)) == "e" {
		return "teleport.e"
	}

	return "teleport"
}
