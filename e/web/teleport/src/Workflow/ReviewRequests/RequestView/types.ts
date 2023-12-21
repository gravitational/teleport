import { AccessList } from 'e-teleport/services/accessmanagement';
import { RequestState } from 'e-teleport/services/workflow';

export type RequestFlags = {
  /** Decides if the button to assume a request should be visible. */
  canAssume: boolean;
  /**
   * Decides if the button to assume a request should be disabled
   * and determines the text on it.
   */
  isAssumed: boolean;
  canReview: boolean;
  canDelete: boolean;
  ownRequest: boolean;
  isPromoted: boolean;
};

export type SubmitReview = {
  state: RequestState;
  reason: string;
  promotedToAccessList?: AccessList;
};
