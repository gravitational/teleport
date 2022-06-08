import React, { useState } from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';
import {
  Text,
  Label,
  LabelState,
  ButtonBorder,
  ButtonPrimary,
  Box,
  Alert,
  Indicator,
} from 'design';
import Table, { Cell } from 'design/DataTable';
import useTeleportE from 'e-teleport/useTeleportE';
import cfg from 'e-teleport/config';
import { ResourceId, AccessRequest } from 'e-teleport/services/workflow';
import useRequestList, { State, Row } from './useRequestList';

export default function Container() {
  const ctx = useTeleportE();
  const state = useRequestList(ctx);
  return <RequestList {...state} />;
}

export function RequestList({ attempt, requests = [], assumeRole }: State) {
  // Delaying indicator is default behavior.
  // This flag is used to show indicator immedidately after
  // user clicks "assume" button, which removes the awkward blank
  // moment where nothing seems to be happening for
  // the duration of the default delay moment.
  const [delayIndicator, setDelayIndicator] = useState(true);

  function onAssumeRole(request: Row) {
    setDelayIndicator(false);
    assumeRole(request);
  }

  return (
    <>
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator delay={delayIndicator ? 'short' : 'none'} />
        </Box>
      )}
      {attempt.status === 'failed' && (
        <Alert kind="danger" children={attempt.statusText} />
      )}
      {attempt.status === 'success' && (
        <StyledTable
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
              render: ({ resourceIds, roles }) => (
                <RequestedCell resourceIds={resourceIds} roles={roles} />
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
              render: request => renderActionCell(request as Row, onAssumeRole),
            },
          ]}
          emptyText="No Requests Found"
          isSearchable
          pagination={{ pageSize: 20 }}
          initialSort={{ key: 'created', dir: 'DESC' }}
          customSearchMatchers={[requestdMatcher]}
        />
      )}
    </>
  );
}

function requestdMatcher(
  targetValue: any,
  searchValue: string,
  propName: keyof AccessRequest & string
) {
  if (propName === 'roles') {
    return targetValue.some((role: string) =>
      role.toUpperCase().includes(searchValue)
    );
  }

  if (propName === 'resourceIds') {
    return targetValue.some((r: ResourceId) =>
      Object.keys(r).some(k => r[k].toUpperCase().includes(searchValue))
    );
  }
}

const renderUserCell = ({ user }: Row) => {
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

const renderIdCell = ({ id }: Row) => {
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

const renderReasonCell = ({ requestReason }: Row) => {
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

const renderStatusCell = ({ state }: Row) => {
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

const renderActionCell = (request: Row, assumeRole: (request: Row) => void) => {
  return (
    <Cell align="right" style={{ whiteSpace: 'nowrap' }}>
      {request.canAssume && (
        <ButtonPrimary
          size="small"
          disabled={request.isAssumed}
          onClick={() => assumeRole(request)}
          width="108px"
        >
          {request.isAssumed ? 'assumed' : 'assume roles'}
        </ButtonPrimary>
      )}
      <ButtonBorder
        as={Link}
        size="small"
        ml={3}
        to={cfg.getAccessRequestRoute(request.id)}
      >
        View
      </ButtonBorder>
    </Cell>
  );
};

const RequestedCell = ({
  roles,
  resourceIds,
}: Pick<Row, 'roles' | 'resourceIds'>) => {
  if (resourceIds?.length > 0) {
    return (
      <Cell>
        {resourceIds.map(id => (
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

const StyledTable = styled(Table)`
  tbody > tr > td {
    vertical-align: baseline;
  }
` as typeof Table;
