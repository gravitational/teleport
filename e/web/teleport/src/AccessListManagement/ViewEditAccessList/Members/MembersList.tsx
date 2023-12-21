import React, { useState } from 'react';
import { Flex, Text, Box, ButtonText } from 'design';
import Table from 'design/DataTable';
import { UsersTriple, Add } from 'design/Icon';

import { AccessListMember } from 'e-teleport/services/accessmanagement';
import { getFormattedDate } from 'e-teleport/AccessListManagement/Shared/date';

import { UserOption } from '../../Shared/Shared';

import { CustomCell, UserRevokeButtonCell } from '../Shared';
import { DeleteUserConfirmDialog } from '../DeleteUserConfirmDialog';
import { AccessListModified } from '../ViewEditAccessList';

import { EnrollNewMembers } from './EnrollNewMembers';

const genericNoAccessMsg = 'You do not have access to edit members';

export function MembersList({
  accessList,
  userOptions,
  canEditMembers,
  fetchAccessList,
}: {
  userOptions: UserOption[];
  canEditMembers: boolean;
  fetchAccessList(): Promise<void | boolean>;
  accessList: AccessListModified;
}) {
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
            title={canEditMembers ? '' : genericNoAccessMsg}
            disabled={!canEditMembers}
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
        canEditMembers={canEditMembers}
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
  canEditMembers,
  onDeleteMember = null,
  hideIneligibleReason = false,
  hideReasonCol = false,
}: {
  members: AccessListMember[];
  canEditMembers: boolean;
  onDeleteMember?(m: AccessListMember): void;
  hideIneligibleReason?: boolean;
  hideReasonCol?: boolean;
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
        !hideReasonCol && {
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
              disabled={!canEditMembers}
              btnTitle={canEditMembers ? '' : genericNoAccessMsg}
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
