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
import { Acl } from './types';

export default function makeAcl(json): Acl {
  json = json || {};
  const logins = makeLogins(json.sshLogins);
  const windowsLogins = json.windowsLogins || [];
  const authConnectors = json.authConnectors || defaultAccess;
  const trustedClusters = json.trustedClusters || defaultAccess;
  const roles = json.roles || defaultAccess;
  const sessions = json.sessions || defaultAccess;
  const events = json.events || defaultAccess;
  const users = json.users || defaultAccess;
  const appServers = json.appServers || defaultAccess;
  const kubeServers = json.kubeServers || defaultAccess;
  const tokens = json.tokens || defaultAccess;
  const accessRequests = json.accessRequests || defaultAccess;
  const billing = json.billing || defaultAccess;
  const dbServers = json.dbServers || defaultAccess;
  const desktopServers = json.desktopServers || defaultAccess;

  return {
    logins,
    windowsLogins,
    authConnectors,
    trustedClusters,
    roles,
    sessions,
    events,
    users,
    appServers,
    kubeServers,
    tokens,
    accessRequests,
    billing,
    dbServers,
    desktopServers,
  };
}

export const defaultAccess = {
  list: false,
  read: false,
  edit: false,
  create: false,
  remove: false,
};
