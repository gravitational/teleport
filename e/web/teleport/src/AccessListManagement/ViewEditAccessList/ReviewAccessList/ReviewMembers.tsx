import { Alert, Box, H2, Text } from 'design';
import { pluralize } from 'shared/utils/text';

import type { AccessListModified } from 'e-teleport/AccessListManagement/ViewEditAccessList/Shared';
import {
  AccessListGrant,
  AccessListMember,
  AccessListOrigin,
  isReadOnly,
  isScim,
} from 'e-teleport/services/accessmanagement';

import { TraitConvenience } from '../../Traits';
import { DeleteMemberWarning } from '../DeleteUserConfirmDialog';
import { AccessListMemberTable } from '../Members/Members';
import { List } from './Shared';
import { getMembersDeleted } from './utils';

export type Grant = Omit<TraitConvenience, 'traitList'> &
  Omit<AccessListGrant, 'traits'>;

type Props = {
  accessList: AccessListModified;
  editedMembers: AccessListMember[];
  onDeleteMember(member: AccessListMember): void;
  originalMembers: AccessListMember[];
  isReadOnlyOktaList?: boolean;
};

export function ReviewMembers({
  accessList,
  isReadOnlyOktaList = false,
  originalMembers,
  editedMembers,
  onDeleteMember,
}: Props) {
  const numMembersDeleted = getMembersDeleted(
    originalMembers,
    editedMembers
  ).length;

  const isOkta = accessList.origin === AccessListOrigin.Okta;

  return (
    <>
      {isOkta && <DeleteMemberWarning isReviewing={true} />}
      <H2 mb={3}>Members</H2>
      {(isReadOnly(accessList.type) || isScim(accessList.type)) && (
        <Alert kind="outline-info">
          <Text>
            Editing members is disabled, this Access List is managed by IaC
            tools or an integration and is read-only in the web UI.
          </Text>
        </Alert>
      )}
      {isReadOnlyOktaList && (
        <Alert kind="outline-info">
          <Text>
            Editing members is disabled, this Access List is managed by Okta and
            is read-only in Teleport
          </Text>
        </Alert>
      )}
      <AccessListMemberTable
        accessList={accessList}
        isReadOnlyOktaList={isReadOnlyOktaList}
        members={editedMembers}
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
