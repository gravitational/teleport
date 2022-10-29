// Copyright 2022 Gravitational, Inc
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

package keystore

import (
	"context"
	"crypto"
	"crypto/rand"
	"errors"
	"sync"
	"testing"

	gax "github.com/googleapis/gax-go/v2"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/iterator"
	kmspb "google.golang.org/genproto/googleapis/cloud/kms/v1"
)

type mockGCPKMSClient struct {
	// map of key version id to state
	keyVersions             map[string]string
	clock                   clockwork.FakeClock
	activateKey             chan struct{}
	keyActivated            chan struct{}
	unblockGetPublicKey     chan struct{}
	unblockDestroyCryptoKey chan struct{}
	unblockIterator         chan struct{}
	signIsBlocked           chan struct{}
	keyIsDestroyed          chan struct{}
	wg                      sync.WaitGroup
	sync.RWMutex
}

func newMockGCPKMSClient() *mockGCPKMSClient {
	return &mockGCPKMSClient{
		keyVersions:             make(map[string]string),
		clock:                   clockwork.NewFakeClock(),
		activateKey:             make(chan struct{}),
		keyActivated:            make(chan struct{}),
		unblockGetPublicKey:     make(chan struct{}),
		unblockDestroyCryptoKey: make(chan struct{}),
		unblockIterator:         make(chan struct{}),
		signIsBlocked:           make(chan struct{}),
		keyIsDestroyed:          make(chan struct{}),
	}
}

// unblockAll services all channels and returns when all goroutines have stopped
// running.
func (m *mockGCPKMSClient) unblockAll() {
	done := make(chan struct{})
	go func() {
		defer close(done)
		m.wg.Wait()
	}()
	for {
		select {
		case m.activateKey <- struct{}{}:
		case m.unblockGetPublicKey <- struct{}{}:
		case m.unblockDestroyCryptoKey <- struct{}{}:
		case m.unblockIterator <- struct{}{}:
		case <-m.keyActivated:
		case <-m.signIsBlocked:
		case <-m.keyIsDestroyed:
		case <-done:
			return
		}
	}
}

func (m *mockGCPKMSClient) CreateCryptoKey(ctx context.Context, req *kmspb.CreateCryptoKeyRequest, opts ...gax.CallOption) (*kmspb.CryptoKey, error) {
	m.Lock()
	defer m.Unlock()
	m.keyVersions[req.CryptoKeyId+keyVersionSuffix] = "PENDING"

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()

		<-m.activateKey
		defer func() { m.keyActivated <- struct{}{} }()

		m.Lock()
		defer m.Unlock()
		m.keyVersions[req.CryptoKeyId+keyVersionSuffix] = "ACTIVE"
	}()

	return &kmspb.CryptoKey{
		Name: req.CryptoKeyId,
	}, nil
}

func (m *mockGCPKMSClient) DestroyCryptoKeyVersion(ctx context.Context, req *kmspb.DestroyCryptoKeyVersionRequest, opts ...gax.CallOption) (*kmspb.CryptoKeyVersion, error) {
	// Don't return until someone sends to unblockDestroyCryptoKey or the
	// context expires.
	select {
	case <-m.unblockDestroyCryptoKey:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	m.Lock()
	defer m.Unlock()
	if m.keyVersions[req.Name] != "ACTIVE" {
		return nil, errors.New("DestroyCryptoKeyVersion failed, state has value " + m.keyVersions[req.Name])
	}
	m.keyVersions[req.Name] = "DESTROY_SCHEDULED"
	m.keyIsDestroyed <- struct{}{}
	return nil, nil
}

func (m *mockGCPKMSClient) ListCryptoKeys(ctx context.Context, req *kmspb.ListCryptoKeysRequest, opts ...gax.CallOption) cryptoKeyIteratorI {
	m.RLock()
	defer m.RUnlock()
	var keys []string
	for keyVersion := range m.keyVersions {
		keyName := keyVersion[:len(keyVersion)-len(keyVersionSuffix)]
		keys = append(keys, keyName)
	}
	return &mockCryptoKeyIterator{
		keys:            keys,
		ctx:             ctx,
		unblockIterator: m.unblockIterator,
	}
}

func (m *mockGCPKMSClient) GetPublicKey(ctx context.Context, req *kmspb.GetPublicKeyRequest, opts ...gax.CallOption) (*kmspb.PublicKey, error) {
	// Don't return until someone sends to unblockGetPublicKey or the context
	// expires. Defer this so that we the state can be determistically changed
	// between calls. If it were not deferred, if a caller were to unblock this
	// and then change the state, it could possibly be reflected in this call or
	// the next.
	defer func() {
		select {
		case <-m.unblockGetPublicKey:
		case <-ctx.Done():
		}
	}()
	m.RLock()
	defer m.RUnlock()
	if m.keyVersions[req.Name] != "ACTIVE" {
		return nil, errors.New("GetPublicKey failed, state has value " + m.keyVersions[req.Name])
	}
	return &kmspb.PublicKey{
		Pem: `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAryQICCl6NZ5gDKrnSztO
3Hy8PEUcuyvg/ikC+VcIo2SFFSf18a3IMYldIugqqqZCs4/4uVW3sbdLs/6PfgdX
7O9D22ZiFWHPYA2k2N744MNiCD1UE+tJyllUhSblK48bn+v1oZHCM0nYQ2NqUkvS
j+hwUU3RiWl7x3D2s9wSdNt7XUtW05a/FXehsPSiJfKvHJJnGOX0BgTvkLnkAOTd
OrUZ/wK69Dzu4IvrN4vs9Nes8vbwPa/ddZEzGR0cQMt0JBkhk9kU/qwqUseP1QRJ
5I1jR4g8aYPL/ke9K35PxZWuDp3U0UPAZ3PjFAh+5T+fc7gzCs9dPzSHloruU+gl
FQIDAQAB
-----END PUBLIC KEY-----`,
	}, nil
}

// Never actually succeeds, just blocks until the context is cancelled
func (m *mockGCPKMSClient) AsymmetricSign(ctx context.Context, req *kmspb.AsymmetricSignRequest, opts ...gax.CallOption) (*kmspb.AsymmetricSignResponse, error) {
	for {
		select {
		case m.signIsBlocked <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

type mockCryptoKeyIterator struct {
	keys            []string
	i               int
	ctx             context.Context
	unblockIterator chan struct{}
	sync.Mutex
}

func (m *mockCryptoKeyIterator) Next() (*kmspb.CryptoKey, error) {
	select {
	case <-m.ctx.Done():
		return nil, m.ctx.Err()
	case <-m.unblockIterator:
	}
	m.Lock()
	defer m.Unlock()
	if m.i >= len(m.keys) {
		return nil, iterator.Done
	}
	m.i = m.i + 1
	return &kmspb.CryptoKey{
		Name: m.keys[m.i-1],
	}, nil
}

// TestGCPKMSKeyStore tests some specific handling of the KMS API. Generic
// keystore tests are in TestKeyStore.
func TestGCPKMSKeyStore(t *testing.T) {
	mockClient := newMockGCPKMSClient()
	s, err := newGCPKMSKeyStore(&GCPKMSConfig{
		KeyRing:           "test-keyring",
		ProtectionLevel:   "HSM",
		HostUUID:          "test-uuid",
		kmsClientOverride: mockClient,
		clockOverride:     mockClient.clock,
	})
	require.NoError(t, err)

	// Make sure the keystore can create the key and fetch the public key, even
	// if the API returns a pending error temporarily.
	t.Run("generateRSA success", func(t *testing.T) {
		done := make(chan struct{})
		go func() {
			defer close(done)
			// Keystore gets a pending key error once...
			mockClient.unblockGetPublicKey <- struct{}{}
			mockClient.clock.Advance(defaultGCPPendingRetryInterval)

			// ... and twice.
			mockClient.unblockGetPublicKey <- struct{}{}

			// Activate the key before advancing the clock so that it can
			// succeed the third time.
			mockClient.activateKey <- struct{}{}
			<-mockClient.keyActivated
			mockClient.clock.Advance(defaultGCPPendingRetryInterval)

			// It should get the active key here.
			mockClient.unblockGetPublicKey <- struct{}{}
		}()

		_, _, err = s.generateRSA()
		require.NoError(t, err)
		<-done
		mockClient.unblockAll()
	})

	// Make sure the request gets cancelled and unblocked if the key is pending
	// for too long.
	t.Run("generateRSA timeout", func(t *testing.T) {
		done := make(chan struct{})
		go func() {
			defer close(done)
			// Let the keystore get the pending public key once.
			mockClient.unblockGetPublicKey <- struct{}{}
			// Then make the mock API "block" until the timeout.
			mockClient.clock.Advance(defaultGCPPendingTimeout)
		}()

		_, _, err = s.generateRSA()
		require.ErrorIs(t, err, context.Canceled)
		<-done
		mockClient.unblockAll()
	})

	// Make sure that signing operations can also timeout.
	t.Run("sign timeout", func(t *testing.T) {
		var wg sync.WaitGroup

		stop := make(chan struct{})
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Don't block key creation, we just want a new key here.
			for {
				select {
				case mockClient.unblockGetPublicKey <- struct{}{}:
				case mockClient.activateKey <- struct{}{}:
				case <-mockClient.keyActivated:
				case <-stop:
					return
				}
				mockClient.clock.Advance(defaultGCPPendingRetryInterval)
			}
		}()

		_, signer, err := s.generateRSA()
		require.NoError(t, err)

		wg.Add(1)
		go func() {
			defer wg.Done()

			// Wait for the signing attempt to be blocked, then advance the
			// clock so it times out.
			<-mockClient.signIsBlocked
			mockClient.clock.Advance(defaultGCPRequestTimeout)
		}()
		_, err = signer.Sign(rand.Reader, []byte("test"), crypto.SHA256)
		require.ErrorIs(t, err, context.Canceled)

		close(stop)
		wg.Wait()
		mockClient.unblockAll()
	})

	// Three keys should have been generated above, make sure we can handle
	// various errors while attempting to destroy them.
	t.Run("delete unused keys", func(t *testing.T) {
		numActiveKeys := func() int {
			n := 0
			for _, state := range mockClient.keyVersions {
				if state == "ACTIVE" {
					n++
				}
			}
			return n
		}

		// Sanity check.
		require.Equal(t, 3, numActiveKeys())

		var wg sync.WaitGroup

		// Iteration should be cancellable
		ctx, cancel := context.WithCancel(context.Background())
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Allow the first iter to succeed
			mockClient.unblockIterator <- struct{}{}

			// Cancel before the second iteration
			cancel()
		}()
		err := s.DeleteUnusedKeys(ctx, nil)
		require.ErrorIs(t, err, context.Canceled)
		require.Equal(t, 3, numActiveKeys())
		wg.Wait()
		mockClient.unblockAll()

		// Destruction should be cancellable
		ctx, cancel = context.WithCancel(context.Background())
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Allow all 3 iterations to succeed, plus one to signal the end.
			mockClient.unblockIterator <- struct{}{}
			mockClient.unblockIterator <- struct{}{}
			mockClient.unblockIterator <- struct{}{}
			mockClient.unblockIterator <- struct{}{}

			// Allow one key to be destroyed
			mockClient.unblockDestroyCryptoKey <- struct{}{}
			<-mockClient.keyIsDestroyed

			// Cancel before the rest can be destroyed
			cancel()
		}()
		err = s.DeleteUnusedKeys(ctx, nil)
		require.ErrorIs(t, err, context.Canceled)
		require.Equal(t, 2, numActiveKeys())
		wg.Wait()
		mockClient.unblockAll()
	})
}
