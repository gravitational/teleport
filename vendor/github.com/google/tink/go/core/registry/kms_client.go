// Copyright 2019 Google LLC
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
//
////////////////////////////////////////////////////////////////////////////////

package registry

import "github.com/google/tink/go/tink"

// KMSClient knows how to produce primitives backed by keys stored in remote KMS services.
type KMSClient interface {
	// Supported true if this client does support keyURI
	Supported(keyURI string) bool

	// GetAEAD  gets an AEAD backend by keyURI.
	GetAEAD(keyURI string) (tink.AEAD, error)
}
