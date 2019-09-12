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
import { Cell } from 'design/DataTable';
import { Flex, Label, LabelState } from 'design';
import { displayDateTime } from 'gravity/lib/dateUtils';
import { UserStatusEnum } from 'gravity/services/enums'
import ActionMenu from './ActionMenu';

const RoleLabel = ({ name }) => (
  <Label title={name} kind="secondary" mb="1" mr="1">
    {name}
  </Label>
)

export const StatusCell = ({ rowIndex, data, ...props }) => {
  let content = 'unknown:';
  const {status, created} = data[rowIndex];
  const createdDate = created ? displayDateTime(created) : 'unknown';

  if(status === UserStatusEnum.ACTIVE){
    content = createdDate;
  }

  if(status === UserStatusEnum.INVITED){
    content = (
      <LabelState>
        invited on: {createdDate}
      </LabelState>
    )
  }

  return (
    <Cell {...props}>
      <Flex dir="row" style={{ minWidth: '150px' }}>
        {content}
      </Flex>
    </Cell>
  )
};

export const UserIdCell = ({ rowIndex, data, ...props }) => {
  const { userId } = data[rowIndex];
  return (
    <Cell {...props}>
      {userId}
    </Cell>
  )
}

export const RoleCell = ({ roleLabels, rowIndex, data, ...props }) => {
  const { roles } = data[rowIndex];

  const roleDisplayNames = roles.map(name =>
    getRoleDisplayName(name, roleLabels));

  let $content = roleDisplayNames.map((r, index) =>
    <RoleLabel key={index} name={r} />);

  if ($content.length === 0) {
     $content = (
      <small>no assigned roles </small>
    );
  }

  return (
    <Cell {...props}>
      <Flex alignItems="center" flexDirection="row" style={{ flexWrap: "wrap" }}>
        {$content}
      </Flex>
    </Cell>
  )
}

export const ButtonCell = ({ rowIndex, onReset, onEdit, onDelete, data, ...props }) => {
  const { status, userId, owner } = data[rowIndex];
  const isInvite = status === UserStatusEnum.INVITED;
  return (
    <Cell {...props}>
      <ActionMenu owner={owner} userId={userId} isInvite={isInvite}
        onEdit={onEdit}
        onReset={onReset}
        onDelete={onDelete}
      />
    </Cell>
  )
}

function getRoleDisplayName(rName, roleLabels){
  const rlabel = roleLabels.find(rl => rl.value === rName);
  if (rlabel) {
    return rlabel.label;
  }

  return rName;
}