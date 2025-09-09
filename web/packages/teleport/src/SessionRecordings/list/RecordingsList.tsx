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

import {
  useCallback,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
} from 'react';
import styled, { useTheme } from 'styled-components';

import { Danger } from 'design/Alert';
import Box from 'design/Box';
import InputSearch from 'design/DataTable/InputSearch';
import Flex, { Stack } from 'design/Flex';
import Text from 'design/Text';
import { SortMenu } from 'shared/components/Controls/SortMenu';
import { useLocalStorage } from 'shared/hooks/useLocalStorage';

import { ClusterDropdown } from 'teleport/components/ClusterDropdown/ClusterDropdown';
import type { Recording } from 'teleport/services/recordings';
import { useSuspenseInfiniteListRecordings } from 'teleport/services/recordings/hooks';
import { KeysEnum } from 'teleport/services/storageService';
import { generateTerminalSVGStyleTag } from 'teleport/SessionRecordings/svg';
import useStickyClusterId from 'teleport/useStickyClusterId';
import useTeleport from 'teleport/useTeleport';

import {
  RecordingFilters,
  type RecordingFilterOptions,
} from './RecordingFilters';
import { RecordingItem, type ActionSlot } from './RecordingItem';
import { RecordingsPagination } from './RecordingsPagination';
import type {
  RecordingsListFilterKey,
  RecordingsListFilters,
  RecordingsListSortDirection,
  RecordingsListSortKey,
  RecordingsListState,
} from './state';
import { Density, ViewMode, ViewSwitcher } from './ViewSwitcher';

interface RecordingsListProps {
  actionSlot?: ActionSlot;
  onFilterChange: (
    key: RecordingsListFilterKey,
    value: string[] | boolean
  ) => void;
  onPageChange: (page: number) => void;
  onSearchChange: (search: string) => void;
  onSortChange: (
    key: RecordingsListSortKey,
    dir: RecordingsListSortDirection
  ) => void;
  state: RecordingsListState;
}

interface RecordingsGridProps {
  viewMode: ViewMode;
  density: Density;
}

interface SortFieldOption {
  value: RecordingsListSortKey;
  label: string;
}

const sortFieldOptions: SortFieldOption[] = [
  { label: 'Type', value: 'type' },
  { label: 'Date', value: 'date' },
];

const RecordingsGrid = styled.div<RecordingsGridProps>(p => {
  const base: CSSProperties = {
    display: 'grid',
    minHeight: 0,
    gap: `${p.theme.space[3]}px`,
  };

  if (p.viewMode === ViewMode.List) {
    return {
      ...base,
      gridAutoRows: p.density === Density.Comfortable ? '200px' : '120px',
      gridTemplateColumns: '1fr',
    };
  }

  return {
    ...base,
    gridAutoRows: p.density === Density.Comfortable ? '300px' : '250px',
    gridTemplateColumns:
      p.density === Density.Comfortable
        ? 'repeat(auto-fill, minmax(500px, 1fr))'
        : 'repeat(auto-fill, minmax(320px, 1fr))',
  };
});

const pageSize = 50;

const ScrollContainer = styled.div`
  flex: 1 1 0;
  overflow-y: auto;
  min-height: 0;
  width: 100%;
  box-sizing: border-box;
  border-top: 1px solid ${p => p.theme.colors.spotBackground[1]};
  padding: ${p => p.theme.space[4]}px ${p => p.theme.space[6]}px;
`;

export function RecordingsList({
  actionSlot,
  onFilterChange,
  onPageChange,
  onSearchChange,
  onSortChange,
  state,
}: RecordingsListProps) {
  const ctx = useTeleport();
  const theme = useTheme();

  const { clusterId } = useStickyClusterId();

  const scrollRef = useRef<HTMLDivElement>(null);

  const [clusterErrorMessage, setClusterErrorMessage] = useState('');

  const [viewMode, setViewMode] = useLocalStorage(
    KeysEnum.SESSION_RECORDINGS_VIEW_MODE,
    ViewMode.Card
  );
  const [density, setDensity] = useLocalStorage(
    KeysEnum.SESSION_RECORDINGS_DENSITY,
    Density.Comfortable
  );

  const { data, isFetchNextPageError, fetchNextPage, hasNextPage, isFetching } =
    useSuspenseInfiniteListRecordings(
      {
        clusterId,
        params: {
          from: state.range.from,
          to: state.range.to,
        },
      },
      {
        getNextPageParam: lastPage =>
          // empty strings will indicate more pages, so we use undefined to indicate no more pages
          lastPage.startKey ? lastPage.startKey : undefined,
        initialPageParam: '',
      }
    );

  const allRecordings = useMemo(
    () => data.pages.flatMap(page => page.recordings),
    [data]
  );

  const filterOptions = useMemo(
    () => createFilterOptions(allRecordings),
    [allRecordings]
  );

  const recordings = useMemo(
    () =>
      filterRecordings(allRecordings, state.filters, state.search).toSorted(
        createRecordingsSortFunction(state.sortKey, state.sortDirection)
      ),
    [
      allRecordings,
      state.filters,
      state.search,
      state.sortKey,
      state.sortDirection,
    ]
  );

  const startIndex = state.page * pageSize;
  const endIndex = Math.min(startIndex + pageSize, recordings.length);

  const handleSortChange = useCallback(
    (newSort: {
      fieldName: RecordingsListSortKey;
      dir: RecordingsListSortDirection;
    }) => {
      onSortChange(newSort.fieldName, newSort.dir);

      scrollRef.current?.scrollTo({ top: 0 });
    },
    [onSortChange]
  );

  const handleFilterChange = useCallback(
    (key: RecordingsListFilterKey, value: string[] | boolean) => {
      onFilterChange(key, value);

      scrollRef.current?.scrollTo({ top: 0 });
    },
    [onFilterChange]
  );

  const handlePageChange = useCallback(
    (page: number) => {
      onPageChange(page);

      scrollRef.current?.scrollTo({ top: 0 });
    },
    [onPageChange]
  );

  const thumbnailStyles = useMemo(
    () => generateTerminalSVGStyleTag(theme),
    [theme]
  );

  const items = useMemo(
    () =>
      recordings
        .slice(startIndex, endIndex)
        .map(recording => (
          <RecordingItem
            actionSlot={actionSlot}
            key={recording.sid}
            recording={recording}
            thumbnailStyles={thumbnailStyles}
            viewMode={viewMode}
            density={density}
          />
        )),
    [
      actionSlot,
      recordings,
      viewMode,
      density,
      startIndex,
      endIndex,
      thumbnailStyles,
    ]
  );

  const filtersDisabled = allRecordings.length === 0;

  return (
    <Stack
      alignItems="stretch"
      gap={3}
      minHeight={0}
      flex={1}
      width="100%"
      overflow="hidden"
    >
      {clusterErrorMessage && (
        <Box px={6}>
          <Danger width="100%">{clusterErrorMessage}</Danger>
        </Box>
      )}

      <Box px={6}>
        <InputSearch
          searchValue={state.search}
          setSearchValue={onSearchChange}
        />
      </Box>

      <Flex justifyContent="space-between" px={6}>
        <Flex gap={2}>
          {!clusterErrorMessage && (
            <ClusterDropdown
              clusterLoader={ctx.clusterService}
              clusterId={clusterId}
              onError={setClusterErrorMessage}
            />
          )}

          <RecordingFilters
            disabled={filtersDisabled}
            filters={state.filters}
            options={filterOptions}
            onFilterChange={handleFilterChange}
          />
        </Flex>

        <Flex gap={2} alignItems="center">
          <ViewSwitcher
            viewMode={viewMode}
            setViewMode={setViewMode}
            density={density}
            setDensity={setDensity}
          />

          <SortMenu
            current={{
              fieldName: state.sortKey,
              dir: state.sortDirection,
            }}
            fields={sortFieldOptions}
            onChange={handleSortChange}
          />

          <RecordingsPagination
            count={recordings.length}
            fetchMoreAvailable={hasNextPage}
            fetchMoreDisabled={isFetching}
            fetchMoreError={isFetchNextPageError}
            from={startIndex}
            onFetchMore={fetchNextPage}
            onPageChange={handlePageChange}
            page={state.page}
            pageSize={pageSize}
            to={endIndex - 1}
          />
        </Flex>
      </Flex>

      <ScrollContainer data-scrollbar="default" ref={scrollRef}>
        {items.length === 0 ? (
          <Flex
            alignItems="center"
            justifyContent="center"
            height="100%"
            width="100%"
            minHeight={0}
          >
            <Text fontSize="large" color="text.secondary">
              No Recordings Found
            </Text>
          </Flex>
        ) : (
          <RecordingsGrid viewMode={viewMode} density={density}>
            {items}
          </RecordingsGrid>
        )}
      </ScrollContainer>
    </Stack>
  );
}

const searchableFields: (keyof Recording)[] = [
  'recordingType',
  'hostname',
  'description',
  'createdDate',
  'sid',
  'users',
  'durationText',
];

function searchMatcher(search: string, recording: Recording): boolean {
  if (!search) {
    return true;
  }

  const lowerSearch = search.toLowerCase().trim();

  return searchableFields.some(field => {
    const value = recording[field];

    if (typeof value === 'string') {
      return value.toLowerCase().includes(lowerSearch);
    }

    if (value instanceof Date) {
      return value.toISOString().toLowerCase().includes(lowerSearch);
    }

    return false;
  });
}

function filterRecordings(
  recordings: Recording[],
  filters: RecordingsListFilters,
  search: string
): Recording[] {
  return recordings.filter(recording => {
    if (
      filters.resources.length &&
      !filters.resources.includes(recording.hostname)
    ) {
      return false;
    }

    if (
      filters.types.length &&
      !filters.types.includes(recording.recordingType)
    ) {
      return false;
    }

    if (filters.users.length && !filters.users.includes(recording.user)) {
      return false;
    }

    if (filters.hideNonInteractive && !recording.playable) {
      return false;
    }

    return searchMatcher(search, recording);
  });
}

function createRecordingsSortFunction(
  key: RecordingsListSortKey,
  direction: RecordingsListSortDirection
): (a: Recording, b: Recording) => number {
  return (a, b) => {
    let valueA: string | number;
    let valueB: string | number;

    switch (key) {
      case 'type':
        valueA = a.recordingType;
        valueB = b.recordingType;
        break;

      case 'date':
        valueA = a.createdDate.getTime();
        valueB = b.createdDate.getTime();
        break;
    }

    if (valueA < valueB) {
      return direction === 'ASC' ? -1 : 1;
    }

    if (valueA > valueB) {
      return direction === 'ASC' ? 1 : -1;
    }

    if (key === 'type') {
      // If types are equal, sort by date
      return direction === 'ASC'
        ? a.createdDate.getTime() - b.createdDate.getTime()
        : b.createdDate.getTime() - a.createdDate.getTime();
    }

    return 0;
  };
}

function createFilterOptions(recordings: Recording[]): RecordingFilterOptions {
  const users = new Set<string>();
  const resources = new Set<string>();

  for (const recording of recordings) {
    resources.add(recording.hostname);
    users.add(recording.user);
  }

  return {
    resources: Array.from(resources).map(resource => ({
      label: resource,
      value: resource,
    })),
    users: Array.from(users).map(user => ({
      label: user,
      value: user,
    })),
  };
}
