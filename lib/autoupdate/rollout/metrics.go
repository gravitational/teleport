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
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/exp/constraints"

	"github.com/gravitational/teleport"
	autoupdatepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types/autoupdate"
)

const (
	metricsSubsystem            = "agent_autoupdates"
	metricVersionLabelRetention = 24 * time.Hour
)

type metrics struct {
	// lock protects previousVersions and groupCount.
	// This should only be acquired by setVersionMetric.
	lock sync.Mutex

	// previousVersions is a list of the version we exported metrics for.
	// We track those to zero every old version if metrics labels contain the version.
	previousVersions map[string]time.Time
	groupCount       int

	// controller metrics
	reconciliations           *prometheus.CounterVec
	reconciliationDuration    *prometheus.HistogramVec
	reconciliationTries       *prometheus.CounterVec
	reconciliationTryDuration *prometheus.HistogramVec

	// resource spec metrics
	versionPresent prometheus.Gauge
	versionStart   *prometheus.GaugeVec
	versionTarget  *prometheus.GaugeVec
	versionMode    prometheus.Gauge

	configPresent prometheus.Gauge
	configMode    prometheus.Gauge

	rolloutPresent  prometheus.Gauge
	rolloutStart    *prometheus.GaugeVec
	rolloutTarget   *prometheus.GaugeVec
	rolloutMode     prometheus.Gauge
	rolloutStrategy *prometheus.GaugeVec

	// rollout status metrics
	rolloutTimeOverride prometheus.Gauge
	rolloutState        prometheus.Gauge
	rolloutGroupState   *prometheus.GaugeVec
}

const (
	metricsReconciliationResultLabelName         = "result"
	metricsReconciliationResultLabelValueFail    = "fail"
	metricsReconciliationResultLabelValuePanic   = "panic"
	metricsReconciliationResultLabelValueRetry   = "retry"
	metricsReconciliationResultLabelValueSuccess = "success"

	metricsGroupNumberLabelName = "group_number"
	metricsVersionLabelName     = "version"

	metricsStrategyLabelName = "strategy"
)

func newMetrics(reg prometheus.Registerer) (*metrics, error) {
	m := metrics{
		previousVersions: make(map[string]time.Time),
		reconciliations: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricsSubsystem,
			Name:      "reconciliations_total",
			Help:      "Count the rollout reconciliations triggered by the controller, and their result (success, failure, panic). One reconciliation might imply several tries in case of conflict.",
		}, []string{metricsReconciliationResultLabelName}),
		reconciliationDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricsSubsystem,
			Name:      "reconciliation_duration_seconds",
			Help:      "Time spent reconciling the autoupdate_agent_rollout resource. One reconciliation might imply several tries in case of conflict.",
		}, []string{metricsReconciliationResultLabelName}),
		reconciliationTries: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricsSubsystem,
			Name:      "reconciliation_tries_total",
			Help:      "Count the rollout reconciliations tried by the controller, and their result (success, failure, conflict).",
		}, []string{metricsReconciliationResultLabelName}),
		reconciliationTryDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricsSubsystem,
			Name:      "reconciliation_try_duration_seconds",
			Help:      "Time spent trying to reconcile the autoupdate_agent_rollout resource.",
		}, []string{metricsReconciliationResultLabelName}),

		versionPresent: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricsSubsystem,
			Name:      "version_present",
			Help:      "Boolean describing if an autoupdate_version resource exists in Teleport and its 'spec.agents' field is not nil.",
		}),
		versionTarget: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricsSubsystem,
			Name:      "version_target",
			Help:      "Metric describing the agent target version from the autoupdate_version resource.",
		}, []string{metricsVersionLabelName}),
		versionStart: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricsSubsystem,
			Name:      "version_start",
			Help:      "Metric describing the agent start version from the autoupdate_version resource.",
		}, []string{"version"}),
		versionMode: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricsSubsystem,
			Name:      "version_mode",
			Help:      fmt.Sprintf("Metric describing the agent update mode from the autoupdate_version resource. %s", valuesHelpString(codeToAgentMode)),
		}),

		configPresent: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricsSubsystem,
			Name:      "config_present",
			Help:      "Boolean describing if an autoupdate_config resource exists in Teleport and its 'spec.agents' field is not nil.",
		}),
		configMode: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricsSubsystem,
			Name:      "config_mode",
			Help:      fmt.Sprintf("Metric describing the agent update mode from the autoupdate_agent_config resource. %s", valuesHelpString(codeToAgentMode)),
		}),

		rolloutPresent: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricsSubsystem,
			Name:      "rollout_present",
			Help:      "Boolean describing if an autoupdate_agent_rollout resource exists in Teleport.",
		}),
		rolloutTarget: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricsSubsystem,
			Name:      "rollout_target",
			Help:      "Metric describing the agent target version from the autoupdate_gent_rollout resource.",
		}, []string{metricsVersionLabelName}),
		rolloutStart: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricsSubsystem,
			Name:      "rollout_start",
			Help:      "Metric describing the agent start version from the autoupdate_agent_rollout resource.",
		}, []string{metricsVersionLabelName}),
		rolloutMode: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricsSubsystem,
			Name:      "rollout_mode",
			Help:      fmt.Sprintf("Metric describing the agent update mode from the autoupdate_agent_rollout resource. %s", valuesHelpString(codeToAgentMode)),
		}),
		rolloutStrategy: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricsSubsystem,
			Name:      "rollout_strategy",
			Help:      "Metric describing the strategy of the autoupdate_agent_rollout resource.",
		}, []string{metricsStrategyLabelName}),
		rolloutTimeOverride: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricsSubsystem,
			Name:      "rollout_time_override_timestamp_seconds",
			Help:      "Describes the autoupdate_agent_rollout time override if set in (seconds since epoch). Zero means no time override.",
		}),
		rolloutState: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricsSubsystem,
			Name:      "rollout_state",
			Help:      fmt.Sprintf("Describes the autoupdate_agent_rollout state. %s", valuesHelpString(autoupdatepb.AutoUpdateAgentRolloutState_name)),
		}),
		rolloutGroupState: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricsSubsystem,
			Name:      "rollout_group_state",
			Help:      fmt.Sprintf("Describes the autoupdate_agent_rollout state for each group. Groups are identified by their position in the schedule. %s", valuesHelpString(autoupdatepb.AutoUpdateAgentGroupState_name)),
		}, []string{metricsGroupNumberLabelName}),
	}

	errs := trace.NewAggregate(
		reg.Register(m.reconciliations),
		reg.Register(m.reconciliationDuration),
		reg.Register(m.reconciliationTries),
		reg.Register(m.reconciliationTryDuration),

		reg.Register(m.versionPresent),
		reg.Register(m.versionTarget),
		reg.Register(m.versionStart),
		reg.Register(m.versionMode),
		reg.Register(m.configPresent),
		reg.Register(m.configMode),
		reg.Register(m.rolloutPresent),
		reg.Register(m.rolloutTarget),
		reg.Register(m.rolloutStart),
		reg.Register(m.rolloutMode),
		reg.Register(m.rolloutStrategy),

		reg.Register(m.rolloutTimeOverride),
		reg.Register(m.rolloutState),
		reg.Register(m.rolloutGroupState),
	)

	return &m, errs
}

func valuesHelpString[K constraints.Integer](possibleValues map[K]string) string {
	sb := strings.Builder{}
	sb.WriteString("Possible values are")

	// maps are nor ordered, so we must sort keys to consistently generate the help message.
	keys := maps.Keys(possibleValues)
	for _, k := range slices.Sorted(keys) {
		sb.WriteString(fmt.Sprintf(" %d:%s", k, possibleValues[k]))
	}

	sb.WriteRune('.')
	return sb.String()
}

func (m *metrics) setVersionMetric(version string, metric *prometheus.GaugeVec, now time.Time) {
	m.lock.Lock()
	defer m.lock.Unlock()

	// for every version we've seen
	for v, ts := range m.previousVersions {
		labels := prometheus.Labels{metricsVersionLabelName: v}
		// if the version is too old, we forget about it to limit cardinality
		if now.After(ts.Add(metricVersionLabelRetention)) {
			metric.Delete(labels)
			delete(m.previousVersions, v)
		} else {
			// Else we just mark the version as not set anymore
			metric.With(labels).Set(0)
		}
	}
	// We set the new version
	metric.With(prometheus.Labels{metricsVersionLabelName: version}).Set(1)
	m.previousVersions[version] = now
}

func (m *metrics) observeReconciliation(result string, duration time.Duration) {
	m.reconciliations.With(prometheus.Labels{metricsReconciliationResultLabelName: result}).Inc()
	m.reconciliationDuration.With(prometheus.Labels{metricsReconciliationResultLabelName: result}).Observe(duration.Seconds())
}

func (m *metrics) observeReconciliationTry(result string, duration time.Duration) {
	m.reconciliationTries.With(prometheus.Labels{metricsReconciliationResultLabelName: result}).Inc()
	m.reconciliationTryDuration.With(prometheus.Labels{metricsReconciliationResultLabelName: result}).Observe(duration.Seconds())
}

func (m *metrics) observeConfig(config *autoupdatepb.AutoUpdateConfig) {
	if config.GetSpec().GetAgents() == nil {
		m.configPresent.Set(0)
		m.configMode.Set(float64(agentModeCode[defaultConfigMode]))
		return
	}
	m.configPresent.Set(1)
	m.configMode.Set(float64(agentModeCode[config.GetSpec().GetAgents().GetMode()]))
}

func (m *metrics) observeVersion(version *autoupdatepb.AutoUpdateVersion, now time.Time) {
	if version.GetSpec().GetAgents() == nil {
		m.versionPresent.Set(0)
		m.versionMode.Set(float64(agentModeCode[defaultConfigMode]))
		return
	}
	m.versionPresent.Set(1)
	m.versionMode.Set(float64(agentModeCode[version.GetSpec().GetAgents().GetMode()]))
	m.setVersionMetric(version.GetSpec().GetAgents().GetStartVersion(), m.versionStart, now)
	m.setVersionMetric(version.GetSpec().GetAgents().GetTargetVersion(), m.versionTarget, now)
}

func (m *metrics) setGroupStates(groups []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup) {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Set the state for the groups specified in the rollout.
	for i, group := range groups {
		labels := prometheus.Labels{metricsGroupNumberLabelName: strconv.Itoa(i)}
		m.rolloutGroupState.With(labels).Set(float64(group.State))
	}

	// If we have as many or more groups than before, no cleanup to do.
	if len(groups) >= m.groupCount {
		m.groupCount = len(groups)
		return
	}

	// If we have less groups than before, we must unset the metrics for higher group numbers.
	for i := len(groups); i < m.groupCount; i++ {
		labels := prometheus.Labels{metricsGroupNumberLabelName: strconv.Itoa(i)}
		m.rolloutGroupState.With(labels).Set(float64(0))
	}
	m.groupCount = len(groups)
}

func (m *metrics) observeRollout(rollout *autoupdatepb.AutoUpdateAgentRollout, now time.Time) {
	if rollout.GetSpec() == nil {
		m.rolloutPresent.Set(0)
		m.rolloutMode.Set(0)
	} else {
		m.rolloutPresent.Set(1)
		m.rolloutMode.Set(float64(agentModeCode[rollout.GetSpec().GetAutoupdateMode()]))
		m.setVersionMetric(rollout.GetSpec().GetStartVersion(), m.rolloutStart, now)
		m.setVersionMetric(rollout.GetSpec().GetTargetVersion(), m.rolloutTarget, now)
	}

	m.setStrategyMetric(rollout.GetSpec().GetStrategy(), m.rolloutStrategy)

	if to := rollout.GetStatus().GetTimeOverride().AsTime(); !(to.IsZero() || to.Unix() == 0) {
		m.rolloutTimeOverride.Set(float64(to.Second()))
	} else {
		m.rolloutTimeOverride.Set(0)
	}

	m.rolloutState.Set(float64(rollout.GetStatus().GetState()))
	m.setGroupStates(rollout.GetStatus().GetGroups())
}

var strategies = []string{autoupdate.AgentsStrategyHaltOnError, autoupdate.AgentsStrategyTimeBased}

func (m *metrics) setStrategyMetric(strategy string, metric *prometheus.GaugeVec) {
	for _, s := range strategies {
		if s == strategy {
			metric.With(prometheus.Labels{metricsStrategyLabelName: s}).Set(1)
		} else {
			metric.With(prometheus.Labels{metricsStrategyLabelName: s}).Set(0)
		}
	}
}
