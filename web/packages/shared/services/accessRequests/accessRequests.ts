/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { RequestableResourceKind } from 'shared/components/AccessRequests/NewRequest/resource';

export type RequestState =
  | 'NONE'
  | 'PENDING'
  | 'APPROVED'
  | 'DENIED'
  | 'APPLIED'
  | 'PROMOTED'
  | '';

export enum RequestKind {
  UNDEFINED = 0,
  SHORT_TERM = 1,
  LONG_TERM = 2,
}

/**
 * LongTermResourceGrouping contains information about how resources can be grouped
 * for long-term Access Requests.
 */
export interface LongTermResourceGrouping {
  /**
   * canProceed represents the validity of the long-term request. If all requested
   * resources cannot be grouped together, this will be false.
   */
  canProceed: boolean;
  /**
   * validationMessage is a user-friendly message explaining any grouping error if `canProceed` is false
   */
  validationMessage?: string;
  /**
   * OptimalGrouping contains requested resources that form the largest possible group
   * that can be requested together for long-term access. If empty, none of the
   * requested resources are possible to grant long-term access to.
   */
  optimalGrouping: ResourceId[];
  /**
   * GroupedByAccessList maps applicable Access Lists by ID to the resources they can grant
   */
  groupedByAccessList: { [key: string]: ResourceId[] };
}

export interface AccessRequest {
  id: string;
  state: RequestState;
  user: string;
  expires: Date;
  expiresDuration: string;
  created: Date;
  createdDuration: string;
  maxDuration: Date;
  maxDurationText: string;
  requestTTL: Date;
  requestTTLDuration: string;
  sessionTTL: Date;
  sessionTTLDuration: string;
  roles: string[];
  requestReason: string;
  resolveReason: string;
  reviewers: AccessRequestReviewer[];
  reviews: AccessRequestReview[];
  thresholdNames: string[];
  resources: Resource[];
  promotedAccessListTitle?: string;
  assumeStartTime?: Date;
  assumeStartTimeDuration?: string;
  reasonMode: string;
  reasonPrompts: string[];
  requestKind?: RequestKind;
  longTermResourceGrouping?: LongTermResourceGrouping;
}

export interface AccessRequestReview {
  author: string;
  roles: string[];
  state: RequestState;
  reason: string;
  createdDuration: string;
  promotedAccessListTitle?: string;
  assumeStartTime?: Date;
}

export interface AccessRequestReviewer {
  name: string;
  state: RequestState;
}

export type Resource = {
  id: ResourceId;
  details?: ResourceDetails;
};

// ResourceID is a unique identifier for a teleport resource.
export type ResourceId = {
  // kind is the resource (agent) kind.
  kind: RequestableResourceKind;
  // name is the name of the specific resource.
  name: string;
  // clusterName is the name of cluster.
  clusterName: string;
  // subResourceName is the sub resource belonging to resource "name" the user
  // is allowed to access.
  subResourceName?: string;
};

// ResourceDetails holds optional details for a resource.
export type ResourceDetails = {
  // hostname is the resource hostname.
  // TODO(mdwn): Remove hostname as it's no longer used.
  hostname?: string;
  friendlyName?: string;
};
