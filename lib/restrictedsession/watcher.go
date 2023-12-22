//go:build bpf && !386
// +build bpf,!386

/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package restrictedsession

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

// NewRestrictionsWatcher returns a new instance of changeset
func NewRestrictionsWatcher(cfg RestrictionsWatcherConfig) (*RestrictionsWatcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  utils.HalfJitter(cfg.MaxRetryPeriod / 10),
		Step:   cfg.MaxRetryPeriod / 5,
		Max:    cfg.MaxRetryPeriod,
		Jitter: retryutils.NewHalfJitter(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancelFn := context.WithCancel(context.Background())

	w := &RestrictionsWatcher{
		retry:                     retry,
		resetC:                    make(chan struct{}),
		cancelFn:                  cancelFn,
		RestrictionsWatcherConfig: cfg,
	}
	w.wg.Add(1)
	go w.watchRestrictions(ctx)
	return w, nil
}

// RestrictionsWatcher is a resource built on top of the events,
// it monitors the changes to restrictions
type RestrictionsWatcher struct {
	RestrictionsWatcherConfig

	resetC chan struct{}

	// retry is used to manage backoff logic for watches
	retry retryutils.Retry

	wg       sync.WaitGroup
	cancelFn context.CancelFunc
}

// RestrictionsWatcherConfig configures restrictions watcher
type RestrictionsWatcherConfig struct {
	// MaxRetryPeriod is the maximum retry period on failed watchers
	MaxRetryPeriod time.Duration
	// ReloadPeriod is a failed period on failed watches
	ReloadPeriod time.Duration
	// Client is used by changeset to monitor restrictions updates
	Client RestrictionsWatcherClient
	// RestrictionsC is a channel that will be used
	// by the watcher to push updated list,
	// it will always receive a fresh list on the start
	// and the subsequent list of new values
	// whenever an addition or deletion to the list is detected
	RestrictionsC chan *NetworkRestrictions
}

// CheckAndSetDefaults checks parameters and sets default values
func (cfg *RestrictionsWatcherConfig) CheckAndSetDefaults() error {
	if cfg.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	if cfg.RestrictionsC == nil {
		return trace.BadParameter("missing parameter RestrictionsC")
	}
	if cfg.MaxRetryPeriod == 0 {
		cfg.MaxRetryPeriod = defaults.MaxWatcherBackoff
	}
	if cfg.ReloadPeriod == 0 {
		cfg.ReloadPeriod = defaults.LowResPollingPeriod
	}
	return nil
}

// Reset returns a channel which notifies of internal
// watcher resets (used in tests).
func (w *RestrictionsWatcher) Reset() <-chan struct{} {
	return w.resetC
}

// Close closes proxy watcher and cancels all the functions
func (w *RestrictionsWatcher) Close() error {
	w.cancelFn()
	w.wg.Wait()
	close(w.RestrictionsC)
	return nil
}

// watchProxies watches new proxies added and removed to the cluster
// and when this happens, notifies all connected agents
// about the proxy set change via discovery requests
func (w *RestrictionsWatcher) watchRestrictions(ctx context.Context) {
	defer w.wg.Done()

	for {
		// Reload period is here to protect against
		// unknown cache going out of sync problems
		// that we did not predict.
		if err := w.watch(ctx); err != nil {
			log.Warningf("Re-init the watcher on error: %v.", trace.Unwrap(err))
		}
		log.Debugf("Reloading %v.", w.retry)
		select {
		case w.resetC <- struct{}{}:
		default:
		}
		select {
		case <-w.retry.After():
			w.retry.Inc()
		case <-ctx.Done():
			log.Debugf("Closed, returning from update loop.")
			return
		}
	}
}

func (w *RestrictionsWatcher) getNetworkRestrictions(ctx context.Context) (*NetworkRestrictions, error) {
	resource, err := w.Client.GetNetworkRestrictions(ctx)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		return &NetworkRestrictions{}, nil
	}

	restrictions, err := protoToNetworkRestrictions(resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return restrictions, nil
}

// watch sets up the watch on proxies
func (w *RestrictionsWatcher) watch(ctx context.Context) error {
	watcher, err := w.Client.NewWatcher(ctx, types.Watch{
		Name: teleport.ComponentRestrictedSession,
		Kinds: []types.WatchKind{
			{
				Kind: types.KindNetworkRestrictions,
			},
		},
		MetricComponent: teleport.ComponentRestrictedSession,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()
	reloadC := time.After(w.ReloadPeriod)
	// before fetch, make sure watcher is synced by receiving init event,
	// to avoid the scenario:
	// 1. Cache process:   w = NewWatcher()
	// 2. Cache process:   c.fetch()
	// 3. Backend process: addItem()
	// 4. Cache process:   <- w.Events()
	//
	// If there is a way that NewWatcher() on line 1 could
	// return without subscription established first,
	// Code line 3 could execute and line 4 could miss event,
	// wrapping up with out of sync replica.
	// To avoid this, before doing fetch,
	// cache process makes sure the connection is established
	// by receiving init event first.
	select {
	case <-reloadC:
		log.Debugf("Triggering scheduled reload.")
		return nil
	case <-ctx.Done():
		return nil
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return trace.BadParameter("expected init event, got %v instead", event.Type)
		}
	}

	restrictions, err := w.getNetworkRestrictions(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	w.retry.Reset()

	select {
	case w.RestrictionsC <- restrictions:
	case <-ctx.Done():
		return nil
	}

	for {
		select {
		case <-reloadC:
			log.Debugf("Triggering scheduled reload.")
			return nil
		case <-ctx.Done():
			return nil
		case event := <-watcher.Events():
			if restrictions := w.processEvent(event); restrictions != nil {
				select {
				case w.RestrictionsC <- restrictions:
				case <-ctx.Done():
					return nil
				}
			}
		}
	}
}

// processEvent updates proxy map and returns true if the proxies list have been modified -
// the proxy has been either added or deleted
func (w *RestrictionsWatcher) processEvent(event types.Event) *NetworkRestrictions {
	if event.Resource.GetKind() != types.KindNetworkRestrictions {
		log.Warningf("Unexpected event: %v.", event.Resource.GetKind())
		return nil
	}

	switch event.Type {
	case types.OpDelete:
		return &NetworkRestrictions{}

	case types.OpPut:
		resource, ok := event.Resource.(types.NetworkRestrictions)
		if !ok {
			log.Warningf("unexpected type %T", event.Resource)
			return nil
		}
		restrictions, err := protoToNetworkRestrictions(resource)
		if err != nil {
			log.Warningf("Bad network restrictions %#v.", resource)
			return nil
		}
		return restrictions

	default:
		log.Warningf("Skipping unsupported event type %v.", event.Type)
		return nil
	}
}

func protoToIPNets(protoAddrs []types.AddressCondition) ([]net.IPNet, error) {
	nets := []net.IPNet{}
	for _, a := range protoAddrs {
		n, err := ParseIPSpec(a.CIDR)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		nets = append(nets, *n)
	}
	return nets, nil
}

func protoToNetworkRestrictions(proto types.NetworkRestrictions) (*NetworkRestrictions, error) {
	deny, err := protoToIPNets(proto.GetDeny())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	allow, err := protoToIPNets(proto.GetAllow())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &NetworkRestrictions{
		Enabled: true,
		Deny:    deny,
		Allow:   allow,
	}, nil
}
