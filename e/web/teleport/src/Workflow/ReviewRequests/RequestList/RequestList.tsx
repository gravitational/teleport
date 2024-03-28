import React from 'react';
import { Link } from 'react-router-dom';
import {
  Text,
  Label,
  LabelState,
  ButtonBorder,
  ButtonPrimary,
  Alert,
  Flex,
  Indicator,
} from 'design';
import Table, { Cell } from 'design/DataTable';
import { ArrowFatLinesUp } from 'design/Icon';
import InputSearch from 'design/DataTable/InputSearch';
import { Attempt } from 'shared/hooks/useAttemptNext';
import { useInfiniteScroll } from 'shared/hooks';
import { AccessRequestScope } from 'teleport/services/agents';
import { ResourceTab } from 'shared/components/UnifiedResources/ResourceTab';

import useTeleportE from 'e-teleport/useTeleportE';
import cfg from 'e-teleport/config';
import { AccessRequest, Resource } from 'e-teleport/services/workflow';
import {
  BlockedByStartTimeButton,
  ButtonPromotedInfo,
} from 'e-teleport/Workflow/Shared/Shared';
import { canAssumeNow } from 'e-teleport/services/workflow/makeAccessRequest';
import { reloginWebUi } from 'e-teleport/Workflow/Shared/utils';

import { formattedName } from '../formattedName';

import useRequestList, {
  State,
  AccessRequestWithFlags,
} from './useRequestList';

export default function Container() {
  const ctx = useTeleportE();
  const state = useRequestList(ctx);
  return <RequestList {...state} />;
}

const scopes: { value: AccessRequestScope; label: string }[] = [
  { value: '', label: 'All Requests' },
  { value: 'my_requests', label: 'My Requests' },
  { value: 'needs_review', label: 'Needs Review' },
  { value: 'reviewed', label: 'Reviewed' },
];

export function RequestList({
  attempt,
  fetchAttempt,
  fetch,
  searchString,
  sortBy,
  scope,
  clear,
  resources,
  updateSort,
  updateScope,
  setSearchString,
  assumeRole,
}: State) {
  const { setTrigger } = useInfiniteScroll({
    fetch: fetch,
  });

  function onAssumeRole(request: AccessRequestWithFlags) {
    assumeRole(request);
  }

  function onSubmitSearch(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    clear();
  }

  return (
    <>
      {attempt.status === 'failed' && (
        <Alert kind="danger" children={attempt.statusText} />
      )}
      {fetchAttempt.status === 'failed' && (
        <Alert kind="danger" children={fetchAttempt.statusText} />
      )}
      <Flex mb={3} gap={3}>
        {scopes.map(s => (
          <ResourceTab
            onClick={() => updateScope(s.value)}
            disabled={false}
            isSelected={s.value === scope}
            key={s.value}
            title={s.label}
          />
        ))}
      </Flex>
      <Flex as="form" onSubmit={onSubmitSearch} mb={3}>
        <InputSearch
          searchValue={searchString}
          setSearchValue={setSearchString}
        />
      </Flex>
      <Table
        data={resources}
        customSort={{
          dir: sortBy.dir,
          fieldName: sortBy.fieldName,
          onSort: updateSort,
        }}
        columns={[
          {
            key: 'id',
            headerText: 'Id',
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
            render: ({ resources, roles, id }) => (
              <RequestedCell resources={resources} roles={roles} id={id} />
            ),
          },
          {
            key: 'resources',
            isNonRender: true,
          },
          {
            key: 'created',
            headerText: 'Created',
            isSortable: true,
            render: ({ createdDuration }) => (
              <Cell width="120px">{createdDuration}</Cell>
            ),
          },
          {
            key: 'assumeStartTime',
            headerText: 'Available',
            isSortable: true,
            render: ({ assumeStartTimeDuration }) => (
              <Cell>{assumeStartTimeDuration}</Cell>
            ),
          },
          {
            key: 'expires',
            headerText: 'Expires',
            render: ({ requestTTLDuration }) => (
              <Cell width="120px">{requestTTLDuration}</Cell>
            ),
          },
          {
            altKey: 'view-btn',
            render: request =>
              renderActionCell(
                request as AccessRequestWithFlags,
                onAssumeRole,
                attempt.status
              ),
          },
        ]}
        emptyText="No Requests Found"
      />
      {fetchAttempt.status === 'processing' && (
        <Flex justifyContent="center">
          <Indicator />
        </Flex>
      )}
      <div ref={setTrigger} />
    </>
  );
}

export function requestdMatcher(
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
    return targetValue.some((r: Resource) =>
      Object.values(r.id)
        .concat(Object.values(r.details.hostname || {}))
        .concat(Object.values(r.details.friendlyName || {}))
        .some(v => v.toUpperCase().includes(searchValue))
    );
  }
}

export const renderUserCell = ({ user }: AccessRequestWithFlags) => {
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

export const renderIdCell = ({ id }: AccessRequestWithFlags) => {
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

export const renderReasonCell = ({ requestReason }: AccessRequestWithFlags) => {
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

export const renderStatusCell = ({ state }: AccessRequestWithFlags) => {
  if (state === 'PROMOTED') {
    return (
      <Cell>
        <Flex alignItems="center">
          <ArrowFatLinesUp size={17} color="success.main" mr={1} ml="-3px" />
          <Text typography="body2">{state}</Text>
        </Flex>
      </Cell>
    );
  }

  let kind = 'warning';
  if (state === 'APPROVED') {
    kind = 'success';
  } else if (state === 'DENIED') {
    kind = 'danger';
  }

  return (
    <Cell>
      <Flex alignItems="center">
        <LabelState
          kind={kind}
          mr={2}
          width="10px"
          p={0}
          style={{ minHeight: '10px' }}
        />
        <Text typography="body2">{state}</Text>
      </Flex>
    </Cell>
  );
};

const renderActionCell = (
  request: AccessRequestWithFlags,
  assumeRole: (request: AccessRequestWithFlags) => void,
  attemptStatus: Attempt['status']
) => {
  let assumeBtn;
  if (request.canAssume) {
    if (canAssumeNow(request.assumeStartTime)) {
      assumeBtn = (
        <ButtonPrimary
          size="small"
          disabled={request.isAssumed || attemptStatus === 'processing'}
          onClick={() => assumeRole(request)}
          width="108px"
        >
          {request.isAssumed ? 'assumed' : 'assume roles'}
        </ButtonPrimary>
      );
    } else {
      assumeBtn = (
        <BlockedByStartTimeButton assumeStartTime={request.assumeStartTime} />
      );
    }
  }

  return (
    <Cell align="right" style={{ whiteSpace: 'nowrap' }}>
      <Flex alignItems="center" justifyContent="right" width="184px">
        {assumeBtn}
        {request.isPromoted && (
          <ButtonPromotedInfo
            request={request}
            ownRequest={request.ownRequest}
            assumeAccessList={reloginWebUi}
          />
        )}
        <ButtonBorder
          as={Link}
          size="small"
          ml={3}
          to={cfg.getAccessRequestRoute(request.id)}
        >
          View
        </ButtonBorder>
      </Flex>
    </Cell>
  );
};

export const RequestedCell = ({
  roles,
  resources,
  id,
}: Pick<AccessRequestWithFlags, 'roles' | 'resources' | 'id'>) => {
  if (resources?.length > 0) {
    return (
      <Cell key={id}>
        {resources.map((resource, index) => (
          <Label
            mb="0"
            mr="1"
            key={`${resource.id.kind}${formattedName(resource)}${index}`}
            kind="secondary"
          >
            {resource.id.kind}:{' '}
            {resource.details?.friendlyName || formattedName(resource)}
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
