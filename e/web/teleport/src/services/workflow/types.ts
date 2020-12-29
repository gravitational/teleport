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
}

export interface CreateAccessRequest {
  reason?: string;
  roles?: string[];
}

export interface AccessRequestFilter {
  user?: string;
}
