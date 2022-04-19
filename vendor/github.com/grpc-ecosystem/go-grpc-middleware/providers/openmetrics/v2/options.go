// Copyright (c) The go-grpc-middleware Authors.
// Licensed under the Apache License 2.0.

package metrics

import (
	openmetrics "github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// FromError returns a grpc status if error code is a valid grpc status.
func FromError(err error) (s *status.Status, ok bool) {
	return status.FromError(err)

	// TODO: @yashrsharma44 - discuss if we require more error handling from the previous package
}

// A CounterOption lets you add options to Counter metrics using With* funcs.
type CounterOption func(*openmetrics.CounterOpts)

type counterOptions []CounterOption

func (co counterOptions) apply(o openmetrics.CounterOpts) openmetrics.CounterOpts {
	for _, f := range co {
		f(&o)
	}
	return o
}

// WithConstLabels allows you to add ConstLabels to Counter metrics.
func WithConstLabels(labels openmetrics.Labels) CounterOption {
	return func(o *openmetrics.CounterOpts) {
		o.ConstLabels = labels
	}
}

// A HistogramOption lets you add options to Histogram metrics using With*
// funcs.
type HistogramOption func(*openmetrics.HistogramOpts)

type histogramOptions []HistogramOption

func (ho histogramOptions) apply(o openmetrics.HistogramOpts) openmetrics.HistogramOpts {
	for _, f := range ho {
		f(&o)
	}
	return o
}

// WithHistogramBuckets allows you to specify custom bucket ranges for histograms if EnableHandlingTimeHistogram is on.
func WithHistogramBuckets(buckets []float64) HistogramOption {
	return func(o *openmetrics.HistogramOpts) { o.Buckets = buckets }
}

// WithHistogramConstLabels allows you to add custom ConstLabels to
// histograms metrics.
func WithHistogramConstLabels(labels openmetrics.Labels) HistogramOption {
	return func(o *openmetrics.HistogramOpts) {
		o.ConstLabels = labels
	}
}

func typeFromMethodInfo(mInfo *grpc.MethodInfo) grpcType {
	if !mInfo.IsClientStream && !mInfo.IsServerStream {
		return Unary
	}
	if mInfo.IsClientStream && !mInfo.IsServerStream {
		return ClientStream
	}
	if !mInfo.IsClientStream && mInfo.IsServerStream {
		return ServerStream
	}
	return BidiStream
}
