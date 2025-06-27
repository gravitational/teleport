import React from 'react';
import styled from 'styled-components';
import { StyledTable as StyledTableBase } from 'web/packages/design/src/DataTable/StyledTable';

import Table, { Cell, LabelCell } from 'design/DataTable';
import { ClientSidePager } from 'design/DataTable/Pager';
import { PagedTableProps } from 'design/DataTable/types';
import Flex from 'design/Flex';
import * as Icons from 'design/Icon';
import { Check } from 'design/Icon';
import { P2 } from 'design/Text';

import { Profile } from 'teleport/Integrations/Enroll/AwsConsole/Access/Profiles';

//  from e/web/teleport/src/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/ImportUserGroupsAndApps/FilterTable.tsx
export function ProfilesTable({
  profiles,
  loading,
}: {
  profiles: Profile[];
  loading: boolean;
}) {
  function getRowStyle(row: Profile): React.CSSProperties {
    // todo mberg need design for non-selected row
    if (profiles?.includes(row)) {
      return { textTransform: 'uppercase' };
    }
    return { textTransform: 'lowercase' };
  }

  return (
    <Table
      data={profiles}
      row={{
        getStyle: getRowStyle,
      }}
      columns={[
        {
          altKey: 'selected',
          headerText: '',
          render: row =>
            profiles?.includes(row) ? (
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
        },
      ]}
      emptyText="No Profiles Found"
      pagination={{
        pageSize: 15,
        CustomTable,
      }}
      fetching={{
        fetchStatus: loading ? 'loading' : '',
      }}
    />
  );
}

//  from e/web/teleport/src/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/ImportUserGroupsAndApps/FilterTable.tsx
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
        <StyledTable>
          {renderHeaders()}
          {renderBody(paginatedData[currentPage])}
        </StyledTable>
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

//  from e/web/teleport/src/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/ImportUserGroupsAndApps/FilterTable.tsx
const TableWrapper = styled.div(
  props => `
      border-bottom: 1px solid ${props.theme.colors.interactive.tonal.neutral[0]};
      overflow-y: auto;
      max-height: 330px;
`
);

//  from e/web/teleport/src/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/ImportUserGroupsAndApps/FilterTable.tsx
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
