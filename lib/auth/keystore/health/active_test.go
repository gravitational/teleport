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

package health

import (
	"context"
	"crypto"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

type mockKM struct {
	KeyManager
	crypto.Signer
}

func (m *mockKM) GetTLSSigner(_ context.Context, ca types.CertAuthority) (crypto.Signer, error) {
	return m, nil
}

func (m *mockKM) Public() crypto.PublicKey {
	return m
}

func (m *mockKM) Equal(o crypto.PublicKey) bool {
	unwrap, ok := o.(*mockKM)
	return ok && unwrap == m
}

func TestActiveHealthCheckerSync(t *testing.T) {
	ca := &types.CertAuthorityV2{}
	err := ca.SetActiveKeys(types.CAKeySet{
		TLS: []*types.TLSKeyPair{{
			Cert: []byte{0},
		}},
	})
	require.NoError(t, err)

	ch := make(chan []types.CertAuthority, 1)
	ch <- []types.CertAuthority{ca}
	calls := make([]error, 0)
	wg := sync.WaitGroup{}
	wg.Add(1)

	hc, err := NewActiveHealthChecker(ActiveHealthCheckConfig{
		Callback: func(err error) {
			calls = append(calls, err)
			wg.Done()
		},
		ResourceC:  ch,
		KeyManager: &mockKM{},
		Logger:     slog.Default(),
	})
	require.NoError(t, err)
	hc.healthFn = func(_ *healthSigner) error {
		return nil
	}

	go hc.Run(t.Context())
	wg.Wait()
	require.Len(t, calls, 1)
	require.NoError(t, calls[0])
}
