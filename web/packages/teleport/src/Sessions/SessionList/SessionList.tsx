/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { ReactNode } from 'react';
import styled from 'styled-components';

import Table, { Cell } from 'design/DataTable';
import * as Icons from 'design/Icon';

import { Participant, Session, SessionKind } from 'teleport/services/session';

import { SessionJoinBtn } from './SessionJoinBtn';

export default function SessionList(props: Props) {
  const {
    sessions,
    pageSize = 100,
    showActiveSessionsCTA,
    showModeratedSessionsCTA,
  } = props;

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
          key: 'command',
          headerText: 'Command',
        },
        {
          key: 'durationText',
          headerText: 'Duration',
          isSortable: true,
          onSort: (a, b) => b.created.getTime() - a.created.getTime(),
        },
        {
          altKey: 'join-btn',
          render: session =>
            renderJoinCell({
              ...session,
              showActiveSessionsCTA,
              showModeratedSessionsCTA,
            }),
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
  [key in SessionKind]: { icon: (any) => ReactNode; joinable: boolean };
} = {
  ssh: { icon: Icons.Cli, joinable: true },
  k8s: { icon: Icons.Kubernetes, joinable: false },
  desktop: { icon: Icons.Desktop, joinable: false },
  app: { icon: Icons.Application, joinable: false },
  db: { icon: Icons.Database, joinable: false },
};

const renderIconCell = (kind: SessionKind) => {
  const { icon } = kinds[kind];
  let Icon = icon;
  return (
    <Cell>
      <Icon p={1} mr={3} size="large" />
    </Cell>
  );
};

type renderJoinCellProps = Session & {
  showActiveSessionsCTA: boolean;
  showModeratedSessionsCTA: boolean;
};
const renderJoinCell = ({
  sid,
  clusterId,
  kind,
  participantModes,
  showActiveSessionsCTA,
  showModeratedSessionsCTA,
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
        showModeratedCTA={showModeratedSessionsCTA}
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
  showModeratedSessionsCTA: boolean;
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
