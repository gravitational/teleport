import { useRef } from 'react';

import { Box, Flex } from 'design';
import { Plus } from 'design/Icon';
import { LabelsViewMode } from 'gen-proto-ts/teleport/userpreferences/v1/unified_resource_preferences_pb';
import { useInfoGuide } from 'shared/components/SlidingSidePanel/InfoGuide';
import {
  ProcessedLabel,
  ResourceLabel,
  UnifiedResources as SharedUnifiedResources,
  UnifiedResourceDefinition,
  UnifiedResourcesQueryParams,
} from 'shared/components/UnifiedResources';
import {
  getResourceId,
  openStatusInfoPanel,
} from 'shared/components/UnifiedResources/shared/StatusInfo';
import { KeyBasedPagination } from 'shared/hooks/useInfiniteScroll/useKeyBasedPagination';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { URLResourceFilter } from 'teleport/components/hooks/useUrlFiltering/useUrlFiltering';
import cfg from 'teleport/config';
import { UnifiedResource } from 'teleport/services/agents';
import { StatusInfo } from 'teleport/UnifiedResources/StatusInfo';
import { ResizingResourceWrapper } from 'teleport/UnifiedResources/UnifiedResources';
import { useUser } from 'teleport/User/UserContext';

import { ListResourceAccessFields } from '../../role/listaccess';
import { EmptyList } from '../EmptyList';
import { getResourceKindName } from '../shared';
import { getPredicateExpression } from '../unifiedResource';
import { EmptyAccess } from './EmptyAccess';
import { Header } from './Header';
import {
  ResourceLabelInput,
  ResourceLabelInputHandle,
} from './ResourceLabelInput';

/**
 * Lists specified resource using the unified resource component.
 *
 * Supports label clicking and applying it as resource filter for
 * label based resources (e.g. apps, db, kube, server, windows).
 *
 * Other non label based resources just lists the resources.
 */
export function ListResourceSection({
  selectedResourceTab,
  resourceFilters,
  updateResourceFilters,
  fetchedResources,
}: {
  selectedResourceTab: ListResourceAccessFields;
  resourceFilters: URLResourceFilter;
  updateResourceFilters(filter: UnifiedResourcesQueryParams): void;
  fetchedResources: KeyBasedPagination<UnifiedResource>;
}) {
  const { guideEditor } = useAccessListManagementContext();
  const { standardRoleState } = guideEditor;
  const { preferences, updatePreferences } = useUser();

  const hasDefinedStandardAccess =
    standardRoleState.definedAccess(selectedResourceTab);

  const {
    fetch: fetchResources,
    resources,
    attempt: resourcesFetchAttempt,
  } = fetchedResources;

  const { setInfoGuideConfig } = useInfoGuide();
  const resourceLabelInputRef = useRef<ResourceLabelInputHandle>(null);

  function onShowStatusInfo(resource: UnifiedResourceDefinition) {
    openStatusInfoPanel({
      isEnterprise: cfg.edition === 'ent',
      resource,
      setInfoGuideConfig,
      guide: (
        <StatusInfo
          resource={resource}
          clusterId={cfg.proxyCluster}
          key={getResourceId(resource)}
        />
      ),
    });
  }

  /**
   * Returns true if a single label matches the user's defined
   * role conditions for the selected resource tab.
   */
  function isLabelSelected(label: ResourceLabel): boolean {
    switch (selectedResourceTab) {
      case 'app_labels':
      case 'db_labels':
      case 'kubernetes_labels':
      case 'node_labels':
      case 'windows_desktop_labels':
        let labelVal =
          standardRoleState.roleConditions[selectedResourceTab][label.name];
        if (labelVal) {
          if (!Array.isArray(labelVal)) {
            labelVal = [labelVal];
          }
          return labelVal.includes(label.value);
        }
        return false;

      case 'github_permissions':
        // git servers do not handle labels
        return false;

      default:
        selectedResourceTab satisfies never;
        return false;
    }
  }

  /**
   * Adds dynamic styling to labels when clicked upon.
   */
  function processLabel(label: ResourceLabel): ProcessedLabel {
    if (isLabelSelected(label)) {
      return { kind: 'primary' };
    }
    return { kind: 'outline-primary' };
  }

  const fetchResourceSuccess = resourcesFetchAttempt.status === 'success';

  const noResourcesFoundInCluster =
    fetchResourceSuccess && !resources.length && !hasDefinedStandardAccess;

  let header;
  let onLabelClick;
  switch (selectedResourceTab) {
    case 'app_labels':
    case 'db_labels':
    case 'kubernetes_labels':
    case 'node_labels':
    case 'windows_desktop_labels':
      const currentLabels =
        standardRoleState.roleConditions[selectedResourceTab];

      onLabelClick = (label: ResourceLabel) => {
        resourceLabelInputRef.current?.onLabelClick(label);
      };

      header = (
        <>
          <Box mt={resourcesFetchAttempt.status === 'failed' ? 4 : undefined}>
            <Header selectedResourceTab={selectedResourceTab} />
          </Box>
          <Flex gap={2} alignItems="center">
            <Box
              css={`
                position: relative;
                width: 100%;
              `}
              mt={1}
            >
              <ResourceLabelInput
                ref={resourceLabelInputRef}
                labels={currentLabels}
                onChange={labels => {
                  const updatedRoleConditions = standardRoleState.updateLabels(
                    selectedResourceTab,
                    labels
                  );
                  updateResourceFilters({
                    query: getPredicateExpression({
                      accessField: selectedResourceTab,
                      roleConditions: updatedRoleConditions,
                    }),
                  });
                }}
              />
            </Box>
          </Flex>
          {fetchResourceSuccess && !hasDefinedStandardAccess && (
            <EmptyAccess resourceField={selectedResourceTab} />
          )}
        </>
      );
      break;

    case 'github_permissions':
      // git servers access definition is handled by GitServerSection.tsx
      // but uses this component to simply list git servers.
      break;

    default:
      selectedResourceTab satisfies never;
  }

  if (noResourcesFoundInCluster) {
    return <EmptyList resourceField={selectedResourceTab} />;
  }

  return (
    <ResizingResourceWrapper data-testid="unified-resource-table">
      <SharedUnifiedResources
        css={`
          width: 100%;
          max-width: 1800px;
          min-width: 450px;
        `}
        noResultCustomText={
          selectedResourceTab === 'github_permissions'
            ? 'No Git servers found'
            : `No ${getResourceKindName(selectedResourceTab).byline} matched the labels`
        }
        onShowStatusInfo={onShowStatusInfo}
        onLabelClick={onLabelClick}
        showResourceSelectedIcon={hasDefinedStandardAccess}
        visibleFilterPanelFields={{
          checkbox: false,
          clusterOpts: false,
          healthStatusOpts: false,
          resourceAvailabilityOpts: false,
          resourceTypeOpts: false,
          collapseLabelBtn: false,
        }}
        visibleResourceItemFields={{
          checkbox: false,
          pin: false,
          hoverState: false,
          copy: false,
        }}
        resourceLabelConfig={{
          IconLeft: Plus,
          processLabel,
        }}
        params={resourceFilters}
        fetchResources={fetchResources}
        resourcesFetchAttempt={resourcesFetchAttempt}
        unifiedResourcePreferences={{
          ...preferences.unifiedResourcePreferences,
          // Default to labels view expanded for list mode.
          labelsViewMode: LabelsViewMode.EXPANDED,
        }}
        updateUnifiedResourcesPreferences={preferences => {
          updatePreferences({ unifiedResourcePreferences: preferences });
        }}
        availableKinds={[]}
        pinning={{ kind: 'hidden' }}
        resources={resources.map(resource => {
          if (resource.kind === 'git_server') {
            return {
              resource,
              ui: {
                ActionButton: undefined,
                noLabelClick: true,
              },
            };
          }

          return {
            resource,
            ui: { ActionButton: undefined },
          };
        })}
        setParams={updateResourceFilters}
        NoResources={<EmptyList resourceField={selectedResourceTab} />}
        Header={<>{header}</>}
      />
    </ResizingResourceWrapper>
  );
}
