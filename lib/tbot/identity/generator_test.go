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

package identity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/gravitational/teleport/api/client/proto"
	issuancev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/issuance/v1"
)

func TestScopedUsage(t *testing.T) {
	t.Run("Existing types are valid", func(t *testing.T) {
		testCases := map[string]*ScopedUsage{
			"identity": UsageIdentity(),
			"app":      UsageApp(proto.RouteToApp{}),
		}

		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				require.NoError(t, tc.Validate())
			})
		}
	})

	t.Run("Nil usage", func(t *testing.T) {
		var scopedUsage *ScopedUsage

		err := scopedUsage.Validate()
		require.Error(t, err)
		require.ErrorContains(t, err, "usage is undefined")
	})

	t.Run("Empty usage", func(t *testing.T) {
		scopedUsage := new(ScopedUsage)

		err := scopedUsage.Validate()
		require.Error(t, err)
		require.ErrorContains(t, err, "usage is incomplete")
	})
}

func TestScopedUsage_Identity(t *testing.T) {
	req := issuancev1pb.IssueScopedBotCertsRequest_builder{
		Ttl: durationpb.New(time.Second),
	}.Build()

	require.Zero(t, req.WhichUsage())
	require.Nil(t, req.GetIdentity())

	usage := UsageIdentity()
	usage.apply(req)

	require.NotNil(t, req.GetIdentity())
	require.Equal(t, issuancev1pb.IssueScopedBotCertsRequest_Identity_case, req.WhichUsage())
}

func TestScopedUsage_App(t *testing.T) {
	req := issuancev1pb.IssueScopedBotCertsRequest_builder{
		Ttl: durationpb.New(time.Second),
	}.Build()

	require.Zero(t, req.WhichUsage())
	require.Nil(t, req.GetApp())

	routeToApp := proto.RouteToApp{
		Name:       "abc",
		PublicAddr: "remotehost",
		Scope:      "/testing",
	}
	usage := UsageApp(routeToApp)
	usage.apply(req)

	require.Equal(t, issuancev1pb.IssueScopedBotCertsRequest_App_case, req.WhichUsage())
	usageApp := req.GetApp()
	require.NotNil(t, usageApp)
	assert.Equal(t, routeToApp.Name, usageApp.GetName())
	assert.Equal(t, routeToApp.PublicAddr, usageApp.GetPublicAddr())
	assert.Equal(t, routeToApp.Scope, usageApp.GetScope())
}
