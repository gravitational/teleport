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

package vnet

import (
	"bytes"
	"context"
	"crypto"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

func TestCallWithBoundedAbandon(t *testing.T) {
	t.Parallel()

	t.Run("returns result before ctx is done", func(t *testing.T) {
		t.Parallel()
		sem := make(chan struct{}, 1)
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		sig, err := callWithBoundedAbandon(ctx, sem, func() ([]byte, error) {
			return []byte("signature"), nil
		})
		require.NoError(t, err)
		require.Equal(t, []byte("signature"), sig)
		// The semaphore slot must be released once fn returns.
		require.Eventually(t, func() bool { return len(sem) == 0 }, time.Second, time.Millisecond)
	})

	t.Run("returns fn error before ctx is done", func(t *testing.T) {
		t.Parallel()
		sem := make(chan struct{}, 1)
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		wantErr := io.ErrUnexpectedEOF
		_, err := callWithBoundedAbandon(ctx, sem, func() ([]byte, error) {
			return nil, wantErr
		})
		require.ErrorIs(t, err, wantErr)
	})

	t.Run("unblocks on ctx deadline when fn stalls", func(t *testing.T) {
		t.Parallel()
		sem := make(chan struct{}, 1)
		// blockC is never closed: fn is abandoned, not cancelled, and stays
		// running until the test process exits.
		blockC := make(chan struct{})

		const timeout = 100 * time.Millisecond
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		start := time.Now()
		_, err := callWithBoundedAbandon(ctx, sem, func() ([]byte, error) {
			<-blockC
			return nil, nil
		})
		elapsed := time.Since(start)

		require.ErrorIs(t, err, context.DeadlineExceeded)
		// The call must return close to the deadline, not hang indefinitely.
		require.Less(t, elapsed, timeout+5*time.Second)
	})

	t.Run("fails fast once the semaphore is saturated", func(t *testing.T) {
		t.Parallel()
		const semCap = 2
		sem := make(chan struct{}, semCap)
		// blockC is never closed, so the semCap goroutines started below hold
		// their semaphore slots for the remaining lifetime of the test binary.
		blockC := make(chan struct{})
		started := make(chan struct{}, semCap)

		stallFn := func() ([]byte, error) {
			started <- struct{}{}
			<-blockC
			return nil, nil
		}
		for range semCap {
			go func() {
				// The semaphore slot is acquired synchronously inside
				// callWithBoundedAbandon before fn runs, so by the time
				// stallFn signals started, the slot is held. ctx never
				// expires here on purpose: these goroutines model an
				// embedded signer that never returns.
				_, _ = callWithBoundedAbandon(context.Background(), sem, stallFn)
			}()
		}
		for range semCap {
			<-started
		}
		require.Len(t, sem, semCap)

		unreached := func() ([]byte, error) {
			t.Error("fn must not run once the semaphore is saturated")
			return nil, nil
		}
		const timeout = 50 * time.Millisecond
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		start := time.Now()
		_, err := callWithBoundedAbandon(ctx, sem, unreached)
		elapsed := time.Since(start)

		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Less(t, elapsed, timeout+5*time.Second)
	})
}

// stallingClientApplicationServiceClient is a fake [vnetv1.ClientApplicationServiceClient]
// whose SignForApp/SignForDB block on blockC, ignoring ctx, to model the
// embedded VNet client application service, which dispatches signing
// synchronously down to a context-unaware crypto.Signer.Sign.
type stallingClientApplicationServiceClient struct {
	vnetv1.ClientApplicationServiceClient
	blockC chan struct{}
}

func (s *stallingClientApplicationServiceClient) SignForApp(context.Context, *vnetv1.SignForAppRequest, ...grpc.CallOption) (*vnetv1.SignForAppResponse, error) {
	<-s.blockC
	return vnetv1.SignForAppResponse_builder{Signature: []byte("signature")}.Build(), nil
}

func (s *stallingClientApplicationServiceClient) SignForDB(context.Context, *vnetv1.SignForDBRequest, ...grpc.CallOption) (*vnetv1.SignForDBResponse, error) {
	<-s.blockC
	return vnetv1.SignForDBResponse_builder{Signature: []byte("signature")}.Build(), nil
}

// TestAppProviderCertSigner_StalledSign proves that a stalled sign on the
// embedded (in-process, context-unaware) path no longer blocks the caller
// past signForAppTimeout.
func TestAppProviderCertSigner_StalledSign(t *testing.T) {
	origTimeout := signForAppTimeout
	signForAppTimeout = 100 * time.Millisecond
	t.Cleanup(func() { signForAppTimeout = origTimeout })

	cert := newSelfSignedCA(t)
	clt := &clientApplicationServiceClient{
		clt:    &stallingClientApplicationServiceClient{blockC: make(chan struct{})},
		closer: io.NopCloser(bytes.NewReader(nil)),
	}
	p := newAppProvider(clt)
	signer, err := p.newAppCertSigner(cert.Certificate[0], vnetv1.AppKey_builder{Name: "some-app"}.Build(), 1234)
	require.NoError(t, err)

	start := time.Now()
	_, err = signer.Sign(nil, make([]byte, 32), crypto.SHA256)
	elapsed := time.Since(start)

	require.Error(t, err)
	require.Less(t, elapsed, signForAppTimeout+5*time.Second)
}

// TestDBProviderCertSigner_StalledSign is the dbProvider counterpart of
// [TestAppProviderCertSigner_StalledSign].
func TestDBProviderCertSigner_StalledSign(t *testing.T) {
	origTimeout := signForDBTimeout
	signForDBTimeout = 100 * time.Millisecond
	t.Cleanup(func() { signForDBTimeout = origTimeout })

	cert := newSelfSignedCA(t)
	clt := &clientApplicationServiceClient{
		clt:    &stallingClientApplicationServiceClient{blockC: make(chan struct{})},
		closer: io.NopCloser(bytes.NewReader(nil)),
	}
	p := newDBProvider(clt)
	signer, err := p.newDBCertSigner(cert.Certificate[0], vnetv1.DatabaseInfo_builder{
		DatabaseKey: vnetv1.DatabaseKey_builder{Name: "some-db"}.Build(),
	}.Build())
	require.NoError(t, err)

	start := time.Now()
	_, err = signer.Sign(nil, make([]byte, 32), crypto.SHA256)
	elapsed := time.Since(start)

	require.Error(t, err)
	require.Less(t, elapsed, signForDBTimeout+5*time.Second)
}
