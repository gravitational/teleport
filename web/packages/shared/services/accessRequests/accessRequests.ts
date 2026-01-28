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
  Undefined = 0,
  ShortTerm = 1,
  LongTerm = 2,
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
   * recommendedAccessList is the name of the Access List that would provide
   * access to the most resources. If multiple Access Lists provide the same
   * number of resources, the first one found will be used.
   */
  recommendedAccessList?: string;
  /**
   * accessListToResources maps applicable Access List names to the resources they can grant,
   * including the optimal grouping.
   */
  accessListToResources: { [key: string]: ResourceId[] };
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

/**
 * Resource represents a {@link ResourceId} with optional additional details
 * such as {@link ResourceDetails} and/or {@link ResourceConstraints} set by Proxy.
 */
export type Resource = {
  id: ResourceId;
  details?: ResourceDetails;
  constraints?: ResourceConstraints;
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

/**
 * Represents a {@link ResourceId} in an Access Request-related context,
 * where additional information such as {@link ResourceConstraints} may be provided.
 */
export type ResourceAccessId = {
  id: ResourceId;
  constraints?: ResourceConstraints;
};

type AwsConsoleConstraints = {
  role_arns: string[];
};

type BaseResourceConstraints = {
  version?: 'v1';
};

/**
 * ResourceConstraints mirrors the gRPC-generated ResourceConstraints struct,
 * with a `oneof details`: exactly one detail variant should be present.
 */
export type ResourceConstraints = BaseResourceConstraints &
  (
    | {
        aws_console: AwsConsoleConstraints;
      }
    | {
        aws_console?: never;
      }
  );

type KeysOfUnion<T> = T extends T ? keyof T : never;
type DetailKeys<TUnion, TBase extends object> = Exclude<
  KeysOfUnion<TUnion>,
  keyof TBase
>;
type StringKeys<T> = Extract<T, string>;

/**
 * ResourceConstraintsKind mirrors the fields assignable
 * to the gRPC-generated ResourceConstraints struct's `details`.
 */
export type ResourceConstraintsKind = StringKeys<
  DetailKeys<ResourceConstraints, BaseResourceConstraints>
>;

/**
 * ResourceConstraintsVariant narrows {@link ResourceConstraints} to the provided
 * {@link ResourceConstraintsKind}.
 */
export type ResourceConstraintsVariant<K extends ResourceConstraintsKind> =
  Extract<ResourceConstraints, Record<K, unknown>>;

/**
 * Augments a resource-like object `R` with strongly-typed {@link ResourceConstraints}
 * based on the specified detail variant key.
 */
export type WithResourceConstraints<
  K extends ResourceConstraintsKind,
  R extends object = object,
> = R & { constraints: ResourceConstraintsVariant<K> };

const isConstraintsVariant = <K extends ResourceConstraintsKind>(
  c: ResourceConstraints | undefined,
  key: K
): c is ResourceConstraintsVariant<K> =>
  !!c && typeof c === 'object' && key in c && !!c[key];

/**
 * Narrows `item.constraints` to the given variant (e.g., 'awsConsole').
 */
export const hasResourceConstraints = <
  K extends ResourceConstraintsKind,
  T extends { constraints?: ResourceConstraints },
>(
  item: T,
  key: K
): item is T & { constraints: ResourceConstraintsVariant<K> } =>
  isConstraintsVariant(item.constraints, key);

declare const __resourceIDBrand: unique symbol;

/**
 * Resource identifier in the format "cluster/kind/name".
 * Use {@link getResourceIDString} to construct; this is a branded type
 * to ensure compile-time type safety.
 */
export type ResourceIDString = `${string}/${string}/${string}` & {
  [__resourceIDBrand]: 'ResourceIDString';
};

/**
 * Creates a {@link ResourceIDString} from its component parts.
 */
export const getResourceIDString = ({
  cluster,
  kind,
  name,
}: {
  cluster: string;
  kind: string;
  name: string;
}): ResourceIDString => `${cluster}/${kind}/${name}` as ResourceIDString;

/**
 * Maps supported {@link ResourceIDString}s to their {@link ResourceConstraints}.
 */
export type ResourceConstraintsMap = Record<
  ResourceIDString,
  ResourceConstraints
>;
