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

// Package native will be deleted as soon as references are removed from
// teleport.e.
package native

import (
	"crypto/rsa"
	"testing"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/modules"
)

// GenerateRSAPrivateKey will be deleted as soon as references are removed from
// teleport.e.
func GenerateRSAPrivateKey() (*rsa.PrivateKey, error) {
	key, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.RSA2048)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return key.(*rsa.PrivateKey), nil
}

// PrecomputeKeys is an alias of [cryptosuites.PrecomputeRSAKeys]. It will be
// deleted as soon as references are removed from teleport.e.
func PrecomputeKeys() {
	cryptosuites.PrecomputeRSAKeys()
}

// PrecomputeTestKeys is an alias of [cryptosuites.PrecomputeRSATestKeys]. It
// will be deleted as soon as references are removed from teleport.e.
func PrecomputeTestKeys(m *testing.M) {
	cryptosuites.PrecomputeRSATestKeys(m)
}

// IsBoringBinary is an alias of [modules.IsBoringBinary]. It will be deleted as
// soon as references are removed from teleport.e.
func IsBoringBinary() bool {
	return modules.IsBoringBinary()
}
