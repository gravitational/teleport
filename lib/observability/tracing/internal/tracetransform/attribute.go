// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package tracetransform provides conversion functionality for the otlptrace
// exporters.
//
// This code has been copied from https://github.com/open-telemetry/opentelemetry-go/blob/main/exporters/otlp/otlptrace/internal/tracetransform/attribute.go
package tracetransform

import (
	"go.opentelemetry.io/otel/attribute"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
)

// KeyValues transforms a slice of attribute KeyValues into OTLP key-values.
func KeyValues(attrs []attribute.KeyValue) []*commonpb.KeyValue {
	if len(attrs) == 0 {
		return nil
	}

	out := make([]*commonpb.KeyValue, 0, len(attrs))
	for _, kv := range attrs {
		out = append(out, KeyValue(kv))
	}
	return out
}

// KeyValue transforms an attribute KeyValue into an OTLP key-value.
func KeyValue(kv attribute.KeyValue) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: string(kv.Key), Value: Value(kv.Value)}
}

// Value transforms an attribute Value into an OTLP AnyValue.
func Value(v attribute.Value) *commonpb.AnyValue {
	av := new(commonpb.AnyValue)
	switch v.Type() {
	case attribute.BOOL:
		av.Value = &commonpb.AnyValue_BoolValue{
			BoolValue: v.AsBool(),
		}
	case attribute.BOOLSLICE:
		av.Value = &commonpb.AnyValue_ArrayValue{
			ArrayValue: &commonpb.ArrayValue{
				Values: boolSliceValues(v.AsBoolSlice()),
			},
		}
	case attribute.INT64:
		av.Value = &commonpb.AnyValue_IntValue{
			IntValue: v.AsInt64(),
		}
	case attribute.INT64SLICE:
		av.Value = &commonpb.AnyValue_ArrayValue{
			ArrayValue: &commonpb.ArrayValue{
				Values: int64SliceValues(v.AsInt64Slice()),
			},
		}
	case attribute.FLOAT64:
		av.Value = &commonpb.AnyValue_DoubleValue{
			DoubleValue: v.AsFloat64(),
		}
	case attribute.FLOAT64SLICE:
		av.Value = &commonpb.AnyValue_ArrayValue{
			ArrayValue: &commonpb.ArrayValue{
				Values: float64SliceValues(v.AsFloat64Slice()),
			},
		}
	case attribute.STRING:
		av.Value = &commonpb.AnyValue_StringValue{
			StringValue: v.AsString(),
		}
	case attribute.STRINGSLICE:
		av.Value = &commonpb.AnyValue_ArrayValue{
			ArrayValue: &commonpb.ArrayValue{
				Values: stringSliceValues(v.AsStringSlice()),
			},
		}
	default:
		av.Value = &commonpb.AnyValue_StringValue{
			StringValue: "INVALID",
		}
	}
	return av
}

func boolSliceValues(vals []bool) []*commonpb.AnyValue {
	converted := make([]*commonpb.AnyValue, len(vals))
	for i, v := range vals {
		converted[i] = &commonpb.AnyValue{
			Value: &commonpb.AnyValue_BoolValue{
				BoolValue: v,
			},
		}
	}
	return converted
}

func int64SliceValues(vals []int64) []*commonpb.AnyValue {
	converted := make([]*commonpb.AnyValue, len(vals))
	for i, v := range vals {
		converted[i] = &commonpb.AnyValue{
			Value: &commonpb.AnyValue_IntValue{
				IntValue: v,
			},
		}
	}
	return converted
}

func float64SliceValues(vals []float64) []*commonpb.AnyValue {
	converted := make([]*commonpb.AnyValue, len(vals))
	for i, v := range vals {
		converted[i] = &commonpb.AnyValue{
			Value: &commonpb.AnyValue_DoubleValue{
				DoubleValue: v,
			},
		}
	}
	return converted
}

func stringSliceValues(vals []string) []*commonpb.AnyValue {
	converted := make([]*commonpb.AnyValue, len(vals))
	for i, v := range vals {
		converted[i] = &commonpb.AnyValue{
			Value: &commonpb.AnyValue_StringValue{
				StringValue: v,
			},
		}
	}
	return converted
}
