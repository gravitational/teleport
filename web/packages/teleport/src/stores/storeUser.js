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

import { Store } from 'shared/libs/stores';
import service from 'teleport/services/user';

export const AuthTypeEnum = {
  LOCAL: 'local',
  SSO: 'sso',
};

export default class StoreUser extends Store {
  state = null;

  isSso() {
    return this.state.authType === AuthTypeEnum.SSO;
  }

  getEventAccess() {
    return this.state.acl.events;
  }

  getConnectorAccess() {
    return this.state.acl.authConnectors;
  }

  getRoleAccess() {
    return this.state.acl.roles;
  }

  getLogins() {
    return this.state.acl.logins;
  }

  getTrustedClusterAccess() {
    return this.state.acl.trustedClusters;
  }

  fetchUser() {
    return service.fetchUser().then(user => {
      this.setState(user);
    });
  }
}
