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
  setMembers(m: Members): void;
  members: Members;
};

export type Members = {
  selectedRolesRequired: RoleOption[];
  eligibleMembers: UserOption[];
  selectedMembers: UserOption[];
};

export const MembersSection = ({
  fetchedRoleOpts,
  isDisabled,
  setMembers,
  members,
}: Props) => {
  return (
    <>
      <Text fontSize="18px" mb={2}>
        Members (Optional)
      </Text>
      <EligibilityRolesFieldSelect
        options={fetchedRoleOpts}
        isDisabled={isDisabled}
        onChange={(option: RoleOption[]) =>
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
      />
    </>
  );
};
