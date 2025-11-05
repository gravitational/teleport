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

package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"

	"github.com/gravitational/teleport/api/client/webclient"
)

// teleportResolverBuilder is responsible for building a wrapped DNS resolver that polls the provided
// PingURL to determine what the service configuration should be.
//
// teleportResolverBuilder is intentionally not registered as a [resolver.Builder] so that gRPC clients
// must explicitly opt in to using a wrapped DNS resolver.
type teleportResolverBuilder struct {
	PingURL string
}

// Build returns a wrapped DNS resolver that polls the configured PingURL to determine what the
// service configuration should be.
func (r teleportResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	dnsBuilder := resolver.Get("dns")
	if dnsBuilder == nil {
		return nil, errors.New("unable to get dns resolver")
	}

	ctx, cancel := context.WithCancel(context.Background())
	wr := wrappedResolver{
		ClientConn: cc,

		ctx:    ctx,
		cancel: cancel,
	}

	dnsResolver, err := dnsBuilder.Build(target, &wr, opts)
	if err != nil {
		return nil, err
	}

	wr.Resolver = dnsResolver

	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	go func() {
		for {
			select {
			case <-time.After(30 * time.Second):
				serviceConfig, err := poll(wr.ctx, wr.ClientConn, r.PingURL)
				if err != nil {
					log.ErrorContext(wr.ctx, "error polling", slog.String("err", err.Error()))
					continue
				}

				wr.mu.Lock()
				state := wr.resolvedState
				wr.mu.Unlock()
				state.ServiceConfig = serviceConfig

				wr.UpdateState(state)
			case <-wr.ctx.Done():
				return
			}
		}
	}()

	return &wr, nil
}

// Scheme is set to dns so that gRPC clients including this Resolver may then use for wrapped the DNS resolver.
func (teleportResolverBuilder) Scheme() string {
	return "dns"
}

// wrappedResolver wraps an underlying [resolver.Resolver] and [resolver.ClientConn] to intercept calls to UpdateState and Close.
type wrappedResolver struct {
	resolver.Resolver
	resolver.ClientConn

	ctx    context.Context
	cancel context.CancelFunc

	// mu guards access to resolvedState
	// other fields are set when a new wrappedResolver is created and only read
	// from after that point.
	mu            sync.Mutex
	resolvedState resolver.State
}

func poll(ctx context.Context, cc resolver.ClientConn, findURL string) (*serviceconfig.ParseResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, findURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating GET request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making GET request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET request failed with status [%d] %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	var pingResponse webclient.PingResponse
	if err := json.Unmarshal(body, &pingResponse); err != nil {
		return nil, fmt.Errorf("error unmarshalling JSON response: %w", err)
	}

	if pingResponse.GRPCClientLoadBalancerPolicy == nil {
		return nil, nil
	}

	serviceConfig, err := json.Marshal(pingResponse.GRPCClientLoadBalancerPolicy)
	if err != nil {
		return nil, fmt.Errorf("error marshaling service config: %w", err)
	}

	return cc.ParseServiceConfig(string(serviceConfig)), nil
}

// Close cancels polling and closes the underlying resolver.
func (w *wrappedResolver) Close() {
	w.cancel()

	w.Resolver.Close()
}

// UpdateState intercepts calls to save resolved state so changes to service configuration
// can use the last resolved addresses.
func (w *wrappedResolver) UpdateState(state resolver.State) error {
	w.mu.Lock()
	w.resolvedState = state
	w.mu.Unlock()

	return w.ClientConn.UpdateState(state)
}
