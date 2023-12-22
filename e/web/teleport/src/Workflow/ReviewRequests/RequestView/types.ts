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

/** Subset of `AccessList` properties required to show a suggestion. */
export type SuggestedAccessList = Pick<
  AccessList,
  'id' | 'title' | 'description' | 'grants'
>;

export type SubmitReview = {
  state: RequestState;
  reason: string;
  promotedToAccessList?: SuggestedAccessList;
};
