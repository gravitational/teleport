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

import { format } from 'date-fns/format';
import { formatDistanceToNowStrict } from 'date-fns/formatDistanceToNowStrict';
import { parseISO } from 'date-fns/parseISO';
import styled from 'styled-components';

import { Info } from 'design/Alert/Alert';
import { Cell, LabelCell } from 'design/DataTable/Cells';
import Table from 'design/DataTable/Table';
import { FetchingConfig, SortType } from 'design/DataTable/types';
import Flex from 'design/Flex';
import Text from 'design/Text';
import { HoverTooltip } from 'design/Tooltip/HoverTooltip';
import { SearchPanel } from 'shared/components/Search';
import { CopyButton } from 'shared/components/UnifiedResources/shared/CopyButton';

import { BotInstanceSummary } from 'teleport/services/bot/types';

const MonoText = styled(Text)`
  font-family: ${({ theme }) => theme.fonts.mono};
`;

export function BotInstancesList({
  data,
  fetchStatus,
  onFetchNext,
  onFetchPrev,
  searchTerm,
  query,
  onSearchChange,
  onQueryChange,
  onItemSelected,
  sortType,
  onSortChanged,
}: {
  data: BotInstanceSummary[];
  searchTerm: string;
  query: string;
  onSearchChange: (term: string) => void;
  onQueryChange: (term: string) => void;
  onItemSelected: (item: BotInstanceSummary) => void;
  sortType: SortType;
  onSortChanged: (sortType: SortType) => void;
} & Omit<FetchingConfig, 'onFetchMore'>) {
  const tableData = data.map(x => ({
    ...x,
    host_name_latest: x.host_name_latest ?? '-',
    instanceIdDisplay: x.instance_id.substring(0, 7),
    version_latest: x.version_latest ? `v${x.version_latest}` : '-',
    active_at_latest: x.active_at_latest
      ? `${formatDistanceToNowStrict(parseISO(x.active_at_latest))} ago`
      : '-',
    activeAtLocal: x.active_at_latest
      ? format(parseISO(x.active_at_latest), 'PP, p z')
      : '-',
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
            updateQuery={onQueryChange}
            hideAdvancedSearch={false}
            filter={{ search: searchTerm, query }}
            disableSearch={fetchStatus !== ''}
          />
        ),
      }}
      row={{
        onClick: onItemSelected,
        getStyle: () => ({ cursor: 'pointer' }),
      }}
      columns={[
        {
          key: 'bot_name',
          headerText: 'Bot',
          isSortable: true,
        },
        {
          key: 'instanceIdDisplay',
          headerText: 'ID',
          isSortable: false,
          render: ({ instance_id, instanceIdDisplay }) => (
            <Cell>
              <Flex inline alignItems={'center'} gap={1} mr={0}>
                <MonoText>{instanceIdDisplay}</MonoText>
                <CopyButton name={instance_id} />
              </Flex>
            </Cell>
          ),
        },
        {
          key: 'join_method_latest',
          headerText: 'Method',
          isSortable: false,
          render: ({ join_method_latest }) =>
            join_method_latest ? (
              <LabelCell data={[join_method_latest]} />
            ) : (
              <Cell>{'-'}</Cell>
            ),
        },
        {
          key: 'host_name_latest',
          headerText: 'Hostname',
          isSortable: true,
        },
        {
          key: 'version_latest',
          headerText: 'Version (tbot)',
          isSortable: true,
        },
        {
          key: 'active_at_latest',
          headerText: 'Last heartbeat',
          isSortable: true,
          render: ({ active_at_latest, activeAtLocal }) => (
            <Cell>
              <HoverTooltip tipContent={activeAtLocal}>
                <span>{active_at_latest}</span>
              </HoverTooltip>
            </Cell>
          ),
        },
      ]}
      emptyText="No active instances found"
      emptyButton={
        <Info mt={5}>
          Bot instances are ephemeral, and disappear once all issued credentials
          have expired.
        </Info>
      }
    />
  );
}
