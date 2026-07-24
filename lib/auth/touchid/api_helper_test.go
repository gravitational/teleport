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

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestHelperNative creates a helperNative wired to a mock helper goroutine.
// The mock reads JSON-RPC requests and dispatches to the provided handler map.
// Unrecognized methods return an error response.
func newTestHelperNative(t *testing.T, handlers map[string]func(json.RawMessage) (any, error)) *helperNative {
	t.Helper()

	// clientReader reads responses from mock helper.
	// mockWriter is where the mock writes responses.
	clientReader, mockWriter := io.Pipe()

	// mockReader reads requests from client.
	// clientWriter is where the client writes requests.
	mockReader, clientWriter := io.Pipe()

	// Start mock helper goroutine.
	go func() {
		defer mockWriter.Close()

		scanner := bufio.NewScanner(mockReader)
		scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
		enc := json.NewEncoder(mockWriter)

		for scanner.Scan() {
			var req helperRequest
			if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
				resp := helperResponse{
					Error: fmt.Sprintf("mock: unmarshal request: %v", err),
				}
				_ = enc.Encode(resp)
				continue
			}

			handler, ok := handlers[req.Method]
			if !ok {
				resp := helperResponse{
					ID:    req.ID,
					Error: fmt.Sprintf("mock: unknown method %q", req.Method),
				}
				_ = enc.Encode(resp)
				continue
			}

			result, err := handler(req.Params)
			if err != nil {
				resp := helperResponse{
					ID:    req.ID,
					Error: err.Error(),
				}
				_ = enc.Encode(resp)
				continue
			}

			var rawResult *json.RawMessage
			if result != nil {
				b, marshalErr := json.Marshal(result)
				if marshalErr != nil {
					resp := helperResponse{
						ID:    req.ID,
						Error: fmt.Sprintf("mock: marshal result: %v", marshalErr),
					}
					_ = enc.Encode(resp)
					continue
				}
				rm := json.RawMessage(b)
				rawResult = &rm
			}

			resp := helperResponse{
				ID:     req.ID,
				Result: rawResult,
			}
			_ = enc.Encode(resp)
		}
	}()

	t.Cleanup(func() {
		clientWriter.Close()
	})

	scanner := bufio.NewScanner(clientReader)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	return &helperNative{
		enc: json.NewEncoder(clientWriter),
		dec: scanner,
	}
}

func TestHelperNativeDiag(t *testing.T) {
	wantDiag := diagResult{
		HasSignature:            true,
		HasEntitlements:         true,
		PassedLAPolicyTest:      true,
		PassedSecureEnclaveTest: true,
		IsAvailable:             true,
	}

	handlers := map[string]func(json.RawMessage) (any, error){
		helperMethodDiag: func(_ json.RawMessage) (any, error) {
			return &wantDiag, nil
		},
	}

	h := newTestHelperNative(t, handlers)

	got, err := h.Diag()
	require.NoError(t, err)
	require.NotNil(t, got)

	// helperNative.Diag() always sets HasCompileSupport to true.
	assert.True(t, got.HasCompileSupport, "HasCompileSupport should be true")
	assert.Equal(t, wantDiag.HasSignature, got.HasSignature, "HasSignature mismatch")
	assert.Equal(t, wantDiag.HasEntitlements, got.HasEntitlements, "HasEntitlements mismatch")
	assert.Equal(t, wantDiag.PassedLAPolicyTest, got.PassedLAPolicyTest, "PassedLAPolicyTest mismatch")
	assert.Equal(t, wantDiag.PassedSecureEnclaveTest, got.PassedSecureEnclaveTest, "PassedSecureEnclaveTest mismatch")
	assert.Equal(t, wantDiag.IsAvailable, got.IsAvailable, "IsAvailable mismatch")
}

func TestHelperNativeRegister(t *testing.T) {
	const (
		wantCredID = "cred-abc-123"
		wantRPID   = "example.com"
		wantUser   = "llama"
	)
	wantUserHandle := []byte{1, 2, 3, 4, 5}
	wantPubKeyRaw := []byte{0x04, 0xAA, 0xBB, 0xCC, 0xDD}

	handlers := map[string]func(json.RawMessage) (any, error){
		helperMethodRegister: func(raw json.RawMessage) (any, error) {
			var params registerParams
			if err := json.Unmarshal(raw, &params); err != nil {
				return nil, fmt.Errorf("unmarshal params: %w", err)
			}

			// Verify the client sent the correct parameters.
			assert.Equal(t, wantRPID, params.RPID, "register RPID mismatch")
			assert.Equal(t, wantUser, params.User, "register User mismatch")
			assert.Equal(t, wantUserHandle, params.UserHandle, "register UserHandle mismatch")

			return &registerResult{
				CredentialID: wantCredID,
				PubKeyRaw:    wantPubKeyRaw,
			}, nil
		},
	}

	h := newTestHelperNative(t, handlers)

	got, err := h.Register(wantRPID, wantUser, wantUserHandle)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, wantCredID, got.CredentialID, "CredentialID mismatch")
	assert.Equal(t, wantPubKeyRaw, got.publicKeyRaw, "publicKeyRaw mismatch")
}

func TestHelperNativeAuthenticate(t *testing.T) {
	const (
		wantContextID    = 42
		wantCredentialID = "cred-auth-456"
	)
	wantDigest := []byte("sha256-digest-here")
	wantSignature := []byte{0xDE, 0xAD, 0xBE, 0xEF}

	handlers := map[string]func(json.RawMessage) (any, error){
		helperMethodAuthenticate: func(raw json.RawMessage) (any, error) {
			var params authenticateParams
			if err := json.Unmarshal(raw, &params); err != nil {
				return nil, fmt.Errorf("unmarshal params: %w", err)
			}

			assert.Equal(t, wantContextID, params.ContextID, "authenticate ContextID mismatch")
			assert.Equal(t, wantCredentialID, params.CredentialID, "authenticate CredentialID mismatch")
			assert.Equal(t, wantDigest, params.Digest, "authenticate Digest mismatch")

			return &authenticateResult{
				Signature: wantSignature,
			}, nil
		},
	}

	h := newTestHelperNative(t, handlers)

	actx := &helperAuthContext{h: h, contextID: wantContextID}
	sig, err := h.Authenticate(actx, wantCredentialID, wantDigest)
	require.NoError(t, err)
	assert.Equal(t, wantSignature, sig, "signature mismatch")
}

func TestHelperNativeFindCredentials(t *testing.T) {
	createTime1 := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	createTime2 := time.Date(2024, 6, 20, 14, 0, 0, 0, time.UTC)

	wantCreds := []credentialInfo{
		{
			CredentialID: "cred-1",
			RPID:         "example.com",
			UserName:     "llama",
			UserHandle:   []byte{1, 2, 3},
			PubKeyRaw:    []byte{0x04, 0xAA},
			CreateTime:   createTime1.Format(time.RFC3339),
		},
		{
			CredentialID: "cred-2",
			RPID:         "example.com",
			UserName:     "alpaca",
			UserHandle:   []byte{4, 5, 6},
			PubKeyRaw:    []byte{0x04, 0xBB},
			CreateTime:   createTime2.Format(time.RFC3339),
		},
	}

	handlers := map[string]func(json.RawMessage) (any, error){
		helperMethodFindCredentials: func(raw json.RawMessage) (any, error) {
			var params findCredentialsParams
			if err := json.Unmarshal(raw, &params); err != nil {
				return nil, fmt.Errorf("unmarshal params: %w", err)
			}

			assert.Equal(t, "example.com", params.RPID, "FindCredentials RPID mismatch")
			assert.Equal(t, "llama", params.User, "FindCredentials User mismatch")

			return &findCredentialsResult{
				Credentials: wantCreds,
			}, nil
		},
	}

	h := newTestHelperNative(t, handlers)

	got, err := h.FindCredentials("example.com", "llama")
	require.NoError(t, err)
	require.Len(t, got, 2, "expected 2 credentials")

	// Verify first credential.
	assert.Equal(t, "cred-1", got[0].CredentialID, "cred[0] CredentialID mismatch")
	assert.Equal(t, "example.com", got[0].RPID, "cred[0] RPID mismatch")
	assert.Equal(t, "llama", got[0].User.Name, "cred[0] User.Name mismatch")
	assert.Equal(t, []byte{1, 2, 3}, got[0].User.UserHandle, "cred[0] User.UserHandle mismatch")
	assert.Equal(t, []byte{0x04, 0xAA}, got[0].publicKeyRaw, "cred[0] publicKeyRaw mismatch")
	assert.True(t, createTime1.Equal(got[0].CreateTime), "cred[0] CreateTime mismatch: got %v, want %v", got[0].CreateTime, createTime1)

	// Verify second credential.
	assert.Equal(t, "cred-2", got[1].CredentialID, "cred[1] CredentialID mismatch")
	assert.Equal(t, "example.com", got[1].RPID, "cred[1] RPID mismatch")
	assert.Equal(t, "alpaca", got[1].User.Name, "cred[1] User.Name mismatch")
	assert.Equal(t, []byte{4, 5, 6}, got[1].User.UserHandle, "cred[1] User.UserHandle mismatch")
	assert.Equal(t, []byte{0x04, 0xBB}, got[1].publicKeyRaw, "cred[1] publicKeyRaw mismatch")
	assert.True(t, createTime2.Equal(got[1].CreateTime), "cred[1] CreateTime mismatch: got %v, want %v", got[1].CreateTime, createTime2)
}

func TestHelperNativeDeleteCredential(t *testing.T) {
	const targetCredID = "cred-to-delete"

	handlers := map[string]func(json.RawMessage) (any, error){
		helperMethodDeleteCredential: func(raw json.RawMessage) (any, error) {
			var params deleteCredentialParams
			if err := json.Unmarshal(raw, &params); err != nil {
				return nil, fmt.Errorf("unmarshal params: %w", err)
			}

			assert.Equal(t, targetCredID, params.CredentialID, "DeleteCredential CredentialID mismatch")
			return nil, nil
		},
	}

	h := newTestHelperNative(t, handlers)

	err := h.DeleteCredential(targetCredID)
	assert.NoError(t, err)
}

func TestHelperNativeError(t *testing.T) {
	const errorMsg = "secure enclave not available"

	handlers := map[string]func(json.RawMessage) (any, error){
		helperMethodDiag: func(_ json.RawMessage) (any, error) {
			return nil, fmt.Errorf("%s", errorMsg)
		},
	}

	h := newTestHelperNative(t, handlers)

	_, err := h.Diag()
	require.Error(t, err)
	assert.Contains(t, err.Error(), errorMsg, "error message should contain the helper error")
	// The error should be wrapped with "helper:" prefix.
	assert.Contains(t, err.Error(), "helper:", "error should have helper prefix")
}

func TestHelperNativeErrorUnknownMethod(t *testing.T) {
	// An empty handler map means all methods are unknown.
	handlers := map[string]func(json.RawMessage) (any, error){}

	h := newTestHelperNative(t, handlers)

	_, err := h.Diag()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown method", "error should indicate unknown method")
}

func TestHelperNativeAuthContext(t *testing.T) {
	const wantContextID = 7

	var (
		guardCalled bool
		closeCalled bool
	)

	handlers := map[string]func(json.RawMessage) (any, error){
		helperMethodNewAuthContext: func(_ json.RawMessage) (any, error) {
			return &newAuthContextResult{
				ContextID: wantContextID,
			}, nil
		},
		helperMethodAuthContextGuard: func(raw json.RawMessage) (any, error) {
			var params authContextGuardParams
			if err := json.Unmarshal(raw, &params); err != nil {
				return nil, fmt.Errorf("unmarshal params: %w", err)
			}
			assert.Equal(t, wantContextID, params.ContextID, "Guard ContextID mismatch")
			guardCalled = true
			return nil, nil
		},
		helperMethodAuthContextClose: func(raw json.RawMessage) (any, error) {
			var params authContextCloseParams
			if err := json.Unmarshal(raw, &params); err != nil {
				return nil, fmt.Errorf("unmarshal params: %w", err)
			}
			assert.Equal(t, wantContextID, params.ContextID, "Close ContextID mismatch")
			closeCalled = true
			return nil, nil
		},
	}

	h := newTestHelperNative(t, handlers)

	// Step 1: NewAuthContext should return a helperAuthContext with the right ID.
	actx := h.NewAuthContext()
	require.NotNil(t, actx)

	hctx, ok := actx.(*helperAuthContext)
	require.True(t, ok, "NewAuthContext should return *helperAuthContext")
	assert.Equal(t, wantContextID, hctx.contextID, "contextID mismatch")

	// Step 2: Guard should send AuthContextGuard RPC and call the callback on success.
	var callbackRan bool
	err := actx.Guard(func() {
		callbackRan = true
	})
	require.NoError(t, err)
	assert.True(t, guardCalled, "Guard RPC was not called")
	assert.True(t, callbackRan, "Guard callback was not called")

	// Step 3: Close should send AuthContextClose RPC.
	actx.Close()
	assert.True(t, closeCalled, "Close RPC was not called")
}

func TestHelperNativeAuthContextGuardError(t *testing.T) {
	const wantContextID = 9
	const guardError = "biometric authentication failed"

	handlers := map[string]func(json.RawMessage) (any, error){
		helperMethodNewAuthContext: func(_ json.RawMessage) (any, error) {
			return &newAuthContextResult{
				ContextID: wantContextID,
			}, nil
		},
		helperMethodAuthContextGuard: func(_ json.RawMessage) (any, error) {
			return nil, fmt.Errorf("%s", guardError)
		},
		helperMethodAuthContextClose: func(_ json.RawMessage) (any, error) {
			return nil, nil
		},
	}

	h := newTestHelperNative(t, handlers)

	actx := h.NewAuthContext()
	require.NotNil(t, actx)

	// Guard should propagate the error and NOT call the callback.
	var callbackRan bool
	err := actx.Guard(func() {
		callbackRan = true
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), guardError, "Guard error should contain helper error message")
	assert.False(t, callbackRan, "Guard callback should not be called on error")

	// Close should still work.
	actx.Close()
}

func TestHelperNativeMultipleRequests(t *testing.T) {
	// Verify that multiple sequential requests work correctly with
	// incrementing IDs.
	diagCount := 0

	handlers := map[string]func(json.RawMessage) (any, error){
		helperMethodDiag: func(_ json.RawMessage) (any, error) {
			diagCount++
			return &diagResult{
				IsAvailable: true,
			}, nil
		},
		helperMethodDeleteCredential: func(raw json.RawMessage) (any, error) {
			var params deleteCredentialParams
			if err := json.Unmarshal(raw, &params); err != nil {
				return nil, fmt.Errorf("unmarshal params: %w", err)
			}
			return nil, nil
		},
	}

	h := newTestHelperNative(t, handlers)

	// Send multiple requests of different types.
	_, err := h.Diag()
	require.NoError(t, err, "first Diag call failed")

	err = h.DeleteCredential("cred-1")
	require.NoError(t, err, "DeleteCredential call failed")

	_, err = h.Diag()
	require.NoError(t, err, "second Diag call failed")

	assert.Equal(t, 2, diagCount, "Diag should have been called twice")
}
