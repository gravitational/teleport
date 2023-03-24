/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package okta

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

// testAccessPoint is a test access point for the Okta service.
type testAccessPoint struct {
	events.Streamer
	io.Closer
	services.Apps
	services.ConnectionsDiagnostic
	services.DatabaseServices
	services.Identity
	services.Okta
	services.Presence
	services.UserGroups
	services.WindowsDesktops
	types.Events
}

var _ auth.OktaAccessPoint = (*testAccessPoint)(nil)

func (*testAccessPoint) NewKeepAliver(ctx context.Context) (types.KeepAliver, error) { return nil, nil }

func (*testAccessPoint) GenerateCertAuthorityCRL(context.Context, types.CertAuthType) ([]byte, error) {
	return nil, nil
}

// newTestAccessPoint will create a memory backed test access point for the Okta service.
func newTestAccessPoint(t *testing.T) *testAccessPoint {
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	streamer := events.NewDiscardEmitter()

	apps := local.NewAppService(backend)
	connectionsDiagnostic := local.NewConnectionsDiagnosticService(backend)
	databaseServices := local.NewDatabaseServicesService(backend)
	identity := local.NewIdentityService(backend)
	okta, err := local.NewOktaService(backend)
	require.NoError(t, err)
	presence := local.NewPresenceService(backend)
	userGroups, err := local.NewUserGroupService(backend)
	require.NoError(t, err)
	windowsDesktops := local.NewWindowsDesktopService(backend)
	events := local.NewEventsService(backend)

	client := &testAccessPoint{
		Streamer:              streamer,
		Closer:                io.NopCloser(nil),
		Apps:                  apps,
		ConnectionsDiagnostic: connectionsDiagnostic,
		DatabaseServices:      databaseServices,
		Identity:              identity,
		Okta:                  okta,
		Presence:              presence,
		UserGroups:            userGroups,
		WindowsDesktops:       windowsDesktops,
		Events:                events,
	}

	return client
}
