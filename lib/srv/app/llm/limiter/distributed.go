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

package limiter

import (
	"cmp"
	"context"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	llmv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/llm/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	llmquota "github.com/gravitational/teleport/lib/llm/quota"
	llmerrors "github.com/gravitational/teleport/lib/srv/app/llm/errors"
	"github.com/gravitational/teleport/lib/utils"
)

// Distributed provides a distributed (shared state) token limiter.
type Distributed struct {
	log       *slog.Logger
	closeCh   chan struct{}
	closeOnce sync.Once
	interval  time.Duration

	endpoints        utils.SyncMap[string, *endpointState]
	reporter         UsageReporter
	reportInProgress atomic.Bool
}

// UsageReporter defines interface used to report LLM usage.
type UsageReporter interface {
	// ReportUsage reports an amount of LLM tokens consumed.
	ReportUsage(ctx context.Context, usages []*llmv1.EndpointUsage) (*llmv1.ReportUsageResponse, error)
}

// DistributedConfig is the configuration used for [Distributed] limiter.
type DistributedConfig struct {
	// Reporter is used to report the local usage and retrieve the
	// cluster-wide usage.
	Reporter UsageReporter
	// Logger is the logger used by the limiter.
	Logger *slog.Logger
	// ReportInterval is the interval the usage is reported.
	ReportInterval time.Duration
}

// CheckAndSetDefaults verifies the configuration and sets default values.
func (c *DistributedConfig) CheckAndSetDefaults() error {
	switch {
	case c.Reporter == nil:
		return trace.BadParameter("must provide Reporter")
	case c.Logger == nil:
		c.Logger = slog.Default()
	}

	c.Logger = c.Logger.With(teleport.ComponentKey, teleport.ComponentLLMLimiter)
	// Right now we're using a interval value that is half of session chunk
	// (5 minutes) to avoid lagging too far behind in reporting usage.
	c.ReportInterval = cmp.Or(c.ReportInterval, 2*time.Minute)
	return nil
}

// NewDistributed creates a new [Distributed] limiter.
func NewDistributed(cfg DistributedConfig) (*Distributed, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Distributed{
		log:      cfg.Logger,
		closeCh:  make(chan struct{}),
		reporter: cfg.Reporter,
		interval: cfg.ReportInterval,
	}, nil
}

// Reserve reserves LLM usage. Returns a settlement function which is used by
// callers (who reserved the token consumption) to adjust the values consumed.
func (d *Distributed) Reserve(ctx context.Context, req ReserveRequest) (ReserveInfo, SettleFunc, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return ReserveInfo{}, EmptySettleFunc, trace.Wrap(err)
	}

	if req.App.GetLLM().Quota == nil || len(req.App.GetLLM().Quota.Intervals) == 0 {
		return ReserveInfo{NotApplicable: true}, EmptySettleFunc, nil
	}

	t := time.Now()
	// LoadOrStore evaluates its value eagerly, so the state is only built when
	// the endpoint is seen for the first time.
	endpoint, ok := d.endpoints.Load(req.App.GetName())
	if !ok {
		endpoint, _ = d.endpoints.LoadOrStore(req.App.GetName(), newEndpointState(d.log, req.App.GetName()))
	}

	granted, err := endpoint.reserve(ctx, d.reporter, req, t, req.App.GetLLM().Quota.Intervals)
	if err != nil {
		return ReserveInfo{}, EmptySettleFunc, trace.Wrap(err)
	}

	var settleOnce sync.Once
	return ReserveInfo{OutputTokens: granted.OutputTokens}, func(ctx context.Context, u llmquota.Usage) {
		settleOnce.Do(func() {
			endpoint.settle(ctx, t, granted, u)
		})
	}, nil
}

// Close closes the limiter.
func (d *Distributed) Close(ctx context.Context) error {
	d.closeOnce.Do(func() {
		d.log.InfoContext(ctx, "closing limiter, sending final report")
		close(d.closeCh)
		d.report(ctx)
	})

	return nil
}

// Run executes the reporting loop. Note this function is blocking, so callers
// might place it on a goroutine.
func (d *Distributed) Run(ctx context.Context) error {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-d.closeCh:
			return nil
		case <-ticker.C:
			d.log.DebugContext(ctx, "sending periodic report")
			d.report(ctx)
		}
	}
}

// report reports usage back to auth. To avoid duplicate reports, it guarantees
// only one report can be inflight at time, making this safe to be called
// multiple times.
//
// TODO(gabrielcorado): consider blocking the report instead of an early return
// to reduce the possibility of missing quota to be reported.
func (d *Distributed) report(ctx context.Context) {
	// If there is a report in progress, we can just skip it.
	if d.reportInProgress.Swap(true) {
		d.log.DebugContext(ctx, "report already in progress, skipping it")
		return
	}
	defer d.reportInProgress.Store(false)

	// TODO(gabrielcorado): limit the amount of endpoints reported at once.
	var usage []*llmv1.EndpointUsage
	d.endpoints.Range(func(name string, endpoint *endpointState) bool {
		buckets := endpoint.beginReport(ctx)
		usage = append(usage, llmv1.EndpointUsage_builder{
			EndpointName: name,
			Buckets:      buckets,
		}.Build())

		return true
	})

	if len(usage) == 0 {
		return
	}

	// TODO(gabrielcorado): right now errors on reporting cause the full report
	// to be sent over again. We should do best effort on reducing/avoiding
	// duplicate usage reports.
	resp, err := d.reporter.ReportUsage(ctx, usage)
	if err != nil {
		d.log.WarnContext(ctx, "usage report failed", "error", err)
		return
	}

	for _, result := range resp.GetResults() {
		endpoint, ok := d.endpoints.Load(result.GetEndpointName())
		if !ok {
			d.log.WarnContext(ctx, "reported endpoint is not available locally", "endpoint", result.GetEndpointName())
			continue
		}

		endpoint.commitReport(ctx, result)
	}
}

type persistedWindow struct {
	window llmquota.IntervalWindow
	usage  llmquota.Usage
}

// endpointState is the LLM endpoint current state. It contains local slot, that
// holds to be reported consumption, and persisted slot, that contains
// consumption that comes from other app services through auth.
type endpointState struct {
	mu sync.Mutex

	log       *slog.Logger
	name      string
	persisted []persistedWindow
	pending   map[time.Time]llmquota.Usage
	inflight  map[time.Time]llmquota.Usage
	reserved  *llmquota.Usage

	ready       bool
	initialized bool
}

func newEndpointState(log *slog.Logger, endpointName string) *endpointState {
	return &endpointState{
		log:      log.With("endpoint", endpointName),
		name:     endpointName,
		pending:  make(map[time.Time]llmquota.Usage),
		inflight: make(map[time.Time]llmquota.Usage),
		reserved: &llmquota.Usage{},
	}
}

// init attempts to retrieve the endpoint current state by submiting an empty
// report. It can be called multiple times, but will only execute once.
//
// This report must only contain the endpoint, and should not include any usage.
// Here we can expect this process to be efficient and only return the requested
// information.
func (e *endpointState) init(ctx context.Context, reporter UsageReporter) {
	e.mu.Lock()
	initialized := e.initialized
	e.initialized = true
	e.mu.Unlock()

	if initialized {
		return
	}

	// We can expect this report call to be fast. Having it low also allows
	// retried requests to do another attempt.
	retryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  500 * time.Millisecond,
		Step:   500 * time.Millisecond,
		Max:    5 * time.Second,
		Jitter: retryutils.HalfJitter,
	})
	if err != nil {
		e.mu.Lock()
		defer e.mu.Unlock()
		e.log.ErrorContext(ctx, "failed to initialize retry (this is a bug)", "error", err)
		// Leave the endpoint "locked" because this error isn't retryable.
		return
	}

	err = retry.For(retryCtx, func() error {
		resp, err := reporter.ReportUsage(retryCtx, []*llmv1.EndpointUsage{
			llmv1.EndpointUsage_builder{EndpointName: e.name}.Build(),
		})
		if err != nil {
			return trace.Wrap(wrapPermanentError(err))
		}
		for _, result := range resp.GetResults() {
			if result.GetEndpointName() != e.name {
				continue
			}

			if err := convertResultError(result); err != nil {
				return trace.Wrap(wrapPermanentError(err))
			}

			e.commitReport(ctx, result)
			return nil
		}

		return trace.Wrap(wrapPermanentError(trace.NotFound("initial report response doesn't include endpoint %q", e.name)))
	})

	e.mu.Lock()
	switch {
	case trace.IsNotImplemented(err):
		// In case the report returns a not implemented error, we can assume
		// there isn't much to be done. For now, we can let the limiter behave
		// in "compatibility mode" where it will not take into account auth
		// state.
		//
		// We may revisit this in the future.
		e.log.WarnContext(ctx, "auth service does not support LLM usage reporting, quota will be enforced using this instance usage only", "error", err)
		e.ready = true
	case err == nil:
		e.log.DebugContext(ctx, "retrieved LLM endpoint quota state", "active_windows", len(e.persisted))
		e.ready = true
	default:
		// Initialization is retried by the next reserve call, which rejects
		// requests while the endpoint state is unknown.
		e.log.WarnContext(ctx, "failed to retrieve LLM endpoint quota state, LLM requests will be rejected until it succeeds", "error", err)
		e.initialized = false
	}
	e.mu.Unlock()
}

// consumptionLocked returns the current endpoint tokens consumption.
//
// The consumption is the sum of:
//
//	reserved  : in-flight requests on this instance, not yet settled.
//	persisted : cluster-wide usage as of the last successful report.
//	pending   : settled locally, not yet reported.
//	inflight  : sent, awaiting confirmation.
func (e *endpointState) consumptionLocked(ctx context.Context, interval *types.LLM_Quota_Interval, t time.Time) (llmquota.Usage, error) {
	w, err := llmquota.NewIntervalWindowFromTime(interval, t)
	if err != nil {
		// Without a window there is nothing to compare the usage against, so
		// the interval is skipped. This is only reachable on a malformed
		// interval, meaning its limits are not being enforced at all.
		e.log.WarnContext(ctx, "unable to determine LLM quota interval window, interval limits will not be enforced",
			"interval", interval.Interval,
			"error", err,
		)
		return llmquota.Usage{}, nil
	}

	consumption := &llmquota.Usage{
		InputTokens:  e.reserved.InputTokens,
		OutputTokens: e.reserved.OutputTokens,
	}

	for _, persisted := range e.persisted {
		// Windows of different intervals can contain the same time, so only
		// the window of this interval counts towards its limit.
		if !persisted.window.Equal(w) {
			continue
		}

		if err := consumption.Add(persisted.usage); err != nil {
			return llmquota.Usage{}, trace.Wrap(err)
		}
	}

	if err := addBucketsToUsage(w, consumption, e.pending); err != nil {
		return llmquota.Usage{}, trace.Wrap(err)
	}

	if err := addBucketsToUsage(w, consumption, e.inflight); err != nil {
		return llmquota.Usage{}, trace.Wrap(err)
	}

	return *consumption, nil
}

// remainingLocked returns the smallest token capacity left across all the
// quota intervals.
func (e *endpointState) remainingLocked(ctx context.Context, intervals []*types.LLM_Quota_Interval, t time.Time) (llmquota.Usage, error) {
	remaining := llmquota.Usage{InputTokens: math.MaxUint, OutputTokens: math.MaxUint}
	for _, interval := range intervals {
		consumption, err := e.consumptionLocked(ctx, interval, t)
		if err != nil {
			return llmquota.Usage{}, trace.Wrap(err)
		}

		// The subtractions below cannot underflow because the consumption is
		// checked against the limit right before them.
		if interval.InputTokens > 0 {
			if consumption.InputTokens > uint(interval.InputTokens) {
				return llmquota.Usage{}, llmerrors.ErrLimitExceeded
			}
			remaining.InputTokens = min(remaining.InputTokens, uint(interval.InputTokens)-consumption.InputTokens)
		}

		if interval.OutputTokens > 0 {
			if consumption.OutputTokens > uint(interval.OutputTokens) {
				return llmquota.Usage{}, llmerrors.ErrLimitExceeded
			}
			remaining.OutputTokens = min(remaining.OutputTokens, uint(interval.OutputTokens)-consumption.OutputTokens)
		}
	}

	return remaining, nil
}

// reserve is executed before an LLM request is sent and checks for consumption
// to see if the request can be forwarded to the provider.
func (e *endpointState) reserve(
	ctx context.Context,
	reporter UsageReporter,
	req ReserveRequest,
	t time.Time,
	intervals []*types.LLM_Quota_Interval,
) (llmquota.Usage, error) {
	e.init(ctx, reporter)

	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.ready {
		return llmquota.Usage{}, llmerrors.ErrNotReady
	}

	remaining, err := e.remainingLocked(ctx, intervals, t)
	if err != nil {
		return llmquota.Usage{}, trace.Wrap(err)
	}

	var granted llmquota.Usage
	switch {
	case req.Usage != nil:
		// Compared against what is left instead of summing the request with
		// the consumption, which can overflow and bypass the limits.
		if req.Usage.InputTokens > remaining.InputTokens || req.Usage.OutputTokens > remaining.OutputTokens {
			return llmquota.Usage{}, llmerrors.ErrLimitExceeded
		}
		granted = *req.Usage
	case req.MaxUsage != nil:
		// Checked before applying the cap so a request asking for no tokens
		// is not rejected by an exhausted quota.
		if remaining.OutputTokens == 0 {
			return llmquota.Usage{}, llmerrors.ErrLimitExceeded
		}
		granted.OutputTokens = min(remaining.OutputTokens, req.MaxUsage.OutputTokens)
	}

	if err := e.reserved.Add(granted); err != nil {
		return llmquota.Usage{}, trace.Wrap(err)
	}

	return granted, nil
}

// settle settles tokens consumption using information from the provider.
func (e *endpointState) settle(ctx context.Context, t time.Time, reserved llmquota.Usage, actual llmquota.Usage) {
	e.mu.Lock()
	defer e.mu.Unlock()

	minute := t.Truncate(time.Minute)
	usage := e.pending[minute]
	if err := usage.Add(actual); err != nil {
		// Add leaves the usage unchanged on overflow, so the consumption of
		// this request is never reported and the quota under-counts.
		e.log.ErrorContext(ctx, "failed to record LLM usage, consumption will not be accounted for",
			"bucket", minute,
			"bucket_input_tokens", usage.InputTokens,
			"bucket_output_tokens", usage.OutputTokens,
			"input_tokens", actual.InputTokens,
			"output_tokens", actual.OutputTokens,
			"error", err,
		)
	}

	e.pending[minute] = usage
	if err := e.reserved.Sub(reserved); err != nil {
		// The reservation stays held until the process restarts, so the
		// endpoint will over-count consumption and reject requests earlier
		// than it should.
		e.log.ErrorContext(ctx, "failed to release LLM usage reservation, endpoint will over-count consumption",
			"reserved_input_tokens", e.reserved.InputTokens,
			"reserved_output_tokens", e.reserved.OutputTokens,
			"release_input_tokens", reserved.InputTokens,
			"release_output_tokens", reserved.OutputTokens,
			"error", err,
		)
	}
}

// beginReport parepares usage reports to be sent to auth.
//
// TODO(gabrielcorado): prune bucket that can no longer contribute to quotas.
// This will help cases where working on "compatibility mode" where auth doesn't
// support reporting quotas.
func (e *endpointState) beginReport(ctx context.Context) []*llmv1.UsageBucket {
	e.mu.Lock()
	defer e.mu.Unlock()

	var buckets []*llmv1.UsageBucket
	for t, pending := range e.pending {
		inflight := e.inflight[t]
		if err := inflight.Add(pending); err != nil {
			// Keep counting at the max instead of dropping the usage.
			e.log.ErrorContext(ctx, "llm usage bucket overflowed, reporting the maximum usage instead",
				"bucket", t,
				"error", err,
			)
			inflight = llmquota.Usage{InputTokens: math.MaxUint, OutputTokens: math.MaxUint}
		}

		e.inflight[t] = inflight
	}

	e.pending = make(map[time.Time]llmquota.Usage)
	for t, usage := range e.inflight {
		buckets = append(buckets, llmv1.UsageBucket_builder{
			Time:         timestamppb.New(t),
			InputTokens:  uint64(usage.InputTokens),
			OutputTokens: uint64(usage.OutputTokens),
		}.Build())
	}

	return buckets
}

// commitReport updates internal state from report response. It can include
// endpoint token usage from different instances.
func (e *endpointState) commitReport(ctx context.Context, result *llmv1.EndpointResult) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if result.HasError() {
		switch result.GetError().GetCode() {
		case llmv1.ReportErrorCode_REPORT_ERROR_CODE_PERMISSION_DENIED,
			llmv1.ReportErrorCode_REPORT_ERROR_CODE_NOT_FOUND,
			llmv1.ReportErrorCode_REPORT_ERROR_CODE_INVALID_ARGUMENT:
			// The usage must not be reported again. Dropping it means the
			// consumption is permanently lost from the cluster-wide quota, so
			// it is worth surfacing how much was discarded.
			var discarded llmquota.Usage
			for _, usage := range e.inflight {
				_ = discarded.Add(usage)
			}
			e.log.WarnContext(ctx, "discarding unreported LLM usage after a permanent report error",
				"code", result.GetError().GetCode(),
				"message", result.GetError().GetMessage(),
				"buckets", len(e.inflight),
				"input_tokens", discarded.InputTokens,
				"output_tokens", discarded.OutputTokens,
			)

			e.inflight = make(map[time.Time]llmquota.Usage)
		default:
			e.log.WarnContext(ctx, "llm usage report failed, usage will be reported again",
				"code", result.GetError().GetCode(),
				"message", result.GetError().GetMessage(),
				"buckets", len(e.inflight),
			)
		}

		// Error results carry no quota state, so the persisted windows are
		// kept. Other codes are transient, keeping the in-flight usage to be
		// reported again.
		return
	}

	var persisted []persistedWindow
	for _, w := range result.GetQuota().GetStatus().GetActiveWindows() {
		// Lifetime windows are unbounded and reported without bounds, so unset
		// timestamps must not be read as the Unix epoch, otherwise the window
		// never matches [llmquota.LifetimeIntervalWindow].
		window := llmquota.LifetimeIntervalWindow
		if w.HasStartsAt() || w.HasEndsAt() {
			window = llmquota.NewIntervalWindow(w.GetStartsAt().AsTime(), w.GetEndsAt().AsTime())
		}

		persisted = append(persisted, persistedWindow{
			window: window,
			usage: llmquota.Usage{
				InputTokens:  uint(w.GetInputTokens()),
				OutputTokens: uint(w.GetOutputTokens()),
			},
		})
	}

	e.persisted = persisted
	e.inflight = make(map[time.Time]llmquota.Usage)
}

func addBucketsToUsage(w llmquota.IntervalWindow, usage *llmquota.Usage, buckets map[time.Time]llmquota.Usage) error {
	for t, bucketUsage := range buckets {
		if !w.Contains(t) {
			continue
		}

		if err := usage.Add(bucketUsage); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func convertResultError(result *llmv1.EndpointResult) error {
	if !result.HasError() {
		return nil
	}

	switch result.GetError().GetCode() {
	case llmv1.ReportErrorCode_REPORT_ERROR_CODE_PERMISSION_DENIED:
		return trace.AccessDenied("report error: %s", result.GetError().GetMessage())
	case llmv1.ReportErrorCode_REPORT_ERROR_CODE_NOT_FOUND:
		return trace.NotFound("report error: %s", result.GetError().GetMessage())
	case llmv1.ReportErrorCode_REPORT_ERROR_CODE_INVALID_ARGUMENT:
		return trace.BadParameter("report error: %s", result.GetError().GetMessage())
	case llmv1.ReportErrorCode_REPORT_ERROR_CODE_UNAVAILABLE:
		return trace.ConnectionProblem(nil, "report error: %s", result.GetError().GetMessage())
	default:
		return trace.Errorf("report error: %s", result.GetError().GetMessage())
	}
}

func wrapPermanentError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case trace.IsConnectionProblem(err):
		return trace.Wrap(err)
	}

	switch status.Code(err) {
	case codes.DeadlineExceeded:
		return trace.Wrap(err)
	}

	return trace.Wrap(retryutils.PermanentRetryError(err))
}
