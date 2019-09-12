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

import reactor from 'gravity/reactor';
import cfg from 'gravity/config';
import { getAcl } from 'gravity/flux/userAcl';
import userGetters from 'gravity/flux/user/getters';

const hasK8sAccess = () => {
  return getAcl().getClusterAccess().connect;
}

export function siteMonitoring() {
  return hasK8sAccess() && cfg.isSiteMonitoringEnabled();
}

export function siteK8s() {
  return hasK8sAccess() && cfg.isSiteK8sEnabled();
}

export function siteConfigMaps() {
  return hasK8sAccess() && cfg.isSiteConfigMapsEnabled();
}

export function siteLogs() {
  return cfg.isSiteLogsEnabled();
}

export function siteAccount(){
  const userStore = reactor.evaluate(userGetters.user)
  return !userStore.isSso();
}

export function siteUsers() {
  return getAcl().getUserAccess().list;
}

export function clusterEvents() {
  return getAcl().getEventAccess().list;
}

export function clusterAuthConnectors() {
  return getAcl().getConnectorAccess().list;
}

export function clusterRoles() {
  return getAcl().getRoleAccess().list;
}

export function clusterCert() {
  return getAcl().getClusterAccess().edit;
}

export function clusterLicense() {
  return cfg.isSiteLicenseEnabled()
}

export function hubLicenses(){
  return getAcl().getLicenseAccess().create;
}

export function hubClusters() {
  return getAcl().getClusterAccess().list;
}

export function hubApps() {
  return getAcl().getAppAccess().list;
}