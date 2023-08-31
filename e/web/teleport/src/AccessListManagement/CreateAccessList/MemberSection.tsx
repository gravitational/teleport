import React from 'react';
import { Box, Text } from 'design';
import { Option } from 'shared/components/Select';

import { UserOption } from '../Shared';
import {
  TraitLabel,
  TraitLookup,
  TraitsCreator,
  convertTraitLabelsToTraitLookup,
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
  selectedMembers: UserOption[];
  traitLabels: TraitLabel[];
  traitLookup: TraitLookup;
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
              traitLookup: convertTraitLabelsToTraitLookup(traitLabels),
            })
          }
        />
      </Box>
      <EligibilityOrGrantRolesFieldSelectAndCreate
        editKind="Member"
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
        label="Add Eligible Members (optional)"
        noEligibleUsersFromNoAccess={noAccess}
      />
    </>
  );
};
