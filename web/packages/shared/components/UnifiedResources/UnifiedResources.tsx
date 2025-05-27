/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import React, {
  Children,
  PropsWithChildren,
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type JSX,
} from 'react';
import styled from 'styled-components';

import { Box, ButtonBorder, ButtonSecondary, Flex, Text } from 'design';
import { Danger } from 'design/Alert';
import { Icon, Magnifier, PushPin } from 'design/Icon';

import './unifiedStyles.css';

import { HoverTooltip } from 'design/Tooltip';
import {
  AvailableResourceMode,
  DefaultTab,
  LabelsViewMode,
  UnifiedResourcePreferences,
  ViewMode,
} from 'gen-proto-ts/teleport/userpreferences/v1/unified_resource_preferences_pb';
import {
  Attempt as AsyncAttempt,
  hasFinished,
  makeEmptyAttempt,
  makeSuccessAttempt,
  useAsync,
} from 'shared/hooks/useAsync';
import { Attempt } from 'shared/hooks/useAttemptNext';
import {
  useInfiniteScroll,
  useKeyBasedPagination,
} from 'shared/hooks/useInfiniteScroll';
import { makeAdvancedSearchQueryForLabel } from 'shared/utils/advancedSearchLabelQuery';

import { ResourcesResponse } from 'teleport/services/agents';

import { useInfoGuide } from '../SlidingSidePanel/InfoGuide';
import { CardsView } from './CardsView/CardsView';
import { FilterPanel } from './FilterPanel';
import { ListView } from './ListView/ListView';
import { ResourceTab } from './ResourceTab';
import { getResourceId } from './shared/StatusInfo';
import { mapResourceToViewItem } from './shared/viewItemsFactory';
import {
  IncludedResourceMode,
  PinningSupport,
  SharedUnifiedResource,
  UnifiedResourceDefinition,
  UnifiedResourcesPinning,
  UnifiedResourcesQueryParams,
} from './types';

// get 48 resources to start
const INITIAL_FETCH_SIZE = 48;
// increment by 24 every fetch
export const FETCH_MORE_SIZE = 24;

// The breakpoint at which to force the card view. This is to improve responsiveness
// since list view looks cluttered on narrow viewports.
const FORCE_CARD_VIEW_BREAKPOINT = 800;

export const PINNING_NOT_SUPPORTED_MESSAGE =
  'This cluster does not support pinning resources. To enable, upgrade to 14.1 or newer.';

const tabs: { label: string; value: DefaultTab }[] = [
  {
    label: 'All Resources',
    value: DefaultTab.ALL,
  },
  {
    label: 'Pinned Resources',
    value: DefaultTab.PINNED,
  },
];

/*
 * BulkAction describes a component that allows you to perform an action
 * on multiple selected resources
 */
export type BulkAction = {
  /*
   * key is an arbitrary name of what the bulk action is, as well
   * as the key used when mapping our action components
   */
  key: string;
  Icon: typeof Icon;
  text: string;
  disabled?: boolean;
  /*
   * a tooltip will be rendered when the action is hovered
   * over if this prop is supplied
   */
  tooltip?: string;
  action: (selectedResources: SelectedResource[]) => void;
};

export type SelectedResource = {
  unifiedResourceId: string;
  resource: SharedUnifiedResource['resource'];
};

export type FilterKind = {
  kind: SharedUnifiedResource['resource']['kind'];
  disabled: boolean;
};

export type ResourceAvailabilityFilter =
  | {
      canRequestAll: true;
      mode: IncludedResourceMode;
    }
  | {
      canRequestAll: false;
      mode: Exclude<IncludedResourceMode, 'all'>;
    };

export interface UnifiedResourcesProps {
  params: UnifiedResourcesQueryParams;
  resourcesFetchAttempt: Attempt;
  fetchResources(options?: { force?: boolean; clear?: boolean }): Promise<void>;
  resources: SharedUnifiedResource[];
  Header?: React.ReactElement;
  /**
   * Typically used to inform the user that there are no matching resources when
   * they want to list resources without filtering the list with a search query.
   * Rendered only when the resource list is empty and there's no search query.
   * */
  NoResources: React.ReactElement;
  /**
   * If pinning is supported, the functions to get and update pinned resources
   * can be passed here.
   */
  pinning: UnifiedResourcesPinning;
  availableKinds: FilterKind[];
  /*
   * ClusterDropdown is an optional prop to add a ClusterDropdown to the
   * FilterPanel component. This is useful to turn off in Connect and use on web only
   */
  ClusterDropdown?: JSX.Element;
  setParams(params: UnifiedResourcesQueryParams): void;
  /** A list of actions that can be performed on the selected items. */
  bulkActions?: BulkAction[];
  /**
   * It is an attempt for initial fetch of preferences.
   * When it is in progress, the component shows loading skeleton.
   * Used only in Connect, where we fetch user preferences
   * while the unified resources component is visible.
   */
  unifiedResourcePreferencesAttempt?: AsyncAttempt<void>;
  availabilityFilter?: ResourceAvailabilityFilter;
  unifiedResourcePreferences: UnifiedResourcePreferences;
  updateUnifiedResourcesPreferences(
    preferences: UnifiedResourcePreferences
  ): void;

  /**
   * When called, slides opens a InfoGuideSidePanel component
   * with selected resources status info.
   */
  onShowStatusInfo(resource: UnifiedResourceDefinition): void;
}

export function UnifiedResources(props: UnifiedResourcesProps) {
  const {
    params,
    setParams,
    resourcesFetchAttempt,
    resources,
    fetchResources,
    availabilityFilter,
    availableKinds,
    pinning,
    unifiedResourcePreferencesAttempt,
    updateUnifiedResourcesPreferences,
    unifiedResourcePreferences,
    ClusterDropdown,
    bulkActions = [],
    onShowStatusInfo,
  } = props;

  const containerRef = useRef<HTMLDivElement>(null);

  const { setTrigger } = useInfiniteScroll({
    fetch: fetchResources,
  });

  const { infoGuideConfig } = useInfoGuide();

  const [selectedResources, setSelectedResources] = useState<string[]>([]);
  const [forceCardView, setForceCardView] = useState(false);

  const pinnedResourcesGetter =
    pinning.kind === 'supported'
      ? pinning.getClusterPinnedResources
      : undefined;
  const [getPinnedResourcesAttempt, getPinnedResources, setPinnedResources] =
    useAsync(
      useCallback(async () => {
        if (pinnedResourcesGetter) {
          return await pinnedResourcesGetter();
        }
        return [];
      }, [pinnedResourcesGetter])
    );

  useEffect(() => {
    getPinnedResources();
  }, [getPinnedResources]);

  const pinnedResources = getPinnedResourcesAttempt.data || [];

  const [
    updatePinnedResourcesAttempt,
    updatePinnedResources,
    setUpdatePinnedResources,
  ] = useAsync(async (newPinnedResources: string[]) => {
    if (pinning.kind === 'supported') {
      await pinning.updateClusterPinnedResources(newPinnedResources);
      setPinnedResources(makeSuccessAttempt(newPinnedResources));
    }
  });

  useEffect(() => {
    const handleKeyDown = event => {
      if (event.key === 'Escape') {
        setSelectedResources([]);
      }
    };
    document.addEventListener('keydown', handleKeyDown);

    return () => {
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, []);

  const handlePinResource = (resourceId: string) => {
    if (pinnedResources.includes(resourceId)) {
      updatePinnedResources(pinnedResources.filter(i => i !== resourceId));
      return;
    }
    updatePinnedResources([...pinnedResources, resourceId]);
  };

  // if every selected resource is already pinned, the bulk action
  // should be to unpin those resources
  const shouldUnpin = selectedResources.every(resource =>
    pinnedResources.includes(resource)
  );

  const handleSelectResource = (resourceId: string) => {
    setSelectedResources(prevResources => {
      if (selectedResources.includes(resourceId)) {
        return prevResources.filter(i => i !== resourceId);
      }
      return [...prevResources, resourceId];
    });
  };

  const handlePinSelected = (unpin: boolean) => {
    let newPinned = [];
    if (unpin) {
      newPinned = pinnedResources.filter(i => !selectedResources.includes(i));
    } else {
      const combined = [...pinnedResources, ...selectedResources];
      newPinned = Array.from(new Set(combined));
    }

    updatePinnedResources(newPinned);
  };

  const noResults =
    resourcesFetchAttempt.status === 'success' && resources.length === 0;

  const [isSearchEmpty, setIsSearchEmpty] = useState(true);

  // Using a useEffect for this prevents the "Add your first resource" component from being
  // shown for a split second when making a search after a search that yielded no results.
  useEffect(() => {
    setIsSearchEmpty(!params?.query && !params?.search);
  }, [params.query, params.search]);

  const onRetryClicked = () => {
    fetchResources({ force: true });
  };

  const allSelected =
    resources.length > 0 &&
    resources.every(({ resource }) =>
      selectedResources.includes(generateUnifiedResourceKey(resource))
    );

  const toggleSelectVisible = () => {
    if (allSelected) {
      setSelectedResources([]);
      return;
    }
    setSelectedResources(
      resources.map(({ resource }) => generateUnifiedResourceKey(resource))
    );
  };

  const selectTab = (value: DefaultTab) => {
    const pinnedOnly = value === DefaultTab.PINNED;
    setParams({
      ...params,
      pinnedOnly,
    });
    setSelectedResources([]);
    setUpdatePinnedResources(makeEmptyAttempt());
    updateUnifiedResourcesPreferences({
      ...unifiedResourcePreferences,
      defaultTab: value,
    });
  };

  const selectViewMode = (viewMode: ViewMode) => {
    updateUnifiedResourcesPreferences({
      ...unifiedResourcePreferences,
      viewMode,
    });
  };

  const setLabelsViewMode = (labelsViewMode: LabelsViewMode) => {
    updateUnifiedResourcesPreferences({
      ...unifiedResourcePreferences,
      labelsViewMode,
    });
  };

  const changeAvailableResourceMode = (
    includedResourceMode: IncludedResourceMode
  ) => {
    let mode = AvailableResourceMode.UNSPECIFIED;
    switch (includedResourceMode) {
      case 'none':
        mode = AvailableResourceMode.NONE;
        break;
      case 'accessible':
        mode = AvailableResourceMode.ACCESSIBLE;
        break;
      case 'requestable':
        mode = AvailableResourceMode.REQUESTABLE;
        break;
      case 'all':
        mode = AvailableResourceMode.ALL;
        break;
      default:
        includedResourceMode satisfies never;
    }
    updateUnifiedResourcesPreferences({
      ...unifiedResourcePreferences,
      availableResourceMode: mode,
    });
    setParams({ ...params, includedResourceMode });
  };

  const getSelectedResources = () => {
    return resources
      .filter(({ resource }) =>
        selectedResources.includes(generateUnifiedResourceKey(resource))
      )
      .map(({ resource }) => ({
        resource: resource,
        unifiedResourceId: generateUnifiedResourceKey(resource),
      }));
  };

  const bulkActionsAndPinning = (): BulkAction[] => {
    if (pinning.kind === 'hidden') {
      return bulkActions;
    }

    return [
      ...bulkActions,
      {
        key: 'pin_resource',
        text: shouldUnpin ? 'Unpin Selected' : 'Pin Selected',
        Icon: PushPin,
        disabled: updatePinnedResourcesAttempt.status === 'processing',
        action: () => handlePinSelected(shouldUnpin),
      },
    ];
  };

  const expandAllLabels =
    unifiedResourcePreferences.labelsViewMode === LabelsViewMode.EXPANDED;

  useLayoutEffect(() => {
    // TODO(ravicious): Use useResizeObserver instead. Ensure that the callback passed to
    // useResizeObserver has a stable identity.
    const resizeObserver = new ResizeObserver(entries => {
      const container = entries[0];

      // In Connect, when a tab becomes active, its outermost DOM element switches from `display:
      // none` to `display: flex`. This callback is then fired with the width reported as zero.
      //
      // As such, when checking whether to force the card view or not, we should consider only
      // values other than zero.
      if (container.contentRect.width === 0) {
        return;
      }

      if (container.contentRect.width <= FORCE_CARD_VIEW_BREAKPOINT) {
        setForceCardView(true);
      } else {
        setForceCardView(false);
      }
    });

    resizeObserver.observe(containerRef.current);
    return () => {
      resizeObserver.disconnect();
    };
  }, []);

  const ViewComponent =
    unifiedResourcePreferences.viewMode === ViewMode.CARD || forceCardView
      ? CardsView
      : ListView;

  return (
    <div
      className="ContainerContext"
      css={`
        width: 100%;
        max-width: 1800px;
        margin: 0 auto;
        min-width: 450px;
      `}
      ref={containerRef}
    >
      <ErrorsContainer>
        {resourcesFetchAttempt.status === 'failed' && (
          <Danger
            mb={0}
            bg="levels.sunken"
            primaryAction={
              // We don't want them to try another request with BAD REQUEST, it will just fail again.
              resourcesFetchAttempt.statusCode !== 400 &&
              resourcesFetchAttempt.statusCode !== 403 && {
                content: 'Retry',
                onClick: onRetryClicked,
              }
            }
            details={resourcesFetchAttempt.statusText}
          >
            Could not fetch resources
          </Danger>
        )}
        {getPinnedResourcesAttempt.status === 'error' && (
          <Danger
            mb={0}
            bg="levels.sunken"
            details={getPinnedResourcesAttempt.statusText}
          >
            Could not fetch pinned resources
          </Danger>
        )}
        {updatePinnedResourcesAttempt.status === 'error' && (
          <Danger
            mb={0}
            bg="levels.sunken"
            details={updatePinnedResourcesAttempt.statusText}
          >
            Could not update pinned resources
          </Danger>
        )}
        {unifiedResourcePreferencesAttempt?.status === 'error' && (
          <Danger
            mb={0}
            bg="levels.sunken"
            details={unifiedResourcePreferencesAttempt.statusText}
          >
            Could not fetch unified view preferences
          </Danger>
        )}
      </ErrorsContainer>

      {props.Header}
      <FilterPanel
        availabilityFilter={availabilityFilter}
        changeAvailableResourceMode={changeAvailableResourceMode}
        params={params}
        setParams={setParams}
        availableKinds={availableKinds}
        selectVisible={toggleSelectVisible}
        selected={allSelected}
        currentViewMode={unifiedResourcePreferences.viewMode}
        setCurrentViewMode={selectViewMode}
        expandAllLabels={expandAllLabels}
        ClusterDropdown={ClusterDropdown}
        setExpandAllLabels={expandAllLabels => {
          setLabelsViewMode(
            expandAllLabels ? LabelsViewMode.EXPANDED : LabelsViewMode.COLLAPSED
          );
        }}
        hideViewModeOptions={forceCardView}
        onRefresh={() => fetchResources({ clear: true })}
        BulkActions={
          <>
            {selectedResources.length > 0 && (
              <>
                {bulkActionsAndPinning().map(
                  ({ key, Icon, text, action, tooltip, disabled = false }) => {
                    const $button = (
                      <ButtonBorder
                        key={key}
                        data-testid={key}
                        textTransform="none"
                        onClick={() => action(getSelectedResources())}
                        disabled={disabled}
                        size="small"
                        css={`
                          border: none;
                          color: ${props => props.theme.colors.brand};
                          height: 22px;
                          font-size: 12px;
                        `}
                      >
                        <Icon size="small" color="brand" mr={2} />
                        <Text className="BulkActionText">{text}</Text>
                      </ButtonBorder>
                    );
                    return (
                      <HoverTooltip tipContent={tooltip} key={key}>
                        {$button}
                      </HoverTooltip>
                    );
                  }
                )}
              </>
            )}
          </>
        }
      />
      {pinning.kind !== 'hidden' && (
        <Flex gap={4} mb={3}>
          {tabs.map(tab => (
            <ResourceTab
              key={tab.value}
              onClick={() => selectTab(tab.value)}
              disabled={false}
              title={tab.label}
              isSelected={
                params.pinnedOnly
                  ? tab.value === DefaultTab.PINNED
                  : tab.value === DefaultTab.ALL
              }
            />
          ))}
        </Flex>
      )}
      <ViewComponent
        onLabelClick={label =>
          setParams({
            ...params,
            search: '',
            query: makeAdvancedSearchQueryForLabel(label, params),
          })
        }
        pinnedResources={pinnedResources}
        selectedResources={selectedResources}
        onSelectResource={handleSelectResource}
        onPinResource={handlePinResource}
        pinningSupport={getResourcePinningSupport(
          pinning.kind,
          updatePinnedResourcesAttempt
        )}
        isProcessing={
          // we don't check for '' in resourcesFetchAttempt because
          // `keyBasedPagination` returns to that status on abort errors.
          resourcesFetchAttempt.status === 'processing' ||
          getPinnedResourcesAttempt.status === '' ||
          getPinnedResourcesAttempt.status === 'processing' ||
          unifiedResourcePreferencesAttempt?.status === '' ||
          unifiedResourcePreferencesAttempt?.status === 'processing'
        }
        mappedResources={
          // Hide the resources until the preferences are fetched.
          // ViewComponent supports infinite scroll, so it shows both already loaded resources
          // and a loading indicator if needed.
          !unifiedResourcePreferencesAttempt ||
          hasFinished(unifiedResourcePreferencesAttempt)
            ? resources.map(({ ui, resource }) => ({
                item: mapResourceToViewItem({
                  ui,
                  resource: {
                    ...resource,
                    // if we are in 'requestable' only mode, then all resources returned
                    // require a request and should be displayed that way
                    requiresRequest:
                      resource.requiresRequest ||
                      availabilityFilter?.mode === 'requestable',
                  },
                }),
                key: generateUnifiedResourceKey(resource),
                onShowStatusInfo: () => onShowStatusInfo(resource),
                showingStatusInfo:
                  infoGuideConfig?.id &&
                  infoGuideConfig.id === getResourceId(resource),
              }))
            : []
        }
        expandAllLabels={expandAllLabels}
      />
      <div ref={setTrigger} />
      <ListFooter>
        {resourcesFetchAttempt.status === 'failed' && resources.length > 0 && (
          <ButtonSecondary onClick={onRetryClicked}>Load more</ButtonSecondary>
        )}
        {noResults && isSearchEmpty && !params.pinnedOnly && props.NoResources}
        {noResults && params.pinnedOnly && isSearchEmpty && <NoPinned />}
        {noResults && !isSearchEmpty && (
          <NoResults
            isPinnedTab={params.pinnedOnly}
            query={params?.query || params?.search}
          />
        )}
      </ListFooter>
    </div>
  );
}

export function useUnifiedResourcesFetch<T>(props: {
  fetchFunc(
    paginationParams: { limit: number; startKey: string },
    signal: AbortSignal
  ): Promise<ResourcesResponse<T>>;
}) {
  return useKeyBasedPagination({
    fetchFunc: props.fetchFunc,
    initialFetchSize: INITIAL_FETCH_SIZE,
    fetchMoreSize: FETCH_MORE_SIZE,
  });
}

export function useResourceServersFetch<T>(props: {
  fetchFunc(
    paginationParams: { limit: number; startKey: string },
    signal: AbortSignal
  ): Promise<ResourcesResponse<T>>;
}) {
  return useKeyBasedPagination({
    fetchFunc: props.fetchFunc,
    initialFetchSize: 20,
    fetchMoreSize: 10,
  });
}

function getResourcePinningSupport(
  pinning: UnifiedResourcesPinning['kind'],
  updatePinnedResourcesAttempt: AsyncAttempt<void>
): PinningSupport {
  if (pinning === 'hidden') {
    return PinningSupport.Hidden;
  }

  if (updatePinnedResourcesAttempt.status === 'processing') {
    return PinningSupport.Disabled;
  }

  return PinningSupport.Supported;
}

function generateUnifiedResourceKey(
  resource: SharedUnifiedResource['resource']
): string {
  if (resource.kind === 'node' || resource.kind == 'git_server') {
    return `${resource.hostname}/${resource.id}/${resource.kind}`.toLowerCase();
  }
  return `${resource.name}/${resource.kind}`.toLowerCase();
}

function NoPinned() {
  return (
    <Box p={8} mt={3} mx="auto" textAlign="center">
      <Text typography="h1">You have not pinned any resources</Text>
    </Box>
  );
}

function NoResults({
  query,
  isPinnedTab,
}: {
  query: string;
  isPinnedTab: boolean;
}) {
  if (query) {
    return (
      <Text
        typography="h1"
        mt={9}
        mx="auto"
        justifyContent="center"
        alignItems="center"
        css={`
          white-space: nowrap;
        `}
        as={Flex}
      >
        <Magnifier mr={2} />
        No {isPinnedTab ? 'pinned ' : ''}resources were found for&nbsp;
        <Text
          as="span"
          bold
          css={`
            max-width: 270px;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
          `}
        >
          {query}
        </Text>
      </Text>
    );
  }

  return null;
}

function ErrorsContainer(props: PropsWithChildren<unknown>) {
  if (!Children.toArray(props.children).length) {
    return null;
  }

  return <ErrorBox>{props.children}</ErrorBox>;
}

const ErrorBox = styled(Flex)`
  position: sticky;
  flex-direction: column;
  top: ${props => props.theme.space[3]}px;
  gap: ${props => props.theme.space[1]}px;
  padding-top: ${props => props.theme.space[1]}px;
  padding-bottom: ${props => props.theme.space[3]}px;
  z-index: 1;
`;

const INDICATOR_SIZE = '48px';

// It's important to make the footer at least as big as the loading indicator,
// since in the typical case, we want to avoid UI "jumping" when loading the
// final fragment finishes, and the final fragment is just one element in the
// final row (i.e. the number of rows doesn't change). It's then important to
// keep the same amount of whitespace below the resource list.
const ListFooter = styled.div`
  margin-top: ${props => props.theme.space[2]}px;
  min-height: ${INDICATOR_SIZE};
  text-align: center;
`;

/**
 * Returns an intersection of `availableResourceMode` and `canRequestAllResources`.
 * Since the cluster admin can change the `showResources`
 * setting, we shouldn't blindly follow the user preferences.
 *
 * Instead, if the user can't see all resources, we should default to accessible ones.
 */
export function getResourceAvailabilityFilter(
  availableResourceMode: AvailableResourceMode,
  canRequestAllResources: boolean
): ResourceAvailabilityFilter {
  switch (availableResourceMode) {
    case AvailableResourceMode.NONE:
      if (!canRequestAllResources) {
        return { mode: 'accessible', canRequestAll: false };
      }
      return { mode: 'none', canRequestAll: true };
    case AvailableResourceMode.ALL:
      if (!canRequestAllResources) {
        return { mode: 'accessible', canRequestAll: false };
      }
      return { mode: 'all', canRequestAll: true };
    case AvailableResourceMode.UNSPECIFIED:
    case AvailableResourceMode.ACCESSIBLE:
      return { mode: 'accessible', canRequestAll: canRequestAllResources };
    case AvailableResourceMode.REQUESTABLE:
      return { mode: 'requestable', canRequestAll: canRequestAllResources };
    default:
      availableResourceMode satisfies never;
  }
}
