import React from 'react';
import { Text } from 'design';

import {
  AccessListGrant,
  AccessListMember,
} from 'e-teleport/services/accessmanagement';

import { TraitConvenience } from '../../Traits';
import { AccessListMemberTable } from '../Members/MembersList';

export type Grant = Omit<TraitConvenience, 'traitList'> &
  Omit<AccessListGrant, 'traits'>;

type Props = {
  editedMembers: AccessListMember[];
  onDeleteMember(member: AccessListMember): void;
};

export function ReviewMembers({ editedMembers, onDeleteMember }: Props) {
  return (
    <>
      <Text fontSize={4} mb={3}>
        Members
      </Text>
      <AccessListMemberTable
        members={editedMembers}
        canEditMembers={true}
        onDeleteMember={onDeleteMember}
        hideIneligibleReason={true}
      />
    </>
  );
}
