/*
Copyright 2015 Gravitational, Inc.

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

import telecfg from 'telebase-app/config';
import { formatPattern } from 'telebase-app/lib/patternUtils';

telecfg.init({

  clusterDocLink: 'http://gravitational.com/teleport/docs/admin-guide/#trusted-clusters',
  oidcDocLink: 'http://gravitational.com/teleport/docs/admin-guide/#openid-oauth2',  
  routes: {    
    settingsBase: '/web/settings',        
    settingsAuth: '/web/settings/auth',
    settingsRoles: '/web/settings/roles',
    settingsCluster: '/web/settings/clusters'
  },

  api: {                
    oidcConnectorsPath: '/v1/enterprise/oidc(/:connectorId)',            
    roles: '/v1/enterprise/roles(/:roleName)',
    clusters: '/v1/enterprise/trustedclusters',
  },
  
  getRolesUrl(roleName) {    
    if (!roleName) {
      return stripParams(telecfg.api.roles);
    } 

    return formatPattern(telecfg.api.roles, {roleName})
  },

  getClusterUrl(name) {    
    if (!name) {
      return stripParams(telecfg.api.clusters);
    } 

    return formatPattern(telecfg.api.clusters, {name})
  },
  
  getOicdConnectorsPath(connectorId) {    
    if (!connectorId) {
      return stripParams(telecfg.api.oidcConnectorsPath);
    } 

    return formatPattern(telecfg.api.oidcConnectorsPath, {connectorId} )    
  }  

})

function stripParams(pattern) {
  return pattern.replace(/\(.*\)/, '');
}

export default telecfg;
