/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package modules

import (
	"context"
	"crypto"
	"testing"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/keys"
)

// TestModules implements the Modules interface for testing.
//
// Setting Test* fields will return those values from interface
// methods. IsBoringBinary and PrintVersion functions return the
// same values from default modules.
//
// See SetTestModules for an example.
type TestModules struct {
	// TestBuildType is returned from the BuiltType function.
	TestBuildType string
	// TestFeatures is returned from the Features function.
	TestFeatures Features
	// FIPS allows tests to toggle fips behavior.
	FIPS bool

	defaultModules

	// MockAttestationData is fake attestation data to return
	// during tests when hardware key support is enabled. This
	// attestation data is shared by all logins when set.
	MockAttestationData *keys.AttestationData
}

// SetTestModules sets the value returned from GetModules to testModules
// and reverts the change in the test cleanup function.
// It must not be used in parallel tests.
//
//	func TestWithFakeModules(t *testing.T) {
//	   modules.SetTestModules(t, &modules.TestModules{
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
func SetTestModules(t *testing.T, testModules Modules) {
	defaultModules := GetModules()
	t.Cleanup(func() { SetModules(defaultModules) })
	t.Setenv("TELEPORT_TEST_NOT_SAFE_FOR_PARALLEL", "true")
	SetModules(testModules)
}

// PrintVersion prints teleport version
func (m *TestModules) PrintVersion() {
	m.defaultModules.PrintVersion()
}

// IsBoringBinary checks if the binary was compiled with BoringCrypto.
func (m *TestModules) IsBoringBinary() bool {
	return m.FIPS
}

// Features returns supported features.
func (m *TestModules) Features() Features {
	return m.TestFeatures
}

// BuildType returns build type (OSS or Enterprise).
func (m *TestModules) BuildType() string {
	return m.TestBuildType
}

func (m *TestModules) IsEnterpriseBuild() bool {
	return m.BuildType() == BuildEnterprise
}

func (m *TestModules) IsOSSBuild() bool {
	return m.BuildType() != BuildEnterprise
}

// AttestHardwareKey attests a hardware key.
func (m *TestModules) AttestHardwareKey(ctx context.Context, obj interface{}, as *keys.AttestationStatement, pk crypto.PublicKey, d time.Duration) (*keys.AttestationData, error) {
	if m.MockAttestationData != nil {
		return m.MockAttestationData, nil
	}
	return nil, trace.NotFound("no attestation data for the given key")
}
