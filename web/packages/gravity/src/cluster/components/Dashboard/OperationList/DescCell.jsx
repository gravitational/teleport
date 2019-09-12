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
import { at } from 'lodash';
import { Cell } from 'design/DataTable';
import { StatusEnum, OpTypeEnum } from 'gravity/services/operations';
import Progress from './Progress';

export default function DescCell({
  rowIndex,
  data,
  progress,
  onFetchProgress,
  nodes,
}) {
  const { isSession, session, operation } = data[rowIndex];
  if (isSession) {
    return renderSession(session, nodes);
  }

  return renderOperation(operation, progress, onFetchProgress);
}

function renderSession(session, nodes) {
  const { serverId, login } = session;
  const server = nodes[serverId];
  const hostname = server ? server.hostname : serverId;

  return <Cell>Session started at {`${login}@${hostname}`}</Cell>;
}

function renderOperation(operation, progress, onFetchProgress) {
  const { status, id } = operation;

  if (status === StatusEnum.PROCESSING) {
    return (
      <Cell>
        <Progress opId={id} progress={progress[id]} onFetch={onFetchProgress} />
      </Cell>
    );
  }

  const description = `${getOpDescPrefix(status)} ${getOpDescription(
    operation
  )}`;

  return <Cell>{description}</Cell>;
}

function getOpDescPrefix(type) {
  switch (type) {
    case StatusEnum.COMPLETED:
      return 'Completed';
    case StatusEnum.FAILED:
      return 'Failed';
    case StatusEnum.PROCESSING:
      return 'In progress';
  }

  return '';
}

function getOpDescription(operation) {
  switch (operation.type) {
    case OpTypeEnum.OPERATION_UPDATE:
      const [app] = at(operation, 'update.update_package');
      return `updating to ${app}`;
    case OpTypeEnum.OPERATION_INSTALL:
      return 'installing this cluster';
    case OpTypeEnum.OPERATION_EXPAND:
      return 'adding a server';
    case OpTypeEnum.OPERATION_SHRINK:
      return 'removing a server';
    case OpTypeEnum.OPERATION_UNINSTALL:
      return 'uninstalling this cluster';
    default:
      return `unknown`;
  }
}
