import { useCallback, useMemo, useRef, useState } from 'react';

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
  useUnifiedResourcesFetch,
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
import ResourceService from 'teleport/services/resources';
import { StatusInfo } from 'teleport/UnifiedResources/StatusInfo';
import { ResizingResourceWrapper } from 'teleport/UnifiedResources/UnifiedResources';
import { useUser } from 'teleport/User/UserContext';

import { defaultStandardRoleConditions } from '../../role/conditions';
import { ListResourceAccessFields } from '../../role/listaccess';
import { EmptyList } from '../EmptyList';
import { getResourceKindName } from '../shared';
import {
  getPredicateExpression,
  getUnifiedResourceKind,
} from '../unifiedResource';
import { EmptyAccess } from './EmptyAccess';
import { Header } from './Header';
import {
  ResourceLabelInput,
  ResourceLabelInputHandle,
} from './ResourceLabelInput';
import { ResourceTab, ResourceTabs } from './ResourceTabs';

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

  /**
   * Default to the "matched" tab when access is already defined
   * so the rendered list shows only matched labels.
   */
  const [activeTab, setActiveTab] = useState(
    hasDefinedStandardAccess
      ? ResourceTab.MatchedResources
      : ResourceTab.AllResources
  );

  const {
    fetch: fetchResources,
    resources,
    attempt: resourcesFetchAttempt,
  } = fetchedResources;

  /**
   * Keeps a separate pagination state/cache for the "all resources" tab. The
   * "matched" resources fetch (defined outside of this component) is tied to the
   * user's label query; reusing it here would require clearing/refetching
   * whenever the user switches tabs, or risk mixing pages from filtered and
   * unfiltered queries.
   */
  const baseQuery = useMemo(
    () =>
      getPredicateExpression({
        accessField: selectedResourceTab,
        roleConditions: defaultStandardRoleConditions(),
      }),
    [selectedResourceTab]
  );
  const baseKind = useMemo(
    () => getUnifiedResourceKind(selectedResourceTab),
    [selectedResourceTab]
  );
  const baseFetch = useUnifiedResourcesFetch({
    fetchFunc: useCallback(
      async (paginationParams, signal) => {
        const resourceService = new ResourceService();
        return resourceService.fetchUnifiedResources(
          cfg.proxyCluster,
          {
            ...resourceFilters,
            query: baseQuery,
            sort: resourceFilters.sort ?? { fieldName: 'name', dir: 'ASC' },
            kinds: [baseKind],
            limit: paginationParams.limit,
            startKey: paginationParams.startKey,
          },
          signal
        );
      },
      [baseQuery, baseKind, resourceFilters]
    ),
  });

  // Reset the all-kind pagination state when filters change.
  const [prevFilters, setPrevFilters] = useState(resourceFilters);
  if (prevFilters !== resourceFilters) {
    setPrevFilters(resourceFilters);
    baseFetch.clear();
  }

  const isGitServer = selectedResourceTab === 'github_permissions';
  const isAllResourcesTab = activeTab === ResourceTab.AllResources;

  /**
   * Returns the resources, fetch function, and attempt for the
   * currently active tab. The "all resources" tab uses an unfiltered
   * fetch, while the "matched" tab (and git servers) uses the
   * label-filtered fetch passed in by props.
   */
  function getActiveFetchedResources() {
    if (!isGitServer && isAllResourcesTab) {
      return {
        activeResources: baseFetch.resources,
        activeFetch: baseFetch.fetch,
        activeAttempt: baseFetch.attempt,
      };
    }
    return {
      activeResources: resources,
      activeFetch: fetchResources,
      activeAttempt: resourcesFetchAttempt,
    };
  }

  const { activeResources, activeFetch, activeAttempt } =
    getActiveFetchedResources();

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

  function showResourceSelectedIcon() {
    if (isGitServer) {
      return true;
    }
    if (!hasDefinedStandardAccess || resources.length === 0) {
      return false;
    }
    if (activeTab === ResourceTab.MatchedResources) {
      return true;
    }

    // Show selected icon only for resources whose labels match
    // the user defined label condition.
    const definedLabels = standardRoleState.roleConditions[selectedResourceTab];
    return (resourceLabels: ResourceLabel[]) =>
      Object.entries(definedLabels).every(([key, vals]) => {
        if (!Array.isArray(vals)) {
          vals = [vals];
        }
        return resourceLabels.some(
          label => label.name === key && vals.includes(label.value)
        );
      });
  }

  const noResourcesFoundInCluster =
    activeAttempt.status === 'success' &&
    !activeResources.length &&
    !hasDefinedStandardAccess;

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
        setActiveTab(ResourceTab.MatchedResources);
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
                  const hasLabels = Object.keys(labels).length > 0;
                  setActiveTab(
                    hasLabels
                      ? ResourceTab.MatchedResources
                      : ResourceTab.AllResources
                  );
                }}
              />
            </Box>
          </Flex>
          {activeAttempt.status === 'success' && !hasDefinedStandardAccess && (
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
        showResourceSelectedIcon={showResourceSelectedIcon()}
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
        fetchResources={activeFetch}
        resourcesFetchAttempt={activeAttempt}
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
        resources={activeResources.map(resource => {
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
        FilterPanelLeftContent={
          isGitServer ? undefined : (
            <ResourceTabs
              activeTab={activeTab}
              setActiveTab={setActiveTab}
              hasDefinedStandardAccess={hasDefinedStandardAccess}
              resourceField={selectedResourceTab}
            />
          )
        }
      />
    </ResizingResourceWrapper>
  );
}
