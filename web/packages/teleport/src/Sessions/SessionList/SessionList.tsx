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

import React from 'react';
import styled from 'styled-components';
import Table, { Cell } from 'design/DataTable';
import { ButtonBorder } from 'design';
import Icon, * as Icons from 'design/Icon/Icon';

import cfg from 'teleport/config';
import { Session, Participant, SessionKind } from 'teleport/services/session';

export default function SessionList(props: Props) {
  const { sessions, pageSize = 100 } = props;

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
          render: renderPlayCell,
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

const renderIconCell = (kind: SessionKind) => {
  let icon = Icons.Cli;
  if (kind === 'k8s') {
    icon = Icons.Kubernetes;
  }

  return (
    <Cell>
      <Icon p={1} mr={3} fontSize={3} as={icon} />
    </Cell>
  );
};

const renderPlayCell = ({ sid, clusterId, kind }: Session) => {
  if (kind === 'k8s') {
    return <Cell align="right" height="26px" />;
  }

  const url = cfg.getSshSessionRoute({ sid, clusterId });
  return (
    <Cell align="right" height="26px">
      <ButtonBorder
        kind="primary"
        as="a"
        href={url}
        width="80px"
        target="_blank"
        size="small"
      >
        Join
      </ButtonBorder>
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
