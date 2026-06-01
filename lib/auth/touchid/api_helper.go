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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

const helperBinaryName = "tsh-touchid-helper"

// Compile-time interface checks.
var _ nativeTID = (*helperNative)(nil)
var _ AuthContext = (*helperAuthContext)(nil)

// helperNative is a nativeTID implementation that proxies all calls to a helper
// subprocess via JSON-RPC over stdin/stdout.
type helperNative struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	enc    *json.Encoder  // writes to helper's stdin
	dec    *bufio.Scanner // reads from helper's stdout
	nextID int
}

// sendRequest sends a JSON-RPC request to the helper and decodes the response.
// All calls are serialized via mutex - one at a time.
func (h *helperNative) sendRequest(method string, params any, result any) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Marshal params to json.RawMessage.
	var rawParams json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("helper: marshal params: %w", err)
		}
		rawParams = b
	}

	// Create request with incrementing ID.
	h.nextID++
	req := helperRequest{
		ID:     h.nextID,
		Method: method,
		Params: rawParams,
	}

	// Encode to helper's stdin.
	if err := h.enc.Encode(req); err != nil {
		return fmt.Errorf("helper: write request: %w", err)
	}

	// Read one line from scanner.
	if !h.dec.Scan() {
		if err := h.dec.Err(); err != nil {
			return fmt.Errorf("helper: read response: %w", err)
		}
		return fmt.Errorf("helper: process exited unexpectedly")
	}

	// Decode the response.
	var resp helperResponse
	if err := json.Unmarshal(h.dec.Bytes(), &resp); err != nil {
		return fmt.Errorf("helper: decode response: %w", err)
	}

	// Check for error field.
	if resp.Error != "" {
		return fmt.Errorf("helper: %s", resp.Error)
	}

	// If result is non-nil, unmarshal response.Result into it.
	if result != nil && resp.Result != nil {
		if err := json.Unmarshal(*resp.Result, result); err != nil {
			return fmt.Errorf("helper: decode result: %w", err)
		}
	}

	return nil
}

// close kills the helper process if it is running.
func (h *helperNative) close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.cmd != nil && h.cmd.Process != nil {
		_ = h.cmd.Process.Kill()
		_ = h.cmd.Wait()
	}
}

// Diag implements nativeTID.
func (h *helperNative) Diag() (*DiagResult, error) {
	var result diagResult
	if err := h.sendRequest(helperMethodDiag, nil, &result); err != nil {
		return nil, err
	}
	return &DiagResult{
		HasCompileSupport:       true, // helper has compile support
		HasSignature:            result.HasSignature,
		HasEntitlements:         result.HasEntitlements,
		PassedLAPolicyTest:      result.PassedLAPolicyTest,
		PassedSecureEnclaveTest: result.PassedSecureEnclaveTest,
		IsAvailable:             result.IsAvailable,
	}, nil
}

// NewAuthContext implements nativeTID.
func (h *helperNative) NewAuthContext() AuthContext {
	var result newAuthContextResult
	if err := h.sendRequest(helperMethodNewAuthContext, nil, &result); err != nil {
		// Return a context that will fail on Guard. We cannot return an error
		// here because the interface does not allow it.
		return &helperAuthContext{h: h, contextID: -1}
	}
	return &helperAuthContext{h: h, contextID: result.ContextID}
}

// Register implements nativeTID.
func (h *helperNative) Register(rpID, user string, userHandle []byte) (*CredentialInfo, error) {
	params := registerParams{
		RPID:       rpID,
		User:       user,
		UserHandle: userHandle,
	}
	var result registerResult
	if err := h.sendRequest(helperMethodRegister, &params, &result); err != nil {
		return nil, err
	}
	return &CredentialInfo{
		CredentialID: result.CredentialID,
		publicKeyRaw: result.PubKeyRaw,
	}, nil
}

// Authenticate implements nativeTID.
func (h *helperNative) Authenticate(actx AuthContext, credentialID string, digest []byte) ([]byte, error) {
	var contextID int
	if hctx, ok := actx.(*helperAuthContext); ok {
		contextID = hctx.contextID
	}
	params := authenticateParams{
		ContextID:    contextID,
		CredentialID: credentialID,
		Digest:       digest,
	}
	var result authenticateResult
	if err := h.sendRequest(helperMethodAuthenticate, &params, &result); err != nil {
		return nil, err
	}
	return result.Signature, nil
}

// FindCredentials implements nativeTID.
func (h *helperNative) FindCredentials(rpID, user string) ([]CredentialInfo, error) {
	params := findCredentialsParams{
		RPID: rpID,
		User: user,
	}
	var result findCredentialsResult
	if err := h.sendRequest(helperMethodFindCredentials, &params, &result); err != nil {
		return nil, err
	}
	return convertCredentialInfos(result.Credentials)
}

// ListCredentials implements nativeTID.
func (h *helperNative) ListCredentials() ([]CredentialInfo, error) {
	var result listCredentialsResult
	if err := h.sendRequest(helperMethodListCredentials, nil, &result); err != nil {
		return nil, err
	}
	return convertCredentialInfos(result.Credentials)
}

// DeleteCredential implements nativeTID.
func (h *helperNative) DeleteCredential(credentialID string) error {
	params := deleteCredentialParams{
		CredentialID: credentialID,
	}
	return h.sendRequest(helperMethodDeleteCredential, &params, nil)
}

// DeleteNonInteractive implements nativeTID.
func (h *helperNative) DeleteNonInteractive(credentialID string) error {
	params := deleteNonInteractiveParams{
		CredentialID: credentialID,
	}
	return h.sendRequest(helperMethodDeleteNonInteractive, &params, nil)
}

// convertCredentialInfos converts protocol credentialInfo slices to package
// CredentialInfo slices, parsing create_time from RFC3339.
func convertCredentialInfos(creds []credentialInfo) ([]CredentialInfo, error) {
	infos := make([]CredentialInfo, 0, len(creds))
	for _, c := range creds {
		var createTime time.Time
		if c.CreateTime != "" {
			var err error
			createTime, err = time.Parse(time.RFC3339, c.CreateTime)
			if err != nil {
				// Best-effort: log but don't fail.
				logger.WarnContext(context.Background(), "Failed to parse create_time from helper",
					"create_time", c.CreateTime,
					"error", err,
				)
			}
		}
		infos = append(infos, CredentialInfo{
			CredentialID: c.CredentialID,
			RPID:         c.RPID,
			User: UserInfo{
				UserHandle: c.UserHandle,
				Name:       c.UserName,
			},
			PublicKey:    nil, // Parsed by caller (ListCredentials in api.go).
			CreateTime:   createTime,
			publicKeyRaw: c.PubKeyRaw,
		})
	}
	return infos, nil
}

// helperAuthContext is an AuthContext backed by the helper subprocess.
type helperAuthContext struct {
	h         *helperNative
	contextID int
}

// Guard implements AuthContext. It sends the AuthContextGuard RPC to the helper.
// On success (the helper ran the biometric prompt), it runs fn locally.
func (c *helperAuthContext) Guard(fn func()) error {
	params := authContextGuardParams{
		ContextID: c.contextID,
	}
	if err := c.h.sendRequest(helperMethodAuthContextGuard, &params, nil); err != nil {
		return err
	}
	// The helper has successfully authenticated the user; run the local callback.
	fn()
	return nil
}

// Close implements AuthContext.
func (c *helperAuthContext) Close() {
	params := authContextCloseParams{
		ContextID: c.contextID,
	}
	// Best-effort: ignore errors on close.
	_ = c.h.sendRequest(helperMethodAuthContextClose, &params, nil)
}

// findHelperBinary searches for the helper binary in the following order:
// 1. Same directory as os.Executable()
// 2. exec.LookPath
// 3. ~/.tsh/bin/
func findHelperBinary() (string, error) {
	// 1. Same directory as the current executable.
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), helperBinaryName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// 2. PATH lookup.
	if p, err := exec.LookPath(helperBinaryName); err == nil {
		return p, nil
	}

	// 3. ~/.tsh/bin/
	if home, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(home, ".tsh", "bin", helperBinaryName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("%s: not found", helperBinaryName)
}

// tryHelperFallback attempts to find and spawn the helper binary.
// It returns a helperNative ready for use, or an error if the helper cannot be
// found or started. The caller is responsible for sending the initial Diag RPC.
func tryHelperFallback() (*helperNative, error) {
	binPath, err := findHelperBinary()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(binPath)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("helper: create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("helper: create stdout pipe: %w", err)
	}

	// Forward helper's stderr to our stderr so diagnostics are visible.
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("helper: start process: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	// Increase scanner buffer for potentially large responses.
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	return &helperNative{
		cmd:  cmd,
		enc:  json.NewEncoder(stdin),
		dec:  scanner,
	}, nil
}
