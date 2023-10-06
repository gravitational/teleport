import type { ResourceIdKind } from 'teleport/services/agents';

export type RequestState =
  | 'NONE'
  | 'PENDING'
  | 'APPROVED'
  | 'DENIED'
  | 'APPLIED'
  | 'PROMOTED'
  | '';

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
}

export interface AccessRequestReview {
  author: string;
  roles: string[];
  state: RequestState;
  reason: string;
  createdDuration: string;
  promotedAccessListTitle?: string;
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
  kind: ResourceIdKind;
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

export interface CreateAccessRequest {
  reason?: string;
  roles?: string[];
  resourceIds?: ResourceId[];
  suggestedReviewers?: string[];
  maxDuration?: Date;
  requestTTL?: Date;
  dryRun?: boolean;
}

export interface UpdateAccessRequest {
  state: RequestState;
  reason?: string;
  roles?: string[];
  id: string;
}

export interface PromoteAccessRequest {
  accessListName: string;
  reason: string;
}

export interface AccessRequestFilter {
  user?: string;
}
