// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package touchid

import "encoding/json"

// Method name constants for the helper JSON-RPC protocol.
const (
	helperMethodDiag             = "Diag"
	helperMethodRegister         = "Register"
	helperMethodAuthenticate     = "Authenticate"
	helperMethodFindCredentials  = "FindCredentials"
	helperMethodListCredentials  = "ListCredentials"
	helperMethodDeleteCredential = "DeleteCredential"
	helperMethodDeleteNonInteractive = "DeleteNonInteractive"
	helperMethodNewAuthContext   = "NewAuthContext"
	helperMethodAuthContextGuard = "AuthContextGuard"
	helperMethodAuthContextClose = "AuthContextClose"
)

// helperRequest is the JSON-RPC request envelope sent from tsh to the helper.
// Encoded as line-delimited JSON over stdin.
type helperRequest struct {
	ID     int             `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// helperResponse is the JSON-RPC response envelope sent from the helper to tsh.
// Encoded as line-delimited JSON over stdout.
type helperResponse struct {
	ID     int              `json:"id"`
	Result *json.RawMessage `json:"result,omitempty"`
	Error  string           `json:"error,omitempty"`
}

// diagResult is the protocol representation of DiagResult.
type diagResult struct {
	HasSignature            bool `json:"has_signature"`
	HasEntitlements         bool `json:"has_entitlements"`
	PassedLAPolicyTest      bool `json:"passed_la_policy_test"`
	PassedSecureEnclaveTest bool `json:"passed_secure_enclave_test"`
	IsAvailable             bool `json:"is_available"`
}

// registerParams are the parameters for the Register method.
type registerParams struct {
	RPID       string `json:"rpid"`
	User       string `json:"user"`
	UserHandle []byte `json:"user_handle"`
}

// registerResult is the result of the Register method.
type registerResult struct {
	CredentialID string `json:"credential_id"`
	PubKeyRaw    []byte `json:"pub_key_raw"`
}

// authenticateParams are the parameters for the Authenticate method.
type authenticateParams struct {
	ContextID    int    `json:"context_id"`
	CredentialID string `json:"credential_id"`
	Digest       []byte `json:"digest"`
}

// authenticateResult is the result of the Authenticate method.
type authenticateResult struct {
	Signature []byte `json:"signature"`
}

// findCredentialsParams are the parameters for the FindCredentials method.
type findCredentialsParams struct {
	RPID string `json:"rpid"`
	User string `json:"user"`
}

// credentialInfo is the protocol representation of CredentialInfo.
// Binary fields (user_handle, pub_key_raw) are base64-encoded by
// encoding/json automatically since they are []byte.
type credentialInfo struct {
	CredentialID string `json:"credential_id"`
	RPID         string `json:"rpid"`
	UserName     string `json:"user_name"`
	UserHandle   []byte `json:"user_handle"`
	PubKeyRaw    []byte `json:"pub_key_raw"`
	CreateTime   string `json:"create_time"`
}

// findCredentialsResult is the result of the FindCredentials method.
type findCredentialsResult struct {
	Credentials []credentialInfo `json:"credentials"`
}

// listCredentialsResult is the result of the ListCredentials method.
type listCredentialsResult struct {
	Credentials []credentialInfo `json:"credentials"`
}

// deleteCredentialParams are the parameters for the DeleteCredential method.
type deleteCredentialParams struct {
	CredentialID string `json:"credential_id"`
}

// deleteNonInteractiveParams are the parameters for the DeleteNonInteractive method.
type deleteNonInteractiveParams struct {
	CredentialID string `json:"credential_id"`
}

// newAuthContextResult is the result of the NewAuthContext method.
type newAuthContextResult struct {
	ContextID int `json:"context_id"`
}

// authContextGuardParams are the parameters for the AuthContextGuard method.
type authContextGuardParams struct {
	ContextID int `json:"context_id"`
}

// authContextCloseParams are the parameters for the AuthContextClose method.
type authContextCloseParams struct {
	ContextID int `json:"context_id"`
}
