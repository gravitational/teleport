package okta

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

// nullStatusUpdate implements an empty status updater that simply drops any
// updates it is given. Handy for tests that require a status updater to exist
// but don't actually care about the updates themselves.
type nullStatusUpdate struct{}

// UpdateUserSync implements serviceStatusUpdater
func (nullStatusUpdate) UpdateUserSync(context.Context, time.Time /* nUsers */, int, error) {
}

// UpdateAppGroupSync implements serviceStatusUpdater
func (nullStatusUpdate) UpdateAppGroupSync(context.Context, time.Time /* nApps*/, int /* nGroups */, int, error) {
}

// UpdateAccessListSync implements serviceStatusUpdater
func (nullStatusUpdate) UpdateAccessListSync(context.Context, time.Time /* nApps */, int /* nGroups */, int, error) {
}

// newTestServiceStatus creates a new serviceStatus instance bound to a mock
// status sink
func newTestServiceStatus() (*serviceStatus, *mockStatusSink) {
	sink := &mockStatusSink{}
	status := &serviceStatus{
		sink: sink,
		code: types.PluginStatusCode_UNKNOWN,
		details: &types.PluginOktaStatusV1{
			AppGroupSyncDetails: &types.PluginOktaStatusDetailsAppGroupSync{
				Enabled: true,
			},
			UsersSyncDetails: &types.PluginOktaStatusDetailsUsersSync{
				Enabled: true,
			},
			ScimDetails: &types.PluginOktaStatusDetailsSCIM{
				Enabled: true,
			},
			AccessListsSyncDetails: &types.PluginOktaStatusDetailsAccessListsSync{
				Enabled: true,
			},
			SystemLogExportDetails: &types.PluginOktaStatusSystemLogExporter{
				Enabled: true,
			},
		},
	}

	return status, sink
}

type mockStatusSink struct {
	status types.PluginStatus
}

func (emitter *mockStatusSink) Emit(ctx context.Context, s types.PluginStatus) error {
	emitter.status = s
	return nil
}

func TestServiceStatusErrorOverridesCode(t *testing.T) {
	ctx := context.Background()

	type errorSetter func(*serviceStatus, error)

	services := []struct {
		name     string
		setError errorSetter
	}{
		{
			name:     "Access List",
			setError: func(s *serviceStatus, err error) { s.UpdateAccessListSync(ctx, time.Time{}, 0, 0, err) },
		},
		{
			name:     "Users",
			setError: func(s *serviceStatus, err error) { s.UpdateUserSync(ctx, time.Time{}, 0, err) },
		},
		{
			name:     "App+Group Sync",
			setError: func(s *serviceStatus, err error) { s.UpdateAppGroupSync(ctx, time.Time{}, 0, 0, err) },
		},
	}

	testCases := []struct {
		name               string
		mutate             func(*testing.T, *serviceStatus, errorSetter)
		expectedStatusCode types.PluginStatusCode
	}{
		{
			name: "Running state is preserved with no error",
			mutate: func(_ *testing.T, s *serviceStatus, setErr errorSetter) {
				setErr(s, nil)
			},
			expectedStatusCode: types.PluginStatusCode_RUNNING,
		},
		{
			name: "Error overrides running state",
			mutate: func(_ *testing.T, s *serviceStatus, setErr errorSetter) {
				setErr(s, errors.New("some err"))
			},
			expectedStatusCode: types.PluginStatusCode_OTHER_ERROR,
		},
		{
			name: "Unauthorized error is recognized",
			mutate: func(_ *testing.T, s *serviceStatus, setErr errorSetter) {
				setErr(s, trace.AccessDenied("Nope. Not for you."))
			},
			expectedStatusCode: types.PluginStatusCode_UNAUTHORIZED,
		},
		{
			name: "Running state is reset after error is cleared",
			mutate: func(subtestT *testing.T, s *serviceStatus, setErr errorSetter) {
				mockSink := s.sink.(*mockStatusSink)

				setErr(s, errors.New("some err"))
				require.Equal(subtestT, types.PluginStatusCode_OTHER_ERROR, mockSink.status.GetCode())

				setErr(s, nil)
			},
			expectedStatusCode: types.PluginStatusCode_RUNNING,
		},
	}

	for _, svc := range services {
		t.Run(svc.name, func(t *testing.T) {
			for _, testCase := range testCases {
				t.Run(testCase.name, func(t *testing.T) {
					// GIVEN a service status struct in the RUNNING state
					status, sink := newTestServiceStatus()
					status.code = types.PluginStatusCode_RUNNING

					// WHEN I manipulate the serviceStatus by setting and/or
					// sync service clearing errors
					testCase.mutate(t, status, svc.setError)

					// EXPECT that the overall plugin state is updated to
					// reflect individual sync service state
					require.Equal(t, testCase.expectedStatusCode, sink.status.GetCode())
				})
			}
		})
	}
}

// TestServiceStatusHandlesMultipleErrors asserts that the status reported
// correctly handles multiple error conditions being set and cleared.
func TestServiceStatusHandlesMultipleErrors(t *testing.T) {
	ctx := context.Background()

	services := []struct {
		name     string
		setError func(*serviceStatus, error)
	}{
		{
			name:     "Access List",
			setError: func(s *serviceStatus, err error) { s.UpdateAccessListSync(ctx, time.Time{}, 0, 0, err) },
		},
		{
			name:     "Users",
			setError: func(s *serviceStatus, err error) { s.UpdateUserSync(ctx, time.Time{}, 0, err) },
		},
		{
			name:     "App+Group Sync",
			setError: func(s *serviceStatus, err error) { s.UpdateAppGroupSync(ctx, time.Time{}, 0, 0, err) },
		},
	}

	permutations := [][]int{
		{0, 1, 2},
		{0, 2, 1},
		{1, 0, 2},
		{1, 2, 0},
		{2, 0, 1},
		{2, 1, 0},
	}

	for _, p := range permutations {
		t.Run("", func(t *testing.T) {
			// GIVEN a service status struct in the RUNNING state
			status, sink := newTestServiceStatus()
			status.code = types.PluginStatusCode_RUNNING

			// WHEN I set the sync service error conditions one-by-one
			for _, svc := range p {
				t.Logf("Setting error: %q", services[svc].name)
				services[svc].setError(status, errors.New("Something bad happened"))

				// EXPECT that the plugin status reflects the error
				require.Equal(t, types.PluginStatusCode_OTHER_ERROR, sink.status.GetCode())
			}

			// WHEN I *clear* the sync service errors one-by-one (except the last one)
			for _, svc := range p[0 : len(p)-1] {
				t.Logf("Clearing error: %q", services[svc].name)
				services[svc].setError(status, nil)

				// EXPECT that the plugin status reflects the service errors
				require.Equal(t, types.PluginStatusCode_OTHER_ERROR, sink.status.GetCode())
			}

			// WHEN I clear the last error
			svc := p[len(p)-1]
			t.Logf("Clearing error: %q", services[svc].name)
			services[svc].setError(status, nil)

			// EXPECT that the plugin status is restored to the running state
			require.Equal(t, types.PluginStatusCode_RUNNING, sink.status.GetCode())
		})
	}
}

func TestServiceStatusDetectsTimeout(t *testing.T) {
	ctx := context.Background()

	services := []struct {
		name     string
		setError func(*serviceStatus, error)
		getError func(*mockStatusSink) string
	}{
		{
			name:     "Access List",
			setError: func(s *serviceStatus, err error) { s.UpdateAccessListSync(ctx, time.Time{}, 0, 0, err) },
			getError: func(s *mockStatusSink) string { return s.status.GetOkta().AccessListsSyncDetails.Error },
		},
		{
			name:     "Users",
			setError: func(s *serviceStatus, err error) { s.UpdateUserSync(ctx, time.Time{}, 0, err) },
			getError: func(s *mockStatusSink) string { return s.status.GetOkta().UsersSyncDetails.Error },
		},
		{
			name:     "App+Group Sync",
			setError: func(s *serviceStatus, err error) { s.UpdateAppGroupSync(ctx, time.Time{}, 0, 0, err) },
			getError: func(s *mockStatusSink) string { return s.status.GetOkta().AppGroupSyncDetails.Error },
		},
		{
			name: "system log exporter",
			setError: func(s *serviceStatus, err error) {
				s.UpdateSystemLogExporter(ctx, time.Time{}, err)
			},
			getError: func(s *mockStatusSink) string {
				return s.status.GetOkta().SystemLogExportDetails.Error
			},
		},
	}

	for _, svc := range services {
		t.Run(svc.name, func(t *testing.T) {
			// GIVEN an Okta service status struct in a non-error state
			status, sink := newTestServiceStatus()
			status.code = types.PluginStatusCode_RUNNING

			// WHEN I update one of the sync service statuses with a context
			// deadline exceeded error
			svc.setError(status, trace.Wrap(context.DeadlineExceeded))

			// EXPECT that the resulting error message contains text suggesting
			// that its probably an API rate limiting issue
			require.Contains(t, svc.getError(sink), deadlineExceededMessage)

			// ALSO EXPECT that the overall plugin status has been set to ERROR
			require.Equal(t, types.PluginStatusCode_OTHER_ERROR, sink.status.GetCode())
		})
	}
}
