// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hardwarekeyagent

import (
	"crypto/x509"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/durationpb"

	hardwarekeyagentv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/hardwarekeyagent/v1"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
)

func privateKeyRefToProto(ref *hardwarekey.PrivateKeyRef) (*hardwarekeyagentv1.FullKeyRef, error) {
	slotKey, err := hardwarekey.PIVSlotKeyToProto(ref.SlotKey)
	if err != nil {
		return nil, trace.Wrap(err)

	}
	publicKeyDER, err := x509.MarshalPKIXPublicKey(ref.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return hardwarekeyagentv1.FullKeyRef_builder{
		KeyRef: hardwarekeyagentv1.KeyRef_builder{
			SerialNumber: ref.SerialNumber,
			SlotKey:      slotKey,
			PublicKeyDer: publicKeyDER,
		}.Build(),
		Policy: hardwarekeyagentv1.KeyPolicy_builder{
			TouchRequired: ref.Policy.TouchRequired,
			PinRequired:   ref.Policy.PINRequired,
		}.Build(),
		AttestationStatement: ref.AttestationStatement.ToProto(),
		PinCacheTtl:          durationpb.New(ref.PINCacheTTL),
	}.Build(), nil
}

func privateKeyRefFromProto(ref *hardwarekeyagentv1.FullKeyRef) (hardwarekey.PrivateKeyRef, error) {
	if ref == nil {
		return hardwarekey.PrivateKeyRef{}, trace.BadParameter("nil proto key ref, this is a bug")
	}
	slotKey, err := hardwarekey.PIVSlotKeyFromProto(ref.GetKeyRef().GetSlotKey())
	if err != nil {
		return hardwarekey.PrivateKeyRef{}, trace.Wrap(err)
	}
	pub, err := x509.ParsePKIXPublicKey(ref.GetKeyRef().GetPublicKeyDer())
	if err != nil {
		return hardwarekey.PrivateKeyRef{}, trace.Wrap(err)
	}
	return hardwarekey.PrivateKeyRef{
		SerialNumber: ref.GetKeyRef().GetSerialNumber(),
		SlotKey:      slotKey,
		PublicKey:    pub,
		Policy: hardwarekey.PromptPolicy{
			TouchRequired: ref.GetPolicy().GetTouchRequired(),
			PINRequired:   ref.GetPolicy().GetPinRequired(),
		},
		AttestationStatement: hardwarekey.AttestationStatementFromProto(ref.GetAttestationStatement()),
		PINCacheTTL:          ref.GetPinCacheTtl().AsDuration(),
	}, nil
}

func privateKeyConfigToProto(config hardwarekey.PrivateKeyConfig) (*hardwarekeyagentv1.PrivateKeyConfig, error) {
	algorithm, err := signatureAlgorithmToProto(config.Algorithm)
	if err != nil {
		return nil, trace.Wrap(err, "encoding signature algorithm to proto")
	}

	return hardwarekeyagentv1.PrivateKeyConfig_builder{
		Policy: hardwarekeyagentv1.KeyPolicy_builder{
			TouchRequired: config.Policy.TouchRequired,
			PinRequired:   config.Policy.PINRequired,
		}.Build(),
		CustomSlot:  string(config.CustomSlot),
		Algorithm:   algorithm,
		KeyInfo:     contextualKeyInfoToProto(config.ContextualKeyInfo),
		PinCacheTtl: durationpb.New(config.PINCacheTTL),
	}.Build(), nil
}

func privateKeyConfigFromProto(config *hardwarekeyagentv1.PrivateKeyConfig) (hardwarekey.PrivateKeyConfig, error) {
	algorithm, err := signatureAlgorithmFromProto(config.GetAlgorithm())
	if err != nil {
		return hardwarekey.PrivateKeyConfig{}, trace.Wrap(err)
	}

	customSlot := hardwarekey.PIVSlotKeyString(config.GetCustomSlot())
	if customSlot != "" {
		if err := customSlot.Validate(); err != nil {
			return hardwarekey.PrivateKeyConfig{}, trace.Wrap(err)
		}
	}

	return hardwarekey.PrivateKeyConfig{
		Policy: hardwarekey.PromptPolicy{
			TouchRequired: config.GetPolicy().GetTouchRequired(),
			PINRequired:   config.GetPolicy().GetPinRequired(),
		},
		CustomSlot:        customSlot,
		Algorithm:         algorithm,
		ContextualKeyInfo: contextualKeyInfoFromProto(config.GetKeyInfo()),
		PINCacheTTL:       config.GetPinCacheTtl().AsDuration(),
	}, err
}

func contextualKeyInfoToProto(info hardwarekey.ContextualKeyInfo) *hardwarekeyagentv1.NewKeyInfo {
	return hardwarekeyagentv1.NewKeyInfo_builder{
		ProxyHost:   info.ProxyHost,
		Username:    info.Username,
		ClusterName: info.ClusterName,
		Command:     info.AgentKeyInfo.Command,
	}.Build()
}

func contextualKeyInfoFromProto(info *hardwarekeyagentv1.NewKeyInfo) hardwarekey.ContextualKeyInfo {
	return hardwarekey.ContextualKeyInfo{
		ProxyHost:   info.GetProxyHost(),
		Username:    info.GetUsername(),
		ClusterName: info.GetClusterName(),
		AgentKeyInfo: hardwarekey.AgentKeyInfo{
			Command: info.GetCommand(),
		},
	}
}

func signatureAlgorithmFromProto(proto hardwarekeyagentv1.SignatureAlgorithm) (hardwarekey.SignatureAlgorithm, error) {
	if _, ok := hardwarekeyagentv1.SignatureAlgorithm_name[int32(proto)]; ok {
		return hardwarekey.SignatureAlgorithm(proto), nil
	}
	return hardwarekey.SignatureAlgorithmUnspecified, trace.BadParameter("unknown signature algorithm %v", proto)
}

func signatureAlgorithmToProto(algorithm hardwarekey.SignatureAlgorithm) (hardwarekeyagentv1.SignatureAlgorithm, error) {
	if _, ok := hardwarekeyagentv1.SignatureAlgorithm_name[int32(algorithm)]; ok {
		return hardwarekeyagentv1.SignatureAlgorithm(algorithm), nil
	}

	return hardwarekeyagentv1.SignatureAlgorithm_SIGNATURE_ALGORITHM_UNSPECIFIED, trace.BadParameter("unknown proto enum for signature algorithm %d", algorithm)
}
