import React from 'react';
import { Box, Text } from 'design';
import { Option } from 'shared/components/Select';
import { AllUserTraits } from 'teleport/services/user';

import { HybridUserOption, UserOption } from '../Shared';
import {
  TraitLabel,
  TraitsCreator,
  convertTraitLabelsToAllUserTraits,
} from '../Traits';

import {
  EligibilityOrGrantRolesFieldSelectAndCreate,
  EligibleUsersFieldSelectAndCreate,
} from './Shared';

type Props = {
  roleOptions: Option[];
  isDisabled: boolean;
  setMembers(m: Members): void;
  members: Members;
  noAccess: boolean;
};

export type Members = {
  selectedRolesRequired: Option[];
  eligibleMembers: UserOption[];
  selectedMembers: HybridUserOption[];
  traitLabels: TraitLabel[];
  traitLookup: AllUserTraits;
};

export const MembersSection = ({
  roleOptions,
  isDisabled,
  setMembers,
  members,
  noAccess,
}: Props) => {
  return (
    <>
      <Text fontSize="18px" mb={2}>
        Members (Optional)
      </Text>
      <Box mb={3} mt={3}>
        <TraitsCreator
          kind="Member"
          traitLabels={members.traitLabels}
          isDisabled={isDisabled}
          updateTraitLabels={(traitLabels: TraitLabel[]) =>
            setMembers({
              ...members,
              traitLabels,
              traitLookup: convertTraitLabelsToAllUserTraits(traitLabels),
            })
          }
        />
      </Box>
      <EligibilityOrGrantRolesFieldSelectAndCreate
        editKind="Member"
        optional={true}
        options={roleOptions}
        isDisabled={isDisabled}
        onChange={(option: Option[]) =>
          setMembers({
            ...members,
            selectedRolesRequired: option || [],
          })
        }
        selected={members.selectedRolesRequired}
      />
      <EligibleUsersFieldSelectAndCreate
        selected={members.selectedMembers || []}
        isDisabled={isDisabled}
        onChange={vals =>
          setMembers({ ...members, selectedMembers: vals || [] })
        }
        options={members.eligibleMembers}
        label="Add Members"
        noEligibleUsersFromNoAccess={noAccess}
      />
    </>
  );
};
