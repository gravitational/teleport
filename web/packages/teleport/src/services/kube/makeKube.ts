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

import { Kube, KubeResource, KubeServer } from './types';

export function makeKube(json): Kube {
  const { name, requiresRequest, targetHealth } = json;
  const labels = json.labels || [];

  return {
    kind: 'kube_cluster',
    name,
    labels,
    users: json.kubernetes_users || [],
    groups: json.kubernetes_groups || [],
    requiresRequest,
    targetHealth: targetHealth && {
      status: targetHealth.status,
      error: targetHealth.transition_error,
      message: targetHealth.message,
    },
  };
}

export function makeKubeResource(json): KubeResource {
  const { kind, name, namespace, cluster } = json;
  const labels = json.labels || [];

  return {
    kind,
    name,
    namespace,
    labels,
    cluster,
  };
}

export function makeKubeServer(json: any): KubeServer {
  const { spec, status } = json;

  return {
    hostname: spec?.hostname,
    hostId: spec?.host_id,
    targetHealth: status &&
      status.target_health && {
        status: status.target_health.status,
        message: status.target_health.message,
        error: status.target_health.transition_error,
      },
  };
}
