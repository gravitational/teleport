import React from 'react';
import styled from 'styled-components';

import {
  Text,
  Label,
  LabelState,
  Alert,
  ButtonBorder,
  Flex,
  ButtonPrimary,
  Box,
} from 'design';
import Table, { Cell } from 'design/DataTable';

import { AccessRequest } from 'e-teleport/services/workflow';
import { Attempt } from 'shared/hooks/useAttemptNext';

export function RequestList({
  attempt,
  requests,
  viewRequest,
  assumeRoleAttempt,
  assumeRole,
  getRequests,
}: Props) {
  return (
    <Layout mx="auto" px={5} pt={3} height="100%">
      {attempt.status === 'failed' && (
        <Alert kind="danger" children={attempt.statusText} />
      )}
      {assumeRoleAttempt.status === 'failed' && (
        <Alert kind="danger" children={assumeRoleAttempt.statusText} />
      )}
      <Flex justifyContent="end" pb={4}>
        <ButtonPrimary
          ml={2}
          size="small"
          onClick={getRequests}
          disabled={attempt.status === 'processing'}
        >
          Refresh
        </ButtonPrimary>
      </Flex>
      <Table
        data={requests}
        columns={[
          {
            key: 'id',
            headerText: 'Id',
            isSortable: true,
            render: renderIdCell,
          },
          {
            key: 'state',
            headerText: 'Status',
            isSortable: true,
            render: renderStatusCell,
          },
          {
            key: 'user',
            headerText: 'User',
            isSortable: true,
            render: renderUserCell,
          },
          {
            key: 'roles',
            headerText: 'Requested',
            render: ({ resources, roles }) => (
              <RequestedCell resources={resources} roles={roles} />
            ),
          },
          {
            key: 'resourceIds',
            isNonRender: true,
          },
          {
            key: 'requestReason',
            headerText: 'Request Reason',
            isSortable: true,
            render: renderReasonCell,
          },
          {
            key: 'created',
            headerText: 'Created',
            isSortable: true,
            render: ({ createdDuration }) => <Cell>{createdDuration}</Cell>,
          },
          {
            altKey: 'view-btn',
            render: request =>
              renderActionCell(
                request,
                assumeRole,
                assumeRoleAttempt,
                viewRequest
              ),
          },
        ]}
        emptyText="No Requests Found"
        isSearchable
        pagination={{ pageSize: 20 }}
        initialSort={{ key: 'created', dir: 'DESC' }}
        customSearchMatchers={[requestMatcher]}
      />
    </Layout>
  );
}

function requestMatcher(
  targetValue: any,
  searchValue: string,
  propName: keyof AccessRequest & string
) {
  if (propName === 'roles') {
    return targetValue.some((role: string) =>
      role.toUpperCase().includes(searchValue)
    );
  }

  if (propName === 'resources') {
    return targetValue.some((r: any) =>
      Object.keys(r).some(k => r[k].toUpperCase().includes(searchValue))
    );
  }
}

const renderUserCell = ({ user }: any) => {
  return (
    <Cell
      style={{
        maxWidth: '100px',
        whiteSpace: 'nowrap',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
      }}
      title={user}
    >
      {user}
    </Cell>
  );
};

const renderIdCell = ({ id }: any) => {
  return (
    <Cell
      style={{
        maxWidth: '100px',
        whiteSpace: 'nowrap',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
      }}
      title={id}
    >
      {id.slice(-5)}
    </Cell>
  );
};

const renderReasonCell = ({ requestReason }: any) => {
  return (
    <Cell
      style={{
        maxWidth: '150px',
        whiteSpace: 'nowrap',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
      }}
      title={requestReason}
    >
      {requestReason}
    </Cell>
  );
};

const renderStatusCell = ({ state }: any) => {
  let kind = 'warning';
  if (state === 'APPROVED') {
    kind = 'success';
  } else if (state === 'DENIED') {
    kind = 'danger';
  }

  return (
    <Cell style={{ display: 'flex', alignItems: 'center' }}>
      <LabelState
        kind={kind}
        mr={2}
        width="10px"
        p={0}
        style={{ minHeight: '10px' }}
      />
      <Text typography="body2">{state}</Text>
    </Cell>
  );
};

const renderActionCell = (
  request: any,
  assumeRole: (request: any) => void,
  assumeRoleAttempt: Attempt,
  viewRequest: (id: string) => void
) => {
  return (
    <Cell align="right" style={{ whiteSpace: 'nowrap' }}>
      {request.canAssume && (
        <ButtonPrimary
          size="small"
          disabled={
            request.isAssumed || assumeRoleAttempt.status === 'processing'
          }
          onClick={() => assumeRole(request)}
          width="108px"
        >
          {request.isAssumed ? 'assumed' : 'assume roles'}
        </ButtonPrimary>
      )}
      <ButtonBorder size="small" ml={3} onClick={() => viewRequest(request.id)}>
        View
      </ButtonBorder>
    </Cell>
  );
};

const RequestedCell = ({
  roles,
  resources,
}: Pick<AccessRequest, 'roles' | 'resources'>) => {
  if (resources?.length > 0) {
    return (
      <Cell>
        {resources.map(({ id }) => (
          <Label mb="0" mr="1" key={`${id.kind}${id.name}`} kind="secondary">
            {id.kind}: {id.name}
          </Label>
        ))}
      </Cell>
    );
  }

  return (
    <Cell>
      {roles.sort().map(role => (
        <Label mb="0" mr="1" key={role} kind="secondary">
          role: {role}
        </Label>
      ))}
    </Cell>
  );
};

const Layout = styled(Box)`
  flex-direction: column;
  display: flex;
  flex: 1;
  max-width: 1248px;

  ::after {
    content: ' ';
    padding-bottom: 24px;
  }
`;

type Props = {
  attempt: Attempt;
  requests: AccessRequest[];
  assumeRole: (request: AccessRequest) => void;
  assumeRoleAttempt: Attempt;
  getRequests: () => void;
  viewRequest: (requestId: string) => void;
};
