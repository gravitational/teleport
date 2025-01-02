import styled from 'styled-components';

import Table from 'design/DataTable';
import { ClientSidePager } from 'design/DataTable/Pager';
import { StyledTable as StyledTableBase } from 'design/DataTable/StyledTable';
import { PagedTableProps } from 'design/DataTable/types';

import {
  PluginConfigOktaApp,
  PluginConfigOktaGroup,
} from 'e-teleport/services/plugins/types';

const PAGE_SIZE = 15;

export function AppTable({
  apps,
  loading,
}: {
  apps: PluginConfigOktaApp[];
  loading: boolean;
}) {
  return (
    <Table
      data={apps}
      columns={[
        {
          key: 'name',
          headerText: 'App Name',
        },
      ]}
      emptyText="No Okta apps found"
      pagination={{
        pageSize: PAGE_SIZE,
        CustomTable,
      }}
      fetching={{
        fetchStatus: loading ? 'loading' : '',
      }}
    />
  );
}

export function UserGroupsTable({
  userGroups,
  loading,
}: {
  userGroups: PluginConfigOktaGroup[];
  loading: boolean;
}) {
  return (
    <Table
      data={userGroups}
      columns={[
        {
          key: 'name',
          headerText: 'Group Name',
        },
        {
          key: 'description',
          headerText: 'Description',
        },
      ]}
      emptyText="No Okta groups found"
      pagination={{
        pageSize: PAGE_SIZE,
        CustomTable,
      }}
      fetching={{
        fetchStatus: loading ? 'loading' : '',
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
      <ClientSidePager
        nextPage={nextPage}
        prevPage={prevPage}
        data={data}
        {...pagination}
      />
      <TableWrapper>
        <StyledTable>
          {renderHeaders()}
          {renderBody(paginatedData[currentPage])}
        </StyledTable>
      </TableWrapper>
    </>
  );
}

const TableWrapper = styled.div`
  overflow-y: scroll;
  border-bottom: 1px solid ${props => props.theme.colors.spotBackground[2]};
  height: 330px;
`;

// We apply a z-index to the thead > tr > th to avoid the table content from being visible in the header space
const StyledTable = styled(StyledTableBase)(
  props => `

   background-color: inherit;
   border-collapse: separate;

  tbody > tr > td, thead > tr > th {
    font-size: ${props.theme.fontSizes[2]}px;
    font-weight: 300;
  }

  thead > tr > th {
    font-weight: bold;
    border-bottom: 1px solid ${props.theme.colors.spotBackground[2]};
    text-transform: none;
    padding: ${props.theme.space[2]}px 0;
    top: 0;
    position: sticky;
    z-index: 1;
    background-color: ${props.theme.colors.levels.elevated};
    opacity: 1;
    padding-top: 0px;
  }

  tbody > tr > td {
    padding: ${props.theme.space[3]}px 0;
  }

  tbody > tr {
    border: none;
  }
`
);
