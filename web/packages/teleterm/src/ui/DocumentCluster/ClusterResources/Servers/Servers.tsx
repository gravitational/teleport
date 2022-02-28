/*
Copyright 2019 Gravitational, Inc.

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
import { useServers, State } from './useServers';
import * as types from 'teleterm/ui/services/clusters/types';
import Table, { Cell } from 'design/DataTable';
import { ButtonBorder } from 'design';
import { renderLabelCell } from '../renderLabelCell';

export default function Container() {
  const state = useServers();
  return <ServerList {...state} />;
}

function ServerList(props: State) {
  const { servers = [], connect } = props;
  return (
    <Table
      columns={[
        {
          key: 'hostname',
          headerText: 'Hostname',
          isSortable: true,
        },
        {
          key: 'addr',
          headerText: 'Address',
          isSortable: true,
          render: renderAddressCell,
        },
        {
          key: 'labelsList',
          headerText: 'Labels',
          render: renderLabelCell,
        },
        {
          altKey: 'connect-btn',
          render: server => renderConnectCell(server.uri, connect),
        },
      ]}
      emptyText="No Nodes Found"
      data={servers}
      pagination={{ pageSize: 100, pagerPosition: 'bottom' }}
    />
  );
}

const renderConnectCell = (
  serverUri: string,
  connect: (serverUri: string) => void
) => {
  return (
    <Cell align="right">
      <ButtonBorder
        size="small"
        onClick={() => {
          connect(serverUri);
        }}
      >
        Connect
      </ButtonBorder>
    </Cell>
  );
};

const renderAddressCell = ({ addr, tunnel }: types.Server) => (
  <Cell>
    {tunnel && (
      <span
        style={{ cursor: 'default' }}
        title="This node is connected to cluster through reverse tunnel"
      >{`⟵ tunnel`}</span>
    )}
    {!tunnel && addr}
  </Cell>
);
