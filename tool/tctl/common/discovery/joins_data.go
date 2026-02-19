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
	"slices"
	"strings"
	"time"

	apievents "github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
)

type joinRecord struct {
	EventTime  string `json:"event_time"`
	Code       string `json:"code"`
	HostID     string `json:"host_id"`
	NodeName   string `json:"node_name"`
	Role       string `json:"role"`
	Method     string `json:"method"`
	TokenName  string `json:"token_name,omitempty"`
	InstanceID string `json:"instance_id,omitempty"`
	AccountID  string `json:"account_id,omitempty"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`

	parsedEventTime time.Time
}

type joinEventFilters struct {
	HostID string
}

type joinGroup struct {
	HostID           string       `json:"host_id"`
	NodeName         string       `json:"node_name"`
	MostRecent       joinRecord   `json:"most_recent"`
	MostRecentFailed bool         `json:"most_recent_failed"`
	TotalJoins       int          `json:"total_joins"`
	FailedJoins      int          `json:"failed_joins"`
	SuccessJoins     int          `json:"success_joins"`
	Joins            []joinRecord `json:"joins"`
}

type joinAnalysis struct {
	Total        int            `json:"total"`
	Success      int            `json:"success"`
	Failed       int            `json:"failed"`
	ByHost       map[string]int `json:"by_host"`
	FailedByHost map[string]int `json:"failed_by_host"`
}

type joinsOutput struct {
	Window         string      `json:"-"`
	From           time.Time   `json:"from"`
	To             time.Time   `json:"to"`
	FetchLimit     int         `json:"fetch_limit"`
	LimitReached   bool        `json:"limit_reached"`
	SuggestedLimit int         `json:"suggested_limit,omitempty"`
	CacheSummary string      `json:"cache_summary,omitempty"`
	TotalJoins   int         `json:"total_joins"`
	SuccessJoins int         `json:"success_joins"`
	FailedJoins  int         `json:"failed_joins"`
	TotalHosts   int         `json:"total_hosts"`
	FailingHosts int         `json:"failing_hosts"`
	HostPage     pageInfo    `json:"host_page"`
	Hosts        []joinGroup `json:"hosts"`

	// HostsByAccount groups hosts by AWS account ID. Populated when --group-by-account is set.
	// When set, Hosts is nil.
	HostsByAccount map[string][]joinGroup `json:"hosts_by_account,omitempty"`
}

type joinHistoryRow struct {
	Timestamp  string `json:"timestamp"`
	Result     string `json:"result"`
	Method     string `json:"method"`
	Role       string `json:"role"`
	TokenName  string `json:"token_name,omitempty"`
	InstanceID string `json:"instance_id,omitempty"`
	AccountID  string `json:"account_id,omitempty"`
}

// getJoinAttributeString safely extracts a string value from an InstanceJoin's
// Attributes protobuf Struct field.
func getJoinAttributeString(join *apievents.InstanceJoin, key string) string {
	if join.Attributes == nil {
		return ""
	}
	v, ok := join.Attributes.Fields[key]
	if !ok || v == nil {
		return ""
	}
	return v.GetStringValue()
}

// extractEC2InstanceID extracts the EC2 instance ID from an IAM role ARN.
// For example: "arn:aws:sts::123456:assumed-role/role-name/i-030a87f439b67b43a"
// returns "i-030a87f439b67b43a".
func extractEC2InstanceID(arn string) string {
	// The instance ID is the last segment after the final "/".
	lastSlash := strings.LastIndex(arn, "/")
	if lastSlash < 0 || lastSlash+1 >= len(arn) {
		return ""
	}
	candidate := arn[lastSlash+1:]
	if strings.HasPrefix(candidate, "i-") {
		return candidate
	}
	return ""
}

func parseInstanceJoinEvents(eventList []apievents.AuditEvent, filters joinEventFilters) []joinRecord {
	records := make([]joinRecord, 0, len(eventList))
	for _, event := range eventList {
		join, ok := event.(*apievents.InstanceJoin)
		if !ok {
			continue
		}

		record := joinRecord{
			Code:       join.Code,
			HostID:     join.HostID,
			NodeName:   join.NodeName,
			Role:       join.Role,
			Method:     join.Method,
			TokenName:  sanitizeTokenName(join.TokenName, join.Method),
			InstanceID: extractEC2InstanceID(getJoinAttributeString(join, "Arn")),
			AccountID:  getJoinAttributeString(join, "Account"),
			Success:    join.Status.Success,
			Error:      join.Status.Error,
		}
		if !join.Time.IsZero() {
			record.parsedEventTime = join.Time.UTC()
			record.EventTime = record.parsedEventTime.Format(time.RFC3339Nano)
		}

		if filters.HostID != "" {
			// Compare against the group key so "show" works with the
			// same identifiers displayed by "ls" (e.g. "unknown (10.0.0.1)").
			if !strings.EqualFold(joinGroupKey(record), strings.TrimSpace(filters.HostID)) {
				continue
			}
		}
		records = append(records, record)
	}

	sortJoinRecords(records)
	return records
}

func isJoinFailure(record joinRecord) bool {
	return record.Code == libevents.InstanceJoinFailureCode || !record.Success
}

// sanitizeTokenName masks the token name when the join method is "token"
// since that's a secret value.
func sanitizeTokenName(tokenName, method string) string {
	if strings.EqualFold(method, "token") && tokenName != "" {
		return "********"
	}
	return tokenName
}

// joinGroupKey returns a stable key for grouping join records by host.
// When HostID is empty (failed joins before identification), returns "unknown".
func joinGroupKey(record joinRecord) string {
	hostID := strings.TrimSpace(record.HostID)
	if hostID != "" {
		return hostID
	}
	return "unknown"
}

func isUnknownHost(hostID string) bool {
	return hostID == "unknown"
}

func analyzeInstanceJoins(records []joinRecord) joinAnalysis {
	analysis := joinAnalysis{
		Total:        len(records),
		ByHost:       map[string]int{},
		FailedByHost: map[string]int{},
	}

	for _, record := range records {
		key := joinGroupKey(record)
		analysis.ByHost[key]++

		if isJoinFailure(record) {
			analysis.Failed++
			analysis.FailedByHost[key]++
		} else {
			analysis.Success++
		}
	}

	return analysis
}

func groupJoinsByHost(records []joinRecord) []joinGroup {
	byHost := map[string][]joinRecord{}
	for _, record := range records {
		key := joinGroupKey(record)
		byHost[key] = append(byHost[key], record)
	}

	groups := make([]joinGroup, 0, len(byHost))
	for hostID, hostJoins := range byHost {
		sortJoinRecords(hostJoins)

		group := joinGroup{
			HostID:           hostID,
			NodeName:         hostJoins[0].NodeName,
			MostRecent:       hostJoins[0],
			MostRecentFailed: isJoinFailure(hostJoins[0]),
			TotalJoins:       len(hostJoins),
			Joins:            hostJoins,
		}
		for _, join := range hostJoins {
			if isJoinFailure(join) {
				group.FailedJoins++
			} else {
				group.SuccessJoins++
			}
		}
		groups = append(groups, group)
	}

	slices.SortFunc(groups, func(a, b joinGroup) int {
		if c := compareTimeDesc(a.MostRecent.parsedEventTime, b.MostRecent.parsedEventTime); c != 0 {
			return c
		}
		return cmp.Compare(a.HostID, b.HostID)
	})

	return groups
}

func selectFailingJoinGroups(groups []joinGroup, limit int) []joinGroup {
	out := make([]joinGroup, 0, len(groups))
	for _, group := range groups {
		if !group.MostRecentFailed {
			continue
		}
		out = append(out, group)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func filterOutUnknownJoinGroups(groups []joinGroup) []joinGroup {
	out := make([]joinGroup, 0, len(groups))
	for _, group := range groups {
		if isUnknownHost(group.HostID) {
			continue
		}
		out = append(out, group)
	}
	return out
}

// groupJoinsByAccount replaces the flat Hosts list with an account-keyed map.
func (o *joinsOutput) groupByAccount() {
	o.HostsByAccount = groupByAccountField(o.Hosts, func(host joinGroup) string {
		return host.MostRecent.AccountID
	})
	o.Hosts = nil
}

func buildJoinHistoryRows(group joinGroup, showAll bool) []joinHistoryRow {
	joins := group.Joins
	if !showAll && len(joins) > 1 {
		joins = joins[:1]
	}

	rows := make([]joinHistoryRow, 0, len(joins))
	for _, join := range joins {
		timestamp := cmp.Or(formatMaybeParsedTime(join.parsedEventTime), join.EventTime)
		result := "success"
		if isJoinFailure(join) {
			result = cmp.Or(join.Error, "failed")
		}
		rows = append(rows, joinHistoryRow{
			Timestamp:  timestamp,
			Result:     result,
			Method:     join.Method,
			Role:       join.Role,
			TokenName:  join.TokenName,
			InstanceID: join.InstanceID,
			AccountID:  join.AccountID,
		})
	}
	return rows
}
