/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gravitational/teleport/integration/helpers/archive"
)

// Source is used for defining Teleport source directory by local ID.
type Source struct {
	ID   int
	Path string
}

// VersionInfo defines what version label must be used during compilation.
type VersionInfo struct {
	Version  string
	SourceID int
}

var sources = []Source{
	// Path to Teleport source code with previous v1 version of CTMU.
	{ID: 1, Path: "../teleport-sync"},
	// Path to Teleport source code with latest version of CTMU.
	{ID: 2, Path: "./"},
}

var versions = []VersionInfo{
	{"17.5.1", 1},
	{"17.5.4", 1},
	{"17.5.7", 1},
	{"18.0.0", 2},
	{"18.5.5", 2},
	{"18.5.7", 2},
}

func main() {
	ctx := context.Background()
	outputDir := "./test-packages"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		slog.ErrorContext(ctx, "Failed to create output directory: %v", err)
		return
	}

	for _, v := range versions {
		source := findSourceByID(v.SourceID)
		if source == nil {
			slog.ErrorContext(ctx, "No source with ID %d", v.SourceID)
			return
		}

		fmt.Printf("Processing version %s from path %s\n", v.Version, source.Path)
		if err := processVersion(v, *source, outputDir); err != nil {
			slog.ErrorContext(ctx, "Error processing version %s: %v", v.Version, err)
			return
		}
	}

	fmt.Printf("\nServing %s on http://localhost:8080\n", outputDir)
	http.Handle("/", http.FileServer(http.Dir(outputDir)))
	if err := http.ListenAndServe(":8080", nil); err != nil {
		slog.ErrorContext(ctx, "Failed to start server: %v", err)
	}
}

func findSourceByID(id int) *Source {
	for _, s := range sources {
		if s.ID == id {
			return &s
		}
	}
	return nil
}

func processVersion(v VersionInfo, source Source, outputDir string) error {
	ctx := context.Background()
	versionDir := filepath.Join(outputDir, "v"+v.Version)
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return fmt.Errorf("create version dir: %w", err)
	}

	versionFile := filepath.Join(source.Path, "api/version.go")
	if err := updateVersionFile(versionFile, v.Version); err != nil {
		return fmt.Errorf("update version file: %w", err)
	}

	if err := runMake(source.Path); err != nil {
		return fmt.Errorf("make build: %w", err)
	}

	for _, bin := range []string{"tsh", "tctl"} {
		src := filepath.Join(source.Path, "build", bin)
		dst := filepath.Join(versionDir, bin)
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copy %s: %w", bin, err)
		}
	}

	archivePath := filepath.Join(outputDir, fmt.Sprintf("teleport-%s.pkg", v.Version))
	if err := archive.CompressDirToPkgFile(ctx, versionDir, archivePath, "com.example.pkgtest"); err != nil {
		return fmt.Errorf("compress dir to pkgfile: %w", err)
	}
	if err := writeSHA256(archivePath); err != nil {
		return fmt.Errorf("sha256sum: %w", err)
	}

	archivePath = filepath.Join(outputDir, fmt.Sprintf("teleport-v%s-windows-amd64-bin.zip", v.Version))
	if err := archive.CompressDirToZipFile(ctx, versionDir, archivePath); err != nil {
		return fmt.Errorf("compress dir to zipfile: %w", err)
	}
	if err := writeSHA256(archivePath); err != nil {
		return fmt.Errorf("sha256sum: %w", err)
	}

	archivePath = filepath.Join(outputDir, fmt.Sprintf("teleport-v%s-linux-%s-bin.tar.gz", v.Version, runtime.GOARCH))
	if err := archive.CompressDirToTarGzFile(ctx, versionDir, archivePath); err != nil {
		return fmt.Errorf("compress dir to tarfile: %w", err)
	}
	if err := writeSHA256(archivePath); err != nil {
		return fmt.Errorf("sha256sum: %w", err)
	}

	return nil
}

func updateVersionFile(path, version string) error {
	tmpPath := path + ".tmp"
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer out.Close()

	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "const Version = ") {
			fmt.Fprintf(out, "const Version = \"%s\"\n", version)
		} else {
			fmt.Fprintln(out, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func runMake(dir string) error {
	cmd := exec.Command("make", "build/tsh", "build/tctl")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GITHUB_REPOSITORY_OWNER=gravitational")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func copyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	srcInfo, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return err
	}

	return out.Close()
}

func writeSHA256(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	sum := hex.EncodeToString(hash.Sum(nil))
	shaPath := path + ".sha256"

	return os.WriteFile(shaPath, []byte(sum+"  "+filepath.Base(path)+"\n"), 0644)
}
