/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
package sigstore

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/stretchr/testify/require"
)

func TestKeychain(t *testing.T) {
	// Note: this test just exercises loading credentials directly in the Docker
	// config file. Docker's cli package handles loading credentials from helpers
	// which we get "for free".
	chain, err := Keychain("./testdata/docker-config.json")
	require.NoError(t, err)

	reg, err := name.NewRegistry("ghcr.io")
	require.NoError(t, err)

	auth, err := chain.Resolve(reg)
	require.NoError(t, err)

	authCfg, err := auth.Authorization()
	require.NoError(t, err)

	require.Equal(t,
		&authn.AuthConfig{
			Username: "foo",
			Password: "bar",
		},
		authCfg,
	)
}
