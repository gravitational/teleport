package eventually

import (
	"context"
	"log/slog"
	"maps"
	"time"

	"github.com/antithesishq/antithesis-sdk-go/assert"
)

const defaultPollInterval = time.Millisecond * 500

// AddDetailFunc adds structured assertion details for the current condition
// check. Not safe for concurrent calls.
type AddDetailFunc = func(key string, value any)

// ConditionFunc returns whether the assert conditions have been met. The
// addDetail callback can be used to attach assertion details for the current
// condition check.
type ConditionFunc = func(ctx context.Context, addDetail AddDetailFunc) (condition bool, err error)

// AssertFunc matches the type of [assert.AlwaysOrUnreachable],
// [assert.Always], and [assert.Sometimes].
type AssertFunc = func(condition bool, message string, details map[string]any)

// AssertParams configures an eventual assertion.
type AssertParams struct {
	// Message is passed to the Antithesis assertion.
	Message string
	// Timeout is the maximum amount of time to wait for Condition to become true.
	Timeout time.Duration
	// Condition is polled until it returns true or Timeout elapses.
	Condition ConditionFunc
	// PollInterval controls how often Condition is called. If zero or negative,
	// a default interval (500ms) is used.
	PollInterval time.Duration
	// Details are copied into the final assertion details.
	Details map[string]any
	// Assertion is called once after polling completes. If nil,
	// [assert.AlwaysOrUnreachable] is used.
	Assertion AssertFunc
}

// Assert polls condition until it returns true or the timeout elapses,
// then fires an assertion with the result. By default [assert.AlwaysOrUnreachable]
// is called, this can be overwritten with [AssertParams.Assertion].
//
// The assertion fires exactly once at the end of the wait window
// which is the correct Antithesis pattern for eventual consistency.
//
// The result is logged via default [slog.Logger].
//
// Example:
//
//	eventually.Assert(ctx, eventually.AssertParams{
//		Timeout: 30 * time.Second,
//		Message: "role example-role visible on auth1 at given revision",
//		Details: map[string]any{
//			"node": "auth1",
//			"foo":  "bar",
//		},
//		Condition: func(ctx context.Context, addDetail eventually.AddDetailFunc) (bool, error) {
//			addDetail("expected_revision", expectedRevision)
//			got, err := getRole(ctx, "example-role")
//			if err != nil {
//				return false, err
//			}
//			addDetail("got_revision", got.GetRevision())
//			return got.GetRevision() == expectedRevision, nil
//		},
//	})
func Assert(ctx context.Context, params AssertParams) error {
	if params.Assertion == nil {
		params.Assertion = assert.AlwaysOrUnreachable
	}
	if params.PollInterval <= 0 {
		params.PollInterval = defaultPollInterval
	}

	logger := slog.Default()
	deadline := time.Now().Add(params.Timeout)
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	startTime := time.Now()
	var (
		condition bool
		err       error
	)

	// Note user option details may overwrite the details added by the condition;
	// it is the user's responsibility to avoid key clashes.
	details := map[string]any{}

	//  params.Details can be nil.
	maps.Copy(details, params.Details)

	ticker := time.NewTicker(params.PollInterval)
	defer ticker.Stop()

	for ctx.Err() == nil {
		condition, err = params.Condition(ctx, func(key string, value any) {
			details[key] = value
		})

		if condition {
			break
		}

		select {
		case <-ctx.Done():
		case <-ticker.C:
		}
	}

	if err != nil {
		details["error"] = err.Error()
	}

	level := slog.LevelDebug

	if !condition {
		level = slog.LevelWarn
	}

	// Log the result, this isn't super useful within Antithesis itself but it helps
	// a lot when running locally to verify/debug workloads.
	logger.LogAttrs(ctx, level, "eventual assertion condition",
		slog.Bool("condition", condition),
		slog.String("assertion", params.Message),
		groupFromMap("details", details),
		slog.Duration("timeout", params.Timeout),
		slog.Duration("elapsed", time.Since(startTime)),
		slog.Duration("poll_interval", params.PollInterval),
	)

	params.Assertion(condition, params.Message, details)
	return err
}

func groupFromMap(key string, m map[string]any) slog.Attr {
	attrs := make([]any, 0, len(m))
	for k, v := range m {
		attrs = append(attrs, slog.Any(k, v))
	}
	return slog.Group(key, attrs...)
}
