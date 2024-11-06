import React from 'react';
import styled from 'styled-components';
import { Alert, Box, ButtonIcon, ButtonSecondary, Flex } from 'design';
import { Cell } from 'design/DataTable';
import { Pencil } from 'design/Icon';
import { Option } from 'shared/components/Select';

import { ToolTipInfo } from 'shared/components/ToolTip';

import Link from 'design/Link';

import {
  AccessList,
  AccessListGrant,
  AccessListMember,
  AccessListMemberKind,
  AccessListOwner,
  AccessListRequires,
} from 'e-teleport/services/accessmanagement';
import {
  convertToTraitConvenience,
  TraitConvenience,
} from 'e-teleport/AccessListManagement/Traits';
import { accessListRequiresReview } from 'e-teleport/stores/storeNotificationsE';

import {
  matchRoles,
  matchTraits,
  TruncatingLabel,
  UserOption,
} from '../Shared/Shared';

import type { PropsWithChildren } from 'react';
import type { MemberSelection } from '../Shared/Shared';
import type { AccessListWithModifiedGrants } from 'e-teleport/AccessListManagement/AccessLists/AccessLists';
import type TeleportContextE from 'e-teleport/teleportContextE';
import type { Access } from 'teleport/services/user';

export type AccessListRequiresWithTraitConvenience = AccessListRequires &
  TraitConvenience;

export type AccessListGrantWithTraitConvenience = AccessListGrant &
  TraitConvenience;

export type AccessListWithNestedOwnersMembersTitles = AccessList & {
  owners: (AccessList['owners'][number] &
    (
      | {
          membershipKind: AccessListMemberKind.List;
          title: string;
          accessListExists: boolean;
        }
      | { membershipKind: AccessListMemberKind.User; title: string }
      | { membershipKind: AccessListMemberKind.Unspecified; title: string }
    ))[];
  members: (AccessList['members'][number] &
    (
      | {
          membershipKind: AccessListMemberKind.List;
          title: string;
          accessListExists: boolean;
        }
      | { membershipKind: AccessListMemberKind.User; title: string }
      | { membershipKind: AccessListMemberKind.Unspecified; title: string }
    ))[];
};

export type AccessListModified = AccessListWithNestedOwnersMembersTitles & {
  membershipRequires: AccessListRequiresWithTraitConvenience;
  ownershipRequires: AccessListRequiresWithTraitConvenience;
  grants: AccessListGrantWithTraitConvenience;
  ownerGrants: AccessListGrantWithTraitConvenience;
  requiresReview: boolean;
};

// Perms defines different types of permissions the viewing
// user has.
//
// TODO: Explore using a PermissionLevel enum instead
// (Owner, Admin, Member) and try to centralize the calculation of
// which type a given user is.
export type Perms = {
  adminWhoCanRead: boolean;
  adminWhoCanDelete: boolean;
  adminWhoCanEdit: boolean;
  isOwner: boolean;
};

export const getPerms = ({
  accessListAccess,
  isOwner,
}: {
  accessListAccess?: Access;
  isOwner?: boolean;
}): Perms => {
  return {
    isOwner,
    adminWhoCanRead: accessListAccess?.read && accessListAccess?.list,
    adminWhoCanEdit: accessListAccess?.edit,
    adminWhoCanDelete: accessListAccess?.remove,
  };
};

export const modifyAccessList = (
  acl: AccessList,
  acls: (AccessList | AccessListWithModifiedGrants)[],
  ctx: TeleportContextE
): [AccessListModified, Perms] => {
  const modifiedAccessList: AccessListModified = {
    ...acl,
    grants: {
      ...acl.grants,
      ...convertToTraitConvenience(acl.grants.traits),
    },
    ownerGrants: {
      ...acl.ownerGrants,
      ...convertToTraitConvenience(acl.ownerGrants.traits),
    },
    ownershipRequires: {
      ...acl.ownershipRequires,
      ...convertToTraitConvenience(acl.ownershipRequires.traits),
    },
    membershipRequires: {
      ...acl.membershipRequires,
      ...convertToTraitConvenience(acl.membershipRequires.traits),
    },
    requiresReview: accessListRequiresReview({
      todayDate: new Date(),
      reviewDate: acl.audit.nextDate,
    }),
    ...getTitlesForNestedListOwnersMembers(
      { members: acl.members, owners: acl.owners },
      acls
    ),
  };

  ctx.storeNotifications.updateOrRemoveAccessListNotification(
    acl,
    ctx.storeUser.state
  );

  const accessListAccess = ctx.storeUser.getAccessListAccess();
  const isOwner = isAccessListOwnerRecursive(
    acl,
    acls,
    ctx.storeUser.getUsername()
  );

  return [modifiedAccessList, getPerms({ accessListAccess, isOwner })];
};

export const getTitlesForNestedListOwnersMembers = (
  acl: Pick<AccessList, 'members' | 'owners'>,
  acls: Pick<AccessList | AccessListWithModifiedGrants, 'id' | 'title'>[]
): Pick<AccessListWithNestedOwnersMembersTitles, 'members' | 'owners'> => {
  const updatedMembers = acl.members.map(m => {
    switch (m.membershipKind) {
      case AccessListMemberKind.List:
        const acl = acls.find(l => l.id === m.name);
        return {
          ...m,
          title: acl?.title || m.name,
          accessListExists: !!acl,
        };
      default:
        return {
          ...m,
          title: m.name,
        };
    }
  }) as AccessListWithNestedOwnersMembersTitles['members'];
  const updatedOwners = acl.owners.map(o => {
    switch (o.membershipKind) {
      case AccessListMemberKind.List:
        const acl = acls.find(l => l.id === o.name);
        return {
          ...o,
          title: acl?.title || o.name,
          accessListExists: !!acl,
        };
      default:
        return {
          ...o,
          title: o.name,
        };
    }
  }) as AccessListWithNestedOwnersMembersTitles['owners'];

  return { members: updatedMembers, owners: updatedOwners };
};

export const isAccessListOwnerRecursive = (
  acl:
    | (Partial<AccessList> & Pick<AccessList, 'owners' | 'members'>)
    | undefined,
  acls: (AccessList | AccessListWithModifiedGrants)[],
  username: string,
  depth = 0
) => {
  if (!acl || depth > 10) {
    return false;
  }
  if (depth === 0) {
    for (const owner of acl.owners) {
      if (owner.name === username) {
        return true;
      }
      if (
        owner.membershipKind === AccessListMemberKind.List &&
        isAccessListOwnerRecursive(
          acls.find(acl => acl.id === owner.name),
          acls,
          username,
          depth + 1
        )
      ) {
        return true;
      }
    }
  } else {
    for (const member of acl.members) {
      if (member.name === username) {
        return true;
      }
      if (
        member.membershipKind === AccessListMemberKind.List &&
        isAccessListOwnerRecursive(
          acls.find(acl => acl.id === member.name),
          acls,
          username,
          depth + 1
        )
      ) {
        return true;
      }
    }
  }
  return false;
};

export const CustomCell: React.FC<
  PropsWithChildren<{ disabled: boolean; title?: string | undefined }>
> = ({ disabled, children, title }) => {
  return (
    <Cell
      css={`
        opacity: ${disabled ? '0.6' : '1'};
        max-width: 150px;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
      `}
      title={title || (typeof children === 'string' ? children : undefined)}
    >
      {children}
    </Cell>
  );
};

export const UserRevokeButtonCell = ({
  disabled,
  btnTitle,
  onClick,
  ineligibleReason,
  hideIneligibleReason = false,
  isReviewing = false,
}: {
  disabled: boolean;
  btnTitle: string;
  onClick(): void;
  ineligibleReason: string;
  // when reviewing, hide ineligible reason
  // since deleting a member during review isn't
  // a dynamic change.
  hideIneligibleReason?: boolean;
  isReviewing?: boolean;
}) => {
  return (
    <Cell align="right">
      <Flex alignItems="center" justifyContent="flex-end">
        {!hideIneligibleReason && ineligibleReason && (
          <Box mr={3}>
            <ToolTipInfo kind="warning" position="left">
              {ineligibleReason}
            </ToolTipInfo>
          </Box>
        )}
        <ButtonSecondary
          textTransform="none"
          disabled={disabled}
          title={btnTitle}
          onClick={onClick}
          size="small"
        >
          {isReviewing ? 'Remove' : 'Delete'}
        </ButtonSecondary>
      </Flex>
    </Cell>
  );
};

// getNewAndExistingUsersForEnrollingNewUsers extracts
// existing users in the selected users list and returns
// the extracted existed users and the unique list of
// new users.
//
// We are extracting existing users so it doesn't
// get overwritten when it gets sent back as an update
// request to add new users.
//
// Duplicate users can be defined due to allowing manual
// defining of user names.
//
// If a user wants to "update" a user, they would
// have to "delete/revoke" the user first and enroll
// again. Or later, an "update" action may be available.
export function getNewAndExistingUsersForAddingNewUsers(
  existingUsers: AccessListMember[] | AccessListOwner[],
  selectedUsers: Option<MemberSelection>[]
) {
  const duplicateUsers: string[] = [];
  const newUsers = selectedUsers.filter(opt => {
    if (existingUsers.some(m => m.name === opt.value.name)) {
      duplicateUsers.push(opt.value.name);
      return false;
    }
    return true;
  });

  return { duplicateUsers, newUsers };
}

// getEligibleUsersForAddingNewUsers returns users
// who hasn't been enrolled already and meets the
// eligibility requirement.
//
// Eligible users can have more roles assigned
// aside from the roles defined in eligibiliy.
export function getEligibleUsersForAddingNewUsers(
  eligibility: AccessListRequires,
  fetchedUsers: UserOption[],
  existingUsers: { name: string }[]
): Option<MemberSelection>[] {
  if (
    fetchedUsers.length === 0 ||
    (eligibility.roles.length === 0 &&
      Object.keys(eligibility.traits).length === 0)
  ) {
    return [];
  }

  let filteredUsers = matchRoles(eligibility.roles, fetchedUsers);
  filteredUsers = matchTraits(eligibility.traits, filteredUsers);

  return filterExistingUsersAndConvertToOption(filteredUsers, existingUsers);
}

export function filterExistingUsersAndConvertToOption(
  userOpts: UserOption[],
  existingUsers: { name: string }[]
) {
  return (
    userOpts
      // Filter out existing users among users.
      .filter(u => existingUsers.every(m => m.name !== u.value.name))
      // Convert to type Option for dropdowns.
      .map(u => ({
        label: u.value.name,
        value: {
          name: u.value.name,
          membershipKind: AccessListMemberKind.User,
        },
      }))
  );
}

export function convertAccessListsToUserOptions(
  excludeSelfID: string,
  acls: AccessList[],
  existingUsers: { name: string }[]
) {
  return acls
    .filter(
      l => l.id !== excludeSelfID && existingUsers.every(m => m.name !== l.id)
    )
    .map(x => ({
      label: x.title,
      value: { name: x.id, membershipKind: AccessListMemberKind.List },
    }));
}

export function ButtonPencil({
  onClick,
  disabled = false,
  title = '',
  mt = 0,
}: {
  onClick(): void;
  disabled?: boolean;
  title?: string;
  mt?: number;
}) {
  return (
    <ButtonIcon
      alignItems="center"
      onClick={onClick}
      disabled={disabled}
      title={title}
      css={mt && { marginTop: `${mt}px` }}
    >
      <Pencil size={16} />
    </ButtonIcon>
  );
}

const TextNoEllipsis = styled(Box)`
  ${p => p.theme.typography.body3}
`;

const renderTruncatingLabels = (
  labels: string[] = [],
  showEllipses?: boolean
) => {
  const labelsToUse = labels.slice(
    0,
    showEllipses ? MAX_DISPLAYED_INHERITED_ROLES_TRAITS : labels.length
  );
  const $labels = labelsToUse.map((label, index) => (
    <TruncatingLabel
      mr={index === labelsToUse.length - 1 ? 0 : 1}
      key={`${label}${index}`}
      kind="secondary"
      title={label}
    >
      {label}
    </TruncatingLabel>
  ));

  return (
    <Flex flexWrap="wrap">
      {$labels}
      {showEllipses && (
        <TruncatingLabel kind="secondary" ml={1}>
          + {labels.length - MAX_DISPLAYED_INHERITED_ROLES_TRAITS} more
        </TruncatingLabel>
      )}
    </Flex>
  );
};

export const RoleAndTraitLabels = ({
  roles,
  traits,
  required = false,
  truncate = false,
}: {
  roles: string[];
  traits: string[];
  required?: boolean;
  truncate?: boolean;
}) => {
  const renderRoles = required ? roles.length > 0 : true;
  const renderTraits = required ? traits.length > 0 : true;

  return renderRoles || renderTraits ? (
    <Flex flexDirection="column" gap={1} alignItems="start">
      {renderRoles && (
        <Flex alignItems="start" gap={2}>
          <TextNoEllipsis mt={1}>Roles:</TextNoEllipsis>
          {renderTruncatingLabels(
            roles,
            truncate && roles.length > MAX_DISPLAYED_INHERITED_ROLES_TRAITS
          )}
        </Flex>
      )}
      {renderTraits && (
        <Flex alignItems="start" gap={2}>
          <TextNoEllipsis mt={1}>Traits:</TextNoEllipsis>
          {renderTruncatingLabels(
            traits,
            truncate && traits.length > MAX_DISPLAYED_INHERITED_ROLES_TRAITS
          )}
        </Flex>
      )}
    </Flex>
  ) : null;
};

export const EnrollingNestedListsAlert = ({
  kind,
  listName,
}: {
  kind: 'owner' | 'member';
  listName: string;
}) => (
  <Alert kind="info" linkColor="buttons.link.default">
    Enrolling Access Lists will grant {kind}ship{' '}
    {kind === 'member' ? 'in' : 'of'} '{listName}' to all their members. Learn
    more in the{' '}
    <Link
      href="https://goteleport.com/docs/reference/access-controls/access-lists/"
      target="_blank"
    >
      Teleport documentation
    </Link>
    .
  </Alert>
);

export const MAX_DISPLAYED_INHERITED_ROLES_TRAITS = 5;
