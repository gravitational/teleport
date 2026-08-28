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

package connection

import (
	"github.com/gravitational/trace"
)

// Config controls how the bot will connect to the Teleport cluster.
type Config struct {
	// Address contains the address string.
	Address string

	// AddressKind is the kind of address the user provided (i.e. auth server or
	// proxy).
	AddressKind AddressKind

	// AuthServerAddressMode controls the behavior of when a proxy address is
	// given as an auth server address.
	AuthServerAddressMode AuthServerAddressMode

	// StaticProxyAddress means the given proxy address will be used as-is
	// rather than using an address discovered by pinging the proxy or auth
	// server.
	//
	// In tbot, it is controlled using the `TBOT_USE_PROXY_ADDR` env var.
	StaticProxyAddress bool

	// Insecure allows the bot to trust the auth server or proxy certificate on
	// first connection without verifying them. It is not recommended for use in
	// production.
	Insecure bool
}

// Validate the connection configuration.
func (cfg *Config) Validate() error {
	if cfg.Address == "" {
		return trace.BadParameter("Address is required")
	}

	switch cfg.AddressKind {
	case AddressKindProxy, AddressKindAuth:
	default:
		return trace.BadParameter("unsupported address kind: %s", cfg.AddressKind)
	}

	switch cfg.AuthServerAddressMode {
	case AllowProxyAsAuthServer, WarnIfAuthServerIsProxy, AuthServerMustBeAuthServer:
	default:
		return trace.BadParameter("unsupported auth server address mode: %d", cfg.AuthServerAddressMode)
	}

	if cfg.StaticProxyAddress && cfg.AddressKind != AddressKindProxy {
		return trace.BadParameter("static proxy address requested (e.g. via the TBOT_USE_PROXY_ADDR environment variable) but no explicit proxy address was configured")
	}

	return nil
}

// AddressKind describes the type of address the user provided.
type AddressKind string

const (
	// AddressKindUnspecified means the user did not provide an address.
	AddressKindUnspecified AddressKind = ""

	// AddressKindProxy means the user provided the `--proxy-server` flag or
	// `proxy_server` config option.
	AddressKindProxy AddressKind = "proxy"

	// AddressKindAuth means the user provided the `--auth-server` flag or
	// `auth_server` config option.
	AddressKindAuth AddressKind = "auth"
)

// AuthServerAddressMode controls the behavior when a proxy address is given
// as an auth server address.
type AuthServerAddressMode int

const (
	// AuthServerMustBeAuthServer means that only an actual auth server address
	// may be given.
	AuthServerMustBeAuthServer AuthServerAddressMode = iota

	// WarnIfAuthServerIsProxy means that a proxy address will be accepted as an
	// auth server address, but we will log a warning that this is going away in
	// v19.
	WarnIfAuthServerIsProxy

	// AllowProxyAsAuthServer means that a proxy address will be accepted as an
	// auth server address.
	AllowProxyAsAuthServer
)
