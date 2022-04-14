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
import { renderLabelCell } from '../renderLabelCell';
import { MenuLogin } from 'shared/components/MenuLogin';
import { MenuLoginTheme } from '../MenuLoginTheme';
import { Danger } from 'design/Alert';

export default function Container() {
  const state = useServers();
  return <ServerList {...state} />;
}

function ServerList(props: State) {
  const { servers = [], getSshLogins, connect, syncStatus } = props;
  return (
    <>
      {syncStatus.status === 'failed' && (
        <Danger>{syncStatus.statusText}</Danger>
      )}
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
            render: server =>
              renderConnectCell(
                () => getSshLogins(server.uri),
                login => connect(server.uri, login)
              ),
          },
        ]}
        emptyText="No Nodes Found"
        data={servers}
        pagination={{ pageSize: 100, pagerPosition: 'bottom' }}
      />
    </>
  );
}

const renderConnectCell = (
  getSshLogins: () => string[],
  onConnect: (login: string) => void
) => {
  return (
    <Cell align="right">
      <MenuLoginTheme>
        <MenuLogin
          getLoginItems={() =>
            getSshLogins().map(login => ({ login, url: '' }))
          }
          onSelect={(e, login) => onConnect(login)}
          transformOrigin={{
            vertical: 'top',
            horizontal: 'right',
          }}
          anchorOrigin={{
            vertical: 'center',
            horizontal: 'right',
          }}
        />
      </MenuLoginTheme>
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
