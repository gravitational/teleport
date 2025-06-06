package collections

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"

	"github.com/gravitational/trace"

	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
)

func NewAutoUpdateConfigCollection(config *autoupdatev1pb.AutoUpdateConfig) ResourceCollection {
	return &autoUpdateConfigCollection{config: config}
}

type autoUpdateConfigCollection struct {
	config *autoupdatev1pb.AutoUpdateConfig
}

func (c *autoUpdateConfigCollection) Resources() []types.Resource {
	return []types.Resource{types.ProtoResource153ToLegacy(c.config)}
}

func (c *autoUpdateConfigCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Tools AutoUpdate Enabled"})
	t.AddRow([]string{
		c.config.GetMetadata().GetName(),
		fmt.Sprintf("%v", c.config.GetSpec().GetTools().GetMode()),
	})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func NewAutoUpdateVersionCollection(version *autoupdatev1pb.AutoUpdateVersion) ResourceCollection {
	return &autoUpdateVersionCollection{version: version}
}

type autoUpdateVersionCollection struct {
	version *autoupdatev1pb.AutoUpdateVersion
}

func (c *autoUpdateVersionCollection) Resources() []types.Resource {
	return []types.Resource{types.ProtoResource153ToLegacy(c.version)}
}

func (c *autoUpdateVersionCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Tools AutoUpdate Version"})
	t.AddRow([]string{
		c.version.GetMetadata().GetName(),
		fmt.Sprintf("%v", c.version.GetSpec().GetTools().TargetVersion),
	})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func NewAutoUpdateAgentRolloutCollection(rollout *autoupdatev1pb.AutoUpdateAgentRollout) ResourceCollection {
	return &autoUpdateAgentRolloutCollection{rollout: rollout}
}

type autoUpdateAgentRolloutCollection struct {
	rollout *autoupdatev1pb.AutoUpdateAgentRollout
}

func (c *autoUpdateAgentRolloutCollection) Resources() []types.Resource {
	return []types.Resource{types.ProtoResource153ToLegacy(c.rollout)}
}

func (c *autoUpdateAgentRolloutCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Start Version", "Target Version", "Mode", "Schedule", "Strategy"})
	t.AddRow([]string{
		c.rollout.GetMetadata().GetName(),
		fmt.Sprintf("%v", c.rollout.GetSpec().GetStartVersion()),
		fmt.Sprintf("%v", c.rollout.GetSpec().GetTargetVersion()),
		fmt.Sprintf("%v", c.rollout.GetSpec().GetAutoupdateMode()),
		fmt.Sprintf("%v", c.rollout.GetSpec().GetSchedule()),
		fmt.Sprintf("%v", c.rollout.GetSpec().GetStrategy()),
	})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func NewAutoUpdateAgentReportCollection(reports []*autoupdatev1pb.AutoUpdateAgentReport) ResourceCollection {
	return &autoUpdateAgentReportCollection{reports: reports}
}

type autoUpdateAgentReportCollection struct {
	reports []*autoupdatev1pb.AutoUpdateAgentReport
}

func (c *autoUpdateAgentReportCollection) Resources() []types.Resource {
	resources := make([]types.Resource, len(c.reports))
	for i, report := range c.reports {
		resources[i] = types.ProtoResource153ToLegacy(report)
	}
	return resources
}

func (c *autoUpdateAgentReportCollection) WriteText(w io.Writer, verbose bool) error {
	groupSet := make(map[string]any)
	versionsSet := make(map[string]any)
	for _, report := range c.reports {
		for groupName, group := range report.GetSpec().GetGroups() {
			groupSet[groupName] = struct{}{}
			for versionName := range group.GetVersions() {
				versionsSet[versionName] = struct{}{}
			}
		}
	}

	groupNames := slices.Collect(maps.Keys(groupSet))
	versionNames := slices.Collect(maps.Keys(versionsSet))
	slices.Sort(groupNames)
	slices.Sort(versionNames)

	t := asciitable.MakeTable(append([]string{"Auth Server ID", "Agent Version"}, groupNames...))
	for _, report := range c.reports {
		for i, versionName := range versionNames {
			row := make([]string, len(groupNames)+2)
			if i == 0 {
				row[0] = report.GetMetadata().GetName()
			}
			row[1] = versionName
			for j, groupName := range groupNames {
				row[j+2] = strconv.Itoa(int(report.GetSpec().GetGroups()[groupName].GetVersions()[versionName].GetCount()))
			}
			t.AddRow(row)
		}
		t.AddRow(make([]string, len(versionNames)+2))
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
