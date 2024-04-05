// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/srv/debug"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// debugServiceClient debug service client.
type debugServiceClient struct {
	clt *http.Client
}

// newDebugServiceClient generates a new debug service client.
func newDebugServiceClient(configPath string) (*debugServiceClient, error) {
	cfg, err := config.ReadConfigFile(configPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &debugServiceClient{
		clt: &http.Client{
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", filepath.Join(cfg.DataDir, debug.ServiceSocketName))
				},
			},
		},
	}, nil
}

// SetLogLevel changes the application's log level and a change status message.
func (c *debugServiceClient) SetLogLevel(ctx context.Context, level string) (string, error) {
	resp, err := c.do(ctx, debug.SetLogLevelMethod, debug.LogLevelEndpoint, []byte(level))
	if err != nil {
		return "", trace.Wrap(err)
	}

	respBody, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return "", trace.Wrap(err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", trace.BadParameter("Unable to change log level: %s", respBody)
	}

	return string(respBody), nil
}

// GetLogLevel fetches the current log level.
func (c *debugServiceClient) GetLogLevel(ctx context.Context) (string, error) {
	resp, err := c.do(ctx, debug.GetLogLevelMethod, debug.LogLevelEndpoint, []byte{})
	if err != nil {
		return "", trace.Wrap(err)
	}

	respBody, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return "", trace.Wrap(err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", trace.BadParameter("Unable to fetch log level: %s", respBody)
	}

	return string(respBody), nil
}

// CollectProfile collects a pprof profile.
func (c *debugServiceClient) CollectProfile(ctx context.Context, profileName string, seconds int) ([]byte, error) {
	path := debug.PProfEndpointsPrefix + profileName
	if seconds > 0 {
		path += fmt.Sprintf("?seconds=%d", seconds)
	}

	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, trace.BadParameter("Unable to collect profile %q: %s", profileName, result)
	}

	return result, nil
}

func (c *debugServiceClient) do(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, "http://debug"+path, bytes.NewBuffer(body))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := c.clt.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

func onSetLogLevel(configPath string, level string) error {
	ctx := context.Background()

	if contains := slices.Contains(logutils.SupportedLevelsText, strings.ToUpper(level)); !contains {
		return trace.BadParameter("%q log level not supported", level)
	}

	clt, err := newDebugServiceClient(configPath)
	if err != nil {
		return trace.Wrap(err)
	}

	setMessage, err := clt.SetLogLevel(ctx, level)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println(setMessage)
	return nil
}

func onGetLogLevel(configPath string) error {
	ctx := context.Background()
	clt, err := newDebugServiceClient(configPath)
	if err != nil {
		return trace.Wrap(err)
	}

	level, err := clt.GetLogLevel(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("Current log level %q\n", level)
	return nil
}

// supportedProfiles list of supported pprof profiles that can be collected.
// This list is composed by runtime/pprof.Profile and http/pprof definitions.
var supportedProfiles = map[string]struct{}{
	"allocs":       {},
	"block":        {},
	"cmdline":      {},
	"goroutine":    {},
	"heap":         {},
	"mutex":        {},
	"profile":      {},
	"threadcreate": {},
	"trace":        {},
}

// defaultCollectProfiles defines the default profiles to be collected in case
// none is provided.
var defaultCollectProfiles = []string{"goroutine", "heap", "profile"}

func onCollectProfile(configPath string, rawProfiles string, seconds int, out io.Writer) error {
	ctx := context.Background()

	profiles := defaultCollectProfiles
	if rawProfiles != "" {
		profiles = strings.Split(rawProfiles, ",")
	}

	for _, profile := range profiles {
		if _, ok := supportedProfiles[profile]; !ok {
			return trace.BadParameter("%q profile not supported", profile)
		}
	}

	clt, err := newDebugServiceClient(configPath)
	if err != nil {
		return trace.Wrap(err)
	}

	var output bytes.Buffer
	if err := createProfilesArchive(ctx, clt, &output, profiles, seconds); err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprint(out, output.String())
	return nil
}

// createProfileArchive collects the profiles and generate a compressed tarball
// file.
func createProfilesArchive(ctx context.Context, clt *debugServiceClient, buf io.Writer, profiles []string, seconds int) error {
	fileTime := time.Now()
	gw := gzip.NewWriter(buf)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	for _, profile := range profiles {
		contents, err := clt.CollectProfile(ctx, profile, seconds)
		if err != nil {
			return trace.Wrap(err)
		}

		hd := &tar.Header{
			Name:    profile + ".pprof",
			Size:    int64(len(contents)),
			Mode:    0600,
			ModTime: fileTime,
		}
		if err := tw.WriteHeader(hd); err != nil {
			return trace.Wrap(err)
		}
		if _, err := tw.Write(contents); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}
