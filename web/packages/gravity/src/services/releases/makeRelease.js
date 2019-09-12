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

import { map } from 'lodash';
import { displayDateTime } from 'gravity/lib/dateUtils';

export const StatusEnum = {
  UNKNOWN: 'UNKNOWN',
  DEPLOYED: 'DEPLOYED',
  DELETED: 'DELETED',
  SUPERSEDED: 'SUPERSEDED',
  FAILED: 'FAILED',
  DELETING: 'DELETING',
  PENDING_INSTALL: 'PENDING_INSTALL',
  PENDING_UPGRADE: 'PENDING_UPGRADE',
  PENDING_ROLLBACK: 'PENDING_ROLLBACK',
}

export default function makeRelease(json){
  const {
    name,
    namespace,
    description,
    chartName,
    chartVersion,
    appVersion: version,
    status,
    updated,
    icon,
    endpoints,
  } = json;

  const id = `${namespace}/${name}/${version}`;

  return {
    id,
    name,
    namespace,
    description,
    chartName,
    chartVersion,
    version,
    status,
    endpoints: map(endpoints, makeEndpoint),
    updated: new Date(updated),
    updatedText: displayDateTime(updated),
    icon,
  }
}

function makeEndpoint(json){
  const { name, description = [], addresses, } = json
  return {
    name,
    description,
    addresses,
  }
}