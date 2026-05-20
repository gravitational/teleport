package usersv1

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/observability/metrics"
)

const (
	LabelUserKind = "user_kind"
	LabelGRPCCode = "grpc_code"
)

type UserKindLabelValue string

const (
	UserKindLocal   UserKindLabelValue = "local"
	UserKindSSO     UserKindLabelValue = "sso"
	UserKindBot     UserKindLabelValue = "bot"
	UserKindUnknown UserKindLabelValue = "unknown"
)

var ResetUserCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: teleport.MetricNamespace,
	Subsystem: "users",
	Name:      "reset_user",
	Help:      "Total number of user reset operations",
}, []string{LabelUserKind, LabelGRPCCode})

func init() {
	println("=========== init()")
	metrics.RegisterPrometheusCollectors(ResetUserCounter)
}
