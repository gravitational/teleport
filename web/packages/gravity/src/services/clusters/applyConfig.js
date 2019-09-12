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

import cfg from 'gravity/config';
import gravityEnterpriseLogo from 'design/assets/images/gravity-logo.svg';
import gravityCommunityLogo from 'design/assets/images/gravity-community-logo.svg';

export default function applyConfig(cluster){
  const {
    monitoringEnabled,
    k8sEnabled,
    logsEnabled,
  } = cluster.features;


  let logoSvg = cluster.logo || gravityEnterpriseLogo;
  if(!cfg.isEnterprise){
    logoSvg = gravityCommunityLogo;
  }

  cfg.setLogo(cluster.logo || logoSvg);
  cfg.enableSiteLicense(!!cluster.license);
  cfg.enableSiteMonitoring(monitoringEnabled);
  cfg.enableSiteK8s(k8sEnabled);
  cfg.enableSiteLogs(logsEnabled);
}