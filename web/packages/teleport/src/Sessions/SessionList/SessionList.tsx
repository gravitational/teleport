/*
Copyright 2019-2020 Gravitational, Inc.

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

import React from 'react';
import Table, { Cell } from 'design/DataTable';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';
import cfg from 'teleport/config';
import { Session, Participant } from 'teleport/services/ssh';
import renderDescCell from './DescCell';

export default function SessionList(props: Props) {
  const { sessions, pageSize = 100 } = props;

  return (
    <Table
      data={sessions}
      columns={[
        {
          altKey: 'description',
          headerText: 'Description',
          render: renderDescCell,
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
          altKey: 'node',
          headerText: 'Node',
          render: renderNodeCell,
        },
        {
          key: 'durationText',
          headerText: 'Duration',
        },
        {
          altKey: 'options-btn',
          render: renderActionCell,
        },
      ]}
      emptyText="No Active Sessions Found"
      pagination={{ pageSize }}
      customSearchMatchers={[participantMatcher]}
      isSearchable
      searchableProps={[
        'addr',
        'sid',
        'clusterId',
        'serverId',
        'hostname',
        'parties',
        'durationText',
        'login',
        'created',
        'parties',
      ]}
    />
  );
}

function renderActionCell({ sid, clusterId }: Session) {
  const url = cfg.getSshSessionRoute({ sid, clusterId });

  return (
    <Cell align="right">
      <MenuButton>
        <MenuItem as="a" href={url} target="_blank">
          Join Session
        </MenuItem>
      </MenuButton>
    </Cell>
  );
}

function renderNodeCell({ hostname, addr }: Session) {
  const nodeAddr = addr ? `[${addr}]` : '';

  return (
    <Cell>
      {hostname} {nodeAddr}
    </Cell>
  );
}

function renderUsersCell({ parties }: Session) {
  const users = parties
    .map(({ user, remoteAddr }) => `${user} [${remoteAddr}]`)
    .join(', ');
  return <Cell>{users}</Cell>;
}

type Props = {
  sessions: Session[];
  pageSize?: number;
};

function participantMatcher(
  targetValue: any,
  searchValue: string,
  propName: keyof Session & string
) {
  if (propName === 'parties') {
    return targetValue.some((participant: Participant) => {
      if (participant.remoteAddr.toLocaleUpperCase().includes(searchValue)) {
        return true;
      }

      return participant.user.toLocaleUpperCase().includes(searchValue);
    });
  }
}
