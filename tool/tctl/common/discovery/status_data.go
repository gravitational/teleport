// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package discovery

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	usertasksapi "github.com/gravitational/teleport/api/types/usertasks"
)

type statusSummary struct {
	GeneratedAt          time.Time             `json:"generated_at" yaml:"generated_at"`
	FilteredIntegration  string                `json:"filtered_integration,omitempty" yaml:"filtered_integration,omitempty"`
	CacheSummary         string                `json:"cache_summary,omitempty" yaml:"cache_summary,omitempty"`
	DiscoveryConfigCount int                   `json:"discovery_config_count" yaml:"discovery_config_count"`
	DiscoveryGroupCount  int                   `json:"discovery_group_count" yaml:"discovery_group_count"`
	UserTasks            []taskListItem        `json:"user_tasks" yaml:"user_tasks"`
	DiscoveryConfigs     []configStatus        `json:"discovery_configs" yaml:"discovery_configs"`
	TotalTasks           int                   `json:"total_tasks" yaml:"total_tasks"`
	OpenTasks            int                   `json:"open_tasks" yaml:"open_tasks"`
	ResolvedTasks        int                   `json:"resolved_tasks" yaml:"resolved_tasks"`
	TasksByType          map[string]int        `json:"tasks_by_type" yaml:"tasks_by_type"`
	TasksByIssue         map[string]int        `json:"tasks_by_issue" yaml:"tasks_by_issue"`
	TasksByIntegration   map[string]int        `json:"tasks_by_integration" yaml:"tasks_by_integration"`
	Integrations         []integrationListItem `json:"integrations" yaml:"integrations"`
	SSMRunStats          *auditEventStats      `json:"ssm_run_stats,omitempty" yaml:"ssm_run_stats,omitempty"`
	JoinStats            *auditEventStats      `json:"join_stats,omitempty" yaml:"join_stats,omitempty"`
}

type auditEventStats struct {
	Window          string    `json:"-" yaml:"-"`
	From            time.Time `json:"from" yaml:"from"`
	To              time.Time `json:"to" yaml:"to"`
	EffectiveWindow string    `json:"effective_window,omitempty" yaml:"effective_window,omitempty"`
	OldestEvent     time.Time `json:"oldest_event,omitempty" yaml:"oldest_event,omitempty"`
	SuggestedLimit  int       `json:"suggested_limit,omitempty" yaml:"suggested_limit,omitempty"`
	Total           int       `json:"total" yaml:"total"`
	Success         int       `json:"success" yaml:"success"`
	Failed          int       `json:"failed" yaml:"failed"`
	DistinctHosts   int       `json:"distinct_hosts" yaml:"distinct_hosts"`
	FailingHosts    int       `json:"failing_hosts" yaml:"failing_hosts"`
	LimitReached    bool      `json:"limit_reached" yaml:"limit_reached"`
	CacheHits       int       `json:"cache_hits,omitempty" yaml:"cache_hits,omitempty"`
	CacheMisses     int       `json:"cache_misses,omitempty" yaml:"cache_misses,omitempty"`
}

type configStatus struct {
	Name       string    `json:"name" yaml:"name"`
	Group      string    `json:"group" yaml:"group"`
	State      string    `json:"state" yaml:"state"`
	Matchers   string    `json:"matchers" yaml:"matchers"`
	Discovered uint64    `json:"discovered" yaml:"discovered"`
	LastSync   time.Time `json:"last_sync" yaml:"last_sync"`
}

type resourcesAggregate struct {
	Found    uint64 `json:"found"`
	Enrolled uint64 `json:"enrolled"`
	Failed   uint64 `json:"failed"`
}

func makeStatusSummary(tasks []*usertasksv1.UserTask, dcs []*discoveryconfig.DiscoveryConfig, integrations []types.Integration, integration string) statusSummary {
	summary := statusSummary{
		GeneratedAt:          time.Now().UTC(),
		FilteredIntegration:  integration,
		DiscoveryConfigCount: len(dcs),
		DiscoveryGroupCount:  countDiscoveryGroups(dcs),
		UserTasks:            make([]taskListItem, 0, len(tasks)),
		DiscoveryConfigs:     make([]configStatus, 0, len(dcs)),
		TotalTasks:           len(tasks),
		TasksByType:          map[string]int{},
		TasksByIssue:         map[string]int{},
		TasksByIntegration:   map[string]int{},
	}

	for _, task := range tasks {
		switch task.GetSpec().GetState() {
		case usertasksapi.TaskStateOpen:
			summary.OpenTasks++
		case usertasksapi.TaskStateResolved:
			summary.ResolvedTasks++
		}
		summary.TasksByType[task.GetSpec().GetTaskType()]++
		summary.TasksByIssue[task.GetSpec().GetIssueType()]++
		summary.TasksByIntegration[task.GetSpec().GetIntegration()]++
	}
	summary.UserTasks = toTaskListItems(tasks)
	slices.SortFunc(summary.UserTasks, func(a, b taskListItem) int {
		if c := compareTimeDesc(a.LastStateChange, b.LastStateChange); c != 0 {
			return c
		}
		return cmp.Compare(a.Name, b.Name)
	})

	for _, dc := range dcs {
		summary.DiscoveryConfigs = append(summary.DiscoveryConfigs, configStatus{
			Name:       dc.GetName(),
			Group:      dc.GetDiscoveryGroup(),
			State:      cmp.Or(strings.TrimSpace(dc.Status.State), "UNKNOWN"),
			Matchers:   configMatchersSummary(dc),
			Discovered: dc.Status.DiscoveredResources,
			LastSync:   dc.Status.LastSyncTime.UTC(),
		})
	}
	slices.SortFunc(summary.DiscoveryConfigs, func(a, b configStatus) int {
		return cmp.Compare(a.Name, b.Name)
	})

	statsMap := buildIntegrationStatsMap(dcs)
	taskCountMap := countTasksByIntegration(tasks)
	summary.Integrations = toIntegrationListItems(integrations, statsMap, taskCountMap)

	return summary
}

func configMatchersSummary(dc *discoveryconfig.DiscoveryConfig) string {
	accessGraphMatchers := 0
	if dc.Spec.AccessGraph != nil {
		accessGraphMatchers = len(dc.Spec.AccessGraph.AWS)
	}
	parts := make([]string, 0, 5)
	appendNonZero := func(label string, value int) {
		if value > 0 {
			parts = append(parts, fmt.Sprintf("%s=%d", label, value))
		}
	}
	appendNonZero("aws", len(dc.Spec.AWS))
	appendNonZero("azure", len(dc.Spec.Azure))
	appendNonZero("gcp", len(dc.Spec.GCP))
	appendNonZero("kube", len(dc.Spec.Kube))
	appendNonZero("ag", accessGraphMatchers)
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, " ")
}

func countDiscoveryGroups(dcs []*discoveryconfig.DiscoveryConfig) int {
	groups := map[string]struct{}{}
	for _, dc := range dcs {
		groups[dc.GetDiscoveryGroup()] = struct{}{}
	}
	return len(groups)
}

func addDiscoveredSummary(total *resourcesAggregate, summary *discoveryconfigv1.ResourcesDiscoveredSummary) {
	if summary == nil {
		return
	}
	total.Found += summary.GetFound()
	total.Enrolled += summary.GetEnrolled()
	total.Failed += summary.GetFailed()
}

func awaitingJoin(stats resourcesAggregate) uint64 {
	joinedOrFailed := stats.Enrolled + stats.Failed
	if joinedOrFailed >= stats.Found {
		return 0
	}
	return stats.Found - joinedOrFailed
}
