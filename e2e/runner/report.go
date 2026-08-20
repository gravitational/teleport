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
	sha       string
	e2eDir    string
	tracePath string
	edition   string
}

func runReport(ctx context.Context, cfg *reportConfig) error {
	tmpDir, err := downloadArtifact(ctx, cfg, "playwright-report-"+cfg.edition)
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

func runTestResults(ctx context.Context, cfg *reportConfig) error {
	tmpDir, err := downloadArtifact(ctx, cfg, "test-results-"+cfg.edition)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	tracePath := filepath.Join(tmpDir, cfg.tracePath)
	slog.InfoContext(ctx, "opening trace", "path", tracePath)

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

func downloadArtifact(ctx context.Context, cfg *reportConfig, artifactName string) (string, error) {
	client, err := ghClient(ctx)
	if err != nil {
		return "", err
	}

	ghRepo := "gravitational/" + cfg.repo
	owner, repoName, _ := strings.Cut(ghRepo, "/")

	headSHA := cfg.sha
	if headSHA == "" {
		pr, _, err := client.PullRequests.Get(ctx, owner, repoName, cfg.prNumber)
		if err != nil {
			return "", fmt.Errorf("getting PR #%d: %w", cfg.prNumber, err)
		}

		headSHA = pr.GetHead().GetSHA()
		slog.DebugContext(ctx, "resolved PR head SHA", "sha", headSHA)
	} else {
		slog.DebugContext(ctx, "using provided SHA", "sha", headSHA)
	}

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
			if strings.HasPrefix(a.GetWorkflowRun().GetHeadSHA(), headSHA) {
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

	slog.DebugContext(ctx, "found artifact", "id", target.GetID(), "run_id", target.GetWorkflowRun().GetID())

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

	slog.InfoContext(ctx, "extracting artifact", "artifact", artifactName, "dir", tmpDir)

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

func ciShortHeadSHA() string {
	sha := os.Getenv("E2E_HEAD_SHA")
	if len(sha) > 8 {
		sha = sha[:8]
	}

	return sha
}

func ciReportCmd(pr int) string {
	cmd := fmt.Sprintf("e2e/run.sh --report %d", pr)
	// This only renders inside CI, where the edition is carried by the environment: the summary
	// writers that call it have no access to the parsed flags.
	if os.Getenv("E2E_ENTERPRISE") != "" {
		cmd += " -e"
	}
	if sha := ciShortHeadSHA(); sha != "" {
		cmd += " --sha " + sha
	}

	return cmd
}

const (
	editionOSS        = "oss"
	editionEnterprise = "ent"
)

// editionName names the half of the suite a run covers, matching the CI artifact name suffixes.
func editionName(enterprise bool) string {
	if enterprise {
		return editionEnterprise
	}

	return editionOSS
}

func detectRepo(e2eDir string) string {
	if filepath.Base(filepath.Dir(e2eDir)) == "e" {
		return "teleport.e"
	}

	return "teleport"
}
