import React from 'react';
import { Text } from 'design';
import { Option } from 'shared/components/Select';

import { UserOption } from '../Shared';

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
