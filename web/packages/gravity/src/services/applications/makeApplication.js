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

import { at, keys, map } from 'lodash';
import cfg from 'gravity/config';
import { displayDateTime } from 'gravity/lib/dateUtils';
import { parseWebConfig } from 'gravity/lib/paramUtils';
import makeNodeProfile from './makeNodeProfile';

export const AppKindEnum = {
  APP: 'Application',
  BUNDLE: 'Bundle',
  CLUSTER: 'Cluster',
}

export default function makeApplicatin(json){
  const [ name, version, repository ] = at(json,
    [
      'package.name',
      'package.version',
      'package.repository'
    ]
  );

  const [ created ] = at(json,
    [
      'envelope.created',
    ]
  );

  const [
    displayName,
    kind,
    logo,
    webConfigJsonStr,
    providersMap,
    licenseRequired,
    nodeProfilesJson,
    setupEndpoint
  ] = at(json,
    [
      'manifest.metadata.displayName',
      'manifest.kind',
      'manifest.logo',
      'manifest.webConfig',
      'manifest.providers',
      'manifest.license.enabled',
      'manifest.nodeProfiles',
      'manifest.installer.setupEndpoints[0]'
    ]
  );

  // node profiles
  const nodeProfiles = map(nodeProfilesJson, makeNodeProfile)
  // eula agreement used during app installation
  const [ eula = null] = at(json, 'manifest.installer.eula.source');
  // parse web config which can be used during app installation
  const config = parseWebConfig(webConfigJsonStr);
  // generate unique id
  const id = `${repository}/${name}/${version}`;
  // application installer URL
  const installUrl = cfg.getInstallNewSiteRoute(name, repository, version);
  // application stand alone installer tarbar URL
  const standaloneInstallerUrl = cfg.getStandaloneInstallerPath(name, repository, version);
  // supported providers
  const providers = keys(providersMap).map(key => ({
    name: key,
    disabled: providersMap[key].disabled
  }))

  return {
    id,
    packageId: `${repository}/${name}:${version}`,
    name,
    displayName: displayName || name,
    version,
    repository,
    installUrl,
    kind,
    standaloneInstallerUrl,
    created: new Date(created),
    createdText: displayDateTime(created),
    logo,
    config,
    providers,
    licenseRequired,
    eula,
    nodeProfiles,
    bandwagon: !!setupEndpoint
  }
}