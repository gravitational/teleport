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
import makeAcl from './makeAcl';
import { User, AccessStrategy } from './types';
import { makeCluster } from '../clusters';

export default function makeUser(json): User {
  const [
    username,
    authType,
    aclJSON,
    clusterJSON,
    accessStrategyJSON,
  ] = at(json, [
    'userName',
    'authType',
    'userAcl',
    'cluster',
    'accessStrategy',
  ]);

  const cluster = makeCluster(clusterJSON);
  const acl = makeAcl(aclJSON);
  const accessStrategy = makeAccessStrategy(accessStrategyJSON);

  return {
    username,
    authType,
    acl,
    cluster,
    accessStrategy,
  };
}

function makeAccessStrategy(json): AccessStrategy {
  const { type = 'optional', prompt } = json;

  return {
    type,
    prompt,
  };
}
