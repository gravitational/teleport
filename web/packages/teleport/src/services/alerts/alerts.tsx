/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import cfg from 'teleport/config';
import api from 'teleport/services/api';

export const LINK_DESTINATION_LABEL = 'teleport.internal/link';
export const LINK_TEXT_LABEL = 'teleport.internal/link-text';

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
