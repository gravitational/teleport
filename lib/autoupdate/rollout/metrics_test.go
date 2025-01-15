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

package rollout

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	autoupdatepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
)

func newMetricsForTest(t *testing.T) *metrics {
	reg := prometheus.NewRegistry()
	m, err := newMetrics(reg)
	require.NoError(t, err)
	return m
}

func Test_setVersionMetric(t *testing.T) {
	now := clockwork.NewFakeClock().Now()
	aMinuteAgo := now.Add(-time.Minute)
	aWeekAgo := now.Add(-time.Hour * 24 * 7)
	testVersion := "1.2.3-alpha.1"
	previousVersion := "1.2.1"
	testMetricLabels := []string{metricsVersionLabelName}
	tests := []struct {
		name             string
		previousVersions map[string]time.Time
		previousMetrics  map[string]float64
		expectedVersions map[string]time.Time
		expectedMetrics  map[string]float64
	}{
		{
			name:             "no versions",
			previousVersions: map[string]time.Time{},
			previousMetrics:  map[string]float64{},
			expectedVersions: map[string]time.Time{
				testVersion: now,
			},
			expectedMetrics: map[string]float64{
				testVersion: 1,
			},
		},
		{
			name: "same version, not expired",
			previousVersions: map[string]time.Time{
				testVersion: aMinuteAgo,
			},
			previousMetrics: map[string]float64{
				testVersion: 1,
			},
			expectedVersions: map[string]time.Time{
				testVersion: now,
			},
			expectedMetrics: map[string]float64{
				testVersion: 1,
			},
		},
		{
			name: "same version, expired",
			previousVersions: map[string]time.Time{
				testVersion: aWeekAgo,
			},
			previousMetrics: map[string]float64{
				testVersion: 1,
			},
			expectedVersions: map[string]time.Time{
				testVersion: now,
			},
			expectedMetrics: map[string]float64{
				testVersion: 1,
			},
		},
		{
			name: "old non-expired versions",
			previousVersions: map[string]time.Time{
				previousVersion: aMinuteAgo,
			},
			previousMetrics: map[string]float64{
				previousVersion: 1,
			},
			expectedVersions: map[string]time.Time{
				previousVersion: aMinuteAgo,
				testVersion:     now,
			},
			expectedMetrics: map[string]float64{
				previousVersion: 0,
				testVersion:     1,
			},
		},
		{
			name: "old expired versions",
			previousVersions: map[string]time.Time{
				previousVersion: aWeekAgo,
			},
			previousMetrics: map[string]float64{
				previousVersion: 1,
			},
			expectedVersions: map[string]time.Time{
				testVersion: now,
			},
			expectedMetrics: map[string]float64{
				testVersion: 1,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Test setup: create metrics and load previous metrics.
			m := metrics{
				previousVersions: test.previousVersions,
			}

			testGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{}, testMetricLabels)
			for k, v := range test.previousMetrics {
				testGauge.With(prometheus.Labels{testMetricLabels[0]: k}).Set(v)
			}

			// Test execution: set the version metric.
			m.setVersionMetric(testVersion, testGauge, now)

			// Test validation: collect the metrics and check that the state match what we expect.
			require.Equal(t, test.expectedVersions, m.previousVersions)
			metricsChan := make(chan prometheus.Metric, 100)
			testGauge.Collect(metricsChan)
			close(metricsChan)
			metricsResult := collectMetricsByLabel(t, metricsChan, testMetricLabels[0])
			require.Equal(t, test.expectedMetrics, metricsResult)
		})
	}
}

func Test_setGroupStates(t *testing.T) {
	testMetricLabels := []string{metricsGroupNumberLabelName}
	testGroups := []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
		{State: autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE},
		{State: autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE},
		{State: autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED},
	}
	tests := []struct {
		name               string
		previousGroupCount int
		previousMetrics    map[string]float64
		expectedGroupCount int
		expectedMetrics    map[string]float64
	}{
		{
			name:               "no groups",
			previousGroupCount: 0,
			previousMetrics:    map[string]float64{},
			expectedGroupCount: len(testGroups),
			expectedMetrics: map[string]float64{
				"0": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE),
				"1": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE),
				"2": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED),
			},
		},
		{
			name:               "same groups, same states",
			previousGroupCount: len(testGroups),
			previousMetrics: map[string]float64{
				"0": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE),
				"1": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE),
				"2": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED),
			},
			expectedGroupCount: len(testGroups),
			expectedMetrics: map[string]float64{
				"0": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE),
				"1": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE),
				"2": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED),
			},
		},
		{
			name:               "same groups, different states",
			previousGroupCount: len(testGroups),
			previousMetrics: map[string]float64{
				"0": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE),
				"1": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED),
				"2": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED),
			},
			expectedGroupCount: len(testGroups),
			expectedMetrics: map[string]float64{
				"0": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE),
				"1": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE),
				"2": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED),
			},
		},
		{
			name:               "less groups",
			previousGroupCount: 1,
			previousMetrics: map[string]float64{
				"0": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE),
			},
			expectedGroupCount: len(testGroups),
			expectedMetrics: map[string]float64{
				"0": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE),
				"1": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE),
				"2": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED),
			},
		},
		{
			name:               "more groups",
			previousGroupCount: 5,
			previousMetrics: map[string]float64{
				"0": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE),
				"1": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED),
				"2": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED),
				"3": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED),
				"4": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED),
			},
			expectedGroupCount: len(testGroups),
			expectedMetrics: map[string]float64{
				"0": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE),
				"1": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE),
				"2": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED),
				"3": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSPECIFIED),
				"4": float64(autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSPECIFIED),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			testGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{}, testMetricLabels)
			for k, v := range test.previousMetrics {
				testGauge.With(prometheus.Labels{testMetricLabels[0]: k}).Set(v)
			}

			// Test setup: create metrics and load previous metrics.
			m := metrics{
				groupCount:        test.previousGroupCount,
				rolloutGroupState: testGauge,
			}

			// Test execution: set the version metric.
			m.setGroupStates(testGroups)

			// Test validation: collect the metrics and check that the state match what we expect.
			require.Equal(t, test.expectedGroupCount, m.groupCount)
			metricsChan := make(chan prometheus.Metric, 100)
			m.rolloutGroupState.Collect(metricsChan)
			close(metricsChan)
			metricsResult := collectMetricsByLabel(t, metricsChan, testMetricLabels[0])
			require.Equal(t, test.expectedMetrics, metricsResult)

		})
	}
}

func collectMetricsByLabel(t *testing.T, ch <-chan prometheus.Metric, labelName string) map[string]float64 {
	t.Helper()
	result := make(map[string]float64)

	var protoMetric dto.Metric
	for {
		m, ok := <-ch
		if !ok {
			return result
		}
		require.NoError(t, m.Write(&protoMetric))
		ll := protoMetric.GetLabel()
		require.Len(t, ll, 1)
		require.Equal(t, labelName, ll[0].GetName())
		gg := protoMetric.GetGauge()
		require.NotNil(t, gg)
		result[ll[0].GetValue()] = gg.GetValue()
	}
}
