import { Flex, Mark, Text } from 'design';
import Table, { Cell } from 'design/DataTable';
import { ClientSidePager } from 'design/DataTable/Pager';
import { StyledTable } from 'design/DataTable/StyledTable';
import { PagedTableProps } from 'design/DataTable/types';

import {
  PluginConfigIcAssignments,
  PluginConfigAwsIcAccounts,
  PluginConfigAwsIcPermissionSetsTable,
  PluginConfigAwsIcUserGroupsWithAssignment,
  PluginConfigAwsIcUserDirectAssignment,
} from 'e-teleport/services/plugins/types';

const PAGE_SIZE = 15;

export function AccountsTable({
  accounts,
  loading,
}: {
  accounts: PluginConfigAwsIcAccounts[];
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
          key: 'id',
          headerText: 'ID',
        },
        {
          key: 'arn',
          headerText: 'ARN',
          render: ({ arn }) => (
            <Cell
              css={`
                text-overflow: ellipsis;
                overflow: hidden;
                left: 5px;
                width: 300px;
                top: 5px;
              `}
              title={arn}
            >
              {arn}
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
  permissionSets: PluginConfigAwsIcPermissionSetsTable[];
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

function AssignmentsRow({
  assignments,
}: {
  assignments: PluginConfigIcAssignments[];
}) {
  return (
    <Flex gap={2}>
      {assignments.map((a: PluginConfigIcAssignments) => {
        return (
          <Text mb={1}>
            <Mark>{a.permission_set_name}</Mark> on{' '}
            <Mark>{a.account_name}</Mark> account.
          </Text>
        );
      })}
    </Flex>
  );
}

export function GroupsWithAssigmentTable({
  userGroups,
  loading,
}: {
  userGroups: PluginConfigAwsIcUserGroupsWithAssignment[];
  loading: boolean;
}) {
  return (
    <Table
      data={userGroups}
      columns={[
        {
          key: 'groupname',
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
              <Flex flexDirection={'column'}>
                <Flex flexDirection={'row'}>
                  <AssignmentsRow assignments={assignments} />
                </Flex>
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

export function DirecthAssigmentTable({
  userGroups,
  loading,
}: {
  userGroups: PluginConfigAwsIcUserDirectAssignment[];
  loading: boolean;
}) {
  return (
    <Table
      data={userGroups}
      columns={[
        {
          key: 'username',
          headerText: 'User',
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
              <Flex flexDirection={'column'}>
                <Flex flexDirection={'row'}>
                  <AssignmentsRow assignments={assignments} />
                </Flex>
              </Flex>
            </Cell>
          ),
        },
      ]}
      emptyText="No direct permission assignments found"
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
      <StyledTable>
        {renderHeaders()}
        {renderBody(paginatedData[currentPage])}
      </StyledTable>
    </>
  );
}
