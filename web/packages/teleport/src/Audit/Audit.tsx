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

import { useState, type PropsWithChildren } from 'react';
import styled from 'styled-components';

import { Box, ButtonSecondary, Flex } from 'design';
import { Danger } from 'design/Alert';
import { SearchPanel } from 'shared/components/Search';
import { useInfiniteScroll } from 'shared/hooks';

import { ExternalAuditStorageCta } from '@gravitational/teleport/src/components/ExternalAuditStorageCta';
import { ClusterDropdown } from 'teleport/components/ClusterDropdown/ClusterDropdown';
import RangePicker from 'teleport/components/EventRangePicker';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import useStickyClusterId from 'teleport/useStickyClusterId';
import useTeleport from 'teleport/useTeleport';

import EventList from './EventList';
import { EventListSkeleton } from './EventListSkeleton';
import useAuditEvents, { State } from './useAuditEvents';

export function AuditContainer() {
  const teleCtx = useTeleport();
  const { clusterId } = useStickyClusterId();
  const state = useAuditEvents(teleCtx, clusterId);
  return <Audit {...state} />;
}

export function Audit(props: State) {
  const {
    range,
    setRange,
    events,
    clusterId,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    error,
    isLoading,
    search,
    setSearch,
    sort,
    setSort,
    ctx,
    refetch,
    isError,
  } = props;

  const [errorMessage, setErrorMessage] = useState('');

  const { setTrigger } = useInfiniteScroll({
    fetch: async () => {
      if (hasNextPage && !isFetchingNextPage && !isError) {
        fetchNextPage();
      }
    },
  });

  const onRetryClicked = () => {
    refetch();
  };

  const onLoadMoreClicked = () => {
    if (hasNextPage && !isFetchingNextPage) {
      fetchNextPage();
    }
  };

  return (
    <FeatureBox unsetHeight>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle mr="8">Audit Log</FeatureHeaderTitle>
        <RangePicker ml="auto" range={range} onChangeRange={setRange} />
      </FeatureHeader>
      <ExternalAuditStorageCta />
      {!isLoading && isError && error && (
        <ErrorsContainer>
          <DangerWithBackground
            primaryAction={{
              content: 'Retry',
              onClick: onRetryClicked,
            }}
          >
            {error.message}
          </DangerWithBackground>
        </ErrorsContainer>
      )}
      {!errorMessage && (
        <ClusterDropdown
          clusterLoader={ctx.clusterService}
          clusterId={clusterId}
          onError={setErrorMessage}
          mb={2}
        />
      )}
      {errorMessage && <Danger>{errorMessage}</Danger>}
      <Box mt={2}>
        <SearchPanel
          updateSearch={setSearch}
          updateQuery={null}
          hideAdvancedSearch={true}
          filter={{ search }}
        />
        {!isLoading && (
          <EventList
            events={events}
            search={search}
            setSearch={setSearch}
            sort={sort}
            setSort={setSort}
          />
        )}
        {((isLoading && events.length === 0) || isFetchingNextPage) && (
          <EventListSkeleton />
        )}
        <div ref={setTrigger} />
        {isError && events.length > 0 && !isLoading && (
          <Box mt={2} textAlign="center">
            <ButtonSecondary onClick={onLoadMoreClicked}>
              Load more
            </ButtonSecondary>
          </Box>
        )}
      </Box>
    </FeatureBox>
  );
}

function ErrorsContainer(props: PropsWithChildren<unknown>) {
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

const DangerWithBackground = styled(Danger)`
  background: ${props => props.theme.colors.levels.sunken};
`;
