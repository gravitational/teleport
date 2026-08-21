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
	testCases := map[string]struct {
		scopedUsage         *ScopedUsage
		verifyReqAfterApply func(t *testing.T, req *issuancev1pb.IssueScopedBotCertsRequest)
		validationWantErr   string
	}{
		"identity": {
			scopedUsage: UsageIdentity(),
			verifyReqAfterApply: func(t *testing.T, req *issuancev1pb.IssueScopedBotCertsRequest) {
				require.Equal(t, issuancev1pb.IssueScopedBotCertsRequest_Identity_case, req.WhichUsage())
				require.NotNil(t, req.GetIdentity())
			},
		},
		"app": {
			scopedUsage: UsageApp(proto.RouteToApp{
				Name:       "abc",
				PublicAddr: "remotehost",
				Scope:      "/testing",
			}),
			verifyReqAfterApply: func(t *testing.T, req *issuancev1pb.IssueScopedBotCertsRequest) {
				require.Equal(t, issuancev1pb.IssueScopedBotCertsRequest_App_case, req.WhichUsage())
				usageApp := req.GetApp()
				require.NotNil(t, usageApp)
				assert.Equal(t, "abc", usageApp.GetName())
				assert.Equal(t, "remotehost", usageApp.GetPublicAddr())
				assert.Equal(t, "/testing", usageApp.GetScope())
			},
		},
		"nil usage": {
			scopedUsage:       nil,
			validationWantErr: "usage is undefined",
		},
		"empty usage": {
			scopedUsage:       new(ScopedUsage),
			validationWantErr: "usage is incomplete",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.scopedUsage.Validate()
			if tc.validationWantErr != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.validationWantErr)
				return
			}
			require.NoError(t, err)

			req := issuancev1pb.IssueScopedBotCertsRequest_builder{
				Ttl: durationpb.New(time.Second),
			}.Build()

			require.Zero(t, req.WhichUsage())
			tc.scopedUsage.apply(req)
			require.NotZero(t, req.WhichUsage())
			tc.verifyReqAfterApply(t, req)
		})
	}
}
