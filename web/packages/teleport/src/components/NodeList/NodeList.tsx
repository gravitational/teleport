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
import styled from 'styled-components';
import Table, { Cell, LabelCell } from 'design/DataTableNext';
import MenuSshLogin, { LoginItem } from 'shared/components/MenuSshLogin';
import { Node } from 'teleport/services/nodes';

function NodeList(props: Props) {
  const { nodes = [], onLoginMenuOpen, onLoginSelect, pageSize = 100 } = props;

  return (
    <StyledTable
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
          render: ({ addr, tunnel }) => (
            <AddressCell addr={addr} tunnel={tunnel} />
          ),
        },
        {
          key: 'tags',
          headerText: 'Labels',
          render: ({ tags }) => <LabelCell data={tags} />,
        },
        {
          altKey: 'connect-btn',
          render: ({ id }) => (
            <LoginCell
              onOpen={onLoginMenuOpen}
              onSelect={onLoginSelect}
              serverId={id}
            />
          ),
        },
      ]}
      emptyText="No Nodes Found"
      data={nodes}
      pagination={{
        pageSize,
      }}
      isSearchable
    />
  );
}

const LoginCell = ({
  onSelect,
  onOpen,
  serverId,
}: {
  onSelect?: (e: React.SyntheticEvent, login: string, serverId: string) => void;
  onOpen: (serverId: string) => LoginItem[];
  serverId: string;
}) => {
  function handleOnOpen() {
    return onOpen(serverId);
  }

  function handleOnSelect(e: React.SyntheticEvent, login: string) {
    if (!onSelect) {
      return [];
    }

    return onSelect(e, login, serverId);
  }

  return (
    <Cell align="right">
      <MenuSshLogin
        onOpen={handleOnOpen}
        onSelect={handleOnSelect}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'right',
        }}
        anchorOrigin={{
          vertical: 'center',
          horizontal: 'right',
        }}
      />
    </Cell>
  );
};

export const AddressCell = ({ addr, tunnel }) => (
  <Cell>{tunnel ? renderTunnel() : addr}</Cell>
);

function renderTunnel() {
  return (
    <span
      style={{ cursor: 'default' }}
      title="This node is connected to cluster through reverse tunnel"
    >{`⟵ tunnel`}</span>
  );
}

const StyledTable = styled(Table)`
  & > tbody > tr > td {
    vertical-align: baseline;
  }
` as typeof Table;

type Props = {
  nodes: Node[];
  onLoginMenuOpen(serverId: string): { login: string; url: string }[];
  onLoginSelect(e: React.SyntheticEvent, login: string, serverId: string): void;
  pageSize?: number;
};

export default NodeList;
