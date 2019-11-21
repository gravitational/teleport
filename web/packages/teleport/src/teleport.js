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
import { StoreNav, StoreNodes, StoreUser, StoreClusters } from './stores';
import { Activator } from 'shared/libs/featureBase';
import { useStore } from 'shared/libs/stores';
import cfg from 'teleport/config';
import StoreSessions from './stores/storeSessions';

class Teleport {
  storeNodes = null;
  storeClusters = null;
  storeNav = null;
  storeUser = null;
  storeSessions = null;

  constructor() {
    this.initStores();
  }

  init({ features, clusterId }) {
    cfg.setClusterId(clusterId);
    this.initStores();
    const fetchClustersPromise = this.storeClusters.fetchClusters();
    const fetchUsersPromise = this.storeUser.fetchUser();

    return Promise.all([fetchClustersPromise, fetchUsersPromise]).then(() => {
      const activator = new Activator(features);
      activator.onload({
        context: this,
      });
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

const TeleportContext = React.createContext(new Teleport());

export default TeleportContext;

export function useTeleport() {
  const value = React.useContext(TeleportContext);

  if (!value) {
    throw new Error('TeleportContext is missing a value');
  }

  return (window.teleContext = value);
}

export function useStoreNav() {
  const context = React.useContext(TeleportContext);
  return useStore(context.storeNav);
}

export function useStoreUser() {
  const context = React.useContext(TeleportContext);
  return useStore(context.storeUser);
}

export function useStoreNodes() {
  const context = React.useContext(TeleportContext);
  return useStore(context.storeNodes);
}

export function useStoreClusters() {
  const context = React.useContext(TeleportContext);
  return useStore(context.storeClusters);
}

export function useStoreSessions() {
  const context = React.useContext(TeleportContext);
  return useStore(context.storeSessions);
}
