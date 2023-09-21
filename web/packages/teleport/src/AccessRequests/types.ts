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
