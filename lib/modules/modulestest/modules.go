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

package modulestest

import (
	"context"
	"crypto"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/tlsca"
)

// Modules is an implementation of [modules.Modules] to be
// used in tests that need to exercise various conditions
// dictacted by a specific feature set.
type Modules struct {
	// TestBuild is returned from the BuildType function.
	TestBuildType string
	// TestFeatures is returned from the Features function.
	TestFeatures modules.Features
	// FIPS is returned from the IsBoringBinary function.
	FIPS bool
	// MockAttestationData is fake attestation data to return
	// during tests when hardware key support is enabled. This
	// attestation data is shared by all logins when set.
	MockAttestationData *keys.AttestationData

	GenerateAccessRequestPromotionsFn  func(ctx context.Context, accessListGetter modules.AccessResourcesGetter, accessReq types.AccessRequest) (*types.AccessRequestAllowedPromotions, error)
	GenerateLongTermResourceGroupingFn func(ctx context.Context, clt modules.AccessResourcesGetter, req types.AccessRequest) (*types.LongTermResourceGrouping, error)
}

// AttestHardwareKey implements modules.Modules.
func (m *Modules) AttestHardwareKey(context.Context, any, *hardwarekey.AttestationStatement, crypto.PublicKey, time.Duration) (*keys.AttestationData, error) {
	if m.MockAttestationData != nil {
		return m.MockAttestationData, nil
	}
	return nil, trace.NotFound("no attestation data for the given key")
}

// BuildType implements modules.Modules.
func (m *Modules) BuildType() string {
	return m.TestBuildType
}

// EnableAccessGraph implements modules.Modules.
func (m *Modules) EnableAccessGraph() {}

// EnableAccessMonitoring implements modules.Modules.
func (m *Modules) EnableAccessMonitoring() {}

// EnablePlugins implements modules.Modules.
func (m *Modules) EnablePlugins() {}

// EnableRecoveryCodes implements modules.Modules.
func (m *Modules) EnableRecoveryCodes() {}

// Features implements modules.Modules.
func (m *Modules) Features() modules.Features {
	return m.TestFeatures
}

// GenerateLongTermResourceGrouping implements modules.Modules.
func (m *Modules) GenerateLongTermResourceGrouping(ctx context.Context, clt modules.AccessResourcesGetter, req types.AccessRequest) (*types.LongTermResourceGrouping, error) {
	if m.GenerateLongTermResourceGroupingFn != nil {
		return m.GenerateLongTermResourceGroupingFn(ctx, clt, req)
	}
	return nil, trace.NotImplemented("GenerateLongTermResourceGrouping not implemented")
}

// GenerateAccessRequestPromotions implements modules.Modules.
func (m *Modules) GenerateAccessRequestPromotions(ctx context.Context, getter modules.AccessResourcesGetter, request types.AccessRequest) (*types.AccessRequestAllowedPromotions, error) {
	if m.GenerateAccessRequestPromotionsFn != nil {
		return m.GenerateAccessRequestPromotionsFn(ctx, getter, request)
	}
	return types.NewAccessRequestAllowedPromotions(nil), nil
}

// GetSuggestedAccessLists implements modules.Modules.
func (m *Modules) GetSuggestedAccessLists(ctx context.Context, identity *tlsca.Identity, clt modules.AccessListSuggestionClient, accessListGetter modules.AccessListAndMembersGetter, requestID string) ([]*accesslist.AccessList, error) {
	return nil, trace.NotImplemented("GetSuggestedAccessLists not implemented")
}

// IsBoringBinary implements modules.Modules.
func (m *Modules) IsBoringBinary() bool {
	return m.FIPS
}

// IsEnterpriseBuild implements modules.Modules.
func (m *Modules) IsEnterpriseBuild() bool {
	return m.BuildType() == modules.BuildEnterprise
}

// IsOSSBuild implements modules.Modules.
func (m *Modules) IsOSSBuild() bool {
	return m.BuildType() != modules.BuildEnterprise
}

// LicenseExpiry implements modules.Modules.
func (m *Modules) LicenseExpiry() time.Time {
	return time.Time{}
}

// PrintVersion implements modules.Modules.
func (m *Modules) PrintVersion() {
	fmt.Printf("Teleport v%s git:%s %s\n", teleport.Version, teleport.Gitref, runtime.Version())
}

// SetFeatures implements modules.Modules.
func (m *Modules) SetFeatures(features modules.Features) {}

// SetTestModules sets the value returned from GetModules to testModules
// and reverts the change in the test cleanup function.
// It cannot be used in parallel tests.
//
//	func TestWithFakeModules(t *testing.T) {
//	   modules.SetTestModules(t, &modulestest.Modules{
//	     TestBuildType: modules.BuildEnterprise,
//	     TestFeatures: modules.Features{
//	        Cloud: true,
//	     },
//	   })
//
//	   // test implementation
//
//	   // cleanup will revert module changes after test completes
//	}
//
// TODO(tross): Get rid of global modules so this can be removed.
func SetTestModules(tb testing.TB, testModules Modules) {
	defaultModules := modules.GetModules()
	tb.Cleanup(func() { modules.SetModules(defaultModules) })
	tb.Setenv("TELEPORT_TEST_NOT_SAFE_FOR_PARALLEL", "true")
	modules.SetModules(&testModules)
}
