/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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

  return middleValues(accessRequest.sessionTTL, accessRequest.maxDuration).map(
    duration => ({
      value: duration.timestamp,
      label: formatDuration(duration.duration),
    })
  );
}
