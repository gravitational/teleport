import { AccessList } from 'e-teleport/services/accessmanagement';
import { RequestState } from 'e-teleport/services/workflow';

export type LongTermAccess = {
  suggestedAccessLists: AccessList[];
  error: string;
};

export type SubmitReview = {
  state: RequestState;
  reason: string;
  promotedToAccessList?: AccessList;
};
