import { Option } from 'shared/components/Select';

import { type ScopedRoleGrant } from 'e-teleport/services/accessmanagement';
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
  scopedRolesToGrant: ScopedRoleGrant[];
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

/**
 * userTypeOptions does not support okta yet since backend support
 * is lacking:
 *
 * TODO(kimlisa): add support for advanced filtering for the listing
 * access lists endpoint e.g. "list access list not having okta origin"
 * (might need predicate support).
 */
export const userTypeOptions: UserTypeOption[] = [
  {
    value: 'access-lists',
    label: 'Access Lists',
  },
  {
    value: 'users',
    label: 'Users',
  },
];

export type EligibleUsersFieldSelectProps = {
  selected: Option<MemberSelection>[];
  isDisabled: boolean;
  onChange(opts: Option<MemberSelection>[]): void;
  loadOptions(input: string): Promise<HybridUserOption[]>;
  label: string;
  requiredErrMsg?: string;
  disableCreate?: boolean;
  placeholder?: string;
  autoFocus?: boolean;
  noOptionsMsg?: string;
  // If undefined, userKind refers to Teleport users.
  userKind?: 'nested-access-list' | undefined;
  key?: string;
};

export const cancelPrompt =
  'Are you sure you want to exit the "Create New Access List" workflow? You’ll have to start from the beginning next time.';
