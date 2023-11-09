/*
Copyright 2019-2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React, { useEffect, useState, useCallback } from 'react';

import styled from 'styled-components';
import {
  Box,
  Flex,
  ButtonLink,
  ButtonSecondary,
  Text,
  Popover,
  ButtonBorder,
} from 'design';
import { Icon, Magnifier, PushPin } from 'design/Icon';
import { Danger } from 'design/Alert';

import './unifiedStyles.css';

import { ResourcesResponse, ResourceLabel } from 'teleport/services/agents';
import {
  UnifiedTabPreference,
  UnifiedResourcePreferences,
} from 'teleport/services/userPreferences/types';

import {
  makeEmptyAttempt,
  makeSuccessAttempt,
  useAsync,
  Attempt as AsyncAttempt,
} from 'shared/hooks/useAsync';

import {
  useKeyBasedPagination,
  useInfiniteScroll,
} from 'shared/hooks/useInfiniteScroll';
import { Attempt } from 'shared/hooks/useAttemptNext';

import { SharedUnifiedResource, UnifiedResourcesQueryParams } from './types';
import {
  makeUnifiedResourceCardNode,
  makeUnifiedResourceCardDatabase,
  makeUnifiedResourceCardKube,
  makeUnifiedResourceCardApp,
  makeUnifiedResourceCardDesktop,
  makeUnifiedResourceCardUserGroup,
} from './cards';

import { ResourceTab } from './ResourceTab';
import { ResourceCard, LoadingCard, PinningSupport } from './ResourceCard';
import { FilterPanel } from './FilterPanel';

// get 48 resources to start
const INITIAL_FETCH_SIZE = 48;
// increment by 24 every fetch
const FETCH_MORE_SIZE = 24;

const loadingCardArray = new Array(FETCH_MORE_SIZE).fill(undefined);

export const PINNING_NOT_SUPPORTED_MESSAGE =
  'This cluster does not support pinning resources. To enable, upgrade to 14.1 or newer.';

const tabs: { label: string; value: UnifiedTabPreference }[] = [
  {
    label: 'All Resources',
    value: UnifiedTabPreference.All,
  },
  {
    label: 'Pinned Resources',
    value: UnifiedTabPreference.Pinned,
  },
];

export type UnifiedResourcesPinning =
  | {
      kind: 'supported';
      /** `getClusterPinnedResources` has to be stable, it is used in `useEffect`. */
      getClusterPinnedResources(): Promise<string[]>;
      updateClusterPinnedResources(pinned: string[]): Promise<void>;
    }
  | {
      kind: 'not-supported';
    }
  | {
      kind: 'hidden';
    };

/*
 * BulkAction describes a component that allows you to perform an action
 * on multiple selected resources
 */
type BulkAction = {
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
  action: (
    selectedResources: {
      unifiedResourceId: string;
      resource: SharedUnifiedResource['resource'];
    }[]
  ) => void;
};

interface UnifiedResourcesProps {
  params: UnifiedResourcesQueryParams;
  resourcesFetchAttempt: Attempt;
  fetchResources(options?: { force?: boolean }): Promise<void>;
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
  availableKinds: SharedUnifiedResource['resource']['kind'][];
  setParams(params: UnifiedResourcesQueryParams): void;
  onLabelClick(label: ResourceLabel): void;
  /** A list of actions that can be performed on the selected items. */
  bulkActions?: BulkAction[];
  updateUnifiedResourcesPreferences(
    preferences: UnifiedResourcePreferences
  ): void;
}

export function UnifiedResources(props: UnifiedResourcesProps) {
  const {
    params,
    setParams,
    resourcesFetchAttempt,
    resources,
    fetchResources,
    onLabelClick,
    availableKinds,
    pinning,
    updateUnifiedResourcesPreferences,
    bulkActions = [],
  } = props;

  const { setTrigger } = useInfiniteScroll({
    fetch: fetchResources,
  });

  const [selectedResources, setSelectedResources] = useState<string[]>([]);

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

  const handleSelectResources = (resourceId: string) => {
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

  const selectTab = (value: UnifiedTabPreference) => {
    const pinnedOnly = value === UnifiedTabPreference.Pinned;
    setParams({
      ...params,
      pinnedOnly,
    });
    setSelectedResources([]);
    setUpdatePinnedResources(makeEmptyAttempt());
    updateUnifiedResourcesPreferences({ defaultTab: value });
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
        tooltip:
          pinning.kind !== 'not-supported'
            ? PINNING_NOT_SUPPORTED_MESSAGE
            : null,
        disabled:
          pinning.kind === 'not-supported' ||
          updatePinnedResourcesAttempt.status === 'processing',
        action: () => handlePinSelected(shouldUnpin),
      },
    ];
  };

  return (
    <div
      className="ContainerContext"
      css={`
        width: 100%;
        max-width: 1800px;
        margin: 0 auto;
      `}
    >
      {resourcesFetchAttempt.status === 'failed' && (
        <ErrorBox>
          {/* If pinning is hidden, we hide the different tabs to select a view (All resources, pinning).
              This causes this error box to cover the search bar. If pinning isn't supported, we push down the
              error by 60px to not hide the search bar.
          */}
          <ErrorBoxInternal
            topPadding={pinning.kind === 'hidden' ? '60px' : '0px'}
          >
            <Danger>
              {resourcesFetchAttempt.statusText}
              {/* we don't want them to try another request with BAD REQUEST, it will just fail again. */}
              {resourcesFetchAttempt.statusCode !== 400 && (
                <Box flex="0 0 auto" ml={2}>
                  <ButtonLink onClick={onRetryClicked}>Retry</ButtonLink>
                </Box>
              )}
            </Danger>
          </ErrorBoxInternal>
        </ErrorBox>
      )}
      {getPinnedResourcesAttempt.status === 'error' && (
        <ErrorBox>
          <Danger>{getPinnedResourcesAttempt.statusText}</Danger>
        </ErrorBox>
      )}
      {updatePinnedResourcesAttempt.status === 'error' && (
        <ErrorBox>
          <Danger>{updatePinnedResourcesAttempt.statusText}</Danger>
        </ErrorBox>
      )}
      {props.Header}
      <FilterPanel
        params={params}
        setParams={setParams}
        availableKinds={availableKinds}
        selectVisible={toggleSelectVisible}
        selected={allSelected}
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
                        css={`
                          border: none;
                          color: ${props => props.theme.colors.brand};
                        `}
                      >
                        <Icon size="small" color="brand" mr={2} />
                        {text}
                      </ButtonBorder>
                    );
                    if (tooltip) {
                      return (
                        <HoverTooltip tipContent={<>{tooltip}</>}>
                          {$button}
                        </HoverTooltip>
                      );
                    }
                    return $button;
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
              disabled={
                tab.value === UnifiedTabPreference.Pinned &&
                pinning.kind === 'not-supported'
              }
              title={tab.label}
              isSelected={
                params.pinnedOnly
                  ? tab.value === UnifiedTabPreference.Pinned
                  : tab.value === UnifiedTabPreference.All
              }
            />
          ))}
        </Flex>
      )}
      {pinning.kind === 'not-supported' && params.pinnedOnly ? (
        <PinningNotSupported />
      ) : (
        <ResourcesContainer gap={2}>
          {resources
            .map(unifiedResource => ({
              card: mapResourceToCard(unifiedResource),
              key: generateUnifiedResourceKey(unifiedResource.resource),
            }))
            .map(({ card, key }) => (
              <ResourceCard
                key={key}
                name={card.name}
                ActionButton={card.ActionButton}
                primaryIconName={card.primaryIconName}
                onLabelClick={onLabelClick}
                SecondaryIcon={card.SecondaryIcon}
                description={card.description}
                labels={card.labels}
                pinned={pinnedResources.includes(key)}
                pinningSupport={getResourcePinningSupport(
                  pinning.kind,
                  updatePinnedResourcesAttempt
                )}
                selected={selectedResources.includes(key)}
                selectResource={() => handleSelectResources(key)}
                pinResource={() => handlePinResource(key)}
              />
            ))}
          {/* Using index as key here is ok because these elements never change order */}
          {(resourcesFetchAttempt.status === 'processing' ||
            getPinnedResourcesAttempt.status === 'processing') &&
            loadingCardArray.map((_, i) => (
              <LoadingCard delay="short" key={i} />
            ))}
        </ResourcesContainer>
      )}
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

function getResourcePinningSupport(
  pinning: UnifiedResourcesPinning['kind'],
  updatePinnedResourcesAttempt: AsyncAttempt<void>
): PinningSupport {
  if (pinning === 'not-supported') {
    return PinningSupport.NotSupported;
  }

  if (pinning === 'hidden') {
    return PinningSupport.Hidden;
  }

  if (updatePinnedResourcesAttempt.status === 'processing') {
    return PinningSupport.Disabled;
  }

  return PinningSupport.Supported;
}

export function generateUnifiedResourceKey(
  resource: SharedUnifiedResource['resource']
): string {
  if (resource.kind === 'node') {
    return `${resource.hostname}/${resource.id}/node`.toLowerCase();
  }
  return `${resource.name}/${resource.kind}`.toLowerCase();
}

function NoPinned() {
  return (
    <Box p={8} mt={3} mx="auto" textAlign="center">
      <Text typography="h3">You have not pinned any resources</Text>
    </Box>
  );
}

function PinningNotSupported() {
  return (
    <Box p={8} mt={3} mx="auto" maxWidth="720px" textAlign="center">
      <Text typography="h3">{PINNING_NOT_SUPPORTED_MESSAGE}</Text>
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
        typography="h3"
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

const ResourcesContainer = styled(Flex)`
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
`;

const ErrorBox = styled(Box)`
  position: sticky;
  top: 0;
  z-index: 1;
`;

const ErrorBoxInternal = styled(Box)`
  position: absolute;
  left: 0;
  right: 0;
  top: ${props => props.topPadding};
  margin: ${props => props.theme.space[1]}px 10% 0 10%;
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

// TODO (avatus) extract to the shared package in ToolTip
export const HoverTooltip: React.FC<{
  tipContent: React.ReactElement;
  fontSize?: number;
}> = ({ tipContent, fontSize = 10, children }) => {
  const [anchorEl, setAnchorEl] = useState();
  const open = Boolean(anchorEl);

  function handlePopoverOpen(event) {
    setAnchorEl(event.currentTarget);
  }

  function handlePopoverClose() {
    setAnchorEl(null);
  }

  return (
    <Flex
      aria-owns={open ? 'mouse-over-popover' : undefined}
      onMouseEnter={handlePopoverOpen}
      onMouseLeave={handlePopoverClose}
    >
      {children}
      <Popover
        modalCss={modalCss}
        onClose={handlePopoverClose}
        open={open}
        anchorEl={anchorEl}
        anchorOrigin={{
          vertical: 'top',
          horizontal: 'center',
        }}
        transformOrigin={{
          vertical: 'bottom',
          horizontal: 'center',
        }}
      >
        <StyledOnHover px={2} py={1} fontSize={`${fontSize}px`}>
          {tipContent}
        </StyledOnHover>
      </Popover>
    </Flex>
  );
};

const modalCss = () => `
  pointer-events: none;
`;

const StyledOnHover = styled(Text)`
  color: ${props => props.theme.colors.text.main};
  background-color: ${props => props.theme.colors.tooltip.background};
  max-width: 350px;
`;

function mapResourceToCard({ resource, ui }: SharedUnifiedResource) {
  switch (resource.kind) {
    case 'node':
      return makeUnifiedResourceCardNode(resource, ui);
    case 'db':
      return makeUnifiedResourceCardDatabase(resource, ui);
    case 'kube_cluster':
      return makeUnifiedResourceCardKube(resource, ui);
    case 'app':
      return makeUnifiedResourceCardApp(resource, ui);
    case 'windows_desktop':
      return makeUnifiedResourceCardDesktop(resource, ui);
    case 'user_group':
      return makeUnifiedResourceCardUserGroup(resource, ui);
  }
}
