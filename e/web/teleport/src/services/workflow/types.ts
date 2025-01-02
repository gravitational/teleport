import type { RequestState, ResourceId } from 'shared/services/accessRequests';

import type { AccessRequestScope } from 'teleport/services/agents';

export interface CreateAccessRequest {
  reason?: string;
  roles?: string[];
  resourceIds?: ResourceId[];
  suggestedReviewers?: string[];
  maxDuration?: Date;
  requestTTL?: Date;
  dryRun?: boolean;
  assumeStartTime?: Date;
}

export interface UpdateAccessRequest {
  state: RequestState;
  reason?: string;
  roles?: string[];
  id: string;
  assumeStartTime?: Date;
}

export interface PromoteAccessRequest {
  accessListName: string;
  reason: string;
}

export interface AccessRequestFilter {
  user?: string;
  search?: string;
  limit?: number;
  startKey?: string;
  sort?: string;
  scope?: AccessRequestScope;
}

export type {
  /** @depreacted Import `AccessRequest` directly. */
  AccessRequest,
  /** @depreacted Import `ResourceId` directly. */
  ResourceId,
} from 'shared/services/accessRequests';
