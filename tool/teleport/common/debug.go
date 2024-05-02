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
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/stopwatch"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

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

func onCollectProfiles(configPath string, rawProfiles string, seconds int) error {
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	clt, dataDir, socketPath, err := newDebugClient(configPath)
	if err != nil {
		return trace.Wrap(err)
	}

	teaProgram := tea.NewProgram(newCollectModel(cancelFunc), tea.WithOutput(os.Stderr))
	var output bytes.Buffer
	errGroup, ctx := errgroup.WithContext(ctx)
	errGroup.Go(func() error {
		return collectProfiles(ctx, clt, &output, rawProfiles, seconds, func(rem int) {
			teaProgram.Send(collectProgressMessage(rem))
		})
	})

	if _, err := teaProgram.Run(); err != nil {
		return trace.Wrap(err)
	}

	if err := errGroup.Wait(); err != nil {
		return convertToReadableErr(err, dataDir, socketPath)
	}

	fmt.Print(output.String())
	return nil
}

const (
	padding  = 2
	maxWidth = 80
)

// progressFunc a function used to report collect profiles progress.
type progressFunc func(int)

// collectProgressMessage the collect progress message.
type collectProgressMessage int

// collectModel represents the progress TUI of collect profiles.
type collectModel struct {
	cancel         context.CancelFunc
	quitKeyBinding key.Binding
	progress       progress.Model
	stopwatch      stopwatch.Model
	percent        float64
	total          float64
	done           bool
}

func newCollectModel(cancelFunc context.CancelFunc) tea.Model {
	return &collectModel{
		cancel:    cancelFunc,
		progress:  progress.New(),
		stopwatch: stopwatch.NewWithInterval(time.Second),
		total:     -1,
		quitKeyBinding: key.NewBinding(
			key.WithKeys("esc", "ctrl+c", "q"),
		),
	}
}

func (m *collectModel) Init() tea.Cmd {
	return m.stopwatch.Init()
}

func (m *collectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.stopwatch, cmd = m.stopwatch.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, m.quitKeyBinding) {
			cmd = tea.Quit
			m.cancel()
		}
	case tea.WindowSizeMsg:
		m.progress.Width = msg.Width - padding*2 - 4
		if m.progress.Width > maxWidth {
			m.progress.Width = maxWidth
		}
	case collectProgressMessage:
		if msg <= 0 {
			m.done = true
			cmd = tea.Quit
			break
		}

		// First time receiving the progress, we set the total.
		if m.total == -1 {
			m.total = float64(msg + 1)
		}

		m.percent = float64(msg) / m.total
	}

	return m, cmd
}

func (m *collectModel) View() string {
	if m.done {
		return ""
	}

	pad := strings.Repeat(" ", padding)
	return "\n" +
		pad + m.progress.ViewAs(m.percent) + "\n" +
		pad + m.stopwatch.View()
}

// collectProfiles collects the profiles and generate a compressed tarball
// file.
func collectProfiles(ctx context.Context, clt DebugClient, buf io.Writer, rawProfiles string, seconds int, progress progressFunc) (err error) {
	defer func() {
		if err != nil {
			progress(-1)
		}
	}()

	profiles := defaultCollectProfiles
	if rawProfiles != "" {
		profiles = strings.Split(rawProfiles, ",")
	}

	for _, profile := range profiles {
		if _, ok := supportedProfiles[profile]; !ok {
			return trace.BadParameter("%q profile not supported", profile)
		}
	}

	rem := len(profiles)
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

		rem--
		progress(rem)
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
