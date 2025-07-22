import { useState } from 'react';

import { Box, ButtonText, Flex } from 'design';
import Table from 'design/DataTable';
import { Add } from 'design/Icon';
import { HoverTooltip, IconTooltip } from 'design/Tooltip';
import type { Option } from 'shared/components/Select';

import type { AccessListWithModifiedGrants } from 'e-teleport/AccessListManagement/AccessLists/AccessLists';
import { useOnClickNestedList } from 'e-teleport/AccessListManagement/Shared/nav';
import { AccessList } from 'e-teleport/services/accessmanagement';
import {
  AccessListMemberKind,
  AccessListOrigin,
} from 'e-teleport/services/accessmanagement/types';

import { EditKind, NestedListLink, type UserOption } from '../../Shared/Shared';
import { DeleteUserConfirmDialog } from '../DeleteUserConfirmDialog';
import { noEditAcessMsg, oktaReadOnlyMsg } from '../errors';
import {
  CustomCell,
  RoleAndTraitLabels,
  UserRevokeButtonCell,
  type AccessListModified,
} from '../Shared';
import { EditEligibilityOrGrantRoles } from '../Specs/EditEligibilityOrGrants';
import { EnrollNewOwners } from './EnrollNewOwners';

const oktaReadOnlyOwnersMsg = oktaReadOnlyMsg({
  userKind: 'owner',
  accessKind: 'edit-user',
});
const oktaReadOnlyEligibilityMsg = oktaReadOnlyMsg({
  userKind: 'owner',
  accessKind: 'eligibility',
});
const oktaReadOnlyGrantMsg = oktaReadOnlyMsg({
  userKind: 'owner',
  accessKind: 'granted-perms',
});

export function Owners({
  accessList,
  canEditOwners,
  userOptions,
  updateAccessList,
  accessLists,
  isReadOnlyOktaList,
  canEditSpecs,
  fetchRoleOptions,
}: {
  userOptions: UserOption[];
  canEditOwners: boolean;
  updateAccessList(accessList: AccessList): void;
  accessList: AccessListModified;
  accessLists: AccessListWithModifiedGrants[];
  isReadOnlyOktaList?: boolean;
  fetchRoleOptions: (input: string) => Promise<Option[]>;
  canEditSpecs: boolean;
}) {
  const { owners, ownershipRequires, ownerGrants } = accessList;
  const [showEnrollNewMembers, setShowEnrollNewMembers] = useState(false);
  const [deleteOwner, setDeleteOwner] =
    useState<(typeof accessList)['owners'][number]>();
  const onClickNestedList = useOnClickNestedList();

  const [editPermKind, setEditPermKind] = useState<EditKind>();

  const isOktaList = accessList.origin === AccessListOrigin.Okta;

  return (
    <Box data-testid="owners-content">
      <Flex justifyContent="space-between" gap={1} alignItems="flex-start">
        <Flex flexDirection="column" gap={3}>
          <Flex mb={4} flexDirection="column" gap={1}>
            <RoleAndTraitLabels
              roles={ownershipRequires.roles}
              traits={ownershipRequires.traitList}
              accessKind="requirements"
              toolTipContent={
                (!canEditSpecs && noEditAcessMsg) ||
                (isReadOnlyOktaList && oktaReadOnlyEligibilityMsg) ||
                undefined
              }
              onEdit={() => setEditPermKind('Owner')}
              editDisabled={!canEditSpecs || isReadOnlyOktaList}
              userKind="owner"
            />

            <RoleAndTraitLabels
              roles={ownerGrants.roles}
              traits={ownerGrants.traitList}
              accessKind="grants"
              toolTipContent={
                (!canEditSpecs && noEditAcessMsg) ||
                (isOktaList && oktaReadOnlyGrantMsg) ||
                undefined
              }
              onEdit={() => setEditPermKind('OwnerGrants')}
              editDisabled={!canEditSpecs || isOktaList}
              userKind="owner"
            />
          </Flex>
        </Flex>
      </Flex>
      <Flex justifyContent="end">
        <HoverTooltip
          tipContent={
            !canEditOwners
              ? noEditAcessMsg
              : isReadOnlyOktaList
                ? oktaReadOnlyOwnersMsg
                : undefined
          }
        >
          <ButtonText
            disabled={!canEditOwners || isReadOnlyOktaList}
            onClick={() => setShowEnrollNewMembers(true)}
            gap={2}
            fill="border"
          >
            <Add size="small" />
            Add New Owners or Access Lists
          </ButtonText>
        </HoverTooltip>
      </Flex>
      <Table
        data={owners}
        columns={[
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
                    <Flex
                      flexDirection="row"
                      alignItems="center"
                      justifyContent="space-between"
                      gap={1}
                    >
                      <NestedListLink
                        title={
                          rest.accessListExists ? `View list '${title}'` : ''
                        }
                        onClick={() => onClickNestedList(name)}
                        disabled={!rest.accessListExists}
                      >
                        {title}
                      </NestedListLink>
                      {!rest.accessListExists && (
                        <IconTooltip
                          kind="warning"
                          children={`Insufficient permissions to view list '${title}'`}
                          css={`
                            margin-left: 5px;
                          `}
                        />
                      )}
                    </Flex>
                  </CustomCell>
                );
              }

              return (
                <CustomCell disabled={!!ineligibleReason}>{name}</CustomCell>
              );
            },
          },
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
                disabled={!canEditOwners || isReadOnlyOktaList}
                tooltip={
                  (!canEditOwners && noEditAcessMsg) ||
                  (isReadOnlyOktaList && oktaReadOnlyOwnersMsg) ||
                  undefined
                }
                onClick={() => setDeleteOwner(owner)}
                ineligibleReason={owner.ineligibleReason}
              />
            ),
          },
        ]}
        emptyText="No Owners Found"
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
      {editPermKind && (
        <EditEligibilityOrGrantRoles
          onClose={() => setEditPermKind(null)}
          editKind={editPermKind}
          fetchRoleOptions={fetchRoleOptions}
          updateAccessList={updateAccessList}
          accessList={accessList}
        />
      )}
    </Box>
  );
}
