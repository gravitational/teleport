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

import makeLogins from './makeLogins';
import { Access, Acl } from './types';

export default function makeAcl(json): Acl {
  json = json || {};
  const logins = makeLogins(json.sshLogins);
  const authConnectors = makeAccess(json.authConnectors);
  const trustedClusters = makeAccess(json.trustedClusters);
  const roles = makeAccess(json.roles);
  const sessions = makeAccess(json.sessions);
  const events = makeAccess({ list: true });

  return {
    logins,
    authConnectors,
    trustedClusters,
    roles,
    sessions,
    events,
  };
}

function makeAccess(json): Access {
  json = json || {};
  const {
    list = false,
    read = false,
    edit = false,
    create = false,
    remove = false,
  } = json;

  return {
    list,
    read,
    edit,
    create,
    remove,
  };
}
