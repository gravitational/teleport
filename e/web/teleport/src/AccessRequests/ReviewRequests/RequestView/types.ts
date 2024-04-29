import { AllUserTraits } from 'teleport/services/user';

import { RequestState } from 'e-teleport/services/accessRequests';

export type RequestFlags = {
  /** Describes request is own request and request is approved */
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
export type SuggestedAccessList = {
  id: string;
  title: string;
  description?: string;
  grants: {
    roles: string[];
    traits: AllUserTraits;
  };
};

export type SubmitReview = {
  state: RequestState;
  reason: string;
  promotedToAccessList?: SuggestedAccessList;
  assumeStartTime?: Date;
};
