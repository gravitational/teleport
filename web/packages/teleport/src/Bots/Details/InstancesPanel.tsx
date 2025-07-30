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
import React, { useEffect, useState } from 'react';
import styled from 'styled-components';

import { Alert } from 'design/Alert/Alert';
import Box from 'design/Box/Box';
import { ButtonSecondary, ButtonText } from 'design/Button';
import Flex from 'design/Flex/Flex';
import { SortAscending, SortDescending } from 'design/Icon';
import { Indicator } from 'design/Indicator/Indicator';
import { H2 } from 'design/Text/Text';
import { fontWeights } from 'design/theme/typography';

import { listBotInstances } from 'teleport/services/bot/bot';
import useTeleport from 'teleport/useTeleport';

import { Instance } from './Instance';

export function InstancesPanel(props: { botName: string }) {
  const { botName } = props;

  const [sort, setSort] = useState<
    'active_at_latest:asc' | 'active_at_latest:desc'
  >('active_at_latest:desc');

  const contentRef = React.useRef<HTMLDivElement>(null);

  const ctx = useTeleport();
  const flags = ctx.getFeatureFlags();
  const hasListPermission = flags.listBotInstances;

  const {
    isSuccess,
    data,
    isLoading,
    isFetchingNextPage,
    isError,
    error,
    hasNextPage,
    fetchNextPage,
  } = useInfiniteQuery({
    enabled: hasListPermission,
    queryKey: ['bot_instances', 'list', sort, botName],
    queryFn: ({ pageParam }) =>
      listBotInstances({
        pageSize: 32,
        pageToken: pageParam,
        sort,
        botName,
      }),
    initialPageParam: '',
    getNextPageParam: data => data?.next_page_token,
    placeholderData: keepPreviousData,
    staleTime: 30_000, // Cached pages are valid for 30 seconds
  });

  const handleToggleSort = () => {
    setSort(sort =>
      sort === 'active_at_latest:desc'
        ? 'active_at_latest:asc'
        : 'active_at_latest:desc'
    );
  };

  // Scrolls to the top when the selected sort changes
  useEffect(() => {
    contentRef.current?.scrollTo({ top: 0, behavior: 'instant' });
  }, [sort]);

  return (
    <Container>
      <TitleContainer>
        <H2 fontWeight={fontWeights.bold}>Active Instances</H2>
        {isSuccess ? (
          <ActionButton onClick={handleToggleSort}>
            Recent
            {sort === 'active_at_latest:desc' ? (
              <SortDescending size={'medium'} />
            ) : (
              <SortAscending size={'medium'} />
            )}
          </ActionButton>
        ) : undefined}
      </TitleContainer>

      <Divider />

      {isLoading ? (
        <Box data-testid="loading-instances" textAlign="center" m={10}>
          <Indicator />
        </Box>
      ) : undefined}

      {!hasListPermission ? (
        <Alert m={3} kind="info">
          You do not have permission to view bot instances. Missing role
          permissions: <code>botInstances.list</code>
        </Alert>
      ) : undefined}

      {isError ? (
        <Alert m={3} kind="danger" details={error.message}>
          Failed to fetch instances
        </Alert>
      ) : undefined}

      {isSuccess ? (
        <>
          {data.pages.length > 0 && data.pages[0].bot_instances.length > 0 ? (
            <ContentContainer ref={contentRef}>
              {data.pages.map((page, i) =>
                page.bot_instances.map((instance, j) => (
                  <React.Fragment key={`${instance.instance_id}`}>
                    {i === 0 && j === 0 ? undefined : <Divider />}
                    <Instance
                      id={instance.instance_id}
                      activeAt={instance.active_at_latest}
                      hostname={instance.host_name_latest}
                      method={instance.join_method_latest}
                      version={instance.version_latest}
                      os={instance.os_latest}
                    />
                  </React.Fragment>
                ))
              )}

              <Divider />

              <LoadMoreContainer>
                <ButtonSecondary
                  onClick={() => fetchNextPage()}
                  disabled={!hasNextPage || isFetchingNextPage}
                >
                  Load More
                </ButtonSecondary>
              </LoadMoreContainer>
            </ContentContainer>
          ) : (
            <Box p={3}>No active instances</Box>
          )}
        </>
      ) : undefined}
    </Container>
  );
}

const Container = styled.section`
  display: flex;
  flex-direction: column;
  height: 100%;
`;

const TitleContainer = styled(Flex)`
  align-items: center;
  justify-content: space-between;
  padding: ${p => p.theme.space[3]}px;
  gap: ${p => p.theme.space[2]}px;
`;

const ActionButton = styled(ButtonText)`
  padding-left: ${p => p.theme.space[2]}px;
  padding-right: ${p => p.theme.space[2]}px;
  gap: ${p => p.theme.space[2]}px;
`;

const ContentContainer = styled.div`
  overflow: auto;
`;

const LoadMoreContainer = styled(Flex)`
  justify-content: center;
  padding: ${props => props.theme.space[3]}px;
`;

const Divider = styled.div`
  height: 1px;
  flex-shrink: 0;
  background-color: ${p => p.theme.colors.interactive.tonal.neutral[0]};
`;
