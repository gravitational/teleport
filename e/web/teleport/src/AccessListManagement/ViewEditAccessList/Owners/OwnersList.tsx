import React, { useState } from 'react';
import { Flex, Text, ButtonText } from 'design';
import Table from 'design/DataTable';
import { Wrench, Add } from 'design/Icon';

import { AccessListOwner } from 'e-teleport/services/accessmanagement';

import { UserOption } from '../../Shared/Shared';

import { CustomCell, UserRevokeButtonCell } from '../Shared';
import { DeleteUserConfirmDialog } from '../DeleteUserConfirmDialog';
import { AccessListModified, EditAccess } from '../ViewEditAccessList';

import { EnrollNewOwners } from './EnrollNewOwners';

type Props = {
  userOptions: UserOption[];
  editAccess: EditAccess;
  fetchAccessList(): Promise<void | boolean>;
  accessList: AccessListModified;
};

export function OwnersList({
  accessList,
  editAccess,
  userOptions,
  fetchAccessList,
}: Props) {
  const { owners } = accessList;
  const [showEnrollNewMembers, setShowEnrollNewMembers] = useState(false);
  const [deleteOwner, setDeleteOwner] = useState<AccessListOwner>();
  return (
    <>
      <Flex justifyContent="space-between" mb={2}>
        <Flex mb={2} alignItems="center">
          <Wrench />
          <Text ml={1} mr={2} fontSize={4}>
            Owners
          </Text>
        </Flex>
        <ButtonText
          title={editAccess.owners.btnTitle}
          disabled={!editAccess.owners.hasAccess}
          onClick={() => setShowEnrollNewMembers(true)}
          mr={0}
        >
          <Add size={16} mr={2} />
          Enroll New Owners
        </ButtonText>
      </Flex>
      <Table
        data={owners}
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
            key: 'description',
            headerText: 'Description',
            isSortable: true,
            render: ({ description, ineligibleReason }) => (
              <CustomCell disabled={!!ineligibleReason}>
                {description}
              </CustomCell>
            ),
          },
          {
            altKey: 'options-btn',
            render: owner => (
              <UserRevokeButtonCell
                disabled={!editAccess.owners.hasAccess}
                btnTitle={editAccess.owners.btnTitle}
                onClick={() => setDeleteOwner(owner)}
                ineligibleReason={owner.ineligibleReason}
              />
            ),
          },
        ]}
        emptyText="No Users Found"
        isSearchable
        pagination={{ pageSize: 5 }}
      />
      {showEnrollNewMembers && (
        <EnrollNewOwners
          onClose={() => setShowEnrollNewMembers(false)}
          userOptions={userOptions}
          fetchAccessList={fetchAccessList}
          accessList={accessList}
        />
      )}
      {deleteOwner && (
        <DeleteUserConfirmDialog
          onClose={() => setDeleteOwner(null)}
          kind="Owner"
          accessList={accessList}
          username={deleteOwner.name}
          fetchAccessList={fetchAccessList}
        />
      )}
    </>
  );
}
