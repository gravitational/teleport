/*
Copyright 2022 Gravitational, Inc.

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

package types

import (
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
)

const (
	// UpgraderKindKuberController is a short name used to identify the kube-controller-based
	// external upgrader variant.
	UpgraderKindKubeController = "kube"

	// UpgraderKindSystemdUnit is a short name used to identify the systemd-unit-based
	// external upgrader variant.
	UpgraderKindSystemdUnit = "unit"
)

var validWeekdays = [7]time.Weekday{
	time.Sunday,
	time.Monday,
	time.Tuesday,
	time.Wednesday,
	time.Thursday,
	time.Friday,
	time.Saturday,
}

// ParseWeekday attempts to interpret a string as a time.Weekday. In the interest of flexibility,
// parsing is case-insensitive and supports the common three-letter shorthand accepted by many
// common scheduling utilites (e.g. contab, systemd timers).
func ParseWeekday(s string) (day time.Weekday, ok bool) {
	for _, w := range validWeekdays {
		if strings.EqualFold(w.String(), s) || strings.EqualFold(w.String()[:3], s) {
			return w, true
		}
	}

	return time.Sunday, false
}

// ParseWeekdays attempts to parse a slice of strings representing week days.
// The slice must not be empty but can also contain a single value "*", representing the whole week.
// Day order doesn't matter but the same week day must not be present multiple times.
// In the interest of flexibility, parsing is case-insensitive and supports the common three-letter shorthand
// accepted by many common scheduling utilites (e.g. contab, systemd timers).
func ParseWeekdays(days []string) (map[time.Weekday]struct{}, error) {
	if len(days) == 0 {
		return nil, trace.BadParameter("empty weekdays list")
	}
	// Special case, we support wildcards.
	if len(days) == 1 && days[0] == Wildcard {
		return map[time.Weekday]struct{}{
			time.Monday:    {},
			time.Tuesday:   {},
			time.Wednesday: {},
			time.Thursday:  {},
			time.Friday:    {},
			time.Saturday:  {},
			time.Sunday:    {},
		}, nil
	}
	weekdays := make(map[time.Weekday]struct{}, 7)
	for _, day := range days {
		weekday, ok := ParseWeekday(day)
		if !ok {
			return nil, trace.BadParameter("failed to parse weekday: %v", day)
		}
		// Check if this is a duplicate
		if _, ok := weekdays[weekday]; ok {
			return nil, trace.BadParameter("duplicate weekday: %v", weekday.String())
		}
		weekdays[weekday] = struct{}{}
	}
	return weekdays, nil
}

// generator builds a closure that iterates valid maintenance config from the current day onward. Used in
// schedule export logic and tests.
func (w *AgentUpgradeWindow) generator(from time.Time) func() (start time.Time, end time.Time) {
	from = from.UTC()
	next := time.Date(
		from.Year(),
		from.Month(),
		from.Day(),
		int(w.UTCStartHour%24),
		0, // min
		0, // sec
		0, // nsec
		time.UTC,
	)

	var weekdays []time.Weekday
	for _, d := range w.Weekdays {
		if p, ok := ParseWeekday(d); ok {
			weekdays = append(weekdays, p)
		}
	}

	return func() (start time.Time, end time.Time) {
		for { // safe because invalid weekdays have been filtered out
			start = next
			end = start.Add(time.Hour)

			next = next.AddDate(0, 0, 1)

			if len(weekdays) == 0 {
				return
			}

			for _, day := range weekdays {
				if start.Weekday() == day {
					return
				}
			}
		}
	}
}

// Export exports the next `n` upgrade windows as a schedule object, starting from `from`.
func (w *AgentUpgradeWindow) Export(from time.Time, n int) AgentUpgradeSchedule {
	gen := w.generator(from)

	sched := AgentUpgradeSchedule{
		Windows: make([]ScheduledAgentUpgradeWindow, 0, n),
	}
	for i := 0; i < n; i++ {
		start, stop := gen()
		sched.Windows = append(sched.Windows, ScheduledAgentUpgradeWindow{
			Start: start.UTC(),
			Stop:  stop.UTC(),
		})
	}

	return sched
}

func (s *AgentUpgradeSchedule) Clone() *AgentUpgradeSchedule {
	return utils.CloneProtoMsg(s)
}

// NewClusterMaintenanceConfig creates a new maintenance config with no parameters set.
func NewClusterMaintenanceConfig() ClusterMaintenanceConfig {
	var cmc ClusterMaintenanceConfigV1
	cmc.setStaticFields()
	return &cmc
}

// ClusterMaintenanceConfig represents a singleton config object used to schedule maintenance
// windows. Currently this config object's only purpose is to configure a global agent
// upgrade window, used to coordinate upgrade timing for non-control-plane agents.
type ClusterMaintenanceConfig interface {
	Resource

	// GetNonce gets the nonce of the maintenance config.
	GetNonce() uint64

	// WithNonce creates a shallow copy with a new nonce.
	WithNonce(nonce uint64) any

	// GetAgentUpgradeWindow gets the agent upgrade window.
	GetAgentUpgradeWindow() (win AgentUpgradeWindow, ok bool)

	// SetAgentUpgradeWindow sets the agent upgrade window.
	SetAgentUpgradeWindow(win AgentUpgradeWindow)

	// WithinUpgradeWindow returns true if the time is within the configured
	// upgrade window.
	WithinUpgradeWindow(t time.Time) bool

	CheckAndSetDefaults() error
}

func (m *ClusterMaintenanceConfigV1) setStaticFields() {
	if m.Version == "" {
		m.Version = V1
	}

	if m.Kind == "" {
		m.Kind = KindClusterMaintenanceConfig
	}

	if m.Metadata.Name == "" {
		m.Metadata.Name = MetaNameClusterMaintenanceConfig
	}
}

func (m *ClusterMaintenanceConfigV1) CheckAndSetDefaults() error {
	m.setStaticFields()

	if err := m.ResourceHeader.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if m.Version != V1 {
		return trace.BadParameter("unexpected maintenance config resource version %q (expected %q)", m.Version, V1)
	}

	if m.Kind == MetaNameClusterMaintenanceConfig {
		// normalize easy mixup
		m.Kind = KindClusterMaintenanceConfig
	}

	if m.Kind != KindClusterMaintenanceConfig {
		return trace.BadParameter("unexpected maintenance config kind %q (expected %q)", m.Kind, KindClusterMaintenanceConfig)
	}

	if m.Metadata.Name == KindClusterMaintenanceConfig {
		// normalize easy mixup
		m.Metadata.Name = MetaNameClusterMaintenanceConfig
	}

	if m.Metadata.Name != MetaNameClusterMaintenanceConfig {
		return trace.BadParameter("unexpected maintenance config name %q (expected %q)", m.Metadata.Name, MetaNameClusterMaintenanceConfig)
	}

	if m.Spec.AgentUpgrades != nil {
		if h := m.Spec.AgentUpgrades.UTCStartHour; h > 23 {
			return trace.BadParameter("agent upgrade window utc start hour must be in range 0..23, got %d", h)
		}

		for _, day := range m.Spec.AgentUpgrades.Weekdays {
			if _, ok := ParseWeekday(day); !ok {
				return trace.BadParameter("invalid weekday in agent upgrade window: %q", day)
			}
		}
	}

	return nil
}

func (m *ClusterMaintenanceConfigV1) GetNonce() uint64 {
	return m.Nonce
}

func (m *ClusterMaintenanceConfigV1) WithNonce(nonce uint64) any {
	shallowCopy := *m
	shallowCopy.Nonce = nonce
	return &shallowCopy
}

func (m *ClusterMaintenanceConfigV1) GetAgentUpgradeWindow() (win AgentUpgradeWindow, ok bool) {
	if m.Spec.AgentUpgrades == nil {
		return AgentUpgradeWindow{}, false
	}

	return *m.Spec.AgentUpgrades, true
}

func (m *ClusterMaintenanceConfigV1) SetAgentUpgradeWindow(win AgentUpgradeWindow) {
	m.Spec.AgentUpgrades = &win
}

// WithinUpgradeWindow returns true if the time is within the configured
// upgrade window.
func (m *ClusterMaintenanceConfigV1) WithinUpgradeWindow(t time.Time) bool {
	upgradeWindow, ok := m.GetAgentUpgradeWindow()
	if !ok {
		return false
	}

	if len(upgradeWindow.Weekdays) == 0 {
		if int(upgradeWindow.UTCStartHour) == t.Hour() {
			return true
		}
	}

	weekday := t.Weekday().String()
	for _, upgradeWeekday := range upgradeWindow.Weekdays {
		if weekday == upgradeWeekday {
			if int(upgradeWindow.UTCStartHour) == t.Hour() {
				return true
			}
		}
	}
	return false
}
