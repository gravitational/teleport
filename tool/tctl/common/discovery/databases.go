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
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	usertaskstypes "github.com/gravitational/teleport/api/types/usertasks"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/usertasks"
)

// statuses
const (
	// databaseStatusOnline indicates a registered database with a Database
	// Service proxying it and no RDS discovery issues or health check failures.
	databaseStatusOnline = "Online"
	// databaseStatusOffline indicates a missing Database Service heartbeat,
	// an RDS discovery issue, or an unhealthy target.
	databaseStatusOffline = "Offline"
)

// details
const (
	databaseDetailNoService = "No Database Service is proxying this database"
	databaseDetailUnhealthy = "The Database Service connection to this database is unhealthy"
)

// databaseInfo holds the output columns for a discovered database.
type databaseInfo struct {
	Cloud     string    `json:"cloud"`
	AccountID string    `json:"account_id"`
	Region    string    `json:"region"`
	Name      string    `json:"name"`
	Time      time.Time `json:"time,omitzero"`
	Status    string    `json:"status"`
	Details   []string  `json:"details,omitempty"`

	noService          bool     `json:"-"`
	unhealthy          bool     `json:"-"`
	userTaskIssueTypes []string `json:"-"`
}

// buildDatabasesInfo correlates discovered AWS databases, Database Service
// heartbeats, and open RDS discovery issue tasks. Time is the latest
// discovery sync or target health transition, if available.
// Database discovery analysis does not use audit events.
func buildDatabasesInfo(ctx context.Context, clt discoveryClient, failuresOnly bool) ([]databaseInfo, error) {
	slog.DebugContext(ctx, "Fetching registered databases")
	var databases []types.Database
	for database, err := range clientutils.Resources(ctx, clt.ListDatabases) {
		if err != nil {
			return nil, trace.Wrap(err, "fetching registered databases")
		}
		if database.Origin() == types.OriginCloud && database.GetStaticLabels()[types.CloudLabel] == types.CloudAWS {
			databases = append(databases, database)
		}
	}
	slog.DebugContext(ctx, "Fetched discovered AWS databases", "count", len(databases))
	if len(databases) == 0 {
		return nil, nil
	}

	slog.DebugContext(ctx, "Fetching database servers")
	servers, err := client.GetAllResources[types.DatabaseServer](ctx, clt, &proto.ListResourcesRequest{
		ResourceType: types.KindDatabaseServer,
		Namespace:    apidefaults.Namespace,
	})
	if err != nil {
		return nil, trace.Wrap(err, "fetching database servers")
	}
	slog.DebugContext(ctx, "Fetched database servers", "count", len(servers))
	serversByDatabase := make(map[string][]types.DatabaseServer)
	for _, server := range servers {
		name := server.GetDatabase().GetName()
		serversByDatabase[name] = append(serversByDatabase[name], server)
	}

	slog.DebugContext(ctx, "Fetching open user tasks")
	tasks, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, limit int, token string) ([]*usertasksv1.UserTask, string, error) {
		return clt.UserTasksClient().ListUserTasks(ctx, int64(limit), token, usertasksv1.ListUserTasksFilters_builder{
			TaskState: usertaskstypes.TaskStateOpen,
		}.Build())
	}))
	if err != nil {
		return nil, trace.Wrap(err, "fetching database user tasks")
	}
	slog.DebugContext(ctx, "Fetched open user tasks", "count", len(tasks))
	issues := collectRDSIssues(tasks)

	infos := make([]databaseInfo, 0, len(databases))
	for _, db := range databases {
		info := buildDatabaseInfo(db, serversByDatabase[db.GetName()], issues)
		if failuresOnly && info.Status == databaseStatusOnline {
			continue
		}
		infos = append(infos, info)
	}
	slices.SortFunc(infos, func(a, b databaseInfo) int {
		return cmp.Or(
			cmp.Compare(a.AccountID, b.AccountID),
			cmp.Compare(a.Region, b.Region),
			cmp.Compare(a.Name, b.Name),
			// Descending time: newer entries first.
			b.Time.Compare(a.Time),
		)
	})
	return infos, nil
}

// rdsDatabaseID identifies an RDS instance or cluster independently of the
// Teleport resource name. Multiple Aurora endpoints can belong to one cluster.
type rdsDatabaseID struct {
	accountID string
	region    string
	name      string
	isCluster bool
}

type rdsDatabaseIssues struct {
	issueTypes []string
	latestSync time.Time
}

// collectRDSIssues records the distinct issue types and latest discovery sync
// for each database with an open RDS user task.
func collectRDSIssues(tasks []*usertasksv1.UserTask) map[rdsDatabaseID]rdsDatabaseIssues {
	issues := make(map[rdsDatabaseID]rdsDatabaseIssues)
	for _, task := range tasks {
		spec := task.GetSpec()
		if spec.GetState() != usertaskstypes.TaskStateOpen ||
			spec.GetTaskType() != usertaskstypes.TaskTypeDiscoverRDS {
			continue
		}
		rds := spec.GetDiscoverRds()
		for name, database := range rds.GetDatabases() {
			id := rdsDatabaseID{
				accountID: rds.GetAccountId(),
				region:    rds.GetRegion(),
				name:      name,
				isCluster: database.GetIsCluster(),
			}
			databaseIssues := issues[id]
			if !slices.Contains(databaseIssues.issueTypes, spec.GetIssueType()) {
				databaseIssues.issueTypes = append(databaseIssues.issueTypes, spec.GetIssueType())
			}
			if syncTime := database.GetSyncTime(); syncTime != nil && syncTime.AsTime().After(databaseIssues.latestSync) {
				databaseIssues.latestSync = syncTime.AsTime()
			}
			issues[id] = databaseIssues
		}
	}
	return issues
}

func buildDatabaseInfo(database types.Database, servers []types.DatabaseServer, issues map[rdsDatabaseID]rdsDatabaseIssues) databaseInfo {
	aws := database.GetAWS()
	info := databaseInfo{
		Cloud:     cloudAWS,
		AccountID: aws.AccountID,
		Region:    aws.Region,
		Name:      database.GetName(),
		Status:    databaseStatusOnline,
	}
	if database.IsRDS() {
		id := rdsDatabaseID{
			accountID: aws.AccountID,
			region:    aws.Region,
			name:      aws.RDS.InstanceID,
		}
		if aws.RDS.ClusterID != "" {
			id.name = aws.RDS.ClusterID
			id.isCluster = true
		}
		if databaseIssues, hasIssue := issues[id]; hasIssue {
			info.Status = databaseStatusOffline
			info.Time = databaseIssues.latestSync
			for _, issueType := range slices.Sorted(slices.Values(databaseIssues.issueTypes)) {
				info.Details = append(info.Details, rdsIssueDetail(issueType))
				info.userTaskIssueTypes = append(info.userTaskIssueTypes, issueType)
			}
		}
	}
	if len(servers) == 0 {
		info.Status = databaseStatusOffline
		info.Details = append(info.Details, databaseDetailNoService)
		info.noService = true
	} else {
		statuses := make([]types.TargetHealthStatus, 0, len(servers))
		for _, server := range servers {
			statuses = append(statuses, server.GetTargetHealthStatus())
			health := server.GetTargetHealth()
			if transitionTime := health.GetTransitionTimestamp(); transitionTime.After(info.Time) {
				info.Time = transitionTime
			}
		}
		if types.AggregateHealthStatus(slices.Values(statuses)) == types.TargetHealthStatusUnhealthy {
			info.Status = databaseStatusOffline
			info.unhealthy = true
			info.Details = append(info.Details, databaseDetailUnhealthy)
		}
	}
	return info
}

func rdsIssueDetail(issueType string) string {
	title, _ := usertasks.DescriptionForDiscoverRDSIssue(issueType)
	return cmp.Or(title, issueType)
}
