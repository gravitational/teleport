/*
Copyright 2024 Gravitational, Inc.

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

package accessgraph

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/gravitational/trace"

	accessgraphv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

// NewPrivateKey creates a new SSH Private key resource with a generated name based on the spec.
func NewPrivateKey(spec *accessgraphv1pb.PrivateKeySpec) (*accessgraphv1pb.PrivateKey, error) {
	name := privKeyHashNameKey(spec)
	v, err := NewPrivateKeyWithName(name, spec)

	return v, trace.Wrap(err)
}

// NewPrivateKeyWithName creates a new SSH Private key resource.
func NewPrivateKeyWithName(name string, spec *accessgraphv1pb.PrivateKeySpec) (*accessgraphv1pb.PrivateKey, error) {
	privKey := &accessgraphv1pb.PrivateKey{
		Kind:    types.KindAccessGraphSecretPrivateKey,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: spec,
	}
	if err := ValidatePrivateKey(privKey); err != nil {
		return nil, trace.Wrap(err)
	}

	return privKey, nil
}

// ValidatePrivateKey checks that required parameters are set
// for the specified PrivateKey
func ValidatePrivateKey(k *accessgraphv1pb.PrivateKey) error {
	if k == nil {
		return trace.BadParameter("PrivateKey is nil")
	}
	if k.Metadata == nil {
		return trace.BadParameter("Metadata is nil")
	}
	if k.Spec == nil {
		return trace.BadParameter("Spec is nil")
	}

	if k.Kind != types.KindAccessGraphSecretPrivateKey {
		return trace.BadParameter("Kind is invalid")
	}

	if k.Version != types.V1 {
		return trace.BadParameter("Version is invalid")
	}

	switch k.Spec.PublicKeyMode {
	case accessgraphv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PROTECTED,
		accessgraphv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PUB_FILE,
		accessgraphv1pb.PublicKeyMode_PUBLIC_KEY_MODE_DERIVED:
	default:
		return trace.BadParameter("PublicKeyMode is invalid")
	}

	if k.Spec.DeviceId == "" {
		return trace.BadParameter("DeviceId is unset")
	}
	if k.Spec.PublicKeyFingerprint == "" && k.Spec.PublicKeyMode != accessgraphv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PROTECTED {
		return trace.BadParameter("PublicKeyFingerprint is unset")
	}

	if k.Metadata.Name == "" {
		return trace.BadParameter("Name is unset")
	}

	return nil
}

func privKeyHashNameKey(k *accessgraphv1pb.PrivateKeySpec) string {
	return hashComp(k.DeviceId, k.PublicKeyFingerprint)
}

func hashComp(values ...string) string {
	h := sha256.New()
	for _, value := range values {
		h.Write([]byte(value))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// DescribePublicKeyMode returns a human-readable description of the public key mode.
func DescribePublicKeyMode(mode accessgraphv1pb.PublicKeyMode) string {
	switch mode {
	case accessgraphv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PUB_FILE:
		return "used public key file"
	case accessgraphv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PROTECTED:
		return "protected private key"
	case accessgraphv1pb.PublicKeyMode_PUBLIC_KEY_MODE_DERIVED:
		return "derived from private key"
	default:
		return "unknown"
	}

}
