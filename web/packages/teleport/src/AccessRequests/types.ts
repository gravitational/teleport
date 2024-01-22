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

import type { Option } from 'shared/components/Select';

import type { ResourceIdKind } from 'teleport/services/agents';

export type DurationOption = Option<number>;

export interface Resource {
  id: {
    kind: ResourceIdKind;
    name: string;
    clusterName: string;
    subResourceName?: string;
  };
  details?: {
    hostname?: string;
    friendlyName?: string;
  };
}

type RequestState = 'NONE' | 'PENDING' | 'APPROVED' | 'DENIED' | 'APPLIED' | '';

export interface ResourceId {
  kind: ResourceIdKind;
  name: string;
  clusterName: string;
  subResourceName?: string;
}

export interface AccessRequest {
  id: string;
  created: Date;
  expires: Date;
  maxDuration?: Date;
  requestReason: string;
  resolveReason: string;
  resources: Resource[];
  reviews: string[];
  roles: string[];
  sessionTTL?: Date;
  state: RequestState;
  suggestedReviewers: string[] | null;
  thresholdNames: string[];
  user: string;
}

export interface CreateAccessRequest {
  reason?: string;
  roles?: string[];
  resourceIds?: ResourceId[];
  suggestedReviewers?: string[];
  maxDuration?: Date;
  dryRun?: boolean;
}
