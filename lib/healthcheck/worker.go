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
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/utils/interval"
	"github.com/gravitational/teleport/lib/utils/log"
)

// workerConfig is the configuration for a [workerI].
type workerConfig struct {
	// Clock is optional and is used to control time in tests.
	Clock clockwork.Clock
	// HealthCheckCfg is the config for performing health checks.
	// If not specified, then health checks are disabled until the worker is
	// given a non-nil health check config to use.
	HealthCheckCfg *healthCheckConfig
	// Log is an optional logger.
	Log *slog.Logger
	// Target is the health check target.
	Target Target
	// getTargetHealthTimeout is the timeout to wait for an initial health
	// check before returning the target health to callers of GetTargetHealth.
	getTargetHealthTimeout time.Duration
	// metricType is the resource type (db, k8s, etc) use in Prometheus metrics.
	metricType string
}

// checkAndSetDefaults checks the worker config and sets defaults.
func (cfg *workerConfig) checkAndSetDefaults() error {
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	if cfg.getTargetHealthTimeout == 0 {
		cfg.getTargetHealthTimeout = 4 * time.Second
	}
	if err := cfg.Target.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// newWorker creates and starts a new worker.
func newWorker(ctx context.Context, cfg workerConfig) (*worker, error) {
	w, err := newUnstartedWorker(ctx, cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go w.run()
	return w, nil
}

// newUnstartedWorker creates a worker without running it.
func newUnstartedWorker(ctx context.Context, cfg workerConfig) (*worker, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(ctx)
	w := &worker{
		closeContext:              ctx,
		cancel:                    cancel,
		clock:                     cfg.Clock,
		healthCheckCfg:            cfg.HealthCheckCfg,
		healthCheckConfigUpdateCh: make(chan *healthCheckConfig, 1),
		log:                       cfg.Log,
		target:                    cfg.Target,
		getTargetHealthTimeout:    cfg.getTargetHealthTimeout,
		metricType:                cfg.metricType,
	}
	if w.healthCheckCfg != nil {
		w.setTargetInit(ctx)
	} else {
		w.setTargetDisabled(ctx)
	}
	return w, nil
}

// worker perform health checks against a target resource and keeps track of
// the target resource's health.
type worker struct {
	// closeContext is the work close context.
	closeContext context.Context
	// cancel stops the worker permanently when called.
	cancel context.CancelFunc
	// clock is used to control time in tests.
	clock clockwork.Clock
	// healthCheckCfg is the current config used for performing health checks.
	// Nil if no health check config currently matches the target resource.
	healthCheckCfg *healthCheckConfig
	// healthCheckConfigUpdateCh is used to notify the worker of a new health
	// check config.
	healthCheckConfigUpdateCh chan *healthCheckConfig
	// healthCheckInterval is the current interval between health checks. Nil
	// if there isn't currently a matching health check config for this worker.
	healthCheckInterval *interval.Interval
	// log emits logs.
	log *slog.Logger
	// target is the health check target.
	target Target

	// lastResultErr is the last health check error or nil if there was no error.
	lastResultErr error
	// lastResultCount is the count of consecutive passing or failing health
	// check results.
	lastResultCount uint32
	// lastResolvedEndpoints are the endpoints last resolved for a health check.
	lastResolvedEndpoints []string

	// mu guards concurrent access to the target health.
	mu sync.RWMutex
	// targetHealth is the latest target health. Initialized to "unknown" status
	// before the worker starts.
	targetHealth types.TargetHealth
	// initCheckPendingCh is non-nil when the target health is unknown because
	// the worker is still running an initial health check. When the worker
	// transitions to any other status, the channel is closed and this field is
	// set to nil.
	initCheckPendingCh chan struct{}
	// getTargetHealthTimeout is the timeout to wait for an initial health
	// check before returning the target health to callers of GetTargetHealth.
	getTargetHealthTimeout time.Duration
	// metricType is the resource type (db, k8s, etc) use in Prometheus metrics.
	metricType string
}

// GetTargetHealth returns the worker's target health.
func (w *worker) GetTargetHealth() *types.TargetHealth {
	w.mu.RLock()
	defer w.mu.RUnlock()
	w.waitForInitCheckLocked(w.getTargetHealthTimeout)
	return utils.CloneProtoMsg(&w.targetHealth)
}

// GetTargetResource returns the target resource.
func (w *worker) GetTargetResource() types.ResourceWithLabels {
	return w.target.GetResource()
}

// UpdateHealthCheckConfig updates the worker to use a new health check
// config.
func (w *worker) UpdateHealthCheckConfig(newCfg *healthCheckConfig) {
	// drain pending config update, if any
	select {
	case <-w.healthCheckConfigUpdateCh:
	default:
	}
	w.healthCheckConfigUpdateCh <- newCfg
}

// Close closes the health checker.
func (w *worker) Close() error {
	w.cancel()
	return nil
}

func (w *worker) run() {
	defer func() {
		if w.healthCheckInterval != nil {
			w.healthCheckInterval.Stop()
		}
		if w.initCheckPendingCh != nil {
			close(w.initCheckPendingCh)
		}
		if w.target.onClose != nil {
			w.target.onClose()
		}
	}()

	if w.healthCheckCfg != nil {
		w.startHealthCheckInterval(w.closeContext)
		// no delay for the first health check after a target is registered
		w.healthCheckInterval.FireNow()
	}

	// for simplicity, the worker runs a single-threaded loop and everything the
	// worker does is synchronized through channels, so it only needs to use its
	// mutex to guard target health and resource exposed in its getter methods
	for {
		select {
		case <-w.nextHealthCheck():
			w.checkHealth(w.closeContext)
			if w.target.onHealthCheck != nil {
				w.target.onHealthCheck(w.lastResultErr)
			}
		case newCfg := <-w.healthCheckConfigUpdateCh:
			w.updateHealthCheckConfig(w.closeContext, newCfg)
		case <-w.closeContext.Done():
			w.mu.RLock()
			targetHealthStatus := w.targetHealth.Status
			w.mu.RUnlock()
			w.decrementPreviousMetric(targetHealthStatus)
			return
		}
	}
}

// startHealthCheckInterval starts the health check interval.
func (w *worker) startHealthCheckInterval(ctx context.Context) {
	w.log.InfoContext(ctx, "Health checker started",
		"health_check_config", w.healthCheckCfg.name,
		"interval", log.StringerAttr(w.healthCheckCfg.interval),
		"timeout", log.StringerAttr(w.healthCheckCfg.timeout),
		"healthy_threshold", w.healthCheckCfg.healthyThreshold,
		"unhealthy_threshold", w.healthCheckCfg.unhealthyThreshold,
	)
	w.healthCheckInterval = interval.New(interval.Config{
		Duration:      w.healthCheckCfg.interval,
		Jitter:        retryutils.SeventhJitter,
		FirstDuration: retryutils.FullJitter(w.healthCheckCfg.interval),
		Clock:         w.clock,
	})
}

// stopHealthCheckInterval stops the health check interval.
func (w *worker) stopHealthCheckInterval(ctx context.Context) {
	w.log.InfoContext(ctx, "Health checker stopped")
	w.lastResultErr = nil
	w.lastResultCount = 0
	w.lastResolvedEndpoints = nil
	w.healthCheckInterval.Stop()
	w.healthCheckInterval = nil
}

// nextHealthCheck returns a receiver channel that fires for the next health
// check. If health checks are currently disabled, then it returns a nil channel
// that blocks forever.
func (w *worker) nextHealthCheck() <-chan time.Time {
	if w.healthCheckInterval != nil {
		return w.healthCheckInterval.Next()
	}
	return nil
}

// checkHealth performs a single health check against resolved endpoints,
// updates the worker's health check result history, and possibly updates the
// target health.
func (w *worker) checkHealth(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, w.healthCheckCfg.timeout)
	defer cancel()

	// check target health
	var curErr error
	w.lastResolvedEndpoints, curErr = w.target.CheckHealth(ctx)

	if ctx.Err() == context.Canceled {
		return
	}
	initializing := w.lastResultCount == 0
	if (curErr == nil) == (w.lastResultErr == nil) {
		w.lastResultCount++
	} else {
		// the passing/failing result streak has ended, so reset the count
		w.lastResultCount = 1
	}
	w.lastResultErr = curErr

	if w.lastResultErr != nil {
		w.log.DebugContext(ctx, "Failed health check",
			"error", w.lastResultErr,
		)
	}
	// update target health when we initialize or exactly reach the threshold
	if initializing || w.getThreshold(w.healthCheckCfg) == w.lastResultCount {
		w.setThresholdReached(ctx)
	}
}

// updateHealthCheckConfig updates the current health check config.
func (w *worker) updateHealthCheckConfig(ctx context.Context, newCfg *healthCheckConfig) {
	oldCfg := w.healthCheckCfg
	w.healthCheckCfg = newCfg
	if newCfg.equivalent(oldCfg) {
		return
	}
	if w.target.onConfigUpdate != nil {
		defer w.target.onConfigUpdate()
	}
	switch {
	case newCfg == nil:
		w.stopHealthCheckInterval(ctx)
		w.setTargetDisabled(ctx)
		return
	case oldCfg == nil:
		w.startHealthCheckInterval(ctx)
		w.setTargetInit(ctx)
		return
	}
	w.log.DebugContext(ctx, "Updated health check config",
		"health_check_config", w.healthCheckCfg.name,
		"interval", log.StringerAttr(w.healthCheckCfg.interval),
		"timeout", log.StringerAttr(w.healthCheckCfg.timeout),
		"healthy_threshold", w.healthCheckCfg.healthyThreshold,
		"unhealthy_threshold", w.healthCheckCfg.unhealthyThreshold,
	)
	if newCfg.interval != oldCfg.interval {
		// we don't want config updates that match multiple targets to align all
		// the interval timers too closely, so create a new interval with full
		// jitter rather than trying to account for elapsed time since last tick
		w.healthCheckInterval.Stop()
		w.healthCheckInterval = interval.New(interval.Config{
			Duration:      w.healthCheckCfg.interval,
			Jitter:        retryutils.SeventhJitter,
			FirstDuration: retryutils.FullJitter(w.healthCheckCfg.interval),
			Clock:         w.clock,
		})
	}
	oldThreshold := w.getThreshold(oldCfg)
	newThreshold := w.getThreshold(newCfg)
	if newThreshold < oldThreshold && w.lastResultCount >= newThreshold {
		w.setThresholdReached(ctx)
	}
}

// getThreshold returns the appropriate threshold to compare against the last
// result.
func (w *worker) getThreshold(cfg *healthCheckConfig) uint32 {
	if w.lastResultErr == nil {
		return cfg.healthyThreshold
	}
	return cfg.unhealthyThreshold
}

func (w *worker) setThresholdReached(ctx context.Context) {
	const transitionReason = types.TargetHealthTransitionReasonThreshold
	checkWord := pluralize(w.lastResultCount, "check")
	if w.lastResultErr == nil {
		msg := fmt.Sprintf("%d health %v passed", w.lastResultCount, checkWord)
		w.setTargetHealthy(ctx, transitionReason, msg)
	} else {
		msg := fmt.Sprintf("%d health %v failed", w.lastResultCount, checkWord)
		w.setTargetUnhealthy(ctx, transitionReason, msg)
	}
}

func pluralize(count uint32, word string) string {
	if count != 1 {
		return word + "s"
	}
	return word
}

func (w *worker) setTargetInit(ctx context.Context) {
	const reason = types.TargetHealthTransitionReasonInit
	const message = "Health checker initialized"
	w.setTargetHealthStatus(ctx, types.TargetHealthStatusUnknown, reason, message)
}

func (w *worker) setTargetHealthy(ctx context.Context, reason types.TargetHealthTransitionReason, message string) {
	w.setTargetHealthStatus(ctx, types.TargetHealthStatusHealthy, reason, message)
}

func (w *worker) setTargetUnhealthy(ctx context.Context, reason types.TargetHealthTransitionReason, message string) {
	w.setTargetHealthStatus(ctx, types.TargetHealthStatusUnhealthy, reason, message)
}

func (w *worker) setTargetDisabled(ctx context.Context) {
	const reason = types.TargetHealthTransitionReasonDisabled
	const message = "No health check config matches this resource"
	w.setTargetHealthStatus(ctx, types.TargetHealthStatusUnknown, reason, message)
}

func (w *worker) setTargetHealthStatus(ctx context.Context, newStatus types.TargetHealthStatus, reason types.TargetHealthTransitionReason, message string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if reason != types.TargetHealthTransitionReasonInit {
		w.notifyInitStatusAvailableLocked()
	} else if w.initCheckPendingCh == nil {
		w.initCheckPendingCh = make(chan struct{})
	}
	oldHealth := w.targetHealth
	if oldHealth.Status == string(newStatus) && oldHealth.TransitionReason == string(reason) {
		return
	}
	switch newStatus {
	case types.TargetHealthStatusHealthy:
		w.log.InfoContext(ctx, "Target became healthy",
			"reason", reason,
			"message", message,
		)
		resourceHealthyGauge.WithLabelValues(w.metricType).Inc()
	case types.TargetHealthStatusUnhealthy:
		w.log.WarnContext(ctx, "Target became unhealthy",
			"reason", reason,
			"message", message,
		)
		resourceUnhealthyGauge.WithLabelValues(w.metricType).Inc()
	case types.TargetHealthStatusUnknown:
		w.log.DebugContext(ctx, "Target health status is unknown",
			"reason", reason,
			"message", message,
		)
		resourceUnknownGauge.WithLabelValues(w.metricType).Inc()
	}
	w.decrementPreviousMetric(oldHealth.Status)
	now := w.clock.Now()
	w.targetHealth = types.TargetHealth{
		Address:             strings.Join(w.lastResolvedEndpoints, ","),
		Protocol:            string(w.target.GetProtocol()),
		Status:              string(newStatus),
		TransitionTimestamp: &now,
		TransitionReason:    string(reason),
		Message:             message,
	}
	if w.lastResultErr != nil {
		w.targetHealth.TransitionError = w.lastResultErr.Error()
	}
}

// notifyInitStatusAvailableLocked closes the pending init status channel, if
// one exists, to notify any waiters that the init health check status is
// available. It is assumed that the caller of this func is holding the lock.
func (w *worker) notifyInitStatusAvailableLocked() {
	if w.initCheckPendingCh != nil {
		close(w.initCheckPendingCh)
		w.initCheckPendingCh = nil
	}
}

// waitForInitCheckLocked waits for the pending init status channel to be nil
// or for a timeout to expire. It is assumed that the caller of this func is
// holding the read lock.
func (w *worker) waitForInitCheckLocked(timeout time.Duration) {
	if w.initCheckPendingCh == nil {
		return
	}
	timeoutCh := time.After(retryutils.HalfJitter(timeout))
	for w.initCheckPendingCh != nil {
		ch := w.initCheckPendingCh
		w.mu.RUnlock()
		select {
		case <-ch:
			w.mu.RLock()
		case <-timeoutCh:
			w.mu.RLock()
			return
		}
	}
}

// decrementPreviousMetric decrements the previous health metric.
func (w *worker) decrementPreviousMetric(previousHealthStatus string) {
	// Decrement previous state when not the initial state.
	// Avoids decrementing the "unknown" gauge below zero.
	if previousHealthStatus != "" {
		switch types.TargetHealthStatus(previousHealthStatus) {
		case types.TargetHealthStatusHealthy:
			resourceHealthyGauge.WithLabelValues(w.metricType).Dec()
		case types.TargetHealthStatusUnhealthy:
			resourceUnhealthyGauge.WithLabelValues(w.metricType).Dec()
		case types.TargetHealthStatusUnknown:
			resourceUnknownGauge.WithLabelValues(w.metricType).Dec()
		}
	}
}

func init() {
	metrics.RegisterPrometheusCollectors(
		resourceHealthyGauge,
		resourceUnhealthyGauge,
		resourceUnknownGauge,
	)
}

var (
	// teleport_resources_health_status_healthy
	resourceHealthyGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: teleport.MetricResourcesHealthStatus,
			Name:      teleport.MetricHealthy,
			Help:      "Number of healthy resources",
		},
		[]string{teleport.TagType}, // db|k8s|etc
	)
	// teleport_resources_health_status_unhealthy
	resourceUnhealthyGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: teleport.MetricResourcesHealthStatus,
			Name:      teleport.MetricUnhealthy,
			Help:      "Number of unhealthy resources",
		},
		[]string{teleport.TagType}, // db|k8s|etc
	)
	// teleport_resources_health_status_unknown
	resourceUnknownGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: teleport.MetricResourcesHealthStatus,
			Name:      teleport.MetricUnknown,
			Help:      "Number of resources in an unknown health state",
		},
		[]string{teleport.TagType}, // db|k8s|etc
	)
)
