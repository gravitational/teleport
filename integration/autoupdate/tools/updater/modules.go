/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package updater

import (
	"context"
	"crypto"
	"fmt"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/tlsca"
)

const (
	TestPassword = "abc123def456"
)

var (
	version = teleport.Version
)

type TestModules struct{}

func (p *TestModules) GenerateAccessRequestPromotions(context.Context, modules.AccessResourcesGetter, types.AccessRequest) (*types.AccessRequestAllowedPromotions, error) {
	return &types.AccessRequestAllowedPromotions{}, nil
}

func (p *TestModules) GetSuggestedAccessLists(context.Context, *tlsca.Identity, modules.AccessListSuggestionClient, modules.AccessListAndMembersGetter, string) ([]*accesslist.AccessList, error) {
	return []*accesslist.AccessList{}, nil
}

// BuildType returns build type (OSS or Enterprise)
func (p *TestModules) BuildType() string {
	return "CLI"
}

// IsEnterpriseBuild returns false for [TestModules].
func (p *TestModules) IsEnterpriseBuild() bool {
	return false
}

// IsOSSBuild returns false for [TestModules].
func (p *TestModules) IsOSSBuild() bool {
	return false
}

// LicenseExpiry returns the expiry date of the enterprise license, if applicable.
func (p *TestModules) LicenseExpiry() time.Time {
	return time.Time{}
}

// PrintVersion prints the Teleport version.
func (p *TestModules) PrintVersion() {
	fmt.Printf("Teleport v%v git\n", version)
}

// Features returns supported features
func (p *TestModules) Features() modules.Features {
	return modules.Features{
		Entitlements:            map[entitlements.EntitlementKind]modules.EntitlementInfo{},
		AdvancedAccessWorkflows: true,
	}
}

// IsBoringBinary checks if the binary was compiled with BoringCrypto.
func (p *TestModules) IsBoringBinary() bool {
	return false
}

// AttestHardwareKey attests a hardware key.
func (p *TestModules) AttestHardwareKey(context.Context, interface{}, *keys.AttestationStatement, crypto.PublicKey, time.Duration) (*keys.AttestationData, error) {
	return nil, trace.NotFound("no attestation data for the given key")
}

func (p *TestModules) EnableRecoveryCodes() {}

func (p *TestModules) EnablePlugins() {}

func (p *TestModules) SetFeatures(f modules.Features) {}

func (p *TestModules) EnableAccessGraph() {}

func (p *TestModules) EnableAccessMonitoring() {}
