import { Option } from 'shared/components/Select';

import { AllUserTraits } from 'teleport/services/user';

import { ReviewDayOfMonthOption, ReviewFrequencyOption } from '../Shared/Audit';
import {
  HybridUserOption,
  MemberSelection,
  UserOption,
} from '../Shared/Shared';
import { TraitLabel } from '../Traits';

export type Owners = {
  selectedRolesRequired: Option[];
  eligibleOwners: UserOption[];
  selectedOwners: Option<MemberSelection>[];
  traitLabels: TraitLabel[];
  traitLookup: AllUserTraits;
};

export type Members = {
  selectedRolesRequired: Option[];
  eligibleMembers: HybridUserOption[];
  selectedMembers: Option<MemberSelection>[];
  traitLabels: TraitLabel[];
  traitLookup: AllUserTraits;
};

export type Grant = {
  rolesToGrant: Option[];
  traitsToGrant: TraitLabel[];
};

export type Spec = {
  title: string;
  description: string;
  reviewDayOfMonth: ReviewDayOfMonthOption;
  reviewFrequency: ReviewFrequencyOption;
  auditStartDate: Date;
};

export type NavView = {
  Component?: React.ReactElement;
};

/**
 * - users: are local Teleport users.
 * - access-lists: are other access lists added as "nested" access list.
 * - okta-access-lists: similar to "access-lists" but access lists
 * that orginated from okta
 */
export type UserType = 'access-lists' | 'users' | 'okta-access-lists';

export type UserTypeOption = {
  value: UserType;
  label: string;
  disabled?: boolean;
};
