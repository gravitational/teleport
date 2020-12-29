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

import makeAcl from './makeAcl';
import { UserContext } from './types';
import { makeCluster } from '../clusters';

export default function makeUserContext(json: any): UserContext {
  json = json || {};
  const username = json.userName;
  const authType = json.authType;

  const cluster = makeCluster(json.cluster);
  const acl = makeAcl(json.userAcl);
  const accessStrategy = json.accessStrategy || defaultStrategy;
  const requestableRoles = json.requestableRoles || [];

  return {
    username,
    authType,
    acl,
    cluster,
    accessStrategy,
    requestableRoles,
  };
}

export const defaultStrategy = {
  type: 'optional',
  prompt: '',
};
