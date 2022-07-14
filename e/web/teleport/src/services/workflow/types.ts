import type { AgentIdKind } from 'teleport/services/agents';

export type RequestState =
  | 'NONE'
  | 'PENDING'
  | 'APPROVED'
  | 'DENIED'
  | 'APPLIED'
  | '';

export interface AccessRequest {
  id: string;
  state: RequestState;
  user: string;
  expires: Date;
  expiresDuration: string;
  created: Date;
  createdDuration: string;
  roles: string[];
  requestReason: string;
  resolveReason: string;
  reviewers: AccessRequestReviewer[];
  reviews: AccessRequestReview[];
  thresholdNames: string[];
  resourceIds: ResourceId[];
}

export interface AccessRequestReview {
  author: string;
  roles: string[];
  state: RequestState;
  reason: string;
  createdDuration: string;
}

export interface AccessRequestReviewer {
  name: string;
  state: RequestState;
}

// ResourceID is a unique identifier for a teleport resource.
export type ResourceId = {
  // kind is the resource (agent) kind.
  kind: AgentIdKind;
  // name is the name of the specific resource.
  name: string;
  // clusterName is the name of cluster.
  clusterName: string;
};

export interface CreateAccessRequest {
  reason?: string;
  roles?: string[];
  resourceIds?: ResourceId[];
  suggestedReviewers?: string[];
}

export interface UpdateAccessRequest {
  state: RequestState;
  reason?: string;
  roles?: string[];
}

export interface AccessRequestFilter {
  user?: string;
}
