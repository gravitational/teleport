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

import { StoreNav, StoreNodes, StoreUser, StoreClusters } from './stores';
import { Activator } from 'shared/libs/featureBase';
import cfg from 'teleport/config';
import StoreSessions from './stores/storeSessions';
import * as teleport from './types';

export default class Context implements teleport.Context {
  storeNodes: StoreNodes = null;
  storeClusters: StoreClusters = null;
  storeNav: StoreNav = null;
  storeUser: StoreUser = null;
  storeSessions: StoreSessions = null;

  constructor() {
    this.initStores();
  }

  init({
    features,
    clusterId,
  }: {
    features: teleport.Feature[];
    clusterId: string;
  }) {
    cfg.setClusterId(clusterId);
    this.initStores();
    const fetchClustersPromise = this.storeClusters.fetchClusters();
    const fetchUserPromise = this.storeUser.fetchUser();

    return Promise.all([fetchClustersPromise, fetchUserPromise]).then(() => {
      const activator = new Activator<Context>(features);
      activator.onload(this);
    });
  }

  isAccountEnabled() {
    return this.storeUser.isSso() === false;
  }

  isAuditEnabled() {
    return this.storeUser.getEventAccess().list;
  }

  isAuthConnectorEnabled() {
    return this.storeUser.getConnectorAccess().list;
  }

  isRolesEnabled() {
    return this.storeUser.getRoleAccess().list;
  }

  isTrustedClustersEnabled() {
    return this.storeUser.getTrustedClusterAccess().list;
  }

  initStores() {
    this.storeNodes = new StoreNodes();
    this.storeClusters = new StoreClusters();
    this.storeNav = new StoreNav();
    this.storeUser = new StoreUser();
    this.storeSessions = new StoreSessions();
  }
}
