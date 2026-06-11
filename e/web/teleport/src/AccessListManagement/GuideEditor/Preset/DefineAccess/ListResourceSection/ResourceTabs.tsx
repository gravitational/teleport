import {
  TabBorder,
  TabContainer,
  TabsContainer,
  useSlidingBottomBorderTabs,
} from 'design/Tabs';
import { HoverTooltip } from 'design/Tooltip';

import { ListResourceAccessFields } from '../../role/listaccess';
import { getResourceKindName } from '../shared';

export enum ResourceTab {
  MatchedResources = 'tab-matched-resources',
  AllResources = 'tab-all-resources',
}

export function ResourceTabs({
  activeTab,
  setActiveTab,
  hasDefinedStandardAccess,
  resourceField,
}: {
  activeTab: ResourceTab;
  setActiveTab: (tab: ResourceTab) => void;
  hasDefinedStandardAccess: boolean;
  resourceField: ListResourceAccessFields;
}) {
  const { borderRef, parentRef } = useSlidingBottomBorderTabs({ activeTab });
  const resourceName = getTabResourceName(resourceField);

  return (
    <TabsContainer ref={parentRef} size="small">
      <TabContainer
        size="small"
        data-tab-id={ResourceTab.AllResources}
        selected={activeTab === ResourceTab.AllResources}
        onClick={() => setActiveTab(ResourceTab.AllResources)}
      >
        All {resourceName}
      </TabContainer>
      <HoverTooltip
        tipContent={
          !hasDefinedStandardAccess
            ? `Define access by typing or clicking on labels to see matched ${getResourceKindName(resourceField).byline}`
            : ''
        }
      >
        <TabContainer
          size="small"
          disabled={!hasDefinedStandardAccess}
          data-tab-id={ResourceTab.MatchedResources}
          selected={activeTab === ResourceTab.MatchedResources}
          onClick={() => setActiveTab(ResourceTab.MatchedResources)}
        >
          Matched {resourceName}
        </TabContainer>
      </HoverTooltip>
      <TabBorder ref={borderRef} />
    </TabsContainer>
  );
}

function getTabResourceName(resourceField: ListResourceAccessFields) {
  switch (resourceField) {
    case 'kubernetes_labels':
      return 'Kubernetes';
    case 'windows_desktop_labels':
      return 'Desktops';
    case 'app_labels':
      return 'Applications';
    case 'db_labels':
      return 'Databases';
    case 'node_labels':
      return 'Servers';
    case 'github_permissions':
      return 'Git Servers';
    default:
      resourceField satisfies never;
  }
}
