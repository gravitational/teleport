import { useState } from 'react';

import { ButtonText, Flex, H2 } from 'design';
import Table from 'design/DataTable';
import { Add, Wrench } from 'design/Icon';
import { IconTooltip } from 'design/Tooltip';

import type { AccessListWithModifiedGrants } from 'e-teleport/AccessListManagement/AccessLists/AccessLists';
import { useOnClickNestedList } from 'e-teleport/AccessListManagement/Shared/nav';
import type { AccessList } from 'e-teleport/services/accessmanagement';
import { AccessListMemberKind } from 'e-teleport/services/accessmanagement/types';

import { NestedListLink, type UserOption } from '../../Shared/Shared';
import { DeleteUserConfirmDialog } from '../DeleteUserConfirmDialog';
import {
  CustomCell,
  UserRevokeButtonCell,
  type AccessListModified,
} from '../Shared';
import { EnrollNewOwners } from './EnrollNewOwners';

const genericNoAccessMsg = 'You do not have access to edit owners';

export function OwnersList({
  accessList,
  canEditOwners,
  userOptions,
  updateAccessList,
  accessLists,
}: {
  userOptions: UserOption[];
  canEditOwners: boolean;
  updateAccessList(accessList: AccessList): void;
  accessList: AccessListModified;
  accessLists: AccessListWithModifiedGrants[];
}) {
  const { owners } = accessList;
  const [showEnrollNewMembers, setShowEnrollNewMembers] = useState(false);
  const [deleteOwner, setDeleteOwner] =
    useState<(typeof accessList)['owners'][number]>();
  const onClickNestedList = useOnClickNestedList();

  return (
    <>
      <Flex justifyContent="space-between" mb={2}>
        <Flex mb={2} alignItems="center">
          <Wrench />
          <H2 ml={1} mr={2}>
            Owners
          </H2>
        </Flex>
        <ButtonText
          title={canEditOwners ? '' : genericNoAccessMsg}
          disabled={!canEditOwners}
          onClick={() => setShowEnrollNewMembers(true)}
          mr={0}
        >
          <Add size={16} mr={2} />
          Enroll New Owners or Access Lists
        </ButtonText>
      </Flex>
      <Table
        data={owners}
        columns={[
          {
            key: 'membershipKind',
            headerText: 'Type',
            isSortable: true,
            onSort: (a, b) => {
              if (a?.membershipKind === b?.membershipKind) {
                return 0;
              }
              return a?.membershipKind === AccessListMemberKind.List ? -1 : 1;
            },
            render: ({ membershipKind, ineligibleReason }) => (
              <CustomCell disabled={!!ineligibleReason}>
                {membershipKind === AccessListMemberKind.List
                  ? 'Access List'
                  : 'User'}
              </CustomCell>
            ),
          },
          {
            key: 'name',
            headerText: 'Name',
            isSortable: true,
            render: ({ name, ineligibleReason, title, ...rest }) => {
              if (rest.membershipKind === AccessListMemberKind.List) {
                return (
                  <CustomCell
                    disabled={false}
                    title={rest.accessListExists ? title : ''}
                  >
                    <NestedListLink
                      title={
                        rest.accessListExists ? `View list '${title}'` : ''
                      }
                      onClick={() => onClickNestedList(name)}
                      disabled={!rest.accessListExists}
                    >
                      {title}
                      {!rest.accessListExists && (
                        <IconTooltip
                          kind="warning"
                          children={`Insufficient permissions to view list '${title}'`}
                          css={`
                            margin-left: 5px;
                          `}
                        />
                      )}
                    </NestedListLink>
                  </CustomCell>
                );
              }

              return (
                <CustomCell disabled={!!ineligibleReason}>{name}</CustomCell>
              );
            },
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
                disabled={!canEditOwners}
                btnTitle={canEditOwners ? '' : genericNoAccessMsg}
                onClick={() => setDeleteOwner(owner)}
                ineligibleReason={owner.ineligibleReason}
              />
            ),
          },
        ]}
        emptyText="No Users Found"
        isSearchable
        pagination={{ pageSize: 5 }}
        initialSort={{ key: 'name', dir: 'ASC' }}
      />
      {showEnrollNewMembers && (
        <EnrollNewOwners
          onClose={() => setShowEnrollNewMembers(false)}
          userOptions={userOptions}
          updateAccessList={updateAccessList}
          accessList={accessList}
          accessLists={accessLists}
        />
      )}
      {deleteOwner && (
        <DeleteUserConfirmDialog
          onClose={() => setDeleteOwner(null)}
          kind="Owner"
          accessList={accessList}
          username={deleteOwner.name}
          displayName={
            deleteOwner.membershipKind === AccessListMemberKind.List
              ? deleteOwner.title
              : undefined
          }
          updateAccessList={updateAccessList}
        />
      )}
    </>
  );
}
