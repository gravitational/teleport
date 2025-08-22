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

import styled from 'styled-components';

import Table, { Cell, LabelCell } from 'design/DataTable';
import { ClientSidePager } from 'design/DataTable/Pager';
import { StyledTable } from 'design/DataTable/StyledTable';
import { FetchingConfig, PagedTableProps } from 'design/DataTable/types';
import Flex from 'design/Flex';
import * as Icons from 'design/Icon';
import { Check } from 'design/Icon';
import { P2 } from 'design/Text';

import { RolesAnywhereProfile } from 'teleport/services/integrations';

export function ProfilesTable({
  data,
  fetchStatus,
  onFetchNext,
  onFetchPrev,
}: {
  data: RolesAnywhereProfile[];
} & Omit<FetchingConfig, 'onFetchMore'>) {
  return (
    <Table
      data={data}
      columns={[
        {
          altKey: 'selected',
          headerText: '',
          render: row =>
            data?.includes(row) ? (
              <Cell width="50px">
                <Check size="small" color="success.main" />
              </Cell>
            ) : (
              <Cell width="50px" />
            ),
        },
        {
          key: 'name',
          headerText: 'Profile Name',
        },
        {
          key: 'tags',
          headerText: 'Tags',
          render: row => <LabelCell data={row.tags} />,
        },
        {
          key: 'roles',
          headerText: 'IAM Roles',
          render: row => <Cell>{row.roles.join(', ')}</Cell>,
        },
      ]}
      emptyText="No Profiles Found"
      pagination={{ pageSize: 20, CustomTable }}
      fetching={{
        fetchStatus,
        onFetchNext,
        onFetchPrev,
        disableLoadingIndicator: true,
      }}
    />
  );
}

function CustomTable<T>({
  nextPage,
  prevPage,
  data,
  pagination,
  renderHeaders,
  renderBody,
}: PagedTableProps<T>) {
  const { paginatedData, currentPage } = pagination;

  return (
    <>
      <TableWrapper>
        <CustomerStyledTable>
          {renderHeaders()}
          {renderBody(paginatedData[currentPage])}
        </CustomerStyledTable>
      </TableWrapper>
      <Flex justifyContent="space-between" alignItems="center">
        <Flex gap={1}>
          <Icons.Info color="text.muted" />
          <P2 color="text.muted">
            New and matching AWS Roles Anywhere Profiles created in the AWS
            Console will be automatically synced with Teleport.
          </P2>
        </Flex>
        <Flex>
          <ClientSidePager
            nextPage={nextPage}
            prevPage={prevPage}
            data={data}
            {...pagination}
          />
        </Flex>
      </Flex>
    </>
  );
}

const TableWrapper = styled.div(
  props => `
      border-bottom: 1px solid ${props.theme.colors.interactive.tonal.neutral[0]};
      overflow-y: auto;
      max-height: 330px;
`
);

const CustomerStyledTable = styled(StyledTable)(
  props => `

   background-color: inherit;
   border-collapse: separate;

  tbody > tr > td, thead > tr > th {
    font-size: ${props.theme.fontSizes[2]}px;
    font-weight: 300;
  }

  thead > tr > th {
    font-weight: bold;
    border-bottom: 1px solid ${props.theme.colors.interactive.tonal.neutral[0]};
    text-transform: none;
    padding: ${props.theme.space[2]}px 0;
    top: 0;
    position: sticky;
    z-index: 1;
    opacity: 1;
  }

  tbody > tr > td {
    padding: ${props.theme.space[3]}px 0;
  }

  tbody > tr {
    background-color: ${props.theme.colors.levels.surface};
    border: none;
    
    transition: all 150ms;
    position: relative;
  }
`
);
