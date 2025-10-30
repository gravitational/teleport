/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { keepPreviousData, useInfiniteQuery } from '@tanstack/react-query';
import { useCallback, useMemo, useRef } from 'react';
import { useHistory, useLocation } from 'react-router';
import styled, { css } from 'styled-components';

import { Alert } from 'design/Alert/Alert';
import { CardTile } from 'design/CardTile/CardTile';
import Flex from 'design/Flex/Flex';
import { SearchPanel } from 'shared/components/Search';
import { InfoGuideButton } from 'shared/components/SlidingSidePanel/InfoGuide/InfoGuide';

import { EmptyState } from 'teleport/Bots/List/EmptyState/EmptyState';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout/Layout';
import { listBotInstances } from 'teleport/services/bot/bot';
import { BotInstanceSummary } from 'teleport/services/bot/types';
import useTeleport from 'teleport/useTeleport';

import { BotInstancesDashboard } from './Dashboard/BotInstanceDashboard';
import { BotInstanceDetails } from './Details/BotInstanceDetails';
import { InfoGuide } from './InfoGuide';
import {
  BotInstancesList,
  BotInstancesListControls,
} from './List/BotInstancesList';

export function BotInstances() {
  const history = useHistory();
  const location = useLocation<{ prevPageTokens?: readonly string[] }>();
  const queryParams = new URLSearchParams(location.search);
  const query = queryParams.get('query') ?? '';
  const isAdvancedQuery = queryParams.get('is_advanced') ?? '';
  const sortField = queryParams.get('sort_field') || 'active_at_latest';
  const sortDir = queryParams.get('sort_dir') || 'DESC';
  const selectedItemId = queryParams.get('selected');
  const activeTab = queryParams.get('tab');

  const listRef = useRef<BotInstancesListControls | null>(null);

  const ctx = useTeleport();
  const flags = ctx.getFeatureFlags();
  const hasListPermission = flags.listBotInstances;

  const {
    isSuccess,
    data,
    isLoading,
    isFetchingNextPage,
    error,
    hasNextPage,
    fetchNextPage,
  } = useInfiniteQuery({
    enabled: hasListPermission,
    queryKey: [
      'bot_instances',
      'list',
      sortField,
      sortDir,
      query,
      isAdvancedQuery,
    ],
    queryFn: ({ pageParam, signal }) =>
      listBotInstances(
        {
          pageSize: 32,
          pageToken: pageParam,
          sortField,
          sortDir,
          searchTerm: isAdvancedQuery ? undefined : query,
          query: isAdvancedQuery ? query : undefined,
        },
        signal
      ),
    initialPageParam: '',
    getNextPageParam: data => data?.next_page_token,
    placeholderData: keepPreviousData,
    staleTime: 30_000, // Cached pages are valid for 30 seconds
  });

  const handleQueryChange = useCallback(
    (query: string, isAdvanced: boolean) => {
      const search = new URLSearchParams(location.search);
      if (query) {
        search.set('query', query);
      } else {
        search.delete('query');
      }
      if (isAdvanced) {
        search.set('is_advanced', '1');
      } else {
        search.delete('is_advanced');
      }

      history.push({
        pathname: `${location.pathname}`,
        search: search.toString(),
      });

      listRef.current?.scrollToTop();
    },
    [history, location.pathname, location.search]
  );

  const handleSortChanged = useCallback(
    (sortField: string, sortDir: string) => {
      const search = new URLSearchParams(location.search);
      search.set('sort_field', sortField);
      search.set('sort_dir', sortDir);

      history.replace({
        pathname: location.pathname,
        search: search.toString(),
      });

      listRef.current?.scrollToTop();
    },
    [history, location.pathname, location.search]
  );

  const handleItemSelected = useCallback(
    (item: BotInstanceSummary | null) => {
      const search = new URLSearchParams(location.search);
      if (item) {
        search.set('selected', `${item.bot_name}/${item.instance_id}`);
      } else {
        search.delete('selected');
        search.delete('tab');
      }

      history.push({
        pathname: location.pathname,
        search: search.toString(),
      });
    },
    [history, location.pathname, location.search]
  );

  const handleDetailsTabSelected = useCallback(
    (tab: string) => {
      const search = new URLSearchParams(location.search);

      search.set('tab', tab);

      history.push({
        pathname: location.pathname,
        search: search.toString(),
      });
    },
    [history, location.pathname, location.search]
  );

  const [selectedBotName, selectedInstanceId] =
    selectedItemId?.split('/') ?? [];

  const flatData = useMemo(
    () => (isSuccess ? data.pages.flatMap(page => page.bot_instances) : null),
    [data?.pages, isSuccess]
  );

  const handleFilterSelected = useCallback(
    (filter: string) => {
      handleQueryChange(filter, true);
    },
    [handleQueryChange]
  );

  if (!hasListPermission) {
    return (
      <FeatureBox>
        <Alert kind="info" mt={4}>
          You do not have permission to access Bot instances. Missing role
          permissions: <code>bot_instance.list</code>
        </Alert>
        <EmptyState />
      </FeatureBox>
    );
  }

  return (
    <FeatureBox hideBottomSpacing>
      <FeatureHeader justifyContent="space-between" mb={0}>
        <FeatureHeaderTitle>Bot Instances</FeatureHeaderTitle>
        <InfoGuideButton config={{ guide: <InfoGuide /> }} />
      </FeatureHeader>

      <Container>
        <SearchPanel
          filter={{
            query: isAdvancedQuery ? query : undefined,
            search: isAdvancedQuery ? undefined : query,
          }}
          updateSearch={query => handleQueryChange(query, false)}
          updateQuery={query => handleQueryChange(query, true)}
          mb={2}
        />
        <ContentContainer>
          <ListAndDetailsContainer $listOnlyMode={!selectedItemId}>
            <BotInstancesList
              ref={listRef}
              data={flatData}
              isLoading={isLoading}
              isFetchingNextPage={isFetchingNextPage}
              error={error}
              hasNextPage={hasNextPage}
              sortField={sortField}
              sortDir={sortDir === 'DESC' ? 'DESC' : 'ASC'}
              onSortChanged={handleSortChanged}
              onLoadNextPage={fetchNextPage}
              selectedItem={selectedItemId}
              onItemSelected={handleItemSelected}
              isFiltering={!!query}
            />
            {selectedItemId ? (
              <BotInstanceDetails
                key={selectedItemId}
                botName={selectedBotName}
                instanceId={selectedInstanceId}
                onClose={() => handleItemSelected(null)}
                activeTab={activeTab}
                onTabSelected={tab => handleDetailsTabSelected(tab)}
              />
            ) : undefined}
          </ListAndDetailsContainer>
          {!selectedItemId ? (
            <BotInstancesDashboard onFilterSelected={handleFilterSelected} />
          ) : undefined}
        </ContentContainer>
      </Container>
    </FeatureBox>
  );
}

const Container = styled(Flex)`
  flex-direction: column;
  flex: 1;
  overflow: auto;
  padding-bottom: ${props => props.theme.space[3]}px;
`;

const ContentContainer = styled(Flex)`
  flex: 1;
  overflow: auto;
  gap: ${props => props.theme.space[2]}px;
`;

const ListAndDetailsContainer = styled(CardTile)<{ $listOnlyMode: boolean }>`
  flex-direction: row;
  overflow: auto;
  padding: 0;
  gap: 0;
  margin: ${props => props.theme.space[1]}px;

  ${p =>
    p.$listOnlyMode
      ? css`
          min-width: 300px;
          max-width: 400px;
        `
      : ''}
`;
