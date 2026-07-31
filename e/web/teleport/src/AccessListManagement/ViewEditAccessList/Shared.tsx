import React, { Ref, type PropsWithChildren } from 'react';
import styled from 'styled-components';

import { Alert, Box, ButtonIcon, ButtonSecondary, Flex } from 'design';
import { Cell } from 'design/DataTable';
import { Pencil, UserCheck, UserIdBadge } from 'design/Icon';
import Link from 'design/Link';
import { HoverTooltip, IconTooltip } from 'design/Tooltip';
import { AccessListUserAssignmentType } from 'gen-proto-ts/teleport/accesslist/v1/accesslist_pb';
import { Option } from 'shared/components/Select';
import { UserDisplayName } from 'shared/components/UserDisplayName';

import {
  convertToTraitConvenience,
  TraitConvenience,
} from 'e-teleport/AccessListManagement/Traits';
import {
  AccessList,
  AccessListGrant,
  AccessListMember,
  AccessListOrigin,
  AccessListOwner,
  AccessListRequires,
  isReviewable,
  ScopedRoleGrant,
} from 'e-teleport/services/accessmanagement';
import useTeleport from 'e-teleport/useTeleportE';
import type { Access } from 'teleport/services/user';

import {
  accessListRequiresReview,
  getReviewDate,
  useAccessListManagementContext,
} from '../AccessListManagementContext';
import { TruncatingLabel, type MemberSelection } from '../Shared/Shared';

export type AccessListRequiresWithTraitConvenience = AccessListRequires &
  TraitConvenience;

export type AccessListGrantWithTraitConvenience = AccessListGrant &
  TraitConvenience;

export type AccessListModified = AccessList & {
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
  accessList,
}: {
  accessListAccess?: Access;
  accessList?: AccessList;
  isOwner?: boolean;
}): Perms => {
  const isOwner = [
    AccessListUserAssignmentType.EXPLICIT,
    AccessListUserAssignmentType.INHERITED,
  ].includes(accessList?.currentUserAssignments?.ownershipType);

  return {
    isOwner: isOwner,
    adminWhoCanRead: accessListAccess?.read && accessListAccess?.list,
    adminWhoCanEdit: accessListAccess?.edit,
    adminWhoCanDelete: accessListAccess?.remove,
  };
};

export type ReviewNotApplicableReason =
  | 'okta-read-only'
  | 'not-due'
  | 'static'
  | null;

export type AccessListReviewStatus = {
  canReview: boolean;
  requiresReview: boolean;
  reason: ReviewNotApplicableReason;
};

/**
 * Centralized hook for determining access list review status.
 * Reads user permissions and Okta config from AccessListManagementContext.
 */
export function useAccessListReviewStatus(
  accessList: AccessList | undefined
): AccessListReviewStatus {
  const { isOktaPluginReadOnly } = useAccessListManagementContext();
  const ctx = useTeleport();

  if (!accessList) {
    return { canReview: false, requiresReview: false, reason: null };
  }

  const accessListAccess = ctx.storeUser.getAccessListAccess();
  const { isOwner, adminWhoCanEdit } = getPerms({
    accessListAccess,
    accessList,
  });
  const canReview = isOwner || adminWhoCanEdit;

  if (!isReviewable(accessList.type)) {
    return { canReview, requiresReview: false, reason: 'static' };
  }

  if (isOktaPluginReadOnly && accessList.origin === AccessListOrigin.Okta) {
    return { canReview, requiresReview: false, reason: 'okta-read-only' };
  }

  const reviewDate = getReviewDate(accessList);
  const requiresReview = accessListRequiresReview({
    todayDate: new Date(),
    reviewDate,
  });

  return {
    canReview,
    requiresReview,
    reason: requiresReview ? null : 'not-due',
  };
}

export const modifyAccessList = (acl: AccessList): AccessListModified => {
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
      reviewDate: getReviewDate(acl),
    }),
    members: [...acl.members],
  };

  return modifiedAccessList;
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
  tooltip = undefined,
  onClick,
  ineligibleReason,
  hideIneligibleReason = false,
  isReviewing = false,
}: {
  disabled: boolean;
  tooltip?: string;
  onClick(): void;
  ineligibleReason: string;
  // when reviewing, hide ineligible reason
  // since deleting a member during review isn't
  // a dynamic change.
  hideIneligibleReason?: boolean;
  isReviewing?: boolean;
}) => (
  <Cell align="right">
    <Flex alignItems="center" justifyContent="flex-end">
      {!hideIneligibleReason && ineligibleReason && (
        <Box mr={3}>
          <IconTooltip kind="warning" position="left">
            {ineligibleReason}
          </IconTooltip>
        </Box>
      )}
      <HoverTooltip tipContent={tooltip} position="left">
        <ButtonSecondary
          textTransform="none"
          disabled={disabled}
          onClick={onClick}
          size="small"
        >
          {isReviewing ? 'Remove' : 'Delete'}
        </ButtonSecondary>
      </HoverTooltip>
    </Flex>
  </Cell>
);

export type AlreadyEnrolledUser = {
  name: string;
  displayPrimary?: string;
};

export function AlreadyEnrolledUsersAlert({
  users,
}: {
  users: AlreadyEnrolledUser[];
}) {
  return (
    <Alert kind="danger">
      The following users are already enrolled. Remove them from the list to
      continue:{' '}
      {users.map((user, index) => (
        <React.Fragment key={user.name}>
          {index > 0 && ', '}
          <UserDisplayName
            username={user.name}
            primaryText={user.displayPrimary}
            layout="inline"
          />
        </React.Fragment>
      ))}
    </Alert>
  );
}

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
  const duplicateUsers: AlreadyEnrolledUser[] = [];
  const newUsers = selectedUsers.filter(opt => {
    const existingUser = existingUsers.find(m => m.name === opt.value.name);
    if (!existingUser) {
      return true;
    }

    duplicateUsers.push({
      name: existingUser.name,
      ...(existingUser.displayPrimary && {
        displayPrimary: existingUser.displayPrimary,
      }),
    });
    return false;
  });

  return { duplicateUsers, newUsers };
}

export function ButtonPencil({
  onClick,
  disabled = false,
  title = '',
  mt = 0,
  ref,
  dataTestId,
}: {
  onClick(): void;
  disabled?: boolean;
  title?: string;
  mt?: number;
  ref?: Ref<HTMLButtonElement>;
  dataTestId?: string;
}) {
  return (
    <ButtonIcon
      onClick={onClick}
      disabled={disabled}
      title={title}
      css={mt && { marginTop: `${mt}px` }}
      ref={ref}
      data-testid={dataTestId}
    >
      <Pencil size={16} />
    </ButtonIcon>
  );
}

const TextNoEllipsis = styled(Box)`
  ${p => p.theme.typography.body3}
`;

const labelPrefixes = {
  trait: '[Trait]',
  role: '[Role]',
  scopedRole: '[Scoped Role]',
} as const;

const renderTruncatingLabels = (
  labels: string[] = [],
  labelKind: 'trait' | 'role' | 'scopedRole'
) => {
  const forRole = labelKind !== 'trait';
  const labelPrefix = labelPrefixes[labelKind];
  return (
    <>
      {labels.map((label, index) => (
        <TruncatingLabel
          key={`${label}${index}`}
          kind="secondary"
          title={`${labelPrefix} ${label}`}
          labelForRole={forRole}
        >
          {forRole ? (
            <UserIdBadge size={16} mr={1} />
          ) : (
            <UserCheck size={14} mr={1} />
          )}
          {label}
        </TruncatingLabel>
      ))}
    </>
  );
};

const formatScopedRoleLabel = ({ role, scope }: ScopedRoleGrant) => {
  return `${role} (${scope})`;
};

export const RoleAndTraitLabels = ({
  roles,
  scopedRoles,
  traits,
  accessKind,
  editDisabled,
  onEdit,
  toolTipContent,
  userKind,
}: {
  roles: string[];
  scopedRoles?: ScopedRoleGrant[];
  traits: string[];
  required?: boolean;
  accessKind: 'requirements' | 'grants' | 'inherited';
  userKind: 'member' | 'owner';
  editDisabled?: boolean;
  onEdit?(): void;
  toolTipContent?: string;
}) => {
  let prefix;
  let tipContent;
  switch (accessKind) {
    case 'requirements':
      prefix = 'Required Permissions';
      tipContent = `If a ${userKind} does not meet all required roles and traits defined here, ${userKind}ship will have no effect; ${userKind}s will not be granted any additional roles or traits.`;
      break;
    case 'grants':
      prefix = 'Granted Permissions';
      break;
    case 'inherited':
      prefix = 'Inherited Permissions';
      tipContent = `Additional permission grants inherited as a result of this Access List being nested into another Access List.`;
      break;
  }

  return (
    <Flex alignItems="center">
      <HoverTooltip tipContent={tipContent}>
        <TextNoEllipsis css={{ minWidth: '128px' }}>
          {accessKind === 'inherited' ? (
            <Link
              target="_blank"
              href="https://goteleport.com/docs/admin-guides/access-controls/access-lists/nested-access-lists/"
            >
              {prefix}
            </Link>
          ) : (
            prefix
          )}
          :
        </TextNoEllipsis>
      </HoverTooltip>
      <Flex alignItems="center" gap={1} flexWrap="wrap">
        {renderTruncatingLabels(roles, 'role')}
        {renderTruncatingLabels(
          scopedRoles?.map(formatScopedRoleLabel),
          'scopedRole'
        )}
        {renderTruncatingLabels(traits, 'trait')}
        {onEdit && (
          <HoverTooltip tipContent={toolTipContent}>
            <ButtonPencil
              onClick={onEdit}
              disabled={editDisabled}
              dataTestId={`btn-${userKind}-${accessKind}`}
            />
          </HoverTooltip>
        )}
      </Flex>
    </Flex>
  );
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
