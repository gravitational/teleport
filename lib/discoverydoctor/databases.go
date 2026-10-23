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

// Package discoverydoctor reports availability and issues for automatically discovered resources.
package discoverydoctor

import (
	"cmp"
	"context"
	"log/slog"
	"maps"
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
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/usertasks"
)

// Database availability statuses.
const (
	// DatabaseStatusOnline indicates a Database Service heartbeat, no matching
	// RDS discovery issues, and an aggregate target health other than unhealthy.
	DatabaseStatusOnline = "Online"
	// DatabaseStatusOffline indicates a missing Database Service heartbeat,
	// a matching RDS discovery issue, or unhealthy targets on all serving agents.
	DatabaseStatusOffline = "Offline"
)

const (
	// DatabaseDetailNoService describes a missing Database Service heartbeat.
	DatabaseDetailNoService = "No Database Service is proxying this database"
	// DatabaseDetailUnhealthy describes unhealthy connections from all serving agents.
	DatabaseDetailUnhealthy = "The Database Service connection to this database is unhealthy"
)

// DatabaseClient provides registered databases and Database Service heartbeats.
type DatabaseClient interface {
	client.GetResourcesClient
	// ListDatabases lists registered databases with pagination.
	ListDatabases(ctx context.Context, limit int, start string) ([]types.Database, string, error)
}

// DatabaseDiagnostic describes a discovered database's availability and issues.
// Its JSON representation matches "tctl discovery databases --format=json".
type DatabaseDiagnostic struct {
	Cloud     string    `json:"cloud"`
	AccountID string    `json:"account_id"`
	Region    string    `json:"region"`
	Name      string    `json:"name"`
	Time      time.Time `json:"time,omitzero"`
	Status    string    `json:"status"`
	Details   []string  `json:"details,omitempty"`

	// NoService indicates that no Database Service is proxying this database.
	NoService bool `json:"-"`
	// Unhealthy indicates that all serving agents report an unhealthy target.
	Unhealthy bool `json:"-"`
	// UserTaskIssueTypes identifies RDS issues for troubleshooting guidance.
	UserTaskIssueTypes []string `json:"-"`
}

// DatabaseDiagnostics describes the availability and issues of discovered databases.
type DatabaseDiagnostics []DatabaseDiagnostic

// Recommendation describes troubleshooting guidance for a discovery issue.
type Recommendation struct {
	// Detail is the issue title.
	Detail string `json:"detail"`
	// Description explains how to resolve the issue.
	Description string `json:"description"`
}

// Recommendations returns guidance for addressing the issues in diagnostics.
// Each recommendation is included once. Service and health issues come first,
// followed by user task issues sorted by type.
func (diagnostics DatabaseDiagnostics) Recommendations() []Recommendation {
	var hasNoService, hasUnhealthy bool
	issueTypes := make(map[string]struct{})
	for _, diagnostic := range diagnostics {
		hasNoService = hasNoService || diagnostic.NoService
		hasUnhealthy = hasUnhealthy || diagnostic.Unhealthy
		for _, issueType := range diagnostic.UserTaskIssueTypes {
			issueTypes[issueType] = struct{}{}
		}
	}

	var recommendations []Recommendation
	if hasNoService {
		recommendations = append(recommendations, Recommendation{
			Detail: DatabaseDetailNoService,
			Description: `
Deploy a Teleport Database Service and ensure its db_service.resources label
selectors match the database's labels [1].

[1]: https://goteleport.com/docs/enroll-resources/database-access/troubleshooting/#no-database-service`,
		})
	}
	if hasUnhealthy {
		recommendations = append(recommendations, Recommendation{
			Detail: DatabaseDetailUnhealthy,
			Description: `
Inspect the health check failure in db_server status.target_health [1] and the
Database Service logs. Ensure the Database Service can reach the database
endpoint over the network, including VPC routes and security group rules [2].

[1]: https://goteleport.com/docs/enroll-resources/database-access/guides/health-checks/#target-health
[2]: https://goteleport.com/docs/enroll-resources/database-access/troubleshooting/#timeout-errors`,
		})
	}
	for _, issueType := range slices.Sorted(maps.Keys(issueTypes)) {
		title, description := usertasks.DescriptionForDiscoverRDSIssue(issueType)
		if issueType == usertaskstypes.AutoDiscoverRDSIssueIAMAuthenticationDisabled {
			description = `The Teleport Database Service uses IAM authentication to communicate with RDS.

These RDS databases do not have IAM authentication enabled.

You can enable IAM authentication by modifying the IAM DB Authentication property of the database [1].

[1]: https://goteleport.com/docs/enroll-resources/database-access/troubleshooting/#iam-authentication-disabled`
		} else {
			description = cmp.Or(description, "No troubleshooting guidance for this issue is available in this version of Teleport.") + `

See Database Access troubleshooting [1].

[1]: https://goteleport.com/docs/enroll-resources/database-access/troubleshooting/`
		}
		recommendations = append(recommendations, Recommendation{
			Detail:      cmp.Or(title, issueType),
			Description: description,
		})
	}
	return recommendations
}

// DiagnoseDatabases correlates automatically discovered AWS databases with
// Database Service heartbeats and open RDS discovery issue tasks.
// If user tasks are not accessible, only heartbeat and target health data is used.
// If failuresOnly is true, only offline databases are returned.
// An empty result is a non-nil slice so that it encodes as a JSON array.
func DiagnoseDatabases(ctx context.Context, clt DatabaseClient, userTasks services.UserTasks, failuresOnly bool) (DatabaseDiagnostics, error) {
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
		return DatabaseDiagnostics{}, nil
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
		return userTasks.ListUserTasks(ctx, int64(limit), token, usertasksv1.ListUserTasksFilters_builder{
			TaskState: usertaskstypes.TaskStateOpen,
		}.Build())
	}))
	if err != nil {
		if !trace.IsAccessDenied(err) {
			return nil, trace.Wrap(err, "fetching database user tasks")
		}
		slog.DebugContext(ctx, "Skipping database discovery issues because user tasks are not accessible")
		tasks = nil
	}
	slog.DebugContext(ctx, "Fetched open user tasks", "count", len(tasks))
	issues := collectRDSIssues(tasks)

	diagnostics := make(DatabaseDiagnostics, 0, len(databases))
	for _, database := range databases {
		diagnostic := buildDatabaseDiagnostic(database, serversByDatabase[database.GetName()], issues)
		if failuresOnly && diagnostic.Status == DatabaseStatusOnline {
			continue
		}
		diagnostics = append(diagnostics, diagnostic)
	}
	slices.SortFunc(diagnostics, func(a, b DatabaseDiagnostic) int {
		return cmp.Or(
			cmp.Compare(a.AccountID, b.AccountID),
			cmp.Compare(a.Region, b.Region),
			cmp.Compare(a.Name, b.Name),
			// Descending time: newer entries first.
			b.Time.Compare(a.Time),
		)
	})
	return diagnostics, nil
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

func buildDatabaseDiagnostic(database types.Database, servers []types.DatabaseServer, issues map[rdsDatabaseID]rdsDatabaseIssues) DatabaseDiagnostic {
	aws := database.GetAWS()
	diagnostic := DatabaseDiagnostic{
		Cloud:     types.CloudAWS,
		AccountID: aws.AccountID,
		Region:    aws.Region,
		Name:      database.GetName(),
		Status:    DatabaseStatusOnline,
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
			diagnostic.Status = DatabaseStatusOffline
			diagnostic.Time = databaseIssues.latestSync
			for _, issueType := range slices.Sorted(slices.Values(databaseIssues.issueTypes)) {
				diagnostic.Details = append(diagnostic.Details, rdsIssueDetail(issueType))
				diagnostic.UserTaskIssueTypes = append(diagnostic.UserTaskIssueTypes, issueType)
			}
		}
	}
	if len(servers) == 0 {
		diagnostic.Status = DatabaseStatusOffline
		diagnostic.Details = append(diagnostic.Details, DatabaseDetailNoService)
		diagnostic.NoService = true
	} else {
		statuses := make([]types.TargetHealthStatus, 0, len(servers))
		for _, server := range servers {
			statuses = append(statuses, server.GetTargetHealthStatus())
			health := server.GetTargetHealth()
			if transitionTime := health.GetTransitionTimestamp(); transitionTime.After(diagnostic.Time) {
				diagnostic.Time = transitionTime
			}
		}
		if types.AggregateHealthStatus(slices.Values(statuses)) == types.TargetHealthStatusUnhealthy {
			diagnostic.Status = DatabaseStatusOffline
			diagnostic.Unhealthy = true
			diagnostic.Details = append(diagnostic.Details, DatabaseDetailUnhealthy)
		}
	}
	return diagnostic
}

func rdsIssueDetail(issueType string) string {
	title, _ := usertasks.DescriptionForDiscoverRDSIssue(issueType)
	return cmp.Or(title, issueType)
}
