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

import React from 'react';
import { ButtonPrimary } from 'design';
import Table, { Cell, TextCell } from 'design/DataTable';
import { dateTimeMatcher } from 'design/utils/match';

import * as Icons from 'design/Icon';

import cfg from 'teleport/config';
import { Recording, RecordingType } from 'teleport/services/recordings';

import { State } from './useRecordings';

export default function RecordingsList(props: Props) {
  const {
    recordings = [],
    clusterId,
    pageSize = 50,
    fetchMore,
    fetchStatus,
  } = props;

  return (
    <Table
      data={recordings}
      columns={[
        {
          headerText: 'Type',
          key: 'recordingType',
          isSortable: true,
          render: ({ recordingType }) => renderIconCell(recordingType),
        },
        {
          key: 'hostname',
          headerText: 'Name',
          isSortable: true,
        },
        {
          key: 'users',
          headerText: 'User(s)',
          render: ({ users }) => (
            <Cell style={{ wordBreak: 'break-word' }}>{users}</Cell>
          ),
          isSortable: true,
        },
        {
          key: 'duration',
          headerText: 'Duration',
          isSortable: true,
          render: ({ durationText }) => <TextCell data={durationText} />,
        },
        {
          key: 'createdDate',
          headerText: 'Created (UTC)',
          isSortable: true,
          render: ({ createdDate }) => <Cell>{createdDate}</Cell>,
        },
        {
          key: 'sid',
          headerText: 'Session ID',
        },
        {
          altKey: 'play-btn',
          render: recording => renderPlayCell(recording, clusterId),
        },
      ]}
      emptyText="No Recordings Found"
      pagination={{ pageSize }}
      fetching={{ onFetchMore: fetchMore, fetchStatus }}
      initialSort={{
        key: 'createdDate',
        dir: 'DESC',
      }}
      isSearchable
      searchableProps={[
        'recordingType',
        'hostname',
        'description',
        'createdDate',
        'sid',
        'users',
        'durationText',
      ]}
      customSearchMatchers={[dateTimeMatcher(['createdDate'])]}
    />
  );
}

const renderIconCell = (type: RecordingType) => {
  let Icon = Icons.Cli;
  if (type === 'desktop') {
    Icon = Icons.Desktop;
  } else if (type === 'k8s') {
    Icon = Icons.Kubernetes;
  } else if (type === 'database') {
    Icon = Icons.Database;
  }

  return (
    <Cell>
      <Icon p={1} mr={3} size="large" />
    </Cell>
  );
};

const renderPlayCell = (
  { description, sid, recordingType, playable, duration }: Recording,
  clusterId: string
) => {
  if (!playable) {
    return (
      <Cell align="right" style={{ color: '#9F9F9F' }}>
        {description}
      </Cell>
    );
  }

  const url = cfg.getPlayerRoute(
    { clusterId, sid },
    {
      recordingType,
      durationMs: duration,
    }
  );
  return (
    <Cell align="right">
      <ButtonPrimary
        as="a"
        href={url}
        width="80px"
        target="_blank"
        size="small"
      >
        Play
      </ButtonPrimary>
    </Cell>
  );
};

type Props = {
  pageSize?: number;
  recordings: State['recordings'];
  clusterId: State['clusterId'];
  fetchMore: State['fetchMore'];
  fetchStatus: State['fetchStatus'];
};
