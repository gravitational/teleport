import React, { useState } from 'react';
import { Flex, Text, Box, ButtonText } from 'design';
import Table from 'design/DataTable';
import { UsersTriple, Add } from 'design/Icon';

import {
  AccessListMember,
  AccessListRequires,
  AccessListGrant,
} from 'e-teleport/services/accessmanagement';

import { UserOption, getFormattedDate } from '../../Shared';

import { CustomCell, UserRevokeButtonCell } from '../Shared';
import { DeleteUserConfirmDialog } from '../DeleteUserConfirmDialog';
import { EditAccess } from '../ViewEditAccessList';

import { EnrollNewMembers } from './EnrollNewMembers';

type Props = {
  members: AccessListMember[];
  membershipRequires: AccessListRequires;
  grants: AccessListGrant;
  userOptions: UserOption[];
  editAccess: EditAccess;
  fetchAccessList(): Promise<void | boolean>;
};

export function MembersList({
  membershipRequires,
  members,
  userOptions,
  editAccess,
  fetchAccessList,
}: Props) {
  const [showEnrollNewMembers, setShowEnrollNewMembers] = useState(false);
  const [deleteMember, setDeleteMember] = useState<AccessListMember>();
  return (
    <>
      <Box>
        <Flex justifyContent="space-between" mb={2}>
          <Flex alignItems="center">
            <UsersTriple />
            <Text ml={1} mr={2} fontSize={4}>
              Members
            </Text>
          </Flex>
          <ButtonText
            title={editAccess.members.btnTitle}
            disabled={!editAccess.members.hasAccess}
            onClick={() => setShowEnrollNewMembers(true)}
            mr={0}
          >
            <Add size={16} mr={2} />
            Enroll New Members
          </ButtonText>
        </Flex>
      </Box>
      <Table
        data={members}
        columns={[
          {
            key: 'name',
            headerText: 'Name',
            isSortable: true,
            render: ({ name, ineligibleReason }) => (
              <CustomCell disabled={!!ineligibleReason}>{name}</CustomCell>
            ),
          },
          {
            key: 'addedBy',
            headerText: 'Added By',
            isSortable: true,
            render: ({ addedBy, ineligibleReason }) => (
              <CustomCell disabled={!!ineligibleReason}>{addedBy}</CustomCell>
            ),
          },
          {
            key: 'reason',
            headerText: 'Reason',
            isSortable: true,
            render: ({ reason, ineligibleReason }) => (
              <CustomCell disabled={!!ineligibleReason}>{reason}</CustomCell>
            ),
          },
          {
            key: 'joined',
            headerText: 'Date Added',
            isSortable: true,
            onSort: sortCustomDate,
            render: ({ joined, ineligibleReason }) => (
              <CustomCell disabled={!!ineligibleReason}>
                {getFormattedDate(joined)}
              </CustomCell>
            ),
          },
          {
            key: 'expires',
            headerText: 'Expires',
            isSortable: true,
            render: ({ expires, ineligibleReason }) => (
              <CustomCell disabled={!!ineligibleReason}>
                {getFormattedDate(expires)}
              </CustomCell>
            ),
          },
          {
            altKey: 'options-btn',
            render: member => (
              <UserRevokeButtonCell
                disabled={!editAccess.members.hasAccess}
                btnTitle={editAccess.members.btnTitle}
                onClick={() => setDeleteMember(member)}
                ineligibleReason={member.ineligibleReason}
              />
            ),
          },
        ]}
        emptyText="No Users Found"
        isSearchable
        pagination={{ pageSize: 10 }}
      />
      {showEnrollNewMembers && (
        <EnrollNewMembers
          onClose={() => setShowEnrollNewMembers(false)}
          membershipRequires={membershipRequires}
          userOptions={userOptions}
          fetchAccessList={fetchAccessList}
          existingMembers={members}
        />
      )}
      {deleteMember && (
        <DeleteUserConfirmDialog
          onClose={() => setDeleteMember(null)}
          kind="Member"
          username={deleteMember.name}
          existingUsers={members}
          fetchAccessList={fetchAccessList}
        />
      )}
    </>
  );
}

function sortCustomDate(a: Date, b: Date) {
  const aStr = getFormattedDate(a);
  const bStr = getFormattedDate(b);

  if (aStr < bStr) {
    return -1;
  }
  if (aStr > bStr) {
    return 1;
  }

  return 0;
}
