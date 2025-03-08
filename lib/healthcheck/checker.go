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

package healthcheck

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/interval"
)

type healthChecker interface {
	// GetTargetHealth returns the health checker's target resource health.
	GetTargetHealth() *types.TargetHealth
	// UpdateHealthCheckConfig takes a list of health check configurations,
	// matches a config to the target resource (if one matches) and updates the
	// checker's health check config to the new config (or lack of config).
	// If there is no matching config, the health checker will enter a disabled
	// state. If there is a matching config and the health checker is disabled,
	// then the checker will start itself.
	UpdateHealthCheckConfig(configs []healthCheckConfig)
	// Close closes the health checker.
	Close() error
}

type healthCheckerConfig struct {
	// resource is the target resource.
	resource types.ResourceWithLabels
	// resolverFn is callback func that returns the target endpoints.
	resolverFn EndpointsResolverFunc
	// clock is optional and can be used to control time in tests.
	clock clockwork.Clock
	// log is the checker's log.
	log *slog.Logger
}

func (c *healthCheckerConfig) checkAndSetDefaults() error {
	if c.resource == nil {
		return trace.BadParameter("missing resource")
	}
	if c.resolverFn == nil {
		return trace.BadParameter("missing resolver")
	}
	if c.clock == nil {
		c.clock = clockwork.NewRealClock()
	}
	if c.log == nil {
		c.log = slog.Default()
	}
	c.log = c.log.With(
		"target_name", c.resource.GetName(),
		"target_kind", c.resource.GetKind(),
		"target_origin", c.resource.Origin(),
	)
	return nil
}

func newHealthChecker(ctx context.Context, cfg healthCheckerConfig) (*checker, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(ctx)
	return &checker{
		cfg:      cfg,
		cancel:   cancel,
		closeCtx: ctx,
	}, nil
}

// checker perform health checks against a target resource and keeps track of
// the target resource's health.
type checker struct {
	cfg      healthCheckerConfig
	cancel   context.CancelFunc
	closeCtx context.Context

	lastErr         error
	lastResultCount int

	mu                sync.RWMutex
	resolvedEndpoints []string
	healthCheckConfig *healthCheckConfig
	targetHealth      types.TargetHealth
}

func (c *checker) GetTargetHealth() *types.TargetHealth {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return utils.CloneProtoMsg(&c.targetHealth)
}

func (c *checker) UpdateHealthCheckConfig(configs []healthCheckConfig) {
	newCfg := c.matchConfig(configs)
	c.mu.Lock()
	defer c.mu.Unlock()
	oldCfg := c.healthCheckConfig
	c.healthCheckConfig = newCfg
	if oldCfg == nil && newCfg != nil {
		go c.start()
	}
}

func (c *checker) matchConfig(configs []healthCheckConfig) *healthCheckConfig {
	for _, config := range configs {
		matched, _, err := services.CheckLabelsMatch(
			types.Allow,
			config.matcher,
			nil, // userTraits
			c.cfg.resource,
			false, // debug
		)
		if err != nil {
			c.cfg.log.WarnContext(c.closeCtx,
				"Skipping a health check config that failed to match during config update.",
				"skipped_health_check_config", config.name,
				"error", err,
			)
			continue
		}
		if matched {
			return &config
		}
	}
	return nil
}

func (c *checker) start() {
	currCfg := c.getHealthCheckConfig()
	if currCfg == nil {
		return
	}
	c.cfg.log.InfoContext(c.closeCtx, "Started health checker.",
		"health_check_config", currCfg.name,
		"protocol", currCfg.protocol,
		"interval", currCfg.interval,
		"timeout", currCfg.timeout,
		"healthy_threshold", currCfg.healthyThreshold,
		"unhealthy_threshold", currCfg.unhealthyThreshold,
	)
	defer c.cfg.log.Info("Stopped health checker.",
		"error", c.closeCtx.Err(),
	)

	if c.lastResultCount == 0 {
		c.cfg.log.InfoContext(c.closeCtx, "Running initial health check.")
		c.checkTarget(c.closeCtx, currCfg)
	}

	ticker := interval.New(interval.Config{
		Duration:      currCfg.interval,
		Jitter:        retryutils.SeventhJitter,
		FirstDuration: retryutils.FullJitter(currCfg.interval),
		Clock:         c.cfg.clock,
	})
	defer ticker.Stop()

	for {
		select {
		case <-ticker.Next():
			c.checkTarget(c.closeCtx, currCfg)
		case <-c.closeCtx.Done():
			return
		}

		// check if our config has changed
		newCfg := c.getHealthCheckConfig()
		if newCfg == nil {
			// health checks were disabled, exit
			return
		}
		if newCfg.interval != currCfg.interval {
			ticker.ResetTo(newCfg.interval)
		}
		newThreshold := newCfg.healthyThreshold
		oldThreshold := currCfg.healthyThreshold
		if c.lastErr != nil {
			newThreshold = newCfg.unhealthyThreshold
			oldThreshold = currCfg.unhealthyThreshold
		}
		if newThreshold < oldThreshold && c.lastResultCount >= newThreshold {
			c.onThresholdReached()
		}
		currCfg = newCfg
	}
}

func (c *checker) getHealthCheckConfig() *healthCheckConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.healthCheckConfig
}

func (c *checker) checkTarget(ctx context.Context, cfg *healthCheckConfig) {
	isInit := c.lastResultCount == 0
	checkErr := c.dialTarget(ctx, cfg)
	if ctx.Err() == context.Canceled {
		return
	}
	if (checkErr == nil) == (c.lastErr == nil) {
		c.lastResultCount++
	} else {
		c.lastResultCount = 1
	}
	if checkErr == nil {
		c.cfg.log.DebugContext(ctx, "Target health check succeeded.")
	} else {
		c.cfg.log.DebugContext(ctx, "Target health check failed.", "error", checkErr)
	}
	c.lastErr = checkErr
	if isInit {
		c.processInitResult(cfg)
		return
	}
	c.processResult(cfg)
}

func (c *checker) processInitResult(cfg *healthCheckConfig) {
	healthy := c.lastErr == nil
	if healthy {
		if cfg.healthyThreshold == 1 {
			c.processResult(cfg)
			return
		}
		const message = "Passed the initial health check"
		c.setTargetHealthy(types.TargetHealthTransitionReasonInit, message)
	} else {
		if cfg.unhealthyThreshold == 1 {
			c.processResult(cfg)
			return
		}
		const message = "Failed the initial health check"
		c.setTargetUnhealthy(types.TargetHealthTransitionReasonInit, message)
	}
}

func (c *checker) processResult(cfg *healthCheckConfig) {
	threshold := cfg.healthyThreshold
	if c.lastErr != nil {
		threshold = cfg.unhealthyThreshold
	}
	if c.lastResultCount == threshold {
		c.onThresholdReached()
	}
}

func (c *checker) onThresholdReached() {
	healthy := c.lastErr == nil
	const transitionReason = types.TargetHealthTransitionReasonThreshold
	if healthy {
		desc := fmt.Sprintf("%d consecutive health checks passed", c.lastResultCount)
		c.setTargetHealthy(transitionReason, desc)
	} else {
		desc := fmt.Sprintf("%d consecutive health checks failed", c.lastResultCount)
		c.setTargetUnhealthy(transitionReason, desc)
	}
}

func (c *checker) dialTarget(ctx context.Context, cfg *healthCheckConfig) error {
	endpoints, err := c.cfg.resolverFn(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	c.resolvedEndpoints = endpoints

	d := net.Dialer{Timeout: cfg.timeout}
	switch len(endpoints) {
	case 0:
		return trace.NotFound("failed to resolve target endpoints")
	case 1:
		return trace.Wrap(c.dialEndpoint(ctx, d, endpoints[0]))
	default:
		eg := &errgroup.Group{}
		for _, ep := range endpoints {
			eg.Go(func() error {
				return trace.Wrap(c.dialEndpoint(ctx, d, ep))
			})
		}
		return eg.Wait()
	}
}

func (c *checker) dialEndpoint(ctx context.Context, d net.Dialer, endpoint string) error {
	endpoint, err := normalizeAddress(endpoint)
	if err != nil {
		return trace.Wrap(err)
	}
	conn, err := d.DialContext(ctx, "tcp", endpoint)
	if err != nil {
		return trace.Wrap(err)
	}
	err = conn.Close()
	if err != nil {
		c.cfg.log.DebugContext(ctx, "Error closing connection.",
			"error", err,
		)
	}
	return nil
}

func (c *checker) setTargetHealthy(reason types.TargetHealthTransitionReason, message string) {
	c.setTargetHealthStatus(types.TargetHealthStatusHealthy, reason, message)
}

func (c *checker) setTargetUnhealthy(reason types.TargetHealthTransitionReason, message string) {
	c.setTargetHealthStatus(types.TargetHealthStatusUnhealthy, reason, message)
}

func (c *checker) setTargetHealthStatus(status types.TargetHealthStatus, reason types.TargetHealthTransitionReason, message string) {
	c.cfg.log.DebugContext(context.TODO(), "Updating target health status.",
		"old_status", c.targetHealth.Status,
		"new_status", status,
	)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.healthCheckConfig == nil {
		return
	}
	now := c.cfg.clock.Now()
	c.targetHealth = types.TargetHealth{
		Address:             strings.Join(c.resolvedEndpoints, ","),
		Protocol:            string(c.healthCheckConfig.protocol),
		Status:              string(status),
		TransitionTimestamp: &now,
		TransitionReason:    string(reason),
		Message:             message,
	}
	if c.lastErr != nil {
		c.targetHealth.TransitionError = c.lastErr.Error()
	}
}

func (c *checker) Close() error {
	c.cancel()
	return nil
}
