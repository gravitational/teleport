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

import {
  InfiniteData,
  keepPreviousData,
  useInfiniteQuery,
  useQueryClient,
} from '@tanstack/react-query';
import { useCallback, useMemo, useState } from 'react';
import { Link, useHistory, useLocation } from 'react-router-dom';

import { Alert, Box, Button, Indicator } from 'design';
import { HoverTooltip } from 'design/Tooltip';
import { SearchPanel } from 'shared/components/Search';
import { InfoGuideButton } from 'shared/components/SlidingSidePanel/InfoGuide';

import { BotList } from 'teleport/Bots/List/BotList';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import cfg from 'teleport/config';
import { fetchAllBotsV1, listBots } from 'teleport/services/bot/bot';
import { FlatBot, ListBotsPage } from 'teleport/services/bot/types';
import useTeleport from 'teleport/useTeleport';

import { InfoGuide } from '../InfoGuide';
import { EmptyState } from './EmptyState/EmptyState';

export function Bots() {
  const ctx = useTeleport();
  const history = useHistory();
  const location = useLocation();
  const queryClient = useQueryClient();
  const flags = ctx.getFeatureFlags();
  const hasAddBotPermissions = flags.addBots;
  const canListBots = flags.listBots;

  const search = new URLSearchParams(location.search).get('search') ?? '';
  const [selectedBot, setSelectedBot] = useState<FlatBot | null>();

  const {
    isSuccess,
    isPlaceholderData,
    isFetching,
    data,
    isLoading,
    isFetchingNextPage,
    error,
    hasNextPage,
    fetchNextPage,
  } = useInfiniteQuery({
    enabled: canListBots,
    queryKey: ['bot', 'list', search],
    queryFn: ({ pageParam, signal }) =>
      listBots(
        { pageSize: 50, pageToken: pageParam, searchTerm: search },
        signal,
        // TODO(strideynet): DELETE IN v20.0.0
        // Older proxies only list every bot at once. Fetch that list afresh
        // for the first page and reuse it for the rest.
        () =>
          queryClient.query({
            queryKey: ['bot', 'v1-list'],
            queryFn: ({ signal }) => fetchAllBotsV1(signal),
            staleTime: pageParam ? Infinity : 0,
          })
      ),
    initialPageParam: '',
    getNextPageParam: data => data?.next_page_token,
    placeholderData: keepPreviousData,
  });

  const bots = useMemo(
    () => (isSuccess ? data.pages.flatMap(page => page.items) : []),
    [data?.pages, isSuccess]
  );

  function updateLoadedBots(update: (bots: FlatBot[]) => FlatBot[]) {
    // Cover every cached search, not just the one on screen.
    queryClient.setQueriesData<InfiniteData<ListBotsPage>>(
      { queryKey: ['bot', 'list'] },
      data =>
        data
          ? {
              ...data,
              pages: data.pages.map(page => ({
                ...page,
                items: update(page.items),
              })),
            }
          : data
    );
  }

  function onDelete() {
    updateLoadedBots(bots => bots.filter(bot => bot.name !== selectedBot.name));
    onClose();
  }

  function onEdit(updated: FlatBot) {
    updateLoadedBots(bots =>
      bots.map(bot =>
        bot.name === selectedBot?.name ? { ...bot, ...updated } : bot
      )
    );
    onClose();
  }

  function onClose() {
    setSelectedBot(null);
  }

  const handleSelect = useCallback(
    (item: FlatBot) => {
      history.push(cfg.getBotDetailsRoute(item.name));
    },
    [history]
  );

  const handleSearchChange = useCallback(
    (search: string) => {
      const params = new URLSearchParams(location.search);
      if (search) {
        params.set('search', search);
      } else {
        params.delete('search');
      }
      history.push({ pathname: location.pathname, search: params.toString() });
    },
    [history, location.pathname, location.search]
  );

  const searchFilter = useMemo(() => ({ search }), [search]);

  if (isLoading) {
    return (
      <FeatureBox>
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      </FeatureBox>
    );
  }

  if (
    !canListBots ||
    (isSuccess && !isPlaceholderData && !search && !hasNextPage && !bots.length)
  ) {
    return (
      <FeatureBox>
        {!canListBots && (
          <Alert kind="info" mt={4}>
            You do not have permission to access Bots. Missing role permissions:{' '}
            <code>bot.list</code>
          </Alert>
        )}
        <EmptyState />
      </FeatureBox>
    );
  }

  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>Bots</FeatureHeaderTitle>
        <Box ml="auto">
          <InfoGuideButton config={{ guide: <InfoGuide /> }}>
            <HoverTooltip
              tipContent={
                hasAddBotPermissions
                  ? ''
                  : `Insufficient permissions. Reach out to your Teleport administrator
    to request bot creation permissions.`
              }
            >
              <Button
                intent="primary"
                fill="border"
                ml="auto"
                width="240px"
                as={Link}
                to={cfg.getBotsNewRoute()}
                disabled={!hasAddBotPermissions}
              >
                Enroll New Bot
              </Button>
            </HoverTooltip>
          </InfoGuideButton>
        </Box>
      </FeatureHeader>
      <SearchPanel
        filter={searchFilter}
        updateSearch={handleSearchChange}
        hideAdvancedSearch
        disableSearch={isFetching}
      />
      {error && <Alert kind="danger">{error.message}</Alert>}
      {isSuccess && (
        <BotList
          bots={bots}
          disabledEdit={!flags.roles || !flags.editBots}
          disabledDelete={!flags.removeBots}
          onClose={onClose}
          onDelete={onDelete}
          onEdit={onEdit}
          onSelect={handleSelect}
          selectedBot={selectedBot}
          setSelectedBot={setSelectedBot}
          // Let a search or refetch settle before loading another page.
          hasNextPage={hasNextPage && !isFetching}
          isFetchingNextPage={isFetchingNextPage}
          onLoadNextPage={fetchNextPage}
        />
      )}
    </FeatureBox>
  );
}
