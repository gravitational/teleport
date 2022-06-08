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
  // kind is the resource kind.
  kind: ResourceIdKind;
  // name is the name of the specific resource.
  name: string;
  // clusterName is the name of cluster.
  clusterName: string;
};

// ResourceIdKind consts are the same resource constants defined in the backend
// and is expected in the request for search based access requests.
export type ResourceIdKind =
  | 'node'
  | 'app'
  | 'db'
  | 'kube_cluster'
  | 'windows_desktop';

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
