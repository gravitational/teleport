// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"time"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

const (
	prometheusGaugePassedName   = "test_passed_count"
	prometheusGaugeFailedName   = "test_failed_count"
	prometheusGaugeSkippedName  = "test_skipped_count"
	prometheusGaugeDurationName = "test_duration"
	prometheusCounterFailed     = "test_failed"
	prometheusBranchLabelName   = "branch"
	prometheusTypeLabelName     = "type"
)

func reportPrometheus(args args, testOutput *outputMap, duration time.Duration) error {
	if args.prometheusURL == "" {
		return nil
	}

	testPassedCount := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: prometheusGaugePassedName,
		Help: "The number of passed tests",
	})

	testFailedCount := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: prometheusGaugeFailedName,
		Help: "The number of failed tests",
	})

	testSkippedCount := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: prometheusGaugeSkippedName,
		Help: "The number of skipped tests",
	})

	testDuration := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: prometheusGaugeDurationName,
		Help: "Test duration",
	})

	testPassedCount.Set(float64(testOutput.actionCounts[actionPass]))
	testFailedCount.Set(float64(testOutput.actionCounts[actionFail]))
	testSkippedCount.Set(float64(testOutput.actionCounts[actionSkip]))
	testDuration.Set(duration.Seconds())

	pusher := push.New(args.prometheusURL, args.prometheusJobName).
		Grouping(prometheusTypeLabelName, args.prometheusTypeLabelValue).
		Grouping(prometheusBranchLabelName, args.prometheusBranchLabelValue)

	if args.prometheusUser != "" && args.prometheusPassword != "" {
		pusher.BasicAuth(args.prometheusUser, args.prometheusPassword)
	}

	err := pusher.
		Collector(testPassedCount).
		Collector(testFailedCount).
		Collector(testSkippedCount).
		Collector(testDuration).
		Push()

	if err != nil {
		return trace.Wrap(err)
	}

	if !args.prometheusReportIndFailures {
		return nil
	}

	for _, n := range testOutput.failedActionNames {
		testFailed := prometheus.NewCounter(prometheus.CounterOpts{
			Name: prometheusCounterFailed,
			Help: "The number of passed tests",
		})

		testFailed.Inc()

		err := pusher.
			Grouping("name", n).
			Collector(testFailed).
			Push()

		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}
