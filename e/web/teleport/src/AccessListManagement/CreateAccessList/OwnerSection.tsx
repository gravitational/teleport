import React from 'react';
import { Text, Box } from 'design';
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
  setOwners(m: Owners): void;
  owners: Owners;
  noAccess: boolean;
};

export type Owners = {
  selectedRolesRequired: Option[];
  eligibleOwners: UserOption[];
  selectedOwners: UserOption[];
  traitLabels: TraitLabel[];
  traitLookup: TraitLookup;
};

export const OwnersSection = ({
  roleOptions,
  isDisabled,
  owners,
  setOwners,
  noAccess,
}: Props) => {
  return (
    <>
      <Text fontSize="18px" mb={2}>
        List Owners
      </Text>
      <Box mb={3} mt={3}>
        <TraitsCreator
          kind="Owner"
          traitLabels={owners.traitLabels}
          isDisabled={isDisabled}
          updateTraitLabels={(traitLabels: TraitLabel[]) =>
            setOwners({
              ...owners,
              traitLabels,
              traitLookup: convertTraitLabelsToTraitLookup(traitLabels),
            })
          }
        />
      </Box>
      <Box>
        <EligibilityOrGrantRolesFieldSelectAndCreate
          editKind="Owner"
          options={roleOptions}
          isDisabled={isDisabled}
          onChange={(option: Option[]) =>
            setOwners({
              ...owners,
              selectedRolesRequired: option || [],
            })
          }
          selected={owners.selectedRolesRequired}
        />
      </Box>
      <EligibleUsersFieldSelectAndCreate
        selected={owners.selectedOwners || []}
        isDisabled={isDisabled}
        onChange={vals => setOwners({ ...owners, selectedOwners: vals || [] })}
        options={owners.eligibleOwners}
        label="Add Eligible List Owners"
        requiredErrMsg="Eligible owners are required"
        noEligibleUsersFromNoAccess={noAccess}
      />
    </>
  );
};
