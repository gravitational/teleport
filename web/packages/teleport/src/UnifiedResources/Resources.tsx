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

import React, { useCallback, useEffect, useState } from 'react';

import styled from 'styled-components';
import {
  Box,
  Flex,
  ButtonLink,
  ButtonSecondary,
  Text,
  ButtonBorder,
  Popover,
} from 'design';
import { Magnifier, PushPin } from 'design/Icon';

import { Danger } from 'design/Alert';
import { makeSuccessAttempt, useAsync } from 'shared/hooks/useAsync';

import { TextIcon } from 'teleport/Discover/Shared';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import Empty, { EmptyStateInfo } from 'teleport/components/Empty';
import useTeleport from 'teleport/useTeleport';
import cfg from 'teleport/config';
import history from 'teleport/services/history/history';
import localStorage from 'teleport/services/localStorage';
import useStickyClusterId from 'teleport/useStickyClusterId';
import AgentButtonAdd from 'teleport/components/AgentButtonAdd';
import { SearchResource } from 'teleport/Discover/SelectResource';
import { useUrlFiltering, useInfiniteScroll } from 'teleport/components/hooks';
import { UnifiedResource } from 'teleport/services/agents';
import { useUser } from 'teleport/User/UserContext';
import { encodeUrlQueryParams } from 'teleport/components/hooks/useUrlFiltering';
import { UnifiedTabPreference } from 'teleport/services/userPreferences/types';

import { ResourceTab } from './ResourceTab';
import { ResourceCard, LoadingCard } from './ResourceCard';
import SearchPanel from './SearchPanel';
import { FilterPanel } from './FilterPanel';
import './unifiedStyles.css';

const RESOURCES_MAX_WIDTH = '1800px';
// get 48 resources to start
const INITIAL_FETCH_SIZE = 48;
// increment by 24 every fetch
const FETCH_MORE_SIZE = 24;

const loadingCardArray = new Array(FETCH_MORE_SIZE).fill(undefined);

export const PINNING_NOT_SUPPORTED_MESSAGE =
  'This cluster does not support pinning resources. To enabled, upgrade to 14.1 or newer.';

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

export function Resources() {
  const { isLeafCluster, clusterId } = useStickyClusterId();
  const enabled = localStorage.areUnifiedResourcesEnabled();
  const pinningNotSupported = localStorage.arePinnedResourcesDisabled();
  const {
    getClusterPinnedResources,
    preferences,
    updatePreferences,
    updateClusterPinnedResources,
  } = useUser();
  const teleCtx = useTeleport();
  const canCreate = teleCtx.storeUser.getTokenAccess().create;
  const [selectedResources, setSelectedResources] = useState<string[]>([]);

  const [getPinnedResourcesAttempt, getPinnedResources, setPinnedResources] =
    useAsync(
      useCallback(
        () => getClusterPinnedResources(clusterId),
        [clusterId, getClusterPinnedResources]
      )
    );

  useEffect(() => {
    getPinnedResources();
  }, [clusterId, getPinnedResources]);

  const pinnedResources = getPinnedResourcesAttempt.data || [];

  const [updatePinnedResourcesAttempt, updatePinnedResources] = useAsync(
    useCallback(
      (newPinnedResources: string[]) => {
        setPinnedResources(makeSuccessAttempt(newPinnedResources));
        return updateClusterPinnedResources(clusterId, newPinnedResources);
      },
      [clusterId, updateClusterPinnedResources]
    )
  );

  const { params, setParams, replaceHistory, pathname, setSort, onLabelClick } =
    useUrlFiltering({
      sort: {
        fieldName: 'name',
        dir: 'ASC',
      },
      pinnedOnly:
        preferences.unifiedResourcePreferences.defaultTab ===
        UnifiedTabPreference.Pinned,
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
    if (selectedResources.includes(resourceId)) {
      setSelectedResources(selectedResources.filter(i => i !== resourceId));
      return;
    }
    setSelectedResources([...selectedResources, resourceId]);
  };

  useEffect(() => {
    setSelectedResources([]);
  }, [clusterId]);

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

  const {
    setTrigger: setScrollDetector,
    forceFetch,
    resources,
    attempt,
  } = useInfiniteScroll({
    fetchFunc: teleCtx.resourceService.fetchUnifiedResources,
    clusterId,
    filter: params,
    initialFetchSize: INITIAL_FETCH_SIZE,
    fetchMoreSize: FETCH_MORE_SIZE,
  });

  const noResults = attempt.status === 'success' && resources.length === 0;

  const [isSearchEmpty, setIsSearchEmpty] = useState(true);

  // Using a useEffect for this prevents the "Add your first resource" component from being
  // shown for a split second when making a search after a search that yielded no results.
  useEffect(() => {
    setIsSearchEmpty(!params?.query && !params?.search);
  }, [params.query, params.search]);

  if (!enabled) {
    history.replace(cfg.getNodesRoute(clusterId));
  }

  const onRetryClicked = () => {
    forceFetch();
  };

  const allSelected =
    resources.length > 0 &&
    resources.every(resource =>
      selectedResources.includes(resourceKey(resource))
    );

  const toggleSelectVisible = () => {
    if (allSelected) {
      setSelectedResources([]);
      return;
    }
    setSelectedResources(resources.map(resource => resourceKey(resource)));
  };

  const selectTab = (value: UnifiedTabPreference) => {
    const pinnedOnly = value === UnifiedTabPreference.Pinned;
    setParams({
      ...params,
      pinnedOnly,
    });
    setSelectedResources([]);
    updatePreferences({ unifiedResourcePreferences: { defaultTab: value } });
    replaceHistory(
      encodeUrlQueryParams(
        pathname,
        params.search,
        params.sort,
        params.kinds,
        !!params.query /* isAdvancedSearch */,
        pinnedOnly
      )
    );
  };

  const $pinAllButton = (
    <ButtonBorder
      onClick={() => handlePinSelected(shouldUnpin)}
      textTransform="none"
      disabled={pinningNotSupported}
      css={`
        border: none;
        color: ${props => props.theme.colors.brand};
      `}
    >
      <PushPin color="brand" size={16} mr={2} />
      {shouldUnpin ? 'Unpin ' : 'Pin '}
      Selected
    </ButtonBorder>
  );

  return (
    <FeatureBox
      className="ContainerContext"
      px={4}
      css={`
        max-width: ${RESOURCES_MAX_WIDTH};
        margin: auto;
      `}
    >
      {attempt.status === 'failed' && (
        <ErrorBox>
          <ErrorBoxInternal>
            <Danger>
              {attempt.statusText}
              <Box flex="0 0 auto" ml={2}>
                <ButtonLink onClick={onRetryClicked}>Retry</ButtonLink>
              </Box>
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
      <FeatureHeader
        css={`
          border-bottom: none;
        `}
        mb={1}
        alignItems="center"
        justifyContent="space-between"
      >
        <FeatureHeaderTitle>Resources</FeatureHeaderTitle>
        <Flex alignItems="center">
          <AgentButtonAdd
            agent={SearchResource.UNIFIED_RESOURCE}
            beginsWithVowel={false}
            isLeafCluster={isLeafCluster}
            canCreate={canCreate}
          />
        </Flex>
      </FeatureHeader>
      <Flex alignItems="center" justifyContent="space-between">
        <SearchPanel
          params={params}
          setParams={setParams}
          pathname={pathname}
          replaceHistory={replaceHistory}
        />
        {selectedResources.length > 0 &&
          (pinningNotSupported ? (
            <HoverTooltip tipContent={<>{PINNING_NOT_SUPPORTED_MESSAGE}</>}>
              {$pinAllButton}
            </HoverTooltip>
          ) : (
            $pinAllButton
          ))}
      </Flex>
      <FilterPanel
        params={params}
        setParams={setParams}
        setSort={setSort}
        pathname={pathname}
        replaceHistory={replaceHistory}
        selectVisible={toggleSelectVisible}
        selected={allSelected}
        shouldUnpin={shouldUnpin}
      />
      <Flex gap={4} mb={3}>
        {tabs.map(tab => (
          <ResourceTab
            key={tab.value}
            onClick={() => selectTab(tab.value)}
            title={tab.label}
            isSelected={
              params.pinnedOnly
                ? tab.value === UnifiedTabPreference.Pinned
                : tab.value === UnifiedTabPreference.All
            }
          />
        ))}
      </Flex>
      <ResourcesContainer className="ResourcesContainer" gap={2}>
        {getPinnedResourcesAttempt.status !== 'processing' &&
          resources.map(res => {
            const key = resourceKey(res);
            return (
              <ResourceCard
                key={key}
                resource={res}
                onLabelClick={onLabelClick}
                pinResource={handlePinResource}
                pinned={pinnedResources.includes(key)}
                pinningNotSupported={pinningNotSupported}
                selected={selectedResources.includes(key)}
                selectResource={handleSelectResources}
              />
            );
          })}
        {/* Using index as key here is ok because these elements never change order */}
        {(attempt.status === 'processing' ||
          getPinnedResourcesAttempt.status === 'processing') &&
          loadingCardArray.map((_, i) => <LoadingCard delay="short" key={i} />)}
      </ResourcesContainer>
      <div ref={setScrollDetector} />
      <ListFooter>
        {attempt.status === 'failed' && resources.length > 0 && (
          <ButtonSecondary onClick={onRetryClicked}>Load more</ButtonSecondary>
        )}
        {noResults && isSearchEmpty && !params.pinnedOnly && (
          <Empty
            clusterId={clusterId}
            canCreate={canCreate && !isLeafCluster}
            emptyStateInfo={emptyStateInfo}
          />
        )}
        {noResults && params.pinnedOnly && isSearchEmpty && <NoPinned />}
        {noResults && !isSearchEmpty && (
          <NoResults
            isPinnedTab={params.pinnedOnly}
            query={params?.query || params?.search}
          />
        )}
      </ListFooter>
    </FeatureBox>
  );
}

export function resourceKey(resource: UnifiedResource) {
  if (resource.kind === 'node') {
    return `${resource.hostname}/${resource.id}/node`;
  }
  return `${resource.name}/${resource.kind}`;
}

export function resourceName(resource: UnifiedResource) {
  if (resource.kind === 'app' && resource.friendlyName) {
    return resource.friendlyName;
  }
  if (resource.kind === 'node') {
    return resource.hostname;
  }
  return resource.name;
}

function NoPinned() {
  return (
    <Box p={8} mt={3} mx="auto" maxWidth="720px" textAlign="center">
      <TextIcon typography="h3">You have not pinned any resources</TextIcon>
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
  // Prevent `No resources were found for ""` flicker.
  if (query) {
    return (
      <Box p={8} mt={3} mx="auto" maxWidth="720px" textAlign="center">
        <TextIcon typography="h3">
          <Magnifier />
          No {isPinnedTab ? 'pinned ' : ''}resources were found for&nbsp;
          <Text
            as="span"
            bold
            css={`
              max-width: 270px;
              overflow: hidden;
              text-overflow: ellipsis;
            `}
          >
            {query}
          </Text>
        </TextIcon>
      </Box>
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

const emptyStateInfo: EmptyStateInfo = {
  title: 'Add your first resource to Teleport',
  byline:
    'Connect SSH servers, Kubernetes clusters, Windows Desktops, Databases, Web apps and more from our integrations catalog.',
  readOnly: {
    title: 'No Resources Found',
    resource: 'resources',
  },
  resourceType: 'unified_resource',
};

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
