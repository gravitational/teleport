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

package resource

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	dbobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/databaseobject"
	"github.com/gravitational/teleport/tool/tctl/common/databaseobjectimportrule"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

var databaseServer = resource{
	getHandler:    getDatabaseServer,
	deleteHandler: deleteDatabaseServer,
}

func getDatabaseServer(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	servers, err := client.GetDatabaseServers(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if ref.Name == "" {
		return collections.NewDatabaseServerCollection(servers), nil
	}

	servers = filterByNameOrDiscoveredName(servers, ref.Name)
	if len(servers) == 0 {
		return nil, trace.NotFound("database server %q not found", ref.Name)
	}
	return collections.NewDatabaseServerCollection(servers), nil
}

func deleteDatabaseServer(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	servers, err := client.GetDatabaseServers(ctx, apidefaults.Namespace)
	if err != nil {
		return trace.Wrap(err)
	}
	resDesc := "database server"
	servers = filterByNameOrDiscoveredName(servers, ref.Name)
	name, err := getOneResourceNameToDelete(servers, ref, resDesc)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, s := range servers {
		err := client.DeleteDatabaseServer(ctx, apidefaults.Namespace, s.GetHostID(), name)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	fmt.Printf("%s %q has been deleted\n", resDesc, name)
	return nil
}

var database = resource{
	getHandler:    getDatabase,
	createHandler: createDatabase,
	deleteHandler: deleteDatabase,
}

func getDatabase(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	databases, err := client.GetDatabases(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if ref.Name == "" {
		return collections.NewDatabaseCollection(databases), nil
	}
	databases = filterByNameOrDiscoveredName(databases, ref.Name)
	if len(databases) == 0 {
		return nil, trace.NotFound("database %q not found", ref.Name)
	}
	return collections.NewDatabaseCollection(databases), nil
}

func createDatabase(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	database, err := services.UnmarshalDatabase(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	database.SetOrigin(types.OriginDynamic)
	if err := client.CreateDatabase(ctx, database); err != nil {
		if trace.IsAlreadyExists(err) {
			if !opts.force {
				return trace.AlreadyExists("database %q already exists", database.GetName())
			}
			if err := client.UpdateDatabase(ctx, database); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("database %q has been updated\n", database.GetName())
			return nil
		}
		return trace.Wrap(err)
	}
	fmt.Printf("database %q has been created\n", database.GetName())
	return nil
}

func deleteDatabase(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	databases, err := client.GetDatabases(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	resDesc := "database"
	databases = filterByNameOrDiscoveredName(databases, ref.Name)
	name, err := getOneResourceNameToDelete(databases, ref, resDesc)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := client.DeleteDatabase(ctx, name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("%s %q has been deleted\n", resDesc, name)
	return nil
}

var databaseService = resource{
	getHandler: getDatabaseService,
}

func getDatabaseService(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	resourceName := ref.Name
	listReq := proto.ListResourcesRequest{
		ResourceType: types.KindDatabaseService,
	}
	if resourceName != "" {
		listReq.PredicateExpression = fmt.Sprintf(`name == "%s"`, resourceName)
	}

	getResp, err := apiclient.GetResourcesWithFilters(ctx, client, listReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	databaseServices, err := types.ResourcesWithLabels(getResp).AsDatabaseServices()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(databaseServices) == 0 && resourceName != "" {
		return nil, trace.NotFound("Database Service %q not found", resourceName)
	}

	return collections.NewDatabaseServiceCollection(databaseServices), nil
}

var databaseObjectImportRule = resource{
	getHandler:    getDatabaseObjectImportRule,
	createHandler: createDatabaseObjectImportRule,
	deleteHandler: deleteDatabaseObjectImportRule,
}

func createDatabaseObjectImportRule(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	rule, err := databaseobjectimportrule.UnmarshalJSON(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}
	if opts.force {
		_, err = client.DatabaseObjectImportRuleClient().UpsertDatabaseObjectImportRule(ctx, &dbobjectimportrulev1.UpsertDatabaseObjectImportRuleRequest{
			Rule: rule,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("rule %q has been created\n", rule.GetMetadata().GetName())
		return nil
	}
	_, err = client.DatabaseObjectImportRuleClient().CreateDatabaseObjectImportRule(ctx, &dbobjectimportrulev1.CreateDatabaseObjectImportRuleRequest{
		Rule: rule,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("rule %q has been created\n", rule.GetMetadata().GetName())
	return nil
}
func getDatabaseObjectImportRule(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	remote := client.DatabaseObjectImportRuleClient()
	if ref.Name != "" {
		rule, err := remote.GetDatabaseObjectImportRule(ctx, &dbobjectimportrulev1.GetDatabaseObjectImportRuleRequest{Name: ref.Name})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewDatabaseObjectImportRuleCollection([]*dbobjectimportrulev1.DatabaseObjectImportRule{rule}), nil
	}

	req := &dbobjectimportrulev1.ListDatabaseObjectImportRulesRequest{}
	var rules []*dbobjectimportrulev1.DatabaseObjectImportRule
	for {
		resp, err := remote.ListDatabaseObjectImportRules(ctx, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		rules = append(rules, resp.Rules...)

		if resp.NextPageToken == "" {
			break
		}
		req.PageToken = resp.NextPageToken
	}
	return collections.NewDatabaseObjectImportRuleCollection(rules), nil
}

func deleteDatabaseObjectImportRule(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if _, err := client.DatabaseObjectImportRuleClient().DeleteDatabaseObjectImportRule(ctx, &dbobjectimportrulev1.DeleteDatabaseObjectImportRuleRequest{Name: ref.Name}); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Rule %q has been deleted\n", ref.Name)
	return nil
}

var databaseObject = resource{
	getHandler:    getDatabaseObject,
	createHandler: createDatabaseObject,
	deleteHandler: deleteDatabaseObject,
}

func createDatabaseObject(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	object, err := databaseobject.UnmarshalJSON(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}
	if opts.force {
		_, err = client.DatabaseObjectsClient().UpsertDatabaseObject(ctx, object)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("object %q has been created\n", object.GetMetadata().GetName())
		return nil
	}
	_, err = client.DatabaseObjectsClient().CreateDatabaseObject(ctx, object)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("object %q has been created\n", object.GetMetadata().GetName())
	return nil
}

func getDatabaseObject(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	remote := client.DatabaseObjectsClient()
	if ref.Name != "" {
		object, err := remote.GetDatabaseObject(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewDatabaseObjectCollection([]*dbobjectv1.DatabaseObject{object}), nil
	}

	token := ""
	var objects []*dbobjectv1.DatabaseObject
	for {
		resp, nextToken, err := remote.ListDatabaseObjects(ctx, 0, token)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		objects = append(objects, resp...)

		if nextToken == "" {
			break
		}
		token = nextToken
	}
	return collections.NewDatabaseObjectCollection(objects), nil
}

func deleteDatabaseObject(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DatabaseObjectsClient().DeleteDatabaseObject(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Object %q has been deleted\n", ref.Name)
	return nil
}

var healthCheckConfig = resource{
	getHandler:    getHealthCheckConfig,
	createHandler: createHealthCheckConfig,
	updateHandler: updateHealthCheckConfig,
	deleteHandler: deleteHealthCheckConfig,
}

func createHealthCheckConfig(ctx context.Context, clt *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	in, err := services.UnmarshalHealthCheckConfig(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	createFn := clt.CreateHealthCheckConfig
	if opts.force {
		createFn = clt.UpsertHealthCheckConfig
	}
	if _, err := createFn(ctx, in); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("health_check_config %q has been created\n", in.GetMetadata().GetName())
	return nil
}

func updateHealthCheckConfig(ctx context.Context, clt *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	in, err := services.UnmarshalHealthCheckConfig(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := clt.UpdateHealthCheckConfig(ctx, in); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("health_check_config %q has been updated\n", in.GetMetadata().GetName())
	return nil
}

func deleteHealthCheckConfig(ctx context.Context, clt *authclient.Client, ref services.Ref) error {
	if err := clt.DeleteHealthCheckConfig(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("health_check_config %q has been deleted\n", ref.Name)
	return nil
}

func getHealthCheckConfig(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name != "" {
		cfg, err := client.GetHealthCheckConfig(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewHealthCheckConfigCollection([]*healthcheckconfigv1.HealthCheckConfig{cfg}), nil
	}
	var items []*healthcheckconfigv1.HealthCheckConfig
	var token string
	for {
		page, nextToken, err := client.ListHealthCheckConfigs(ctx, 0, token)
		token = nextToken
		if err != nil {
			return nil, trace.Wrap(err)
		}
		items = append(items, page...)
		if token == "" {
			break
		}
	}
	return collections.NewHealthCheckConfigCollection(items), nil
}
