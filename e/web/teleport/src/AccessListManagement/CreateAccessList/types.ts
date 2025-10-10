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
