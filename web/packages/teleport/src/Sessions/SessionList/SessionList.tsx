/*
Copyright 2019-2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import Table, { Cell } from 'design/DataTable';
import Icon, * as Icons from 'design/Icon/Icon';
import React from 'react';
import styled from 'styled-components';

import { Participant, Session, SessionKind } from 'teleport/services/session';

import { SessionJoinBtn } from './SessionJoinBtn';

export default function SessionList(props: Props) {
  const { sessions, pageSize = 100, showActiveSessionsCTA } = props;

  return (
    <StyledTable
      data={sessions}
      columns={[
        {
          key: 'kind',
          headerText: 'Type',
          isSortable: true,
          render: ({ kind }) => renderIconCell(kind),
        },
        {
          key: 'resourceName',
          headerText: 'Name',
          isSortable: true,
        },
        {
          key: 'sid',
          headerText: 'Session ID',
        },
        {
          altKey: 'users',
          headerText: 'Users',
          render: renderUsersCell,
        },
        {
          key: 'durationText',
          altSortKey: 'created',
          headerText: 'Duration',
          isSortable: true,
          onSort: (a, b) => b - a,
        },
        {
          altKey: 'join-btn',
          render: session =>
            renderJoinCell({ ...session, showActiveSessionsCTA }),
        },
      ]}
      emptyText="No Active Sessions Found"
      pagination={{ pageSize }}
      customSearchMatchers={[participantMatcher]}
      isSearchable
      initialSort={{ altSortKey: 'created', dir: 'ASC' }}
      searchableProps={[
        'addr',
        'sid',
        'clusterId',
        'resourceName',
        'serverId',
        'parties',
        'durationText',
        'login',
        'created',
        'parties',
      ]}
    />
  );
}

const kinds: {
  [key in SessionKind]: { icon: React.ReactNode; joinable: boolean };
} = {
  ssh: { icon: Icons.Cli, joinable: true },
  k8s: { icon: Icons.Kubernetes, joinable: false },
  desktop: { icon: Icons.Desktop, joinable: false },
  app: { icon: Icons.NewTab, joinable: false },
  db: { icon: Icons.Database, joinable: false },
};

const renderIconCell = (kind: SessionKind) => {
  const { icon } = kinds[kind];
  return (
    <Cell>
      <Icon p={1} mr={3} fontSize={3} as={icon} />
    </Cell>
  );
};

type renderJoinCellProps = Session & { showActiveSessionsCTA: boolean };
const renderJoinCell = ({
  sid,
  clusterId,
  kind,
  participantModes,
  showActiveSessionsCTA,
}: renderJoinCellProps) => {
  const { joinable } = kinds[kind];
  if (!joinable) {
    return <Cell align="right" height="26px" />;
  }

  return (
    <Cell align="right" height="26px">
      <SessionJoinBtn
        sid={sid}
        clusterId={clusterId}
        participantModes={participantModes}
        showCTA={showActiveSessionsCTA}
      />
    </Cell>
  );
};

function renderUsersCell({ parties }: Session) {
  const users = parties.map(({ user }) => `${user}`).join(', ');
  return <Cell>{users}</Cell>;
}

type Props = {
  sessions: Session[];
  pageSize?: number;
  showActiveSessionsCTA: boolean;
};

function participantMatcher(
  targetValue: any,
  searchValue: string,
  propName: keyof Session & string
) {
  if (propName === 'parties') {
    return targetValue.some((participant: Participant) => {
      return participant.user.toLocaleUpperCase().includes(searchValue);
    });
  }
}

const StyledTable = styled(Table)`
  tbody > tr > td {
    vertical-align: middle;
  }
` as typeof Table;
