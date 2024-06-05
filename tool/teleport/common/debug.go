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
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	debugclient "github.com/gravitational/teleport/lib/client/debug"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// DebugClient specifies the debug service client.
type DebugClient interface {
	// SetLogLevel changes the application's log level and a change status message.
	SetLogLevel(context.Context, string) (string, error)
	// GetLogLevel fetches the current log level.
	GetLogLevel(context.Context) (string, error)
	// CollectProfile collects a pprof profile.
	CollectProfile(context.Context, string, int) ([]byte, error)
}

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

func setLogLevel(ctx context.Context, clt DebugClient, level string) (string, error) {
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

func getLogLevel(ctx context.Context, clt DebugClient) (string, error) {
	return clt.GetLogLevel(ctx)
}

// defaultCollectProfileSeconds defines the default collect profiles seconds
// value.
const defaultCollectProfileSeconds = 10

// profilesWithoutSnapshotSupport list of profiles that DO NOT support taking
// snapshots (seconds = 0).
var profilesWithoutSnapshotSupport = map[string]struct{}{
	"profile": {},
	"trace":   {},
}

// defaultCollectProfiles defines the default profiles to be collected in case
// none is provided.
var defaultCollectProfiles = []string{"goroutine", "heap", "profile"}

func onCollectProfiles(configPath string, rawProfiles string, seconds int) error {
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	clt, dataDir, socketPath, err := newDebugClient(configPath)
	if err != nil {
		return trace.Wrap(err)
	}

	var output bytes.Buffer
	if err := collectProfiles(ctx, clt, &output, rawProfiles, seconds); err != nil {
		return convertToReadableErr(err, dataDir, socketPath)
	}

	fmt.Print(output.String())
	return nil
}

// collectProfiles collects the profiles and generate a compressed tarball
// file.
func collectProfiles(ctx context.Context, clt DebugClient, buf io.Writer, rawProfiles string, seconds int) error {
	profiles := defaultCollectProfiles
	if rawProfiles != "" {
		profiles = slices.Compact(strings.Split(rawProfiles, ","))
	}

	fileTime := time.Now()
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)

	for _, profile := range profiles {
		profileSeconds := seconds
		// When the profile doesn't support snapshot, we set a default seconds
		// value to avoid blocking users when they consume those profiles.
		if _, ok := profilesWithoutSnapshotSupport[profile]; seconds == 0 && ok {
			profileSeconds = 10
		}

		contents, err := clt.CollectProfile(ctx, profile, profileSeconds)
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
func newDebugClient(configPath string) (DebugClient, string, string, error) {
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

	socketPath := filepath.Join(dataDir, teleport.DebugServiceSocketName)
	return debugclient.NewClient(socketPath), dataDir, socketPath, nil
}

// convertToReadableErr converts debug service client error into a more friendly
// messages.
func convertToReadableErr(err error, dataDir, socketPath string) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, context.Canceled):
		return fmt.Errorf("Request canceled")
	case trace.IsConnectionProblem(err):
		return trace.BadParameter("Unable to reach debug service socket at %q."+
			"\n\nVerify if you have enough permissions to open the socket and if the path"+
			" to your data directory (%q) is correct. The command assumes the data"+
			" directory from your configuration file, you can provide the path to it using the --config flag.", socketPath, dataDir)
	default:
		return err
	}
}
