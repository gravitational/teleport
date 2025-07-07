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

import format from 'date-fns/format';
import formatDistanceToNowStrict from 'date-fns/formatDistanceToNowStrict';
import parseISO from 'date-fns/parseISO';
import { useMemo } from 'react';
import styled from 'styled-components';

import { Info } from 'design/Alert/Alert';
import { Cell, LabelCell } from 'design/DataTable/Cells';
import Table from 'design/DataTable/Table';
import { FetchingConfig } from 'design/DataTable/types';
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
  onSearchChange,
  onItemSelected,
}: {
  data: BotInstanceSummary[];
  searchTerm: string;
  onSearchChange: (term: string) => void;
  onItemSelected: (item: BotInstanceSummary) => void;
} & Omit<FetchingConfig, 'onFetchMore'>) {
  const tableData = data.map(x => ({
    ...x,
    hostnameDisplay: x.host_name_latest ?? '-',
    instanceIdDisplay: x.instance_id.substring(0, 7),
    versionDisplay: x.version_latest ? `v${x.version_latest}` : '-',
    activeAtDisplay: x.active_at_latest
      ? `${formatDistanceToNowStrict(parseISO(x.active_at_latest))} ago`
      : '-',
    activeAtLocal: x.active_at_latest
      ? format(parseISO(x.active_at_latest), 'PP, p z')
      : '-',
  }));

  const rowConfig = useMemo(
    () => ({
      onClick: onItemSelected,
      getStyle: () => ({ cursor: 'pointer' }),
    }),
    [onItemSelected]
  );

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
        sort: undefined,
        setSort: () => undefined,
        serversideSearchPanel: (
          <SearchPanel
            updateSearch={onSearchChange}
            updateQuery={null}
            hideAdvancedSearch={true}
            filter={{ search: searchTerm }}
            disableSearch={fetchStatus !== ''}
          />
        ),
      }}
      row={rowConfig}
      columns={[
        {
          key: 'bot_name',
          headerText: 'Bot',
          isSortable: false,
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
          key: 'hostnameDisplay',
          headerText: 'Hostname',
          isSortable: false,
        },
        {
          key: 'versionDisplay',
          headerText: 'Version (tbot)',
          isSortable: false,
        },
        {
          key: 'activeAtDisplay',
          headerText: 'Last heartbeat',
          isSortable: false,
          render: ({ activeAtDisplay, activeAtLocal }) => (
            <Cell>
              <Flex>
                <HoverTooltip tipContent={activeAtLocal}>
                  {activeAtDisplay}
                </HoverTooltip>
              </Flex>
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
