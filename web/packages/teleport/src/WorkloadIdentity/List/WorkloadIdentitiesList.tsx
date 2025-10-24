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

import { ReactElement } from 'react';

import { Cell, LabelCell } from 'design/DataTable/Cells';
import Table from 'design/DataTable/Table';
import { FetchingConfig, SortType } from 'design/DataTable/types';
import Flex from 'design/Flex';
import Text from 'design/Text';
import { SearchPanel } from 'shared/components/Search/SearchPanel';
import { CopyButton } from 'shared/components/UnifiedResources/shared/CopyButton';

import { WorkloadIdentity } from 'teleport/services/workloadIdentity/types';

export function WorkloadIdetitiesList({
  data,
  fetchStatus,
  onFetchNext,
  onFetchPrev,
  sortType,
  onSortChanged,
  searchTerm,
  onSearchChange,
}: {
  data: WorkloadIdentity[];
  sortType: SortType;
  onSortChanged: (sortType: SortType) => void;
  searchTerm: string;
  onSearchChange: (term: string) => void;
} & Omit<FetchingConfig, 'onFetchMore'>) {
  const tableData = data.map(d => ({
    ...d,
    spiffe_hint: valueOrEmpty(d.spiffe_hint),
  }));

  return (
    <Table<(typeof tableData)[number]>
      data={tableData}
      fetching={{
        fetchStatus,
        onFetchNext,
        onFetchPrev,
        disableLoadingIndicator: true,
      }}
      serversideProps={{
        sort: sortType,
        setSort: onSortChanged,
        serversideSearchPanel: (
          <SearchPanel
            updateSearch={onSearchChange}
            hideAdvancedSearch={true}
            filter={{ search: searchTerm }}
            disableSearch={fetchStatus !== ''}
          />
        ),
      }}
      columns={[
        {
          key: 'name',
          headerText: 'Name',
          isSortable: true,
        },
        {
          key: 'spiffe_id',
          headerText: 'SPIFFE ID',
          isSortable: true,
          render: ({ spiffe_id }) => {
            return spiffe_id ? (
              <Cell>
                <Flex inline alignItems={'center'} gap={1} mr={0}>
                  <Text>
                    {spiffe_id
                      .split('/')
                      .reduce<(ReactElement | string)[]>((acc, cur, i) => {
                        if (i === 0) {
                          acc.push(cur);
                        } else {
                          // Add break opportunities after each slash
                          acc.push('/', <wbr key={cur} />, cur);
                        }
                        return acc;
                      }, [])}
                  </Text>
                  <CopyButton name={spiffe_id} />
                </Flex>
              </Cell>
            ) : (
              <Cell>{valueOrEmpty(spiffe_id)}</Cell>
            );
          },
        },
        {
          key: 'labels',
          headerText: 'Labels',
          isSortable: false,
          render: ({ labels: labelsMap }) => {
            const labels = labelsMap ? Object.entries(labelsMap) : undefined;
            return labels?.length ? (
              <LabelCell data={labels.map(([k, v]) => `${k}: ${v || '-'}`)} />
            ) : (
              <Cell>{valueOrEmpty('')}</Cell>
            );
          },
        },
        {
          key: 'spiffe_hint',
          headerText: 'Hint',
          isSortable: false,
        },
      ]}
      emptyText="No workload identities found"
    />
  );
}

function valueOrEmpty(value: string | null | undefined, empty = '-') {
  return value || empty;
}
