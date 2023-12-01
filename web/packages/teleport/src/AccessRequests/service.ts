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

import { formatDuration } from 'date-fns';

import { AccessRequestResource } from 'teleport/Assist/types';
import { ResourceIdKind } from 'teleport/services/agents';
import api from 'teleport/services/api';
import cfg from 'teleport/config';
import {
  AccessRequest,
  CreateAccessRequest,
  DurationOption,
} from 'teleport/AccessRequests/types';
import { middleValues } from 'teleport/AccessRequests/utils';

export async function createAccessRequest(
  clusterId: string,
  roles: string[],
  resources: AccessRequestResource[],
  reason: string,
  dryRun: boolean,
  maxDuration?: Date
): Promise<AccessRequest> {
  const request: CreateAccessRequest = {
    reason,
    roles,
    resourceIds: resources.map(item => ({
      kind: item.type as ResourceIdKind,
      name: item.id,
      clusterName: clusterId,
    })),
    suggestedReviewers: [],
    maxDuration,
    dryRun,
  };

  const accessRequest = await api.post(cfg.getAccessRequestUrl(), request);

  return {
    id: accessRequest.id,
    created: new Date(accessRequest.created),
    expires: new Date(accessRequest.expires),
    requestReason: accessRequest.requestReason,
    resolveReason: accessRequest.resolveReason,
    resources: accessRequest.resources,
    reviews: accessRequest.reviews,
    roles: accessRequest.roles,
    state: accessRequest.state,
    suggestedReviewers: accessRequest.suggestedReviewers,
    thresholdNames: accessRequest.thresholdNames,
    user: accessRequest.user,
  };
}

export async function getDurationOptions(
  clusterId: string,
  roles: string[],
  resources: AccessRequestResource[]
): Promise<DurationOption[]> {
  const accessRequest = await createAccessRequest(
    clusterId,
    roles,
    resources,
    '',
    true
  );

  if (!accessRequest.sessionTTL || !accessRequest.maxDuration) {
    return [];
  }

  return middleValues(
    accessRequest.created,
    accessRequest.sessionTTL,
    accessRequest.maxDuration
  ).map(duration => ({
    value: duration.timestamp,
    label: formatDuration(duration.duration),
  }));
}
