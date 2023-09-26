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

import React, { useEffect, useState } from 'react';

import styled from 'styled-components';
import { Box, Flex, ButtonLink, ButtonSecondary, Text } from 'design';
import { Magnifier } from 'design/Icon';

import { Danger } from 'design/Alert';

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

export function Resources() {
  const { isLeafCluster } = useStickyClusterId();
  const enabled = localStorage.areUnifiedResourcesEnabled();
  const teleCtx = useTeleport();
  const canCreate = teleCtx.storeUser.getTokenAccess().create;
  const { clusterId } = useStickyClusterId();

  const { params, setParams, replaceHistory, pathname, setSort, onLabelClick } =
    useUrlFiltering({
      fieldName: 'name',
      dir: 'ASC',
    });

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
      <SearchPanel
        params={params}
        setParams={setParams}
        pathname={pathname}
        replaceHistory={replaceHistory}
      />
      <FilterPanel
        params={params}
        setParams={setParams}
        setSort={setSort}
        pathname={pathname}
        replaceHistory={replaceHistory}
      />
      <ResourcesContainer className="ResourcesContainer" gap={2}>
        {resources.map(res => (
          <ResourceCard
            key={resourceKey(res)}
            resource={res}
            onLabelClick={onLabelClick}
          />
        ))}
        {/* Using index as key here is ok because these elements never change order */}
        {attempt.status === 'processing' &&
          loadingCardArray.map((_, i) => <LoadingCard delay="short" key={i} />)}
      </ResourcesContainer>
      <div ref={setScrollDetector} />
      <ListFooter>
        {attempt.status === 'failed' && resources.length > 0 && (
          <ButtonSecondary onClick={onRetryClicked}>Load more</ButtonSecondary>
        )}
        {noResults && isSearchEmpty && (
          <Empty
            clusterId={clusterId}
            canCreate={canCreate && !isLeafCluster}
            emptyStateInfo={emptyStateInfo}
          />
        )}
        {noResults && !isSearchEmpty && (
          <NoResults query={params?.query || params?.search} />
        )}
      </ListFooter>
    </FeatureBox>
  );
}

export function resourceKey(resource: UnifiedResource) {
  if (resource.kind === 'node') {
    return `${resource.hostname}/node`;
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

function NoResults({ query }: { query: string }) {
  // Prevent `No resources were found for ""` flicker.
  if (query) {
    return (
      <Box p={8} mt={3} mx="auto" maxWidth="720px" textAlign="center">
        <TextIcon typography="h3">
          <Magnifier />
          No resources were found for&nbsp;
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
