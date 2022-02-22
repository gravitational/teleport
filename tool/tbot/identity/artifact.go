/*
Copyright 2021-2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package identity

import (
	"bytes"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/tool/tbot/destination"
)

type Artifact struct {
	Key       string
	Kind      ArtifactKind
	ModeHint  destination.ModeHint
	ToBytes   func(*Identity) []byte
	FromBytes func(*proto.Certs, *LoadIdentityParams, []byte)
}

// Matches returns true if this artifact's Kind matches any one of the given
// kinds or if it's kind is KindAlways
func (a *Artifact) Matches(kinds ...ArtifactKind) bool {
	if a.Kind == KindAlways {
		return true
	}

	for _, kind := range kinds {
		if a.Kind == kind {
			return true
		}
	}

	return false
}

var artifacts []Artifact = []Artifact{
	// SSH artifacts
	{
		Key:      SSHCertKey,
		Kind:     KindSSH,
		ModeHint: destination.ModeHintSecret,
		ToBytes: func(i *Identity) []byte {
			return i.CertBytes
		},
		FromBytes: func(c *proto.Certs, p *LoadIdentityParams, b []byte) {
			c.SSH = b
		},
	},
	{
		Key:      SSHCACertsKey,
		Kind:     KindSSH,
		ModeHint: destination.ModeHintSecret,
		ToBytes: func(i *Identity) []byte {
			return bytes.Join(i.SSHCACertBytes, []byte("$"))
		},
		FromBytes: func(c *proto.Certs, p *LoadIdentityParams, b []byte) {
			c.SSHCACerts = bytes.Split(b, []byte("$"))
		},
	},

	// TLS artifacts
	{
		Key:      TLSCertKey,
		Kind:     KindTLS,
		ModeHint: destination.ModeHintSecret,
		ToBytes: func(i *Identity) []byte {
			return i.TLSCertBytes
		},
		FromBytes: func(c *proto.Certs, p *LoadIdentityParams, b []byte) {
			c.TLS = b
		},
	},
	{
		Key:      TLSCACertsKey,
		Kind:     KindTLS,
		ModeHint: destination.ModeHintSecret,
		ToBytes: func(i *Identity) []byte {
			return bytes.Join(i.TLSCACertsBytes, []byte("$"))
		},
		FromBytes: func(c *proto.Certs, p *LoadIdentityParams, b []byte) {
			c.TLSCACerts = bytes.Split(b, []byte("$"))
		},
	},

	// Common artifacts
	{
		Key:      PrivateKeyKey,
		Kind:     KindAlways,
		ModeHint: destination.ModeHintSecret,
		ToBytes: func(i *Identity) []byte {
			return i.PrivateKeyBytes
		},
		FromBytes: func(c *proto.Certs, p *LoadIdentityParams, b []byte) {
			p.PrivateKeyBytes = b
		},
	},
	{
		Key:      PublicKeyKey,
		Kind:     KindAlways,
		ModeHint: destination.ModeHintUnspecified,
		ToBytes: func(i *Identity) []byte {
			return i.PublicKeyBytes
		},
		FromBytes: func(c *proto.Certs, p *LoadIdentityParams, b []byte) {
			p.PublicKeyBytes = b
		},
	},
	{
		Key:      TokenHashKey,
		Kind:     KindBotInternal,
		ModeHint: destination.ModeHintSecret,
		ToBytes: func(i *Identity) []byte {
			return i.TokenHashBytes
		},
		FromBytes: func(c *proto.Certs, p *LoadIdentityParams, b []byte) {
			p.TokenHashBytes = b
		},
	},
}

func GetArtifacts() []Artifact {
	return artifacts
}
