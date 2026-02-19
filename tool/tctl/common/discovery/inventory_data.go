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

	"github.com/gravitational/teleport/api/types"
)

// inventoryHostState represents where a host is in the discovery pipeline.
type inventoryHostState string

const (
	inventoryStateOnline       inventoryHostState = "Online"
	inventoryStateOffline      inventoryHostState = "Offline"
	inventoryStateJoinFailed   inventoryHostState = "Join Failed"
	inventoryStateSSMFailed    inventoryHostState = "SSM Failed"
	inventoryStateSSMAttempted inventoryHostState = "SSM Attempted"
	inventoryStateJoinedOnly   inventoryHostState = "Joined Only"
)

type inventoryHost struct {
	// DisplayID is the preferred unified identifier: instance ID when
	// available, otherwise the Teleport node UUID.
	DisplayID  string             `json:"display_id"`
	HostID     string             `json:"host_id"`
	InstanceID string             `json:"instance_id,omitempty"`
	AccountID  string             `json:"account_id,omitempty"`
	Region     string             `json:"region,omitempty"`
	NodeName   string             `json:"node_name"`
	State      inventoryHostState `json:"state"`
	Method     string             `json:"method,omitempty"`

	LastSSMRun time.Time `json:"last_ssm_run,omitempty"`
	LastJoin   time.Time `json:"last_join,omitempty"`
	LastSeen   time.Time `json:"last_seen,omitempty"`
	IsOnline   bool      `json:"is_online"`

	SSMRuns          int            `json:"ssm_runs"`
	SSMSuccess       int            `json:"ssm_success"`
	SSMFailed        int            `json:"ssm_failed"`
	MostRecentSSMRun *ssmRunRecord  `json:"most_recent_ssm_run,omitempty"`
	Joins            int            `json:"joins"`
	JoinSuccess      int            `json:"join_success"`
	JoinFailed       int            `json:"join_failed"`
	MostRecentJoin   *joinRecord    `json:"most_recent_join,omitempty"`

	// SSMRecords and JoinRecords are kept for the inventory show timeline
	// but excluded from JSON/YAML serialization to avoid bloating output.
	SSMRecords  []ssmRunRecord `json:"-" yaml:"-"`
	JoinRecords []joinRecord   `json:"-" yaml:"-"`

	// mostRecentActivity is the latest timestamp across all sources, used for sorting.
	mostRecentActivity time.Time
}

type inventoryOutput struct {
	Window         string          `json:"-"`
	From           time.Time       `json:"from"`
	To             time.Time       `json:"to"`
	CacheSummary   string          `json:"cache_summary,omitempty"`
	TotalHosts     int             `json:"total_hosts"`
	OnlineHosts    int             `json:"online_hosts"`
	OfflineHosts   int             `json:"offline_hosts"`
	FailedHosts    int             `json:"failed_hosts"`
	FetchLimit     int             `json:"fetch_limit"`
	LimitReached   bool            `json:"limit_reached"`
	SuggestedLimit int             `json:"suggested_limit,omitempty"`
	HostPage       pageInfo        `json:"host_page"`
	Hosts          []inventoryHost `json:"hosts"`

	// HostsByAccount groups hosts by AWS account ID. Populated when --group-by-account is set.
	// When set, Hosts is nil.
	HostsByAccount map[string][]inventoryHost `json:"hosts_by_account,omitempty"`
}

// hostData is the intermediate accumulator used while correlating data sources.
type hostData struct {
	nodeName   string
	instanceID string
	accountID  string
	region     string
	isOnline   bool
	lastSeen   time.Time
	method     string
	ssmRuns    []ssmRunRecord
	joinRecs   []joinRecord
}

// buildInventoryHosts correlates three data sources (online nodes, SSM run
// records, and instance join records) into a unified host list in five phases:
// (1) index online nodes, (2) index SSM runs by instance ID, (3) index joins
// by instance ID or host ID, (4) merge duplicates where the same host appears
// under both a UUID key and an instance-ID key, (5) build the final sorted
// result with derived pipeline state.
func buildInventoryHosts(
	nodes []types.Server,
	ssmRecords []ssmRunRecord,
	joinRecords []joinRecord,
) []inventoryHost {
	hosts := make(map[string]*hostData)
	getOrCreate := func(id string) *hostData {
		if h, ok := hosts[id]; ok {
			return h
		}
		h := &hostData{}
		hosts[id] = h
		return h
	}

	addNodeHosts(getOrCreate, nodes)
	addSSMHosts(getOrCreate, ssmRecords)
	addJoinHosts(getOrCreate, joinRecords)
	mergeHostDuplicates(hosts)
	return buildInventoryResult(hosts)
}

// addNodeHosts populates hosts from currently online Teleport nodes.
func addNodeHosts(getOrCreate func(string) *hostData, nodes []types.Server) {
	// Nodes (currently online). Extract AWS instance/account IDs from
	// labels (teleport.dev/instance-id, teleport.dev/account-id) set during
	// EC2 discovery so that joined nodes show consistent AWS-native names.
	for _, node := range nodes {
		id := node.GetName()
		if id == "" {
			continue
		}
		h := getOrCreate(id)
		h.isOnline = true
		h.lastSeen = node.Expiry()
		if h.nodeName == "" {
			h.nodeName = node.GetHostname()
		}
		if awsID := node.GetAWSInstanceID(); awsID != "" && h.instanceID == "" {
			h.instanceID = awsID
		}
		if awsAcct := node.GetAWSAccountID(); awsAcct != "" && h.accountID == "" {
			h.accountID = awsAcct
		}
		if h.region == "" {
			if r, ok := node.GetLabel(types.AWSInstanceRegion); ok && r != "" {
				h.region = r
			} else if awsMeta := node.GetAWSInfo(); awsMeta != nil && awsMeta.Region != "" {
				h.region = awsMeta.Region
			}
		}
	}
}

// addSSMHosts populates hosts from SSM run records, keyed by EC2 instance ID.
func addSSMHosts(getOrCreate func(string) *hostData, ssmRecords []ssmRunRecord) {
	// SSM runs — keyed by EC2 instance ID (e.g. i-030a87f439b67b43a).
	for _, rec := range ssmRecords {
		id := rec.InstanceID
		if id == "" {
			continue
		}
		h := getOrCreate(id)
		h.ssmRuns = append(h.ssmRuns, rec)
		if h.instanceID == "" {
			h.instanceID = id
		}
		if h.accountID == "" {
			h.accountID = rec.AccountID
		}
		if h.region == "" {
			h.region = rec.Region
		}
	}
}

// addJoinHosts populates hosts from join event records.
func addJoinHosts(getOrCreate func(string) *hostData, joinRecords []joinRecord) {
	// Join events — use InstanceID (from ARN) when available for
	// correlation with SSM runs; otherwise fall back to HostID.
	for _, rec := range joinRecords {
		var id string
		if rec.InstanceID != "" {
			id = rec.InstanceID
		} else {
			id = joinGroupKey(rec)
		}
		if id == "" {
			continue
		}
		h := getOrCreate(id)
		h.joinRecs = append(h.joinRecs, rec)
		if h.nodeName == "" && rec.NodeName != "" {
			h.nodeName = rec.NodeName
		}
		if h.method == "" && rec.Method != "" {
			h.method = rec.Method
		}
		if h.instanceID == "" && rec.InstanceID != "" {
			h.instanceID = rec.InstanceID
		}
		if h.accountID == "" && rec.AccountID != "" {
			h.accountID = rec.AccountID
		}
	}
}

// mergeHostDuplicates merges instance-ID-keyed entries into UUID-keyed entries
// so that a host appears only once. An online node (keyed by UUID) may have
// its AWS instance ID from labels, while SSM runs and join events are keyed
// by that same instance ID.
func mergeHostDuplicates(hosts map[string]*hostData) {
	// Build a reverse map: instanceID → UUID key for nodes that have one.
	instanceToUUID := make(map[string]string)
	for key, data := range hosts {
		if data.instanceID != "" && data.isOnline {
			instanceToUUID[data.instanceID] = key
		}
	}
	for instanceID, data := range hosts {
		if !strings.HasPrefix(instanceID, "i-") {
			continue
		}
		// Find a UUID-keyed node entry to merge into. First check
		// the reverse map (node labels), then fall back to join records.
		targetKey := ""
		if uuidKey, ok := instanceToUUID[instanceID]; ok && uuidKey != instanceID {
			targetKey = uuidKey
		} else {
			for _, rec := range data.joinRecs {
				hostID := strings.TrimSpace(rec.HostID)
				if hostID == "" || hostID == instanceID {
					continue
				}
				if _, ok := hosts[hostID]; ok {
					targetKey = hostID
					break
				}
			}
		}
		if targetKey == "" {
			continue
		}
		nodeEntry := hosts[targetKey]
		nodeEntry.ssmRuns = append(nodeEntry.ssmRuns, data.ssmRuns...)
		nodeEntry.joinRecs = append(nodeEntry.joinRecs, data.joinRecs...)
		// Re-sort after merge so [0] is always the most recent record.
		sortSSMRunRecords(nodeEntry.ssmRuns)
		sortJoinRecords(nodeEntry.joinRecs)
		if nodeEntry.instanceID == "" {
			nodeEntry.instanceID = data.instanceID
		}
		if nodeEntry.accountID == "" {
			nodeEntry.accountID = data.accountID
		}
		if nodeEntry.method == "" && data.method != "" {
			nodeEntry.method = data.method
		}
		if nodeEntry.nodeName == "" && data.nodeName != "" {
			nodeEntry.nodeName = data.nodeName
		}
		if nodeEntry.region == "" && data.region != "" {
			nodeEntry.region = data.region
		}
		delete(hosts, instanceID)
	}
}

// buildInventoryResult converts the intermediate host map into a sorted slice of inventoryHost.
func buildInventoryResult(hosts map[string]*hostData) []inventoryHost {
	result := make([]inventoryHost, 0, len(hosts))
	for hostID, data := range hosts {
		displayID := cmp.Or(data.instanceID, hostID)
		ih := inventoryHost{
			DisplayID:   displayID,
			HostID:      hostID,
			InstanceID:  data.instanceID,
			AccountID:   data.accountID,
			Region:      data.region,
			NodeName:    data.nodeName,
			IsOnline:    data.isOnline,
			LastSeen:    data.lastSeen,
			Method:      data.method,
			SSMRecords:  data.ssmRuns,
			JoinRecords: data.joinRecs,
		}

		// SSM stats
		ih.SSMRuns = len(data.ssmRuns)
		for _, r := range data.ssmRuns {
			if isSSMRunFailure(r) {
				ih.SSMFailed++
			} else {
				ih.SSMSuccess++
			}
		}
		if len(data.ssmRuns) > 0 {
			ih.LastSSMRun = data.ssmRuns[0].parsedEventTime
			ih.MostRecentSSMRun = &data.ssmRuns[0]
		}

		// Join stats
		ih.Joins = len(data.joinRecs)
		hasSuccessfulJoin := false
		for _, r := range data.joinRecs {
			if isJoinFailure(r) {
				ih.JoinFailed++
			} else {
				ih.JoinSuccess++
				hasSuccessfulJoin = true
			}
		}
		if len(data.joinRecs) > 0 {
			ih.LastJoin = data.joinRecs[0].parsedEventTime
			ih.MostRecentJoin = &data.joinRecs[0]
			if ih.Method == "" {
				ih.Method = data.joinRecs[0].Method
			}
		}

		// Derive state
		ih.State = deriveInventoryState(data.isOnline, hasSuccessfulJoin, data.joinRecs, data.ssmRuns)

		// Most recent activity for sorting
		ih.mostRecentActivity = maxTime(ih.LastSeen, ih.LastSSMRun, ih.LastJoin)

		result = append(result, ih)
	}

	slices.SortFunc(result, func(a, b inventoryHost) int {
		if c := compareTimeDesc(a.mostRecentActivity, b.mostRecentActivity); c != 0 {
			return c
		}
		return cmp.Compare(a.HostID, b.HostID)
	})

	return result
}

func deriveInventoryState(isOnline, hasSuccessfulJoin bool, joinRecs []joinRecord, ssmRuns []ssmRunRecord) inventoryHostState {
	if isOnline {
		return inventoryStateOnline
	}
	if len(joinRecs) > 0 {
		if hasSuccessfulJoin {
			return inventoryStateOffline
		}
		return inventoryStateJoinFailed
	}
	if len(ssmRuns) > 0 {
		if isSSMRunFailure(ssmRuns[0]) {
			return inventoryStateSSMFailed
		}
		return inventoryStateSSMAttempted
	}
	return inventoryStateJoinedOnly
}

func maxTime(times ...time.Time) time.Time {
	var best time.Time
	for _, t := range times {
		if t.After(best) {
			best = t
		}
	}
	return best
}

// groupInventoryByAccount replaces the flat Hosts list with an account-keyed map.
func (o *inventoryOutput) groupByAccount() {
	o.HostsByAccount = groupByAccountField(o.Hosts, func(host inventoryHost) string {
		return host.AccountID
	})
	o.Hosts = nil
}
