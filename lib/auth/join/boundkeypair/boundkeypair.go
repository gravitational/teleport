/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package boundkeypair

import (
	"context"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/join"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

const (
	PrivateKeyPath = "id_bkp"
	PublicKeyPath  = PrivateKeyPath + ".pub"
	JoinStatePath  = "bkp_state"

	StandardFileWriteMode = 0600
)

// ClientState contains state parameters stored on disk needed to complete the
// bound keypair join process.
type ClientState struct {
	// PrivateKey is the parsed private key.
	PrivateKey *keys.PrivateKey

	// PrivateKeyBytes contains the private key bytes. This value should always
	// be nonempty.
	PrivateKeyBytes []byte

	// PublicKeyBytes contains the public key bytes. This value is not used at
	// runtime, and is only set when a public key should be written to disk,
	// like on first creation or during rotation. To consistently access the
	// public key, use `.PrivateKey.Public()`.
	PublicKeyBytes []byte

	// JoinStateBytes contains join state bytes. This value will be empty if
	// this client has not yet joined.
	JoinStateBytes []byte
}

// ToJoinParams creates joining parameters for use with `join.Register()` from
// this client state.
func (c *ClientState) ToJoinParams(initialJoinSecret string) *join.BoundKeypairParams {
	if len(c.JoinStateBytes) > 0 {
		// This identity has been bound, so don't pass along the join secret (if
		// any)
		initialJoinSecret = ""
	}

	return &join.BoundKeypairParams{
		// Note: pass the internal signer because go-jose does type assertions
		// on the standard library types.
		CurrentKey:        c.PrivateKey.Signer,
		PreviousJoinState: c.JoinStateBytes,
		InitialJoinSecret: initialJoinSecret,
	}
}

// UpdateFromRegisterResult updates this client state from the register result.
func (c *ClientState) UpdateFromRegisterResult(result *join.RegisterResult) error {
	if result.BoundKeypair == nil {
		return trace.BadParameter("register result is missing bound keypair parameters")
	}

	c.JoinStateBytes = result.BoundKeypair.JoinState

	// TODO: When implementing rotation, use the bound public key value to set
	// the current public key.

	return nil
}

// ToPublicKeyBytes returns the public key bytes in ssh authorized_keys format.
func (c *ClientState) ToPublicKeyBytes() ([]byte, error) {
	sshPubKey, err := ssh.NewPublicKey(c.PrivateKey.Public())
	if err != nil {
		return nil, trace.Wrap(err, "creating ssh public key")
	}

	return ssh.MarshalAuthorizedKey(sshPubKey), nil
}

type FS interface {
	Read(ctx context.Context, name string) ([]byte, error)
	Write(ctx context.Context, name string, data []byte) error
}

type StandardFS struct {
	parentDir string
}

func (f *StandardFS) Read(ctx context.Context, name string) ([]byte, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return data, nil
}

func (f *StandardFS) Write(ctx context.Context, name string, data []byte) error {
	path := filepath.Join(f.parentDir, name)

	return trace.Wrap(os.WriteFile(path, data, StandardFileWriteMode))
}

// NewStandardFS creates a new standard FS implementation.
func NewStandardFS(parentDir string) FS {
	return &StandardFS{
		parentDir: parentDir,
	}
}

// LoadClientState attempts to load bound keypair client state from the given
// filesystem implementation. Callers should expect to handle NotFound errors
// returned here if a private key is not found; this indicates no prior client
// state exists and initial secret joining should be attempted if possible. If
// a keypair has been pregenerated, no prior join state will exist, and the
// join state will be empty; any corresponding errors while reading nonexistent
// join state documents will be ignored.
func LoadClientState(ctx context.Context, fs FS) (*ClientState, error) {
	privateKeyBytes, err := fs.Read(ctx, PrivateKeyPath)
	if err != nil {
		return nil, trace.Wrap(err, "reading private key")
	}

	joinStateBytes, err := fs.Read(ctx, JoinStatePath)
	if trace.IsNotFound(err) {
		// Join state doesn't exist, this is allowed.
	} else if err != nil {
		return nil, trace.Wrap(err, "reading previous join state")
	}

	pk, err := keys.ParsePrivateKey(privateKeyBytes)
	if err != nil {
		return nil, trace.Wrap(err, "parsing private key")
	}

	return &ClientState{
		PrivateKey: pk,

		PrivateKeyBytes: privateKeyBytes,
		JoinStateBytes:  joinStateBytes,
	}, nil
}

// StoreClientState writes bound keypair client state to the given filesystem
// wrapper. Public keys and join state will only be written if
func StoreClientState(ctx context.Context, fs FS, state *ClientState) error {
	if err := fs.Write(ctx, PrivateKeyPath, state.PrivateKeyBytes); err != nil {
		return trace.Wrap(err, "writing private key")
	}

	// TODO: maybe consider just not writing the public key at all. End users
	// aren't really meant to look in the internal storage, and we can just
	// derive the public key whenever we want.

	// Only write the public key if it was explicitly provided. This helps save
	// an unnecessary file write.
	if len(state.PublicKeyBytes) > 0 {
		if err := fs.Write(ctx, PublicKeyPath, state.PublicKeyBytes); err != nil {
			return trace.Wrap(err, "writing public key")
		}
	}

	if len(state.JoinStateBytes) > 0 {
		if err := fs.Write(ctx, JoinStatePath, state.JoinStateBytes); err != nil {
			return trace.Wrap(err, "writing previous join state")
		}
	}

	return nil
}

// NewUnboundClientState creates a new client state that has not yet been bound,
// i.e. a new keypair that has not been registered with Auth, and no prior join
// state.
func NewUnboundClientState(ctx context.Context, getSuite cryptosuites.GetSuiteFunc) (*ClientState, error) {
	key, err := cryptosuites.GenerateKey(ctx, getSuite, cryptosuites.BoundKeypairJoining)
	if err != nil {
		return nil, trace.Wrap(err, "generating keypair")
	}

	privateKeyBytes, err := keys.MarshalPrivateKey(key)
	if err != nil {
		return nil, trace.Wrap(err, "marshaling private key")
	}

	sshPubKey, err := ssh.NewPublicKey(key.Public())
	if err != nil {
		return nil, trace.Wrap(err, "creating ssh public key")
	}

	publicKeyBytes := ssh.MarshalAuthorizedKey(sshPubKey)

	pk, err := keys.NewPrivateKey(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ClientState{
		PrivateKeyBytes: privateKeyBytes,
		PublicKeyBytes:  publicKeyBytes,
		PrivateKey:      pk,
	}, nil
}
