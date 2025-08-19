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
	"context"
	"os"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/types"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/gravitational/trace"
)

// Keychain creates an authn.Keychain backed by the given Docker/Podman
// configuration file on disk.
//
// If it's empty (i.e. the user didn't provide a configuration file) we'll look
// in the default places:
//
//   - $HOME/.docker/config.json
//   - $DOCKER_CONFIG
//   - $REGISTRY_AUTH_FILE
//   - $XDG_RUNTIME_DIR/containers/auth.json
func Keychain(cfgPath string) (authn.Keychain, error) {
	if cfgPath == "" {
		return authn.DefaultKeychain, nil
	}
	fi, err := os.Stat(cfgPath)
	if err != nil {
		return nil, trace.Wrap(err, "checking config file")
	}
	if fi.IsDir() {
		return nil, trace.BadParameter("must be path to a file")
	}
	return &keychain{cfgPath}, nil
}

type keychain struct{ path string }

func (k *keychain) Resolve(r authn.Resource) (authn.Authenticator, error) {
	return k.ResolveContext(context.Background(), r)
}

func (k *keychain) ResolveContext(_ context.Context, target authn.Resource) (authn.Authenticator, error) {
	f, err := os.Open(k.path)
	if err != nil {
		return nil, trace.Wrap(err, "opening keychain config file")
	}
	defer func() { _ = f.Close() }()

	cf, err := config.LoadFromReader(f)
	if err != nil {
		return nil, trace.Wrap(err, "loading keychain config file")
	}

	// Copied from: https://github.com/google/go-containerregistry/blob/098045d5e61ff426a61a0eecc19ad0c433cd35a9/pkg/authn/keychain.go#L144-L179
	var cfg, empty types.AuthConfig
	for _, key := range []string{
		target.String(),
		target.RegistryStr(),
	} {
		if key == name.DefaultRegistry {
			key = authn.DefaultAuthKey
		}

		cfg, err = cf.GetAuthConfig(key)
		if err != nil {
			return nil, err
		}

		cfg.ServerAddress = ""
		if cfg != empty {
			break
		}
	}
	if cfg == empty {
		return authn.Anonymous, nil
	}
	return authn.FromConfig(authn.AuthConfig{
		Username:      cfg.Username,
		Password:      cfg.Password,
		Auth:          cfg.Auth,
		IdentityToken: cfg.IdentityToken,
		RegistryToken: cfg.RegistryToken,
	}), nil
}
