/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { Alert, Box, ButtonBorder, ButtonPrimary, Flex, Label } from 'design';
import Table, { Cell } from 'design/DataTable';
import { displayDateTime } from 'design/datetime';
import { requestMatcher } from 'shared/components/AccessRequests/NewRequest/matcher';
import {
  renderIdCell,
  renderStatusCell,
  renderUserCell,
  RequestFlags,
} from 'shared/components/AccessRequests/ReviewRequests';
import {
  BlockedByStartTimeButton,
  ButtonPromotedInfo,
  getResourcesOrRolesFromRequest,
} from 'shared/components/AccessRequests/Shared/Shared';
import { Attempt } from 'shared/hooks/useAsync';
import { AccessRequest, canAssumeNow } from 'shared/services/accessRequests';

export function RequestList({
  attempt,
  getFlags,
  viewRequest,
  assumeRoleAttempt,
  assumeRole,
  getRequests,
  assumeAccessList,
}: {
  attempt: Attempt<AccessRequest[]>;
  getFlags(accessRequest: AccessRequest): RequestFlags;
  assumeRole(request: AccessRequest): void;
  assumeRoleAttempt: Attempt<void>;
  getRequests(): void;
  viewRequest(requestId: string): void;
  assumeAccessList(): void;
}) {
  return (
    <Layout mx="auto" px={5} pt={3} height="100%">
      {attempt.status === 'error' && (
        <Alert kind="danger" details={attempt.statusText}>
          Could not fetch access requests
        </Alert>
      )}
      {assumeRoleAttempt.status === 'error' && (
        <Alert kind="danger" details={assumeRoleAttempt.statusText}>
          Could not assume the role
        </Alert>
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
        data={attempt.data || []}
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
            render: request => <RequestedCell request={request} />,
          },
          {
            key: 'resources',
            isNonRender: true,
          },
          {
            key: 'created',
            headerText: 'Created',
            isSortable: true,
            render: ({ createdDuration, created }) => (
              <Cell title={displayDateTime(created)}>{createdDuration}</Cell>
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
            isSortable: true,
            render: ({ requestTTLDuration, requestTTL }) => (
              <Cell title={displayDateTime(requestTTL)}>
                {requestTTLDuration}
              </Cell>
            ),
          },
          {
            altKey: 'view-btn',
            render: request =>
              renderActionCell(
                request,
                getFlags(request),
                assumeRole,
                assumeRoleAttempt,
                viewRequest,
                assumeAccessList
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

const renderActionCell = (
  request: AccessRequest,
  flags: RequestFlags,
  assumeRole: (request: AccessRequest) => void,
  assumeRoleAttempt: Attempt<void>,
  viewRequest: (id: string) => void,
  assumeAccessList: () => void
) => {
  let assumeBtn;
  if (flags.canAssume) {
    if (canAssumeNow(request.assumeStartTime)) {
      assumeBtn = (
        <ButtonPrimary
          size="small"
          disabled={
            flags.isAssumed || assumeRoleAttempt.status === 'processing'
          }
          onClick={() => assumeRole(request)}
          width="108px"
        >
          {flags.isAssumed ? 'Assumed' : 'Assume Roles'}
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
        {flags.isPromoted && (
          <ButtonPromotedInfo
            request={request}
            ownRequest={flags.ownRequest}
            assumeAccessList={assumeAccessList}
          />
        )}
        <ButtonBorder
          size="small"
          ml={3}
          onClick={() => viewRequest(request.id)}
        >
          View
        </ButtonBorder>
      </Flex>
    </Cell>
  );
};

const RequestedCell = ({ request }: { request: AccessRequest }) => (
  <Cell>
    <Flex gap={1} flexWrap="wrap">
      {getResourcesOrRolesFromRequest(request).map((r, index) => (
        <Label
          kind="secondary"
          key={`${r.title}${index}`}
          title={r.title}
          m={0}
          css={`
            display: flex;
            gap: ${props => props.theme.space[1]}px;
          `}
        >
          <r.Icon size="small" />
          <span
            css={`
              white-space: nowrap;
            `}
          >
            {r.name}
          </span>
        </Label>
      ))}
    </Flex>
  </Cell>
);

const Layout = styled(Box)`
  flex-direction: column;
  display: flex;
  flex: 1;
  max-width: 1248px;

  &::after {
    content: ' ';
    padding-bottom: 24px;
  }
`;
