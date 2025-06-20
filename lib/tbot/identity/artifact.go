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

package identity

import (
	"bytes"
	"slices"

	"github.com/gravitational/teleport/api/client/proto"
)

// Artifact is a component of a serialized identity.
type Artifact struct {
	// Key is the name that this artifact should be stored under within a
	// destination. For a file based destination, this will be the file name.
	Key       string
	Kind      ArtifactKind
	ToBytes   func(*Identity) []byte
	FromBytes func(*proto.Certs, *LoadIdentityParams, []byte)

	// Optional indicates whether or not an identity should fail to load if this
	// key is missing.
	Optional bool

	// OldKey allows an artifact to be migrated from an older key to a new key.
	// If this value is set, and we are unable to load from Key, we will try
	// and load from OldKey
	OldKey string
}

// Matches returns true if this artifact's Kind matches any one of the given
// kinds or if its kind is KindAlways
func (a *Artifact) Matches(kinds ...ArtifactKind) bool {
	if a.Kind == KindAlways {
		return true
	}

	return slices.Contains(kinds, a.Kind)
}

var artifacts = []Artifact{
	// SSH artifacts
	{
		Key:  SSHCertKey,
		Kind: KindAlways,
		ToBytes: func(i *Identity) []byte {
			return i.CertBytes
		},
		FromBytes: func(c *proto.Certs, p *LoadIdentityParams, b []byte) {
			c.SSH = b
		},
	},
	{
		Key: SSHCACertsKey,

		// SSH CAs in this format are only used for saving/loading of bot
		// identities and are not particularly useful to end users. We encode
		// the current SSH CAs inside the known_hosts file generated with the
		// `ssh_config` template, which is actually readable by OpenSSH.
		Kind: KindBotInternal,
		ToBytes: func(i *Identity) []byte {
			return bytes.Join(i.SSHCACertBytes, []byte("$"))
		},
		FromBytes: func(c *proto.Certs, p *LoadIdentityParams, b []byte) {
			c.SSHCACerts = bytes.Split(b, []byte("$"))
		},
	},

	// TLS artifacts
	{
		Key:  TLSCertKey,
		Kind: KindAlways,
		ToBytes: func(i *Identity) []byte {
			return i.TLSCertBytes
		},
		FromBytes: func(c *proto.Certs, p *LoadIdentityParams, b []byte) {
			c.TLS = b
		},
	},
	{
		Key: TLSCACertsKey,

		// TLS CA certs are useful to end users, but this artifact contains an
		// arbitrary number of CAs, including both Teleport's user and host CAs
		// and potentially multiple sets if they've been rotated.
		// Instead of exposing this mess of CAs to end users, we'll keep these
		// for internal use and just present single standard CAs in destination
		// dirs.
		Kind: KindBotInternal,
		ToBytes: func(i *Identity) []byte {
			return bytes.Join(i.TLSCACertsBytes, []byte("$"))
		},
		FromBytes: func(c *proto.Certs, p *LoadIdentityParams, b []byte) {
			c.TLSCACerts = bytes.Split(b, []byte("$"))
		},
	},

	// Common artifacts
	{
		Key:  PrivateKeyKey,
		Kind: KindAlways,
		ToBytes: func(i *Identity) []byte {
			return i.PrivateKeyBytes
		},
		FromBytes: func(c *proto.Certs, p *LoadIdentityParams, b []byte) {
			p.PrivateKeyBytes = b
		},
	},
	{
		Key:  PublicKeyKey,
		Kind: KindAlways,
		ToBytes: func(i *Identity) []byte {
			return i.PublicKeyBytes
		},
		FromBytes: func(c *proto.Certs, p *LoadIdentityParams, b []byte) {
			p.PublicKeyBytes = b
		},
	},
	{
		// The token hash is used to detect changes to the token and
		// request a new identity when changes are detected.
		Key:  TokenHashKey,
		Kind: KindBotInternal,
		ToBytes: func(i *Identity) []byte {
			return i.TokenHashBytes
		},
		FromBytes: func(c *proto.Certs, p *LoadIdentityParams, b []byte) {
			p.TokenHashBytes = b
		},
	},
	{
		// The write test is used to ensure the destination is writable before
		// attempting a renewal.
		Key:  WriteTestKey,
		Kind: KindAlways,
		ToBytes: func(i *Identity) []byte {
			// always empty
			return []byte{}
		},
		FromBytes: func(c *proto.Certs, p *LoadIdentityParams, b []byte) {
			// nothing to do
		},
		Optional: true,
	},
}

func GetArtifacts() []Artifact {
	return artifacts
}
