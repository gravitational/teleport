import { Alert, Box, H2, Text } from 'design';
import { pluralize } from 'shared/utils/text';

import type { AccessListWithNestedOwnersMembersTitles } from 'e-teleport/AccessListManagement/ViewEditAccessList/Shared';
import {
  AccessListGrant,
  AccessListMember,
} from 'e-teleport/services/accessmanagement';

import { TraitConvenience } from '../../Traits';
import { DeleteMemberWarning } from '../DeleteUserConfirmDialog';
import { AccessListMemberTable } from '../Members/Members';
import { List } from './Shared';
import { getMembersDeleted } from './utils';

export type Grant = Omit<TraitConvenience, 'traitList'> &
  Omit<AccessListGrant, 'traits'>;

type Props = {
  editedMembers: AccessListWithNestedOwnersMembersTitles['members'];
  onDeleteMember(member: AccessListMember): void;
  originalMembers: AccessListMember[];
  isOkta: boolean;
  isReadOnlyOktaList?: boolean;
};

export function ReviewMembers({
  editedMembers,
  onDeleteMember,
  originalMembers,
  isOkta,
  isReadOnlyOktaList = false,
}: Props) {
  const numMembersDeleted = getMembersDeleted(
    originalMembers,
    editedMembers
  ).length;

  return (
    <>
      {isOkta && <DeleteMemberWarning isReviewing={true} />}
      <H2 mb={3}>Members</H2>
      {isReadOnlyOktaList && (
        <Alert kind="outline-info">
          <Text>
            Editing members is disabled, this Access List is managed by Okta and
            is read-only in Teleport
          </Text>
        </Alert>
      )}
      <AccessListMemberTable
        members={editedMembers}
        canEditMembers={true}
        onDeleteMember={onDeleteMember}
        hideIneligibleReason={true}
        isReviewing={true}
        isReadOnlyOktaList={isReadOnlyOktaList}
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
