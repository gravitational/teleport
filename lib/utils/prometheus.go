/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"github.com/gravitational/trace"

	"github.com/prometheus/client_golang/prometheus"
)

// RegisterPrometheusCollectors is a wrapper around prometheus.Register that
// - ignores equal or re-registered collectors
// - returns an error if a collector does not fulfill the consistency and
//   uniqueness criteria
func RegisterPrometheusCollectors(collectors ...prometheus.Collector) error {
	for _, c := range collectors {
		if err := prometheus.Register(c); err != nil {
			if _, ok := err.(prometheus.AlreadyRegisteredError); ok {
				continue
			}
			return trace.Wrap(err)
		}
	}
	return nil
}
