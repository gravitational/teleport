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
import { Table, Column, Cell, TextCell } from 'design/DataTable';
import { Label, Text } from 'design';
import MenuLogin from './../MenuLogin';
import NodeMenuActon from './NodeMenuAction';

const LoginCell = ({ rowIndex, data}) => {
  const { sshLogins, id, hostname } = data[rowIndex];
  return (
    <Cell>
      <MenuLogin serverId={id || hostname} logins={sshLogins} />
    </Cell>
  );
}

export const ActionCell = ({ rowIndex, onDelete, data}) => {
  const node = data[rowIndex];
  return (
    <Cell>
      <NodeMenuActon onDelete={() => onDelete(node)} />
    </Cell>
  )
}

function Detail({ children }){
  return (
    <Text typography="subtitle2" color="text.primary">{children}</Text>
  )
}

const NameCell = ({ rowIndex, data }) => {
  const { k8s, instanceType } = data[rowIndex];
  const { cpu, osImage, memory, name } = k8s;

  // show empty cell when k8s data is not available
  if(!name){
    return ( <Cell/>)
  }

  const desc = `CPU: ${cpu}, RAM: ${memory}, OS: ${osImage}`;
  return (
    <Cell>
      <Text typography="body2" mb="2" bold>{name}</Text>
      <Detail>{desc}</Detail>
      { instanceType && <Detail>Instance Type: {instanceType} </Detail> }
    </Cell>
  )
};

export function LabelCell({ rowIndex, data }){
  const { k8s } = data[rowIndex];
  const labels  = k8s.labels || {};
  const $labels = Object.getOwnPropertyNames(labels).map(name => (
    <Label mb="1" mr="1" key={name} kind="secondary">
      {`${name}: ${labels[name]}`}
    </Label>
  ));

  return (
    <Cell>
      {$labels}
    </Cell>
  )
}

class NodeList extends React.Component {
  render() {
    const { nodes, onDelete } = this.props;
    return (
      <StyledTable data={nodes}>
        <Column
          header={<Cell>Session</Cell> }
          cell={<LoginCell /> }
        />
        <Column
          header={<Cell>Name</Cell> }
          cell={<NameCell /> }
        />
        <Column
          columnKey="hostname"
          header={<Cell>Address</Cell> }
          cell={<TextCell/> }
        />
        <Column
          columnKey="displayRole"
          header={<Cell>Profile</Cell> }
          cell={<TextCell/> }
        />
        <Column
          header={<Cell>Labels</Cell> }
          cell={<LabelCell /> }
        />
        <Column
          header={<Cell>Actions</Cell> }
          cell={ <ActionCell onDelete={onDelete} /> }
        />
      </StyledTable>
    )
  }
}

const StyledTable = styled(Table)`
  & > tbody > tr > td  {
    vertical-align: baseline;
  }
`

export default NodeList;
