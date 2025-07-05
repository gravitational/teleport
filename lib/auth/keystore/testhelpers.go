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

package keystore

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/keystore/internal"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

// NewSoftwareKeystoreForTests returns a new *Manager that is valid for tests not specifically testing the
// keystore functionality.
// Deprecated: Prefer using keystoretest.NewTestKeystore instead.
// TODO(tross): delete after e reference is updated.
func NewSoftwareKeystoreForTests(_ *testing.T) *Manager {
	softwareBackend := internal.NewSoftwareKeyStore(nil)
	return &Manager{
		backendForNewKeys: softwareBackend,
		usableBackends:    []internal.Backend{softwareBackend},
		currentSuiteGetter: cryptosuites.GetSuiteFunc(func(context.Context) (types.SignatureAlgorithmSuite, error) {
			return types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1, nil
		}),
	}
}
