import React, { useState } from 'react';
import { Flex, Text, Box, ButtonText } from 'design';
import Table from 'design/DataTable';
import { UsersTriple, Add } from 'design/Icon';

import { AccessListMember } from 'e-teleport/services/accessmanagement';
import { getFormattedDate } from 'e-teleport/AccessListManagement/Shared/date';

import { UserOption } from '../../Shared/Shared';

import { CustomCell, UserRevokeButtonCell } from '../Shared';
import { DeleteUserConfirmDialog } from '../DeleteUserConfirmDialog';
import {
  AccessListModified,
  EditAccess,
  EditAccessMeta,
} from '../ViewEditAccessList';

import { EnrollNewMembers } from './EnrollNewMembers';

type Props = {
  userOptions: UserOption[];
  editAccess: EditAccess;
  fetchAccessList(): Promise<void | boolean>;
  accessList: AccessListModified;
};

export function MembersList({
  accessList,
  userOptions,
  editAccess,
  fetchAccessList,
}: Props) {
  const { members } = accessList;
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
      <AccessListMemberTable
        members={members}
        memberEditAccess={editAccess.members}
        onDeleteMember={setDeleteMember}
      />
      {showEnrollNewMembers && (
        <EnrollNewMembers
          onClose={() => setShowEnrollNewMembers(false)}
          accessList={accessList}
          userOptions={userOptions}
          fetchAccessList={fetchAccessList}
        />
      )}
      {deleteMember && (
        <DeleteUserConfirmDialog
          onClose={() => setDeleteMember(null)}
          kind="Member"
          username={deleteMember.name}
          accessList={accessList}
          fetchAccessList={fetchAccessList}
        />
      )}
    </>
  );
}

export const AccessListMemberTable = ({
  members,
  memberEditAccess,
  onDeleteMember = null,
  hideIneligibleReason = false,
}: {
  members: AccessListMember[];
  memberEditAccess: EditAccessMeta;
  onDeleteMember?(m: AccessListMember): void;
  hideIneligibleReason?: boolean;
}) => {
  return (
    <Table
      data={members}
      columns={[
        {
          key: 'name',
          headerText: 'Name',
          isSortable: true,
          render: ({ name, ineligibleReason }) => (
            <CustomCell disabled={!hideIneligibleReason && !!ineligibleReason}>
              {name}
            </CustomCell>
          ),
        },
        {
          key: 'addedBy',
          headerText: 'Added By',
          isSortable: true,
          render: ({ addedBy, ineligibleReason }) => (
            <CustomCell disabled={!hideIneligibleReason && !!ineligibleReason}>
              {addedBy}
            </CustomCell>
          ),
        },
        {
          key: 'reason',
          headerText: 'Reason',
          isSortable: true,
          render: ({ reason, ineligibleReason }) => (
            <CustomCell disabled={!hideIneligibleReason && !!ineligibleReason}>
              {reason}
            </CustomCell>
          ),
        },
        {
          key: 'joined',
          headerText: 'Date Added',
          isSortable: true,
          onSort: sortCustomDate,
          render: ({ joined, ineligibleReason }) => (
            <CustomCell disabled={!hideIneligibleReason && !!ineligibleReason}>
              {getFormattedDate(joined)}
            </CustomCell>
          ),
        },
        {
          key: 'expires',
          headerText: 'Expires',
          isSortable: true,
          render: ({ expires, ineligibleReason }) => (
            <CustomCell disabled={!hideIneligibleReason && !!ineligibleReason}>
              {getFormattedDate(expires)}
            </CustomCell>
          ),
        },
        {
          altKey: 'options-btn',
          isNonRender: !onDeleteMember,
          render: member => (
            <UserRevokeButtonCell
              disabled={!memberEditAccess.hasAccess}
              btnTitle={memberEditAccess.btnTitle}
              onClick={() => onDeleteMember(member)}
              ineligibleReason={member.ineligibleReason}
              hideIneligibleReason={hideIneligibleReason}
            />
          ),
        },
      ]}
      emptyText="No Users Found"
      isSearchable
      pagination={{ pageSize: 10 }}
    />
  );
};

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
