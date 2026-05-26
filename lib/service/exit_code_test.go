package service

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

type fakeRunProcess struct {
	startErr error
	waitErr  error
}

type foreignExitCodeErr struct{}

func (foreignExitCodeErr) Error() string {
	return "foreign exit code"
}

func (foreignExitCodeErr) ExitCode() int {
	return 77
}

func (f *fakeRunProcess) Close() error {
	return nil
}

func (f *fakeRunProcess) Start() error {
	return f.startErr
}

func (f *fakeRunProcess) WaitForSignals(context.Context, <-chan os.Signal) error {
	return f.waitErr
}

func (f *fakeRunProcess) ExportFileDescriptors() ([]*servicecfg.FileDescriptor, error) {
	return nil, nil
}

func (f *fakeRunProcess) Shutdown(context.Context) {}

func (f *fakeRunProcess) WaitForEvent(context.Context, string) (Event, error) {
	return Event{}, nil
}

func (f *fakeRunProcess) WaitWithContext(context.Context) {}

func TestRunWithSignalChannel_InitializationFailureExitCode(t *testing.T) {
	err := RunWithSignalChannel(context.Background(), servicecfg.Config{}, func(*servicecfg.Config) (Process, error) {
		return nil, errors.New("init failed")
	}, nil)
	require.Error(t, err)
	exitCode, ok := ErrorExitCode(err)
	require.True(t, ok)
	require.Equal(t, TeleportExitCodeBeforeReady, exitCode)
}

func TestRunWithSignalChannel_StartFailureExitCode(t *testing.T) {
	err := RunWithSignalChannel(context.Background(), servicecfg.Config{}, func(*servicecfg.Config) (Process, error) {
		return &fakeRunProcess{startErr: errors.New("start failed")}, nil
	}, nil)
	require.Error(t, err)
	exitCode, ok := ErrorExitCode(err)
	require.True(t, ok)
	require.Equal(t, TeleportExitCodeBeforeReady, exitCode)
}

func TestErrorExitCode_OnlyMatchesServiceExitCodeError(t *testing.T) {
	exitCode, ok := ErrorExitCode(foreignExitCodeErr{})
	require.False(t, ok)
	require.Zero(t, exitCode)
}

func TestWaitForSignals_CriticalServiceBeforeReadyExitCode(t *testing.T) {
	process := newTestProcessForExitCodes(t)
	process.BroadcastEvent(Event{
		Name: ServiceExitedWithErrorEvent,
		Payload: ExitEventPayload{
			Service: &LocalService{ServiceName: "critical", Critical: true},
			Error:   errors.New("critical service failed"),
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := process.WaitForSignals(ctx, nil)
	require.Error(t, err)
	exitCode, ok := ErrorExitCode(err)
	require.True(t, ok)
	require.Equal(t, TeleportExitCodeBeforeReady, exitCode)
}

func TestWaitForSignals_CriticalServiceAfterReadyExitCode(t *testing.T) {
	process := newTestProcessForExitCodes(t)
	process.BroadcastEvent(Event{Name: TeleportReadyEvent})
	process.BroadcastEvent(Event{
		Name: ServiceExitedWithErrorEvent,
		Payload: ExitEventPayload{
			Service: &LocalService{ServiceName: "critical", Critical: true},
			Error:   errors.New("critical service failed"),
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := process.WaitForSignals(ctx, nil)
	require.Error(t, err)
	exitCode, ok := ErrorExitCode(err)
	require.True(t, ok)
	require.Equal(t, TeleportExitCodeAfterReady, exitCode)
}

func newTestProcessForExitCodes(t *testing.T) *TeleportProcess {
	t.Helper()
	logger := logtest.NewLogger()
	supervisor, err := NewSupervisor("test-exit-codes", logger, clockwork.NewRealClock())
	require.NoError(t, err)

	process := &TeleportProcess{
		Supervisor: supervisor,
		logger:     logger,
	}

	t.Cleanup(func() {
		require.NoError(t, process.Wait())
	})

	return process
}
