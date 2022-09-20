// Go FIDO U2F Library
// Copyright 2015 The Go FIDO U2F Library Authors. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package u2f

import (
	"encoding/json"
)

// JwkKey represents a public key used by a browser for the Channel ID TLS
// extension.
type JwkKey struct {
	KTy string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

// ClientData as defined by the FIDO U2F Raw Message Formats specification.
type ClientData struct {
	Typ       string          `json:"typ"`
	Challenge string          `json:"challenge"`
	Origin    string          `json:"origin"`
	CIDPubKey json.RawMessage `json:"cid_pubkey"`
}

// RegisterRequest as defined by the FIDO U2F Javascript API.
type RegisterRequest struct {
	Version   string `json:"version"`
	Challenge string `json:"challenge"`
	AppID     string `json:"appId"`
}

// RegisterResponse as defined by the FIDO U2F Javascript API.
type RegisterResponse struct {
	RegistrationData string `json:"registrationData"`
	ClientData       string `json:"clientData"`
}

// SignRequest as defined by the FIDO U2F Javascript API.
type SignRequest struct {
	Version   string `json:"version"`
	Challenge string `json:"challenge"`
	KeyHandle string `json:"keyHandle"`
	AppID     string `json:"appId"`
}

// SignResponse as defined by the FIDO U2F Javascript API.
type SignResponse struct {
	KeyHandle     string `json:"keyHandle"`
	SignatureData string `json:"signatureData"`
	ClientData    string `json:"clientData"`
}

// TrustedFacets as defined by the FIDO AppID and Facet Specification.
type TrustedFacets struct {
	Version struct {
		Major int `json:"major"`
		Minor int `json:"minor"`
	} `json:"version"`
	Ids []string `json:"ids"`
}

// TrustedFacetsEndpoint is a container of TrustedFacets.
// It is used as the response for an appId URL endpoint.
type TrustedFacetsEndpoint struct {
	TrustedFacets []TrustedFacets `json:"trustedFacets"`
}
