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

package cache

import (
	"context"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestDelegationProfileCache(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs[*delegationv1.DelegationProfile]{
		newResource: func(key string) (*delegationv1.DelegationProfile, error) {
			return newDelegationProfile(key), nil
		},
		cacheGet:  p.cache.GetDelegationProfile,
		cacheList: p.cache.ListDelegationProfiles,
		create: func(ctx context.Context, prof *delegationv1.DelegationProfile) error {
			_, err := p.delegationProfiles.CreateDelegationProfile(ctx, prof)
			return err
		},
		list: p.delegationProfiles.ListDelegationProfiles,
		update: func(ctx context.Context, prof *delegationv1.DelegationProfile) error {
			_, err := p.delegationProfiles.UpdateDelegationProfile(ctx, prof)
			return err
		},
		delete:    p.delegationProfiles.DeleteDelegationProfile,
		deleteAll: p.delegationProfiles.DeleteAllDelegationProfiles,
	})
}

func newDelegationProfile(name string) *delegationv1.DelegationProfile {
	return &delegationv1.DelegationProfile{
		Kind:    types.KindDelegationProfile,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: &delegationv1.DelegationProfileSpec{
			RequiredResources: []*delegationv1.DelegationResourceSpec{
				{
					Kind: types.KindApp,
					Name: "hr-system",
				},
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
