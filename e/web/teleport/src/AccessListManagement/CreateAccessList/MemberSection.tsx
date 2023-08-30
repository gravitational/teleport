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
  setMembers(m: Members): void;
  members: Members;
  noAccess: boolean;
};

export type Members = {
  selectedRolesRequired: Option[];
  eligibleMembers: UserOption[];
  selectedMembers: UserOption[];
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
