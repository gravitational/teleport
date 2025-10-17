// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package services_test

import (
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func TestValidateDelegationProfile(t *testing.T) {
	t.Parallel()

	require.NoError(t, services.ValidateDelegationProfile(validDelegationProfile()))

	testCases := map[string]struct {
		modFn func(*delegationv1.DelegationProfile)
		error string
	}{
		"wrong kind": {
			func(p *delegationv1.DelegationProfile) { p.Kind = "" },
			"kind: must be delegation_profile",
		},
		"wrong version": {
			func(p *delegationv1.DelegationProfile) { p.Version = "" },
			"version: must be v1",
		},
		"missing name": {
			func(p *delegationv1.DelegationProfile) { p.Metadata.Name = "" },
			"metadata.name: is required",
		},
		"no required resources": {
			func(p *delegationv1.DelegationProfile) { p.Spec.RequiredResources = nil },
			"spec.required_resources: at least one resource is required",
		},
		"invalid resource identifier": {
			func(p *delegationv1.DelegationProfile) {
				p.Spec.RequiredResources[0] = "this is not valid"
			},
			"spec.required_resources[0]: invalid resource identifier",
		},
		"no authorized users": {
			func(p *delegationv1.DelegationProfile) { p.Spec.AuthorizedUsers = nil },
			"spec.authorized_users: at least one user is required",
		},
		"invalid user type": {
			func(p *delegationv1.DelegationProfile) {
				p.Spec.AuthorizedUsers[0].Type = "dragon"
			},
			"spec.authorized_users[0].type: must be bot",
		},
		"no bot name": {
			func(p *delegationv1.DelegationProfile) {
				p.Spec.AuthorizedUsers[0].Matcher = &delegationv1.DelegationUserSpec_BotName{}
			},
			"spec.authorized_users[0].bot_name: is required",
		},
		"invalid default session length": {
			func(p *delegationv1.DelegationProfile) {
				p.Spec.DefaultSessionLength = durationpb.New(-1)
			},
			"spec.default_session_length: must be non-negative",
		},
		"invalid redirect url": {
			func(p *delegationv1.DelegationProfile) {
				p.Spec.Consent.AllowedRedirectUrls[0] = "not a url//!"
			},
			"spec.consent.allowed_redirect_urls[0]: invalid URL",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			profile := validDelegationProfile()

			tc.modFn(profile)

			err := services.ValidateDelegationProfile(profile)
			require.ErrorContains(t, err, tc.error)
			require.True(t, trace.IsBadParameter(err))
		})
	}
}

func validDelegationProfile() *delegationv1.DelegationProfile {
	return &delegationv1.DelegationProfile{
		Kind:    types.KindDelegationProfile,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "payroll-agent",
		},
		Spec: &delegationv1.DelegationProfileSpec{
			RequiredResources: []string{
				"/test.teleport.sh/app/hr-system",
			},
			AuthorizedUsers: []*delegationv1.DelegationUserSpec{
				{
					Type: types.DelegationUserTypeBot,
					Matcher: &delegationv1.DelegationUserSpec_BotName{
						BotName: "payroll-agent",
					},
				},
			},
			DefaultSessionLength: durationpb.New(1 * time.Hour),
			Consent: &delegationv1.DelegationConsentSpec{
				Title:       "Payroll Agent",
				Description: "Automates the payroll process",
				AllowedRedirectUrls: []string{
					"https://payroll.intranet.corp",
				},
			},
		},
	}
}
