/*
Copyright 2022 Gravitational, Inc.

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

import api from 'teleport/services/api';
import cfg from 'teleport/config';

export const LINK_LABEL = 'teleport.internal/link';

export type ClusterAlert = {
  kind: string;
  version: string;
  metadata: {
    name: string;
    labels: { [key: string]: string }; //"teleport.internal/alert-on-login": "yes",
    expires: string; //2022-08-31T17:26:05.728149Z
  };
  spec: {
    severity: number;
    message: string;
    created: string; //2022-08-31T17:26:05.728149Z
  };
};

export const alertNames = {
  RELEASE_ALERT_ID: 'upgrade-suggestion',
  SEC_ALERT_ID: 'security-patch-available',
  VER_IN_USE: 'teleport.internal/ver-in-use',
};

export function fetchClusterAlerts(clusterId: string) {
  const url = cfg.getClusterAlertsUrl(clusterId);
  return api.get(url).then(json => {
    let alerts = json.alerts;
    if (!Array.isArray(alerts)) {
      alerts = [];
    }
    return alerts as ClusterAlert[];
  });
}
