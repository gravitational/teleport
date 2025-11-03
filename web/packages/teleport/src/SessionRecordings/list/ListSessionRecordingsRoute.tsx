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

import { useCallback, useMemo, useState, type ReactNode } from 'react';
import { type FallbackProps } from 'react-error-boundary';

import { Danger } from 'design/Alert';
import Box from 'design/Box';
import Flex, { Stack } from 'design/Flex';
import { Indicator } from 'design/Indicator';
import { ErrorSuspenseWrapper } from 'shared/components/ErrorSuspenseWrapper/ErrorSuspenseWrapper';
import { getErrorMessage } from 'shared/utils/error';

import { ClusterDropdown } from 'teleport/components/ClusterDropdown/ClusterDropdown';
import RangePicker, {
  EventRange,
  getRangeOptions,
} from 'teleport/components/EventRangePicker';
import { ExternalAuditStorageCta } from 'teleport/components/ExternalAuditStorageCta';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { SessionSummariesCta } from 'teleport/SessionRecordings/list/SessionSummariesCta';
import useStickyClusterId from 'teleport/useStickyClusterId';
import useTeleport from 'teleport/useTeleport';

import type { ActionSlot } from './RecordingItem';
import { RecordingsList } from './RecordingsList';
import {
  useRecordingsListState,
  type RecordingsListFilterKey,
  type RecordingsListSortKey,
} from './state';

interface ListSessionRecordingsRouteProps {
  actionSlot?: ActionSlot;
  headerSlot?: ReactNode;
}

export function ListSessionRecordingsRoute() {
  return (
    <ListSessionRecordings
      actionSlot={null}
      headerSlot={<SessionSummariesCta />}
    />
  );
}

export function ListSessionRecordings({
  actionSlot,
  headerSlot,
}: ListSessionRecordingsRouteProps) {
  const ranges = useMemo(() => getRangeOptions(), []);

  const [state, setState] = useRecordingsListState(ranges);

  const handleSetRange = useCallback(
    (range: EventRange) => setState(prev => ({ ...prev, range })),
    [setState]
  );

  const handleFilterChange = useCallback(
    (key: RecordingsListFilterKey, value: string[] | boolean) =>
      setState(prev => ({
        ...prev,
        page: 0,
        filters: {
          ...prev.filters,
          [key]: value,
        },
      })),
    [setState]
  );

  const handleSortChange = useCallback(
    (key: RecordingsListSortKey, direction: 'ASC' | 'DESC') =>
      setState(prev => ({
        ...prev,
        sortKey: key,
        sortDirection: direction,
      })),
    [setState]
  );

  const handlePageChange = useCallback(
    (page: number) => setState(prev => ({ ...prev, page })),
    [setState]
  );

  const handleSearchChange = useCallback(
    (search: string) => setState(prev => ({ ...prev, search })),
    [setState]
  );

  return (
    <FeatureBox minHeight={0} padding={0} hideBottomSpacing={true}>
      <FeatureHeader
        alignItems="center"
        mx={0}
        mb={1}
        justifyContent="space-between"
      >
        <FeatureHeaderTitle mr="8">Session Recordings</FeatureHeaderTitle>

        <Flex alignItems="center" gap={3}>
          {headerSlot}

          <RangePicker
            ml="auto"
            range={state.range}
            ranges={ranges}
            onChangeRange={handleSetRange}
          />
        </Flex>
      </FeatureHeader>

      <ExternalAuditStorageCta />

      <Flex flex={1} minHeight={0} overflow="hidden" width="100%">
        <ErrorSuspenseWrapper
          errorComponent={RecordingsListError}
          loadingComponent={RecordingsListLoading}
        >
          <RecordingsList
            actionSlot={actionSlot}
            onFilterChange={handleFilterChange}
            onPageChange={handlePageChange}
            onSearchChange={handleSearchChange}
            onSortChange={handleSortChange}
            state={state}
          />
        </ErrorSuspenseWrapper>
      </Flex>
    </FeatureBox>
  );
}

function RecordingsListLoading() {
  return (
    <Box textAlign="center" m={10} width="100%">
      <Indicator />
    </Box>
  );
}

function RecordingsListError({ error, resetErrorBoundary }: FallbackProps) {
  const ctx = useTeleport();

  const { clusterId } = useStickyClusterId();

  const [errorMessage, setErrorMessage] = useState('');

  return (
    <Stack
      alignItems="stretch"
      gap={3}
      minHeight={0}
      flex={1}
      width="100%"
      overflow="hidden"
    >
      <Flex px={6} width="100%" justifyContent="stretch">
        {errorMessage ? (
          <Danger width="100%">{errorMessage}</Danger>
        ) : (
          <ClusterDropdown
            clusterLoader={ctx.clusterService}
            clusterId={clusterId}
            onError={setErrorMessage}
          />
        )}
      </Flex>
      <Box px={6}>
        <Danger
          primaryAction={{
            content: 'Retry',
            onClick: resetErrorBoundary,
          }}
        >
          {getErrorMessage(error)}
        </Danger>
      </Box>
    </Stack>
  );
}
