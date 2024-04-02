/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package installer

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/automaticupgrades/basichttp"
	"github.com/gravitational/teleport/lib/utils"
)

const defaultTeleportCDNDomain = "cdn.teleport.dev"
const defaultTeleportChecksumDomain = "get.gravitational.com"

// Config specifies TeleportInstaller config
type Config struct {
	// TeleportBinDir specifies the directory where Teleport binaries are installed
	TeleportBinDir string
	// TeleportCDNDomain specifies the Teleport CDN domain
	TeleportCDNDomain string
	// TeleportChecksumDomain specifies the Teleport checksum domain
	TeleportChecksumDomain string
}

// CheckAndSetDefaults checks and sets default config
func (cfg *Config) CheckAndSetDefaults() error {
	if cfg.TeleportBinDir == "" {
		return trace.BadParameter("TeleportBinDir is required")
	}
	if cfg.TeleportCDNDomain == "" {
		cfg.TeleportCDNDomain = defaultTeleportCDNDomain
	}
	if cfg.TeleportChecksumDomain == "" {
		cfg.TeleportChecksumDomain = defaultTeleportChecksumDomain
	}
	return nil
}

// TeleportInstaller manages the installation of Teleport
type TeleportInstaller struct {
	// Config specifies the TeleportInstaller config values
	Config

	httpClient *basichttp.Client
}

// NewTeleportInstaller returns a new TeleportInstaller
func NewTeleportInstaller(cfg Config) (*TeleportInstaller, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &TeleportInstaller{
		Config:     cfg,
		httpClient: &basichttp.Client{Client: http.DefaultClient},
	}, nil
}

// InstallTeleport installs the desired Teleport binary
func (i *TeleportInstaller) InstallTeleport(ctx context.Context, req Request) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	tarballPath, err := i.downloadTeleport(ctx, selectArtifact(req))
	if err != nil {
		return trace.Wrap(err, "failed to download Teleport")
	}
	defer os.RemoveAll(tarballPath)

	tarballFile, err := os.Open(tarballPath)
	if err != nil {
		return trace.Wrap(err)
	}
	defer tarballFile.Close()

	dst := strings.TrimSuffix(tarballPath, ".tar.gz")
	if err := utils.ExtractGzip(tarballFile, dst); err != nil {
		return trace.Wrap(err, "failed to extract Teleport tarball")
	}
	defer os.RemoveAll(dst)

	// Install the teleport binary into the local teleport bin directory
	err = os.Rename(filepath.Join(dst, req.Flavor, "teleport"), filepath.Join(i.TeleportBinDir, "teleport"))
	return trace.Wrap(err, "failed to install into Teleport bin directory")
}

func (i *TeleportInstaller) downloadTeleport(ctx context.Context, artifact string) (string, error) {
	// Download checksum
	checksumURL, err := i.checksumURL(artifact)
	if err != nil {
		return "", trace.Wrap(err)
	}
	checksumBuf := new(bytes.Buffer)
	if err := i.httpClient.Download(ctx, checksumBuf, checksumURL); err != nil {
		return "", trace.Wrap(err)
	}

	// Download tarball
	tarballDst, err := os.Create(filepath.Join(os.TempDir(), artifact))
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer tarballDst.Close()

	tarballURL, err := i.tarballURL(artifact)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if err := i.httpClient.Download(ctx, tarballDst, tarballURL); err != nil {
		return "", trace.Wrap(err)
	}

	// Verify checksum
	if _, err := tarballDst.Seek(0, 0); err != nil {
		return "", trace.Wrap(err)
	}
	if err := verifyChecksum(artifact, tarballDst, checksumBuf); err != nil {
		return "", trace.Wrap(err)
	}

	return tarballDst.Name(), nil
}

// tarballURL returns the url for the tarball download
// The tarball file is in the format `teleport-ent-v15.1.10-linux-arm64-bin.tar.gz`
func (i *TeleportInstaller) tarballURL(artifact string) (*url.URL, error) {
	tarballURL, err := url.Parse(fmt.Sprintf("https://%s", i.TeleportCDNDomain))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return tarballURL.JoinPath(artifact), nil
}

// checksumURL returns the url for the checksum download
// The checksum file is in the format `teleport-ent-v15.1.10-linux-arm64-bin.tar.gz.sha256`
func (i *TeleportInstaller) checksumURL(artifact string) (*url.URL, error) {
	checksumURL, err := url.Parse(fmt.Sprintf("https://%s", i.TeleportChecksumDomain))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return checksumURL.JoinPath(fmt.Sprintf("%s.sha256", artifact)), nil
}

// Request specifies the Teleport installation request
type Request struct {
	// Version specifies the Teleport version
	Version string
	// Arch specifies the system architecture
	Arch string
	// OS specifies the system OS
	OS string
	// Flavor specifies the Teleport flavor. OSS or Enterprise
	Flavor string
}

// CheckAndSetDefaults checks and sets default config
func (r *Request) CheckAndSetDefaults() error {
	if r.Version == "" {
		return trace.BadParameter("version is required")
	}
	if r.Arch == "" {
		return trace.BadParameter("arch is required")
	}
	if r.OS == "" {
		return trace.BadParameter("OS is required")
	}
	if r.Flavor == "" {
		return trace.BadParameter("flavor is required")
	}
	return nil
}

// selectArtifact constructs the Teleport artifact name from the specified request.
// The tarball file is in the format `teleport-ent-v15.1.10-linux-arm64-bin.tar.gz`.
func selectArtifact(req Request) string {
	return fmt.Sprintf("%s-%s-%s-%s-bin.tar.gz", req.Flavor, req.Version, req.OS, req.Arch)
}

// verifyChecksum verifies the sha256 checksum of the tarball matches the expected checksum.
func verifyChecksum(filename string, tarballReader io.Reader, checksumReader io.Reader) error {
	// The sha256 files are expected in the format: <checksum> <filename>
	checksums := make(map[string]string)
	scanner := bufio.NewScanner(checksumReader)
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())

		if len(parts) != 2 {
			continue
		}

		checksum := parts[0]
		filename := parts[1]

		if _, exists := checksums[checksum]; exists {
			return trace.Errorf("found duplicate checksum %s", checksum)
		}
		checksums[checksum] = filename
	}

	// Generate sha256 checksum for the tarball
	hash := sha256.New()
	if _, err := io.Copy(hash, tarballReader); err != nil {
		return trace.Wrap(err)
	}
	checksum := hex.EncodeToString(hash.Sum(nil))

	// Verify checksum matches filename
	expected, ok := checksums[checksum]
	if !ok {
		return trace.Errorf("matching checksum not found: %s", checksum)
	}
	if filename != expected {
		return trace.Errorf("checksum does not match filename. expected %s, actual %s", expected, filename)
	}
	return nil
}
