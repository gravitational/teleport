/*
Copyright 2023 Gravitational, Inc.

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

package keystore

import "github.com/gravitational/trace"

var pkcs11Prefix = []byte("pkcs11:")

// PKCS11Config is used to pass PKCS11 HSM client configuration parameters.
type PKCS11Config struct {
	// Path is the path to the PKCS11 module.
	Path string
	// SlotNumber is the PKCS11 slot to use.
	SlotNumber *int
	// TokenLabel is the label of the PKCS11 token to use.
	TokenLabel string
	// Pin is the PKCS11 pin for the given token.
	Pin string

	// HostUUID is the UUID of the local auth server this HSM is connected to.
	HostUUID string
}

func (cfg *PKCS11Config) CheckAndSetDefaults() error {
	if cfg.SlotNumber == nil && cfg.TokenLabel == "" {
		return trace.BadParameter("must provide one of SlotNumber or TokenLabel")
	}
	if cfg.HostUUID == "" {
		return trace.BadParameter("must provide HostUUID")
	}
	return nil
}
