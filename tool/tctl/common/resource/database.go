package resource

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
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

func (rc *ResourceCommand) getDatabaseServer(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	servers, err := client.GetDatabaseServers(ctx, rc.namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if rc.ref.Name == "" {
		return collections.NewDatabaseServerCollection(servers), nil
	}

	servers = filterByNameOrDiscoveredName(servers, rc.ref.Name)
	if len(servers) == 0 {
		return nil, trace.NotFound("database server %q not found", rc.ref.Name)
	}
	return collections.NewDatabaseServerCollection(servers), nil
}

func (rc *ResourceCommand) getDatabase(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	databases, err := client.GetDatabases(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if rc.ref.Name == "" {
		return collections.NewDatabaseCollection(databases), nil
	}
	databases = filterByNameOrDiscoveredName(databases, rc.ref.Name)
	if len(databases) == 0 {
		return nil, trace.NotFound("database %q not found", rc.ref.Name)
	}
	return collections.NewDatabaseCollection(databases), nil
}

func (rc *ResourceCommand) createDatabase(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	database, err := services.UnmarshalDatabase(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	database.SetOrigin(types.OriginDynamic)
	if err := client.CreateDatabase(ctx, database); err != nil {
		if trace.IsAlreadyExists(err) {
			if !rc.force {
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

func (rc *ResourceCommand) getDatabaseService(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	resourceName := rc.ref.Name
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
func (rc *ResourceCommand) createDatabaseObjectImportRule(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	rule, err := databaseobjectimportrule.UnmarshalJSON(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}
	if rc.IsForced() {
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
func (rc *ResourceCommand) getDatabaseObjectImportRule(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	remote := client.DatabaseObjectImportRuleClient()
	if rc.ref.Name != "" {
		rule, err := remote.GetDatabaseObjectImportRule(ctx, &dbobjectimportrulev1.GetDatabaseObjectImportRuleRequest{Name: rc.ref.Name})
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

func (rc *ResourceCommand) createDatabaseObject(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	object, err := databaseobject.UnmarshalJSON(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}
	if rc.IsForced() {
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

func (rc *ResourceCommand) getDatabaseObject(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	remote := client.DatabaseObjectsClient()
	if rc.ref.Name != "" {
		object, err := remote.GetDatabaseObject(ctx, rc.ref.Name)
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

func (rc *ResourceCommand) createHealthCheckConfig(ctx context.Context, clt *authclient.Client, raw services.UnknownResource) error {
	in, err := services.UnmarshalHealthCheckConfig(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	createFn := clt.CreateHealthCheckConfig
	if rc.IsForced() {
		createFn = clt.UpsertHealthCheckConfig
	}
	if _, err := createFn(ctx, in); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("health_check_config %q has been created\n", in.GetMetadata().GetName())
	return nil
}

func (rc *ResourceCommand) updateHealthCheckConfig(ctx context.Context, clt *authclient.Client, raw services.UnknownResource) error {
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

func (rc *ResourceCommand) deleteHealthCheckConfig(ctx context.Context, clt *authclient.Client) error {
	if err := clt.DeleteHealthCheckConfig(ctx, rc.ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("health_check_config %q has been deleted\n", rc.ref.Name)
	return nil
}

func (rc *ResourceCommand) getHealthCheckConfig(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name != "" {
		cfg, err := client.GetHealthCheckConfig(ctx, rc.ref.Name)
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
