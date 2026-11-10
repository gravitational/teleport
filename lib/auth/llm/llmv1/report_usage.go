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

package llmv1

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	llmv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/llm/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	llmquota "github.com/gravitational/teleport/lib/llm/quota"
	"github.com/gravitational/teleport/lib/services"
)

// ReportUsage implements [llmv1.UsageServiceServer].
func (u *UsageService) ReportUsage(ctx context.Context, req *llmv1.ReportUsageRequest) (*llmv1.ReportUsageResponse, error) {
	hostID, isProxy, err := u.authorizeReport(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var (
		resp          []*llmv1.EndpointResult
		reportedItems []reportItem
	)
	for _, endpointUsage := range req.GetEndpoints() {
		if err := validateEndpointReport(endpointUsage); err != nil {
			// Error could be that it is missing the endpoint name. For those
			// cases there isn't much we can do to improve the returned error,
			// so it will return and empty endpoint name.
			resp = append(resp, buildEndpointResultWithError(endpointUsage.GetEndpointName(), err))
			continue
		}

		intervals, err := getAppIntervals(ctx, u.cache, endpointUsage.GetEndpointName(), isProxy, hostID)
		if err != nil {
			resp = append(resp, buildEndpointResultWithError(endpointUsage.GetEndpointName(), err))
			continue
		}

		// Empty reports only work as a "request current consumption".
		if len(endpointUsage.GetBuckets()) == 0 {
			endpoint, err := u.cache.GetLLMEndpointQuota(ctx, endpointUsage.GetEndpointName())
			switch {
			case trace.IsNotFound(err):
				endpoint = services.NewLLMEndpointQuota(endpointUsage.GetEndpointName(), nil)
			case err != nil:
				resp = append(resp, buildEndpointResultWithError(endpointUsage.GetEndpointName(), err))
				continue
			}

			resp = append(resp, llmv1.EndpointResult_builder{
				EndpointName: endpointUsage.GetEndpointName(),
				Quota:        endpoint,
			}.Build())
			continue
		}

		reportedItems = append(reportedItems, reportItem{
			endpointName: endpointUsage.GetEndpointName(),
			usage:        endpointUsage,
			intervals:    intervals,
		})
	}

	select {
	case <-ctx.Done():
		return nil, trace.Wrap(ctx.Err())
	case processed := <-u.processReport(reportedItems, time.Now()):
		return llmv1.ReportUsageResponse_builder{
			Results: append(resp, processed...),
		}.Build(), nil
	}
}

func (u *UsageService) processReport(items []reportItem, now time.Time) <-chan []*llmv1.EndpointResult {
	// Make the channel buffered, so the goroutine survives if the client gives up.
	resultsCh := make(chan []*llmv1.EndpointResult, 1)
	go func() {
		results := make([]*llmv1.EndpointResult, 0, len(items))
		for _, item := range items {
			results = append(results, u.applyUsage(item, now))
		}
		resultsCh <- results
	}()
	return resultsCh
}

func (u *UsageService) applyUsage(item reportItem, now time.Time) *llmv1.EndpointResult {
	log := u.logger.With("endpoint", item.endpointName)
	buildErrorAndResponse := func(err error) (*llmv1.EndpointResult, error) {
		return buildEndpointResultWithError(item.endpointName, err), retryutils.PermanentRetryError(trace.Wrap(err))
	}

	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  500 * time.Millisecond,
		Step:   500 * time.Millisecond,
		Max:    5 * time.Second,
		Jitter: retryutils.HalfJitter,
	})
	if err != nil {
		resp, err := buildErrorAndResponse(err)
		log.ErrorContext(u.ctx, "failed to initialize retry (this is a bug)", "error", err)
		return resp
	}

	// Establishes for how long the retry will run.
	retryCtx, cancel := context.WithTimeout(u.ctx, 30*time.Second)
	defer cancel()

	var resp *llmv1.EndpointResult
	retry.For(retryCtx, func() error {
		var status *llmv1.LLMEndpointQuotaStatus
		quota, err := u.cache.GetLLMEndpointQuota(retryCtx, item.endpointName)
		switch {
		case trace.IsNotFound(err):
		case err != nil:
			log.ErrorContext(retryCtx, "failed to retrieve llm endpoint quota from cache", "error", err)
			resp, err = buildErrorAndResponse(err)
			return err
		default:
			status = quota.GetStatus()
		}

		currentWindows, err := activeWindows(now, status, item.intervals)
		if err != nil {
			log.ErrorContext(retryCtx, "failed to initialize a window saved on backend (this is a bug)", "error", err)
			resp, err = buildErrorAndResponse(err)
			return err
		}
		windows := assignConsumptionToWindows(currentWindows, item.usage.GetBuckets())

		updated, err := updateQuota(
			retryCtx,
			u.quotaWriter,
			cmp.Or(quota, services.NewLLMEndpointQuota(item.endpointName, nil)),
			windows,
		)
		switch {
		case errors.Is(err, backend.ErrConditionFailed):
			// The quota was concurrently updated, so the revision read from the
			// cache is stale. Retry to pick up a new one.
			return trace.Wrap(err)
		case err != nil:
			log.ErrorContext(retryCtx, "failed to update llm endpoint", "error", err)
			resp, err = buildErrorAndResponse(err)
			return err
		}

		resp = llmv1.EndpointResult_builder{
			EndpointName: item.endpointName,
			Quota:        updated,
		}.Build()
		return nil
	})

	// The retry loop gave up. Surface a client side retryable error
	// error so the reported usage isn't silently dropped.
	if resp == nil {
		log.ErrorContext(retryCtx, "giving up on applying llm usage report")
		return buildEndpointResultWithError(
			item.endpointName,
			trace.ConnectionProblem(nil, "unable to update current quota consumption"),
		)
	}

	return resp
}

func validateEndpointReport(report *llmv1.EndpointUsage) error {
	if report.GetEndpointName() == "" {
		return trace.BadParameter("endpoint usage report must include endpoint name")
	}

	for _, bucket := range report.GetBuckets() {
		if bucket.GetTime() == nil {
			return trace.BadParameter("usage buckets must define time")
		}
		if err := bucket.GetTime().CheckValid(); err != nil {
			return trace.BadParameter("invalid usage bucket time: %v", err)
		}
	}

	return nil
}

type reportItem struct {
	endpointName string
	usage        *llmv1.EndpointUsage
	intervals    []*types.LLM_Quota_Interval
}

func updateQuota(ctx context.Context, writer QuotaWriter, quota *llmv1.LLMEndpointQuota, activeWindows []activeWindow) (*llmv1.LLMEndpointQuota, error) {
	var windows []*llmv1.LLMEndpointQuotaStatus_Window
	for _, window := range activeWindows {
		windows = append(windows, window.ToProto())
	}

	quota.SetStatus(llmv1.LLMEndpointQuotaStatus_builder{ActiveWindows: windows}.Build())
	updated, err := writer.CreateOrUpdateLLMEndpointQuota(ctx, quota)
	return updated, trace.Wrap(err)
}

type activeWindow struct {
	intervalName string
	usage        *llmquota.Usage
	window       llmquota.IntervalWindow
}

func (a activeWindow) ToProto() *llmv1.LLMEndpointQuotaStatus_Window {
	p := llmv1.LLMEndpointQuotaStatus_Window_builder{
		IntervalName: a.intervalName,
		InputTokens:  uint64(a.usage.InputTokens),
		OutputTokens: uint64(a.usage.OutputTokens),
	}.Build()
	if a.intervalName != types.AppLLMQuotaIntervalLifetime {
		p.SetStartsAt(timestamppb.New(a.window.StartsAt()))
		p.SetEndsAt(timestamppb.New(a.window.EndsAt()))
	}

	return p
}

func activeWindows(now time.Time, status *llmv1.LLMEndpointQuotaStatus, intervals []*types.LLM_Quota_Interval) ([]activeWindow, error) {
	var activeWindows []activeWindow
intervalLoop:
	for _, interval := range intervals {
		intervalWindow, err := llmquota.NewIntervalWindowFromTime(interval, now)
		if err != nil {
			// Consider this a hard failure to avoid overwriting the entire set
			// of active windows.
			//
			// Note that this scenario is unlikely to happen it is an indication
			// of a bug on the [NewIntervalWindowFromTime] function.
			return nil, trace.Wrap(err)
		}

		for _, window := range status.GetActiveWindows() {
			if window.GetIntervalName() != interval.Interval {
				continue
			}

			var statusWindow llmquota.IntervalWindow
			switch interval.Interval {
			case types.AppLLMQuotaIntervalLifetime:
				statusWindow = llmquota.LifetimeIntervalWindow
			default:
				statusWindow = llmquota.NewIntervalWindow(window.GetStartsAt().AsTime(), window.GetEndsAt().AsTime())
			}

			if statusWindow.Equal(intervalWindow) {
				activeWindows = append(activeWindows, activeWindow{
					intervalName: interval.Interval,
					window:       statusWindow,
					usage: &llmquota.Usage{
						InputTokens:  uint(window.GetInputTokens()),
						OutputTokens: uint(window.GetOutputTokens()),
					},
				})
				continue intervalLoop
			}

			break
		}

		// No active windows found, so we'll initialize an empty one.
		activeWindows = append(activeWindows, activeWindow{
			intervalName: interval.Interval,
			window:       intervalWindow,
			usage:        &llmquota.Usage{},
		})
	}

	return activeWindows, nil
}

func assignConsumptionToWindows(activeWindows []activeWindow, buckets []*llmv1.UsageBucket) []activeWindow {
	for _, bucket := range buckets {
		bucketTime := bucket.GetTime().AsTime()
		for _, window := range activeWindows {
			if window.window.Contains(bucketTime) {
				window.usage.Add(llmquota.Usage{
					InputTokens:  uint(bucket.GetInputTokens()),
					OutputTokens: uint(bucket.GetOutputTokens()),
				})
			}
		}
	}

	return activeWindows
}

// getAppIntervals retrieves the app LLM endpoint quota interval from a
// registered AppServer.
//
// This is not a strong authorization check: any app service that advertises the
// target app can satisfy this lookup, even if it is not actually serving the
// specific session referenced by the app session certificate. Also when serving
// requests from proxy (for integration cases) this won't ensure the proxy is
// the one serving the request.
func getAppIntervals(ctx context.Context, cache Cache, name string, isProxy bool, hostID string) ([]*types.LLM_Quota_Interval, error) {
	var app types.Application
	for appServer, err := range clientutils.Resources(ctx, func(ctx context.Context, pageSize int, pageToken string) ([]types.AppServer, string, error) {
		resp, err := cache.ListResources(ctx, proto.ListResourcesRequest{
			Namespace:           apidefaults.Namespace,
			ResourceType:        types.KindAppServer,
			PredicateExpression: fmt.Sprintf("name == %q", name),
			Limit:               int32(pageSize),
			StartKey:            pageToken,
		})
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		servers, err := types.ResourcesWithLabels(resp.Resources).AsAppServers()
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		return servers, resp.NextKey, nil
	}) {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		isIntegrationApp := isProxy && appServer.GetApp().GetIntegration() != "" && appServer.GetApp().SupportsIntegration()
		if isIntegrationApp || appServer.GetHostID() == hostID {
			app = appServer.GetApp()
			break
		}
	}
	if app == nil || app.GetLLM() == nil || app.GetLLM().Quota == nil {
		return nil, trace.NotFound("unable to find LLM endpoint %q", name)
	}

	return app.GetLLM().Quota.Intervals, nil
}

func buildEndpointResultWithError(endpointName string, err error) *llmv1.EndpointResult {
	var code llmv1.ReportErrorCode
	switch {
	case trace.IsNotFound(err), trace.IsAccessDenied(err):
		code = llmv1.ReportErrorCode_REPORT_ERROR_CODE_NOT_FOUND
	case trace.IsBadParameter(err):
		code = llmv1.ReportErrorCode_REPORT_ERROR_CODE_INVALID_ARGUMENT
	case trace.IsConnectionProblem(err):
		code = llmv1.ReportErrorCode_REPORT_ERROR_CODE_UNAVAILABLE
	default:
		code = llmv1.ReportErrorCode_REPORT_ERROR_CODE_UNSPECIFIED
	}

	return llmv1.EndpointResult_builder{
		EndpointName: endpointName,
		Error: llmv1.ReportError_builder{
			Code:    code,
			Message: err.Error(),
		}.Build(),
	}.Build()
}
