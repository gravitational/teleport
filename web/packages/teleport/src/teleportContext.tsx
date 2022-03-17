/*
Copyright 2019-2021 Gravitational, Inc.

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

import { StoreNav, StoreUserContext } from './stores';
import cfg from 'teleport/config';
import * as types from './types';
import AuditService from './services/audit';
import RecordingsService from './services/recordings';
import nodeService from './services/nodes';
import clusterService from './services/clusters';
import sshService from './services/ssh';
import ResourceService from './services/resources';
import userService from './services/user';
import appService from './services/apps';
import JoinTokenService from './services/joinToken';
import KubeService from './services/kube';
import DatabaseService from './services/databases';
import desktopService from './services/desktops';
import MfaService from './services/mfa';

class TeleportContext implements types.Context {
  // stores
  storeNav = new StoreNav();
  storeUser = new StoreUserContext();

  // features
  features: types.Feature[] = [];

  // services
  auditService = new AuditService();
  recordingsService = new RecordingsService();
  nodeService = nodeService;
  clusterService = clusterService;
  sshService = sshService;
  resourceService = new ResourceService();
  userService = userService;
  appService = appService;
  joinTokenService = new JoinTokenService();
  kubeService = new KubeService();
  databaseService = new DatabaseService();
  desktopService = desktopService;
  mfaService = new MfaService();
  isEnterprise = cfg.isEnterprise;

  init() {
    return userService.fetchUserContext().then(user => {
      this.storeUser.setState(user);
    });
  }

  getFeatureFlags() {
    const userContext = this.storeUser;

    return {
      audit: userContext.getEventAccess().list,
      recordings: userContext.getSessionsAccess().list,
      authConnector: userContext.getConnectorAccess().list,
      roles: userContext.getRoleAccess().list,
      trustedClusters: userContext.getTrustedClusterAccess().list,
      users: userContext.getUserAccess().list,
      applications: userContext.getAppServerAccess().list,
      kubernetes: userContext.getKubeServerAccess().list,
      billing: userContext.getBillingAccess().list,
      databases: userContext.getDatabaseAccess().list,
      desktops: userContext.getDesktopAccess().list,
    };
  }
}

export default TeleportContext;
