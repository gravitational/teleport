import React from 'react';
import { Box } from 'design';
import { pluralize } from 'shared/utils/text';

import { H2 } from 'design';

import {
  AccessListGrant,
  AccessListMember,
} from 'e-teleport/services/accessmanagement';

import { TraitConvenience } from '../../Traits';
import { AccessListMemberTable } from '../Members/MembersList';
import { DeleteMemberWarning } from '../DeleteUserConfirmDialog';

import { List } from './Shared';
import { getMembersDeleted } from './utils';

import type { AccessListWithNestedOwnersMembersTitles } from 'e-teleport/AccessListManagement/ViewEditAccessList/Shared';

export type Grant = Omit<TraitConvenience, 'traitList'> &
  Omit<AccessListGrant, 'traits'>;

type Props = {
  editedMembers: AccessListWithNestedOwnersMembersTitles['members'];
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
      <H2 mb={3}>Members</H2>
      {isOkta && <DeleteMemberWarning isReviewing={true} />}
      <AccessListMemberTable
        members={editedMembers}
        canEditMembers={true}
        onDeleteMember={onDeleteMember}
        hideIneligibleReason={true}
        isReviewing={true}
      />
      <Box mt={5} mb={-8}>
        <H2>Changes</H2>
        <List>
          <li>
            {numMembersDeleted} {pluralize(numMembersDeleted, 'member')} revoked
          </li>
        </List>
      </Box>
    </>
  );
}
