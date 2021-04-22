package utils

import (
	"github.com/prometheus/client_golang/prometheus"
)

// RegisterPrometheusCollectors is a wrapper around prometheus.Register that
// - ignores equal or re-registered collectors
// - return an error if a collector does not fulfill the consistency and
//   uniqueness criteria
func RegisterPrometheusCollectors(collectors ...prometheus.Collector) error {
	for _, c := range collectors {
		if err := prometheus.Register(c); err != nil {
			if _, ok := err.(prometheus.AlreadyRegisteredError); ok {
				continue
			}
			return err
		}
	}
	return nil
}
