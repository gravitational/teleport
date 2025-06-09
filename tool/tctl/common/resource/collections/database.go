package collections

import (
	"fmt"
	"io"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	dbobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/label"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/tool/common"
	"github.com/gravitational/teleport/tool/tctl/common/databaseobject"
	"github.com/gravitational/teleport/tool/tctl/common/databaseobjectimportrule"
)

func NewDatabaseServerCollection(servers []types.DatabaseServer) ResourceCollection {
	return &databaseServerCollection{servers: servers}

}

type databaseServerCollection struct {
	servers []types.DatabaseServer
}

func (c *databaseServerCollection) Resources() (r []types.Resource) {
	for _, resource := range c.servers {
		r = append(r, resource)
	}
	return r
}

func (c *databaseServerCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, server := range c.servers {
		labels := common.FormatLabels(server.GetDatabase().GetAllLabels(), verbose)
		rows = append(rows, []string{
			server.GetHostname(),
			common.FormatResourceName(server.GetDatabase(), verbose),
			server.GetDatabase().GetProtocol(),
			server.GetDatabase().GetURI(),
			labels,
			server.GetTeleportVersion(),
		})
	}
	headers := []string{"Host", "Name", "Protocol", "URI", "Labels", "Version"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	// stable sort by hostname then by name.
	t.SortRowsBy([]int{0, 1}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func NewDatabaseCollection(databases []types.Database) ResourceCollection {
	return &databaseCollection{databases: databases}
}

type databaseCollection struct {
	databases []types.Database
}

func (c *databaseCollection) Resources() (r []types.Resource) {
	for _, resource := range c.databases {
		r = append(r, resource)
	}
	return r
}

func (c *databaseCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, database := range c.databases {
		labels := common.FormatLabels(database.GetAllLabels(), verbose)
		rows = append(rows, []string{
			common.FormatResourceName(database, verbose),
			database.GetProtocol(),
			database.GetURI(),
			labels,
		})
	}
	headers := []string{"Name", "Protocol", "URI", "Labels"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	// stable sort by name.
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func NewDatabaseServiceCollection(databaseServices []types.DatabaseService) ResourceCollection {
	return &databaseServiceCollection{databaseServices: databaseServices}
}

type databaseServiceCollection struct {
	databaseServices []types.DatabaseService
}

func (c *databaseServiceCollection) Resources() (r []types.Resource) {
	for _, service := range c.databaseServices {
		r = append(r, service)
	}
	return r
}

func databaseResourceMatchersToString(in []*types.DatabaseResourceMatcher) string {
	resourceMatchersStrings := make([]string, 0, len(in))

	for _, resMatcher := range in {
		if resMatcher == nil || resMatcher.Labels == nil {
			continue
		}

		labelsString := make([]string, 0, len(*resMatcher.Labels))
		for key, values := range *resMatcher.Labels {
			if key == types.Wildcard {
				labelsString = append(labelsString, "<all databases>")
				continue
			}
			labelsString = append(labelsString, fmt.Sprintf("%v=%v", key, values))
		}

		resourceMatchersStrings = append(resourceMatchersStrings, fmt.Sprintf("(Labels: %s)", strings.Join(labelsString, ",")))
	}
	return strings.Join(resourceMatchersStrings, ",")
}

// writeText formats the DatabaseServices into a table and writes them into w.
// Example:
//
// Name                                 Resource Matchers
// ------------------------------------ --------------------------------------
// a6065ee9-d5ee-4555-8d47-94a78625277b (Labels: <all databases>)
// d4e13f2b-0a55-4e0a-b363-bacfb1a11294 (Labels: env=[prod],aws-tag=[xyz abc])
func (c *databaseServiceCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Resource Matchers"})

	for _, dbService := range c.databaseServices {
		t.AddRow([]string{
			dbService.GetName(), databaseResourceMatchersToString(dbService.GetResourceMatchers()),
		})
	}

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func NewDatabaseObjectImportRuleCollection(rules []*dbobjectimportrulev1.DatabaseObjectImportRule) ResourceCollection {
	return &databaseObjectImportRuleCollection{rules: rules}
}

type databaseObjectImportRuleCollection struct {
	rules []*dbobjectimportrulev1.DatabaseObjectImportRule
}

func (c *databaseObjectImportRuleCollection) Resources() []types.Resource {
	resources := make([]types.Resource, len(c.rules))
	for i, b := range c.rules {
		resources[i] = databaseobjectimportrule.ProtoToResource(b)
	}
	return resources
}

func (c *databaseObjectImportRuleCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Priority", "Mapping Count", "DB Label Count"})
	for _, b := range c.rules {
		t.AddRow([]string{
			b.GetMetadata().GetName(),
			fmt.Sprintf("%v", b.GetSpec().GetPriority()),
			fmt.Sprintf("%v", len(b.GetSpec().GetMappings())),
			fmt.Sprintf("%v", len(b.GetSpec().GetDatabaseLabels())),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type databaseObjectCollection struct {
	objects []*dbobjectv1.DatabaseObject
}

func NewDatabaseObjectCollection(objects []*dbobjectv1.DatabaseObject) ResourceCollection {
	return &databaseObjectCollection{objects: objects}
}

func (c *databaseObjectCollection) Resources() []types.Resource {
	resources := make([]types.Resource, len(c.objects))
	for i, b := range c.objects {
		resources[i] = databaseobject.ProtoToResource(b)
	}
	return resources
}

func (c *databaseObjectCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Kind", "DB Service", "Protocol"})
	for _, b := range c.objects {
		t.AddRow([]string{
			b.GetMetadata().GetName(),
			fmt.Sprintf("%v", b.GetSpec().GetObjectKind()),
			fmt.Sprintf("%v", b.GetSpec().GetDatabaseServiceName()),
			fmt.Sprintf("%v", b.GetSpec().GetProtocol()),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type healthCheckConfigCollection struct {
	items []*healthcheckconfigv1.HealthCheckConfig
}

func NewHealthCheckConfigCollection(items []*healthcheckconfigv1.HealthCheckConfig) ResourceCollection {
	return &healthCheckConfigCollection{items: items}
}

func (c *healthCheckConfigCollection) Resources() []types.Resource {
	out := make([]types.Resource, 0, len(c.items))
	for _, item := range c.items {
		out = append(out, types.ProtoResource153ToLegacy(item))
	}
	return out
}

func (c *healthCheckConfigCollection) WriteText(w io.Writer, verbose bool) error {
	headers := []string{"Name", "Interval", "Timeout", "Healthy Threshold", "Unhealthy Threshold", "DB Labels", "DB Expression"}
	var rows [][]string
	for _, item := range c.items {
		meta := item.GetMetadata()
		spec := item.GetSpec()
		rows = append(rows, []string{
			meta.GetName(),
			common.FormatDefault(spec.GetInterval().AsDuration(), defaults.HealthCheckInterval),
			common.FormatDefault(spec.GetTimeout().AsDuration(), defaults.HealthCheckTimeout),
			common.FormatDefault(spec.GetHealthyThreshold(), defaults.HealthCheckHealthyThreshold),
			common.FormatDefault(spec.GetUnhealthyThreshold(), defaults.HealthCheckUnhealthyThreshold),
			common.FormatMultiValueLabels(label.ToMap(spec.GetMatch().GetDbLabels()), verbose),
			spec.GetMatch().GetDbLabelsExpression(),
		})
	}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "DB Labels")
	}

	// stable sort by name.
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
