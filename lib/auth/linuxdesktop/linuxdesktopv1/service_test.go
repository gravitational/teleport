/*
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

package linuxdesktopv1

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
	linuxdesktopv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

type check struct {
	kind string
	verb string
}

type fakeChecker struct {
	services.AccessChecker
	allow  map[check]bool
	checks []check
}

func (f *fakeChecker) CheckAccessToRule(_ services.RuleContext, _ string, kind string, verb string) error {
	c := check{kind: kind, verb: verb}
	f.checks = append(f.checks, c)
	if f.allow[c] {
		return nil
	}
	return trace.AccessDenied("access to %s with verb %s is not allowed", kind, verb)
}

func (f *fakeChecker) CheckAccess(services.AccessCheckable, services.AccessState, ...services.RoleMatcher) error {
	return nil
}

type fakeAuthorizer struct {
	checker    *fakeChecker
	adminState authz.AdminActionAuthState
}

func (f *fakeAuthorizer) Authorize(_ context.Context) (*authz.Context, error) {
	return &authz.Context{
		Checker:              f.checker,
		AdminActionAuthState: f.adminState,
	}, nil
}

type fakeBackend struct {
	createCalled bool
	createReq    *linuxdesktopv1pb.LinuxDesktop
	createResp   *linuxdesktopv1pb.LinuxDesktop
	createErr    error

	updateCalled bool
	updateReq    *linuxdesktopv1pb.LinuxDesktop
	updateResp   *linuxdesktopv1pb.LinuxDesktop
	updateErr    error

	upsertCalled bool
	upsertReq    *linuxdesktopv1pb.LinuxDesktop
	upsertResp   *linuxdesktopv1pb.LinuxDesktop
	upsertErr    error

	deleteCalled bool
	deleteName   string
	deleteErr    error
}

func (f *fakeBackend) CreateLinuxDesktop(_ context.Context, desktop *linuxdesktopv1pb.LinuxDesktop) (*linuxdesktopv1pb.LinuxDesktop, error) {
	f.createCalled = true
	f.createReq = desktop
	if f.createResp != nil {
		return f.createResp, f.createErr
	}
	return desktop, f.createErr
}

func (f *fakeBackend) UpdateLinuxDesktop(_ context.Context, desktop *linuxdesktopv1pb.LinuxDesktop) (*linuxdesktopv1pb.LinuxDesktop, error) {
	f.updateCalled = true
	f.updateReq = desktop
	if f.updateResp != nil {
		return f.updateResp, f.updateErr
	}
	return desktop, f.updateErr
}

func (f *fakeBackend) UpsertLinuxDesktop(_ context.Context, desktop *linuxdesktopv1pb.LinuxDesktop) (*linuxdesktopv1pb.LinuxDesktop, error) {
	f.upsertCalled = true
	f.upsertReq = desktop
	if f.upsertResp != nil {
		return f.upsertResp, f.upsertErr
	}
	return desktop, f.upsertErr
}

func (f *fakeBackend) DeleteLinuxDesktop(_ context.Context, name string) error {
	f.deleteCalled = true
	f.deleteName = name
	return f.deleteErr
}

func (f *fakeBackend) ListLinuxDesktops(context.Context, int, string) ([]*linuxdesktopv1pb.LinuxDesktop, string, error) {
	return nil, "", nil
}

func (f *fakeBackend) GetLinuxDesktop(context.Context, string) (*linuxdesktopv1pb.LinuxDesktop, error) {
	return nil, nil
}

type fakeReader struct {
	listCalled   bool
	listPageSize int
	listToken    string
	listResp     []*linuxdesktopv1pb.LinuxDesktop
	listNext     string
	listErr      error

	getCalled bool
	getName   string
	getResp   *linuxdesktopv1pb.LinuxDesktop
	getErr    error
}

func (f *fakeReader) ListLinuxDesktops(_ context.Context, pageSize int, nextToken string) ([]*linuxdesktopv1pb.LinuxDesktop, string, error) {
	f.listCalled = true
	f.listPageSize = pageSize
	f.listToken = nextToken
	return f.listResp, f.listNext, f.listErr
}

func (f *fakeReader) GetLinuxDesktop(_ context.Context, name string) (*linuxdesktopv1pb.LinuxDesktop, error) {
	f.getCalled = true
	f.getName = name
	return f.getResp, f.getErr
}

func newTestService(t *testing.T, checker *fakeChecker, adminState authz.AdminActionAuthState, backend *fakeBackend, reader *fakeReader) *Service {
	t.Helper()

	svc, err := NewService(ServiceConfig{
		Authorizer: &fakeAuthorizer{
			checker:    checker,
			adminState: adminState,
		},
		Backend: backend,
		Reader:  reader,
		Emitter: events.NewDiscardEmitter(),
	})
	require.NoError(t, err)
	return svc
}

func allowChecks(checks ...check) map[check]bool {
	allow := make(map[check]bool, len(checks))
	for _, c := range checks {
		allow[c] = true
	}
	return allow
}

func newTestDesktop(t *testing.T, name string) *linuxdesktopv1pb.LinuxDesktop {
	t.Helper()

	desktop, err := NewLinuxDesktop(name, &linuxdesktopv1pb.LinuxDesktopSpec{
		Addr:     "127.0.0.1:22",
		Hostname: "desktop-host",
	})
	require.NoError(t, err)
	return desktop
}

func TestServiceListLinuxDesktops(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	desktop := newTestDesktop(t, "desktop-1")

	reader := &fakeReader{
		listResp: []*linuxdesktopv1pb.LinuxDesktop{desktop},
		listNext: "",
	}
	backend := &fakeBackend{}
	checker := &fakeChecker{
		allow: allowChecks(
			check{kind: types.KindLinuxDesktop, verb: types.VerbList},
			check{kind: types.KindLinuxDesktop, verb: types.VerbRead},
		),
	}
	service := newTestService(t, checker, authz.AdminActionAuthNotRequired, backend, reader)

	resp, err := service.ListLinuxDesktops(ctx, &linuxdesktopv1pb.ListLinuxDesktopsRequest{
		PageSize:  10,
		PageToken: "next-token",
	})
	require.NoError(t, err)
	require.Equal(t, []*linuxdesktopv1pb.LinuxDesktop{desktop}, resp.GetLinuxDesktops())
	require.Empty(t, resp.GetNextPageToken())
	require.True(t, reader.listCalled)
	require.Equal(t, defaults.DefaultChunkSize, reader.listPageSize)
	require.Empty(t, reader.listToken)
	require.Equal(t, []check{
		{kind: types.KindLinuxDesktop, verb: types.VerbList},
		{kind: types.KindLinuxDesktop, verb: types.VerbRead},
	}, checker.checks)
}

func TestServiceGetLinuxDesktop(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	desktop := newTestDesktop(t, "desktop-1")

	reader := &fakeReader{
		getResp: desktop,
	}
	backend := &fakeBackend{}
	checker := &fakeChecker{
		allow: allowChecks(
			check{kind: types.KindLinuxDesktop, verb: types.VerbRead},
		),
	}
	service := newTestService(t, checker, authz.AdminActionAuthNotRequired, backend, reader)

	resp, err := service.GetLinuxDesktop(ctx, &linuxdesktopv1pb.GetLinuxDesktopRequest{
		Name: "desktop-1",
	})
	require.NoError(t, err)
	require.Equal(t, desktop, resp)
	require.True(t, reader.getCalled)
	require.Equal(t, "desktop-1", reader.getName)
	require.Equal(t, []check{
		{kind: types.KindLinuxDesktop, verb: types.VerbRead},
	}, checker.checks)
}

func TestServiceCreateLinuxDesktop(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	desktop := newTestDesktop(t, "desktop-1")

	reader := &fakeReader{}
	backend := &fakeBackend{createResp: desktop}
	checker := &fakeChecker{
		allow: allowChecks(
			check{kind: types.KindLinuxDesktop, verb: types.VerbCreate},
		),
	}
	service := newTestService(t, checker, authz.AdminActionAuthNotRequired, backend, reader)

	resp, err := service.CreateLinuxDesktop(ctx, &linuxdesktopv1pb.CreateLinuxDesktopRequest{
		LinuxDesktop: desktop,
	})
	require.NoError(t, err)
	require.Equal(t, desktop, resp)
	require.True(t, backend.createCalled)
	require.Equal(t, desktop, backend.createReq)
	require.Equal(t, []check{
		{kind: types.KindLinuxDesktop, verb: types.VerbCreate},
	}, checker.checks)
}

func TestServiceCreateLinuxDesktopMissingDesktop(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	reader := &fakeReader{}
	backend := &fakeBackend{}
	checker := &fakeChecker{
		allow: allowChecks(
			check{kind: types.KindLinuxDesktop, verb: types.VerbCreate},
		),
	}
	service := newTestService(t, checker, authz.AdminActionAuthNotRequired, backend, reader)

	_, err := service.CreateLinuxDesktop(ctx, &linuxdesktopv1pb.CreateLinuxDesktopRequest{})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.False(t, backend.createCalled)
}

func TestServiceUpdateLinuxDesktop(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	desktop := newTestDesktop(t, "desktop-1")

	reader := &fakeReader{getResp: desktop}
	backend := &fakeBackend{updateResp: desktop}
	checker := &fakeChecker{
		allow: allowChecks(
			check{kind: types.KindLinuxDesktop, verb: types.VerbUpdate},
		),
	}
	service := newTestService(t, checker, authz.AdminActionAuthNotRequired, backend, reader)

	resp, err := service.UpdateLinuxDesktop(ctx, &linuxdesktopv1pb.UpdateLinuxDesktopRequest{
		LinuxDesktop: desktop,
	})
	require.NoError(t, err)
	require.Equal(t, desktop, resp)
	require.True(t, backend.updateCalled)
	require.Equal(t, desktop, backend.updateReq)
	require.Equal(t, []check{
		{kind: types.KindLinuxDesktop, verb: types.VerbUpdate},
	}, checker.checks)
}

func TestServiceUpdateLinuxDesktopMissingDesktop(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	reader := &fakeReader{}
	backend := &fakeBackend{}
	checker := &fakeChecker{
		allow: allowChecks(
			check{kind: types.KindLinuxDesktop, verb: types.VerbUpdate},
		),
	}
	service := newTestService(t, checker, authz.AdminActionAuthNotRequired, backend, reader)

	_, err := service.UpdateLinuxDesktop(ctx, &linuxdesktopv1pb.UpdateLinuxDesktopRequest{})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.False(t, backend.updateCalled)
}

func TestServiceUpsertLinuxDesktop(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	desktop := newTestDesktop(t, "desktop-1")

	reader := &fakeReader{getResp: desktop}
	backend := &fakeBackend{upsertResp: desktop}
	checker := &fakeChecker{
		allow: allowChecks(
			check{kind: types.KindLinuxDesktop, verb: types.VerbUpdate},
			check{kind: types.KindLinuxDesktop, verb: types.VerbCreate},
		),
	}
	service := newTestService(t, checker, authz.AdminActionAuthNotRequired, backend, reader)

	resp, err := service.UpsertLinuxDesktop(ctx, &linuxdesktopv1pb.UpsertLinuxDesktopRequest{
		LinuxDesktop: desktop,
	})
	require.NoError(t, err)
	require.Equal(t, desktop, resp)
	require.True(t, backend.upsertCalled)
	require.Equal(t, desktop, backend.upsertReq)
	require.Equal(t, []check{
		{kind: types.KindLinuxDesktop, verb: types.VerbUpdate},
		{kind: types.KindLinuxDesktop, verb: types.VerbCreate},
	}, checker.checks)
}

func TestServiceUpsertLinuxDesktopMissingDesktop(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	reader := &fakeReader{}
	backend := &fakeBackend{}
	checker := &fakeChecker{
		allow: allowChecks(
			check{kind: types.KindLinuxDesktop, verb: types.VerbUpdate},
			check{kind: types.KindLinuxDesktop, verb: types.VerbCreate},
		),
	}
	service := newTestService(t, checker, authz.AdminActionAuthNotRequired, backend, reader)

	_, err := service.UpsertLinuxDesktop(ctx, &linuxdesktopv1pb.UpsertLinuxDesktopRequest{})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.False(t, backend.upsertCalled)
}

func TestServiceDeleteLinuxDesktop(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	reader := &fakeReader{getResp: newTestDesktop(t, "desktop-1")}
	backend := &fakeBackend{}
	checker := &fakeChecker{
		allow: allowChecks(
			check{kind: types.KindLinuxDesktop, verb: types.VerbDelete},
		),
	}
	service := newTestService(t, checker, authz.AdminActionAuthNotRequired, backend, reader)

	_, err := service.DeleteLinuxDesktop(ctx, &linuxdesktopv1pb.DeleteLinuxDesktopRequest{
		Name: "desktop-1",
	})
	require.NoError(t, err)
	require.True(t, backend.deleteCalled)
	require.Equal(t, "desktop-1", backend.deleteName)
	require.Equal(t, []check{
		{kind: types.KindLinuxDesktop, verb: types.VerbDelete},
	}, checker.checks)
}

func TestServiceAdminActionRequired(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	desktop := newTestDesktop(t, "desktop-1")

	tests := []struct {
		name         string
		call         func(ctx context.Context, svc *Service) error
		backendCheck func(t *testing.T, backend *fakeBackend)
		allowed      []check
	}{
		{
			name: "create",
			call: func(ctx context.Context, svc *Service) error {
				_, err := svc.CreateLinuxDesktop(ctx, &linuxdesktopv1pb.CreateLinuxDesktopRequest{
					LinuxDesktop: desktop,
				})
				return err
			},
			backendCheck: func(t *testing.T, backend *fakeBackend) {
				require.False(t, backend.createCalled)
			},
			allowed: []check{{kind: types.KindLinuxDesktop, verb: types.VerbCreate}},
		},
		{
			name: "update",
			call: func(ctx context.Context, svc *Service) error {
				_, err := svc.UpdateLinuxDesktop(ctx, &linuxdesktopv1pb.UpdateLinuxDesktopRequest{
					LinuxDesktop: desktop,
				})
				return err
			},
			backendCheck: func(t *testing.T, backend *fakeBackend) {
				require.False(t, backend.updateCalled)
			},
			allowed: []check{{kind: types.KindLinuxDesktop, verb: types.VerbUpdate}},
		},
		{
			name: "upsert",
			call: func(ctx context.Context, svc *Service) error {
				_, err := svc.UpsertLinuxDesktop(ctx, &linuxdesktopv1pb.UpsertLinuxDesktopRequest{
					LinuxDesktop: desktop,
				})
				return err
			},
			backendCheck: func(t *testing.T, backend *fakeBackend) {
				require.False(t, backend.upsertCalled)
			},
			allowed: []check{
				{kind: types.KindLinuxDesktop, verb: types.VerbUpdate},
				{kind: types.KindLinuxDesktop, verb: types.VerbCreate},
			},
		},
		{
			name: "delete",
			call: func(ctx context.Context, svc *Service) error {
				_, err := svc.DeleteLinuxDesktop(ctx, &linuxdesktopv1pb.DeleteLinuxDesktopRequest{
					Name: "desktop-1",
				})
				return err
			},
			backendCheck: func(t *testing.T, backend *fakeBackend) {
				require.False(t, backend.deleteCalled)
			},
			allowed: []check{{kind: types.KindLinuxDesktop, verb: types.VerbDelete}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &fakeChecker{allow: allowChecks(tt.allowed...)}
			backend := &fakeBackend{}
			reader := &fakeReader{}
			service := newTestService(t, checker, authz.AdminActionAuthUnauthorized, backend, reader)

			err := tt.call(ctx, service)
			require.ErrorIs(t, err, &mfa.ErrAdminActionMFARequired)
			tt.backendCheck(t, backend)
		})
	}
}
