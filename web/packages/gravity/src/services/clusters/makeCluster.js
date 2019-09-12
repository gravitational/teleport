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

import { at, map, keyBy } from 'lodash';
import { SiteStateEnum } from 'gravity/services/enums';
import cfg from 'gravity/config';
import { parseWebConfig } from 'gravity/lib/paramUtils'
import { displayDateTime } from 'gravity/lib/dateUtils';
import { makeRelease } from './../releases';
import makeLicense from './makeLicense';
import { makeNodeProfile } from './../applications';

export default function makeCluster(json){
  const [
    created_by,
    created,
    domain,
    labels,
    local,
    location,
    provider,
    releases,
    state,
  ] = at(json,
    [
      'created_by',
      'created',
      'domain',
      'labels',
      'local',
      'location',
      'provider',
      'releases',
      'state',
    ]
  );

  const [ monitoringDisabled, k8sDisabled, logsDisabled ] = at(json,
    [
      'app.manifest.extensions.monitoring.disabled',
      'app.manifest.extensions.kubernetes.disabled',
      'app.manifest.extensions.logs.disabled'
    ]
  )

  const [ logo, webConfigJsonStr, profiles ] = at(json,
    [
      'app.manifest.logo',
      'app.manifest.webConfig',
      'app.manifest.nodeProfiles'
    ]
  );

  const [ packageName, packageVersion ] = at(json,
    [
      'app.package.name',
      'app.package.version',
    ]
  );

  const [ servers ] = at(json, 'cluster_state.servers');

  const license = makeLicense(json);
  const webConfig = parseWebConfig(webConfigJsonStr);
  const installerUrl = cfg.getSiteInstallerRoute(domain);
  const siteUrl = cfg.getSiteRoute(domain);
  const apps = keyBy(map(releases, makeRelease), 'id');
  const nodeProfiles = map(profiles, makeNodeProfile);
  return {
    id: domain,
    apps,
    packageName,
    packageVersion,
    labels,
    location,
    logo,
    license,
    provider,
    webConfig,
    installerUrl,
    siteUrl,
    state,
    local,
    nodeProfiles,
    serverCount: Array.isArray(servers) ? servers.length : 0,
    status: makeStatus(state),
    created: new Date(created),
    createdText: displayDateTime(created),
    createdBy: created_by,
    features: {
      monitoringEnabled: !monitoringDisabled,
      k8sEnabled: !k8sDisabled,
      logsEnabled: !logsDisabled,
    }
  }
}

export function makeStatus(siteState){
  switch (siteState) {
    case SiteStateEnum.ACTIVE:
      return StatusEnum.READY;
    case SiteStateEnum.EXPANDING:
    case SiteStateEnum.SHRINKING:
    case SiteStateEnum.UPDATING:
    case SiteStateEnum.UNINSTALLING:
    case SiteStateEnum.INSTALLING:
      return StatusEnum.PROCESSING;
    case SiteStateEnum.FAILED:
    case SiteStateEnum.DEGRADED:
      return StatusEnum.ERROR;
    case SiteStateEnum.OFFLINE:
      return StatusEnum.OFFLINE;
   }

   return StatusEnum.UNKNOWN;
}

export const StatusEnum = {
  READY: 'ready',
  PROCESSING: 'processing',
  ERROR: 'error',
  OFFLINE: 'offline',
  UNKNOWN: 'unknown',
}