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

export interface CreateAccessRequest {
  reason?: string;
  roles?: string[];
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
