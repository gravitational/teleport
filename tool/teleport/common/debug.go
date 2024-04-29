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
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"

	debugclient "github.com/gravitational/teleport/lib/client/debug"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/debug"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

func onSetLogLevel(configPath string, level string) error {
	ctx := context.Background()
	clt, dataDir, socketPath, err := newDebugClient(configPath)
	if err != nil {
		return trace.Wrap(err)
	}

	setMessage, err := setLogLevel(ctx, clt, level)
	if err != nil {
		return convertToReadableErr(err, dataDir, socketPath)
	}

	fmt.Println(setMessage)
	return nil
}

func setLogLevel(ctx context.Context, clt debugclient.Client, level string) (string, error) {
	if contains := slices.Contains(logutils.SupportedLevelsText, strings.ToUpper(level)); !contains {
		return "", trace.BadParameter("%q log level not supported", level)
	}

	return clt.SetLogLevel(ctx, level)
}

func onGetLogLevel(configPath string) error {
	ctx := context.Background()
	clt, dataDir, socketPath, err := newDebugClient(configPath)
	if err != nil {
		return trace.Wrap(err)
	}

	currentLogLevel, err := getLogLevel(ctx, clt)
	if err != nil {
		return convertToReadableErr(err, dataDir, socketPath)
	}

	fmt.Printf("Current log level %q\n", currentLogLevel)
	return nil
}

func getLogLevel(ctx context.Context, clt debugclient.Client) (string, error) {
	return clt.GetLogLevel(ctx)
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
	clt, dataDir, socketPath, err := newDebugClient(configPath)
	if err != nil {
		return trace.Wrap(err)
	}

	var output bytes.Buffer
	if err := collectProfiles(ctx, clt, &output, rawProfiles, seconds); err != nil {
		return convertToReadableErr(err, dataDir, socketPath)
	}

	fmt.Fprint(out, output.String())
	return nil
}

// collectProfiles collects the profiles and generate a compressed tarball
// file.
func collectProfiles(ctx context.Context, clt debugclient.Client, buf io.Writer, rawProfiles string, seconds int) error {
	profiles := defaultCollectProfiles
	if rawProfiles != "" {
		profiles = strings.Split(rawProfiles, ",")
	}

	for _, profile := range profiles {
		if _, ok := supportedProfiles[profile]; !ok {
			return trace.BadParameter("%q profile not supported", profile)
		}
	}

	fileTime := time.Now()
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)

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

	return trace.NewAggregate(tw.Close(), gw.Close())
}

// newDebugClient initializes the debug client based on the Teleport
// configuration. It also returns the data dir and socket path used.
func newDebugClient(configPath string) (debugclient.Client, string, string, error) {
	cfg, err := config.ReadConfigFile(configPath)
	if err != nil {
		return nil, "", "", trace.Wrap(err)
	}

	// ReadConfigFile returns nil configuration if the file doesn't exists.
	// In that case, fallback to default data dir path.
	dataDir := defaults.DataDir
	if cfg != nil {
		dataDir = cfg.DataDir
	}

	socketPath := filepath.Join(dataDir, debug.ServiceSocketName)
	return debugclient.NewClient(socketPath), dataDir, socketPath, nil
}

// convertToReadableErr converts debug service client error into a more friendly
// messages.
func convertToReadableErr(err error, dataDir, socketPath string) error {
	if err == nil {
		return nil
	}

	if !trace.IsConnectionProblem(err) {
		return err
	}

	return trace.BadParameter("Unable to reach debug service socket at %q."+
		"\n\nVerify if you have enough permissions to open the socket and if the path"+
		" to your data directory (%q) is correct. The command assumes the data"+
		" directory from your configuration file, you can provide the path to it using the --config flag.", socketPath, dataDir)
}
