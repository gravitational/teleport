// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package internal

import (
	"bytes"

	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// keyUsage marks a given key to be used either with signing or decryption
type keyUsage string

const (
	keyUsageNone    keyUsage = ""
	keyUsageSign    keyUsage = "sign"
	keyUsageDecrypt keyUsage = "decrypt"
)

func (u keyUsage) toAWS() kmstypes.KeyUsageType {
	switch u {
	case keyUsageDecrypt:
		return kmstypes.KeyUsageTypeEncryptDecrypt
	default:
		return kmstypes.KeyUsageTypeSignVerify
	}
}

// KeyType returns the type of the given private key.
func KeyType(key []byte) types.PrivateKeyType {
	if bytes.HasPrefix(key, pkcs11Prefix) {
		return types.PrivateKeyType_PKCS11
	}
	if bytes.HasPrefix(key, []byte(gcpkmsPrefix)) {
		return types.PrivateKeyType_GCP_KMS
	}
	if bytes.HasPrefix(key, []byte(awskmsPrefix)) {
		return types.PrivateKeyType_AWS_KMS
	}
	return types.PrivateKeyType_RAW
}

func KeyDescription(key []byte) (string, error) {
	switch KeyType(key) {
	case types.PrivateKeyType_PKCS11:
		keyID, err := parsePKCS11KeyID(key)
		if err != nil {
			return "", trace.Wrap(err)
		}
		return "PKCS#11 HSM keys created by " + keyID.HostID, nil
	case types.PrivateKeyType_GCP_KMS:
		keyID, err := ParseGCPKMSKeyID(key)
		if err != nil {
			return "", trace.Wrap(err)
		}
		keyring, err := keyID.keyring()
		if err != nil {
			return "", trace.Wrap(err)
		}
		return "GCP KMS keys in keyring " + keyring, nil
	case types.PrivateKeyType_AWS_KMS:
		keyID, err := ParseAWSKMSKeyID(key)
		if err != nil {
			return "", trace.Wrap(err)
		}
		return "AWS KMS keys in account " + keyID.Account + " and region " + keyID.Region, nil
	default:
		return "raw software keys", nil
	}
}
