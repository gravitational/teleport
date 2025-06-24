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
	"crypto"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/join"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/sshutils"
)

const (
	PrivateKeyPath   = "id_bkp"
	PublicKeyPath    = PrivateKeyPath + ".pub"
	JoinStatePath    = "bkp_state"
	KeyHistoryPath   = "bkp_key_history.json"
	KeyHistoryLength = 10

	StandardFileWriteMode = 0600
)

// KeyHistoryEntry records a private key and the timestamp it was generated.
type KeyHistoryEntry struct {
	// Time is the time this key was inserted into the history.
	Time time.Time `json:"time"`

	// PrivateKey is the private key, encoded in PEM format.
	PrivateKey string `json:"private_key"`
}

// KeyHistory is a collection of `KeyHistoryEntry`.
type KeyHistory struct {
	Entries []KeyHistoryEntry `json:"entries"`
}

// ClientState contains state parameters stored on disk needed to complete the
// bound keypair join process.
type ClientState struct {
	mu sync.Mutex
	fs FS

	// PrivateKey is the parsed private key.
	PrivateKey *keys.PrivateKey

	// PrivateKeyBytes contains the active private key bytes. This value should
	// always be nonempty.
	PrivateKeyBytes []byte

	// PublicKeyBytes contains the active public key bytes. This value is not
	// used at runtime, and is only set when a public key should be written to
	// disk, like on first creation or during rotation. To consistently access
	// the public key, use `.PrivateKey.Public()`.
	PublicKeyBytes []byte

	// JoinStateBytes contains join state bytes. This value will be empty if
	// this client has not yet joined.
	JoinStateBytes []byte

	// KeyHistory records previous keypairs. In the event of a cluster rollback,
	// this history will allow clients to rejoin if the cluster requests a
	// keypair not currently marked as active.
	KeyHistory []KeyHistoryEntry
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
		PreviousJoinState: c.JoinStateBytes,
		InitialJoinSecret: initialJoinSecret,
		GetSigner: func(pubKey string) (crypto.Signer, error) {
			return c.SignerForPublicKey([]byte(pubKey))
		},
		RequestNewKeypair: func(ctx context.Context, getSuite cryptosuites.GetSuiteFunc) (crypto.Signer, error) {
			signer, err := c.GenerateKeypair(ctx, getSuite)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			// Make sure to store the intermediate state. We don't want to risk
			// losing a private key if an error occurs between here and the end
			// of the join process, but also don't want to force
			// `GenerateKeypair()` to trigger a `Store()` on every call, so it's
			// reasonably done here.
			if err := c.Store(ctx); err != nil {
				return nil, trace.Wrap(err)
			}

			return signer, nil
		},
	}
}

// UpdateFromRegisterResult updates this client state from the register result.
func (c *ClientState) UpdateFromRegisterResult(result *join.RegisterResult) error {
	if result.BoundKeypair == nil {
		return trace.BadParameter("register result is missing bound keypair parameters")
	}

	signer, err := c.SignerForPublicKey([]byte(result.BoundKeypair.BoundPublicKey))
	if err != nil {
		return trace.Wrap(err, "fetching key requested by auth")
	}

	if err := c.SetActiveKey(signer); err != nil {
		return trace.Wrap(err, "setting new active key")
	}

	// The helpers above may lock the mutex, so don't lock it until we're
	// touching fields directly.
	c.mu.Lock()
	defer c.mu.Unlock()

	c.JoinStateBytes = result.BoundKeypair.JoinState

	return nil
}

// ToPublicKeyBytes returns the active public key in ssh authorized_keys format.
func (c *ClientState) ToPublicKeyBytes() ([]byte, error) {
	sshPubKey, err := ssh.NewPublicKey(c.PrivateKey.Public())
	if err != nil {
		return nil, trace.Wrap(err, "creating ssh public key")
	}

	return ssh.MarshalAuthorizedKey(sshPubKey), nil
}

// pubKeyEqual compares the two public keys per their `Equal()` implementation.
func pubKeyEqual(a, b crypto.PublicKey) (bool, error) {
	aEq, ok := a.(interface {
		Equal(x crypto.PublicKey) bool
	})
	if !ok {
		return false, trace.BadParameter("unsupported key type %T", a)
	}

	return aEq.Equal(b), nil
}

// SignerForPublicKey attempts to resolve a signer for the given public key
// encoded in authorized_keys format.
func (c *ClientState) SignerForPublicKey(authorizedKeysBytes []byte) (crypto.Signer, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	desiredPubKey, err := sshutils.CryptoPublicKey(authorizedKeysBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Check the active key first, if available.
	if c.PrivateKey != nil {
		activePubKeyBytes, err := c.ToPublicKeyBytes()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		equal, err := pubKeyEqual(desiredPubKey, activePubKeyBytes)
		if err != nil {
			return nil, trace.Wrap(err)
		} else if equal {
			// Parse a fresh copy of the key since this will escape the mutex and we
			// can't be sure our local copy is thread safe.
			key, err := keys.ParsePrivateKey(c.PrivateKeyBytes)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return key.Signer, nil
		}
	}

	// Otherwise, search through the key history. If a keypair rotation was
	// requested an `GenerateKeypair` was called, the new keypair should have
	// been inserted at the top of this list.
	for _, entry := range c.KeyHistory {
		pk, err := keys.ParsePrivateKey([]byte(entry.PrivateKey))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		equal, err := pubKeyEqual(desiredPubKey, pk.Signer.Public())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if equal {
			return pk.Signer, nil
		}
	}

	return nil, trace.NotFound("no matching key found")
}

// GenerateKeypair generates a new keypair, adds it to the key history, and
// returns the resulting signer signer.
func (c *ClientState) GenerateKeypair(ctx context.Context, getSuite cryptosuites.GetSuiteFunc) (crypto.Signer, error) {
	key, err := cryptosuites.GenerateKey(ctx, getSuite, cryptosuites.BoundKeypairJoining)
	if err != nil {
		return nil, trace.Wrap(err, "generating keypair")
	}

	privateKeyBytes, err := keys.MarshalPrivateKey(key)
	if err != nil {
		return nil, trace.Wrap(err, "marshaling private key")
	}

	// prepend the new key to the top of the list for faster lookup
	c.KeyHistory = append([]KeyHistoryEntry{{
		Time:       time.Now(),
		PrivateKey: string(privateKeyBytes),
	}}, c.KeyHistory...)

	// Trim if necessary.
	if len(c.KeyHistory) > KeyHistoryLength {
		c.KeyHistory = c.KeyHistory[:min(len(c.KeyHistory), KeyHistoryLength)]
	}

	sshPubKey, err := ssh.NewPublicKey(key.Public())
	if err != nil {
		return nil, trace.Wrap(err, "creating ssh public key")
	}

	slog.InfoContext(ctx, "Generated new keypair", "public_key", string(ssh.MarshalAuthorizedKey(sshPubKey)))

	return key, nil
}

// SetActiveKey updates the active keypair to reflect the given signer. Has no
// effect if the active keypair's public key is already equal to the given
// signer's public key, per its `Equals` implementation. Note that
// `StoreClientState` still must be called after this to commit the changes to
// the storage backend.
func (c *ClientState) SetActiveKey(signer crypto.Signer) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.PrivateKey != nil {
		equal, err := pubKeyEqual(signer.Public(), c.PrivateKey.Public())
		if err != nil {
			return trace.Wrap(err)
		}

		if equal {
			// nothing to do; specified key is already the active key
			return nil
		}
	}

	key, err := keys.NewPrivateKey(signer)
	if err != nil {
		return trace.Wrap(err)
	}

	privateKeyBytes, err := keys.MarshalPrivateKey(key.Signer)
	if err != nil {
		return trace.Wrap(err, "marshaling private key")
	}

	sshPubKey, err := ssh.NewPublicKey(key.Public())
	if err != nil {
		return trace.Wrap(err, "creating ssh public key")
	}

	c.PrivateKey = key
	c.PrivateKeyBytes = privateKeyBytes
	c.PublicKeyBytes = ssh.MarshalAuthorizedKey(sshPubKey)

	slog.InfoContext(context.Background(), "Set new active keypair", "public_key", string(c.PublicKeyBytes))

	return nil
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

// parseKeyHistory parses marshaled key history from JSON bytes
func parseKeyHistory(data []byte) (KeyHistory, error) {
	var history KeyHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return KeyHistory{}, trace.Wrap(err)
	}

	return history, nil
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

	// The private key may be empty if this is an initial join attempt using a
	// configured registration secret. This is allowed, but callers should
	// handle this via `NewEmptyClientState()`
	if len(privateKeyBytes) == 0 {
		return nil, trace.NotFound("no active private key found")
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

	var keyHistory KeyHistory
	keyHistoryBytes, err := fs.Read(ctx, KeyHistoryPath)
	if trace.IsNotFound(err) {
		// No history, this is allowed.
	} else if err != nil {
		slog.WarnContext(ctx, "unable to read key history, may be unable to recover in the event of a cluster rollback", "error", err)
	} else if len(keyHistoryBytes) > 0 {
		keyHistory, err = parseKeyHistory(keyHistoryBytes)
		if err != nil {
			slog.WarnContext(ctx, "unable to parse key history, may be unable to recover in the event of a cluster rollback", "error", err)
		}
	}

	// If the key history is empty, initialize it with just the current key.
	if len(keyHistory.Entries) == 0 {
		keyHistory.Entries = []KeyHistoryEntry{{
			Time:       time.Now(),
			PrivateKey: string(privateKeyBytes),
		}}
	}

	return &ClientState{
		fs: fs,

		PrivateKey:      pk,
		PrivateKeyBytes: privateKeyBytes,
		JoinStateBytes:  joinStateBytes,
		KeyHistory:      keyHistory.Entries,
	}, nil
}

// StoreClientState writes bound keypair client state to the given filesystem
// wrapper. Public keys and join state will only be written if
func (c *ClientState) Store(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.fs.Write(ctx, PrivateKeyPath, c.PrivateKeyBytes); err != nil {
		return trace.Wrap(err, "writing private key")
	}

	// Only write the public key if it was explicitly provided. This helps save
	// an unnecessary file write.
	if len(c.PublicKeyBytes) > 0 {
		if err := c.fs.Write(ctx, PublicKeyPath, c.PublicKeyBytes); err != nil {
			return trace.Wrap(err, "writing public key")
		}
	}

	if len(c.JoinStateBytes) > 0 {
		if err := c.fs.Write(ctx, JoinStatePath, c.JoinStateBytes); err != nil {
			return trace.Wrap(err, "writing previous join state")
		}
	}

	if len(c.KeyHistory) > 0 {
		bytes, err := json.Marshal(KeyHistory{
			Entries: c.KeyHistory,
		})
		if err != nil {
			return trace.Wrap(err, "marshaling key key history")
		}

		if err := c.fs.Write(ctx, KeyHistoryPath, bytes); err != nil {
			return trace.Wrap(err, "writing key history")
		}
	}

	slog.DebugContext(ctx, "stored new bound keypair client state")

	return nil
}

// NewUnboundClientState creates a new client state that has not yet been bound,
// i.e. a new keypair that has not been registered with Auth, and no prior join
// state. Join attempts using registration secrets should instead use
// `NewEmptyClientState`, which does not immediately generate a keypair.
func NewUnboundClientState(ctx context.Context, fs FS, getSuite cryptosuites.GetSuiteFunc) (*ClientState, error) {
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

	history := []KeyHistoryEntry{
		{
			Time:       time.Now(),
			PrivateKey: string(privateKeyBytes),
		},
	}

	return &ClientState{
		fs: fs,

		PrivateKeyBytes: privateKeyBytes,
		PublicKeyBytes:  publicKeyBytes,
		PrivateKey:      pk,
		KeyHistory:      history,
	}, nil
}

// NewEmptyClientState creates a new ClientState with no existing active private
// key or key history. This is only appropriate when a registration secret
// should be used.
func NewEmptyClientState(fs FS) *ClientState {
	return &ClientState{
		fs: fs,
	}
}
