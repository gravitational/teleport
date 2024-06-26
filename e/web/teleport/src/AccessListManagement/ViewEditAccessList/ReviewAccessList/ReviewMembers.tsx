import React from 'react';
import { Box, Text } from 'design';
import { pluralize } from 'shared/utils/text';

import {
  AccessListGrant,
  AccessListMember,
} from 'e-teleport/services/accessmanagement';

import { TraitConvenience } from '../../Traits';
import { AccessListMemberTable } from '../Members/MembersList';
import { DeleteMemberWarning } from '../DeleteUserConfirmDialog';

import { List } from './Shared';
import { getMembersDeleted } from './utils';

export type Grant = Omit<TraitConvenience, 'traitList'> &
  Omit<AccessListGrant, 'traits'>;

type Props = {
  editedMembers: AccessListMember[];
  onDeleteMember(member: AccessListMember): void;
  originalMembers: AccessListMember[];
  isOkta: boolean;
};

export function ReviewMembers({
  editedMembers,
  onDeleteMember,
  originalMembers,
  isOkta,
}: Props) {
  const numMembersDeleted = getMembersDeleted(
    originalMembers,
    editedMembers
  ).length;

  return (
    <>
      <Text fontSize={4} mb={3}>
        Members
      </Text>
      {isOkta && <DeleteMemberWarning isReviewing={true} />}
      <AccessListMemberTable
        members={editedMembers}
        canEditMembers={true}
        onDeleteMember={onDeleteMember}
        hideIneligibleReason={true}
        isReviewing={true}
      />
      <Box mt={5} mb={-8}>
        <Text fontSize={4}>Changes</Text>
        <List>
          <li>
            {numMembersDeleted} {pluralize(numMembersDeleted, 'member')} revoked
          </li>
        </List>
      </Box>
    </>
  );
}
