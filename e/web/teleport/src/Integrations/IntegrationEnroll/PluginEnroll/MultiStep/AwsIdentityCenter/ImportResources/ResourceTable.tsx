import styled from 'styled-components';

import { Flex, Text } from 'design';
import Table, { Cell } from 'design/DataTable';
import { ClientSidePager } from 'design/DataTable/Pager';
import { StyledTable } from 'design/DataTable/StyledTable';
import { PagedTableProps } from 'design/DataTable/types';

import {
  AwsIcAccounts,
  AwsIcGroupsWithAssignment,
  AwsIcPermissionAssignments,
  AwsIcPermissionSets,
} from 'e-teleport/services/plugins/types';

const PAGE_SIZE = 15;

export function AccountsTable({
  accounts,
  loading,
}: {
  accounts: AwsIcAccounts[];
  loading: boolean;
}) {
  return (
    <Table
      data={accounts}
      columns={[
        {
          key: 'name',
          headerText: 'Name',
        },
        {
          key: 'permissionSets',
          headerText: 'Permission Sets',
          render: ({ permissionSets }) => (
            <Cell>
              <Flex gap={2} flexDirection="column">
                <Text mb={1}>
                  {permissionSets
                    .map((p: AwsIcPermissionSets) => {
                      return p.name;
                    })
                    .join(', ')}
                </Text>
              </Flex>
            </Cell>
          ),
        },
      ]}
      emptyText="No accounts found"
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

export function PermissionSetsTable({
  permissionSets,
  loading,
}: {
  permissionSets: AwsIcPermissionSets[];
  loading: boolean;
}) {
  return (
    <Table
      data={permissionSets}
      columns={[
        {
          key: 'name',
          headerText: 'Permission Set',
          render: ({ name }) => (
            <Cell
              css={`
                width: 170px;
                white-space: nowrap;
                overflow: hidden;
                text-overflow: ellipsis;
              `}
              title={name}
            >
              {name}
            </Cell>
          ),
        },
        {
          key: 'description',
          headerText: 'Description',
          render: ({ description }) => (
            <Cell
              css={`
                width: 270px;
                text-overflow: ellipsis;
                overflow: hidden;
              `}
              title={description}
            >
              {description}
            </Cell>
          ),
        },
        {
          key: 'arn',
          headerText: 'ARN',
          render: ({ arn }) => (
            <Cell
              css={`
                width: 370px;
                text-overflow: ellipsis;
                overflow: hidden;
                left: 1px;
                top: 5px;
              `}
              title={arn}
            >
              {arn}
            </Cell>
          ),
        },
      ]}
      emptyText="No permission sets found"
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

export function GroupsWithAssignmentTable({
  userGroups,
  loading,
}: {
  userGroups: AwsIcGroupsWithAssignment[];
  loading: boolean;
}) {
  return (
    <Table
      data={userGroups}
      columns={[
        {
          key: 'name',
          headerText: 'Group',
        },
        {
          key: 'assignments',
          headerText: 'Assignments',
          render: ({ assignments }) => (
            <Cell
              css={`
                text-overflow: ellipsis;
                overflow: hidden;
                left: 10px;
                width: 500px;
                top: 5px;
              `}
            >
              <Flex gap={2} flexDirection="column">
                {assignments.map((a: AwsIcPermissionAssignments, i: number) => {
                  return (
                    <Text mb={1} key={`${i}${a.permissionSetName}`}>
                      {`Permission set "${a.permissionSetName}" on  "${a.accountName}" account.`}
                    </Text>
                  );
                })}
              </Flex>
            </Cell>
          ),
        },
      ]}
      emptyText="No user groups found"
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
      <StyledTableB>
        {renderHeaders()}
        {renderBody(paginatedData[currentPage])}
      </StyledTableB>
    </>
  );
}

const StyledTableB = styled(StyledTable)`
  tbody > tr > td {
    font-size: 12px;
  }
`;
