import React from 'react';
import { Text } from 'design';

import { RoleOption, UserOption } from '../Shared';

import {
  EligibilityRolesFieldSelect,
  EligibleUsersFieldSelectAndCreate,
} from './Shared';

type Props = {
  fetchedRoleOpts: RoleOption[];
  isDisabled: boolean;
  setOwners(m: Owners): void;
  owners: Owners;
};

export type Owners = {
  selectedRolesRequired: RoleOption[];
  eligibleOwners: UserOption[];
  selectedOwners: UserOption[];
};

export const OwnersSection = ({
  fetchedRoleOpts,
  isDisabled,
  owners,
  setOwners,
}: Props) => {
  return (
    <>
      <Text fontSize="18px" mb={2}>
        List Owners
      </Text>
      <EligibilityRolesFieldSelect
        requiredErrMsg={`Owner eligibility role's are required`}
        options={fetchedRoleOpts}
        isDisabled={isDisabled}
        onChange={(option: RoleOption[]) =>
          setOwners({
            ...owners,
            selectedRolesRequired: option || [],
          })
        }
        selected={owners.selectedRolesRequired}
      />
      <EligibleUsersFieldSelectAndCreate
        selected={owners.selectedOwners || []}
        isDisabled={isDisabled}
        onChange={vals => setOwners({ ...owners, selectedOwners: vals || [] })}
        options={owners.eligibleOwners}
        label="Add Eligible List Owners"
        requiredErrMsg="Eligible owners are required"
      />
    </>
  );
};
