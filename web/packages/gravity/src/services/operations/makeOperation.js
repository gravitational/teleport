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

import { at } from 'lodash';
import { OpTypeEnum, OpStateEnum } from 'gravity/services/enums';

export const StatusEnum = {
  PROCESSING: 'processing',
  FAILED: 'failed',
  COMPLETED: 'completed',
}

export default function makeOperation(json){
  const [
    id,
    siteId,
    created,
    createdBy,
    updated,
    state,
    type,
    provisioner,
    installExpand,
    update,
  ] = at(json, [
    'id',
    'site_domain',
    'created',
    'created_by',
    'updated',
    'state',
    'type',
    'provisioner',
    'install_expand',
    'update'
  ])

  const details = installExpand;
  const description = getDescription(json);
  const status = getStatus(state);
  return {
    id,
    siteId,
    createdBy,
    created: new Date(created),
    updated: new Date(updated),
    state,
    type,
    provisioner,
    installExpand,
    details,
    status,
    update,
    description
  }
}

function getDescription(json){
  switch (json.type) {
    case OpTypeEnum.OPERATION_UPDATE:
      const  [ app ] = at(json, 'update.update_package');
      return `Updating to ${app}`;
    case OpTypeEnum.OPERATION_INSTALL:
      return 'Installing this cluster';
    case OpTypeEnum.OPERATION_EXPAND:
      return 'Adding a server';
    case OpTypeEnum.OPERATION_SHRINK:
      return 'Removing a server';
    case OpTypeEnum.OPERATION_UNINSTALL:
      return 'Uninstalling this cluster';
    default:
      return `Unknown`;
  }
}

function getStatus(state){
  switch (state) {
    case OpStateEnum.UPDATE_IN_PROGRESS:
    case OpStateEnum.SHRINK_IN_PROGRESS:
    case OpStateEnum.EXPAND_PRECHECKS:
    case OpStateEnum.EXPAND_SETTING_PLAN:
    case OpStateEnum.EXPAND_PLANSET:
    case OpStateEnum.EXPAND_PROVISIONING:
    case OpStateEnum.EXPAND_DEPLOYING:
    case OpStateEnum.EXPAND_INITIATED:
    case OpStateEnum.READY:
      return StatusEnum.PROCESSING;
    case OpStateEnum.FAILED:
      return StatusEnum.FAILED;
    case OpStateEnum.COMPLETED:
      return StatusEnum.COMPLETED;
    default:
      return ''
    }
}