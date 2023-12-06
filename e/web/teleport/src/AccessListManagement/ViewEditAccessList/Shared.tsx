import React, { PropsWithChildren } from 'react';
import styled from 'styled-components';
import { Flex, ButtonSecondary, ButtonIcon, Box } from 'design';
import { Cell } from 'design/DataTable';
import { Pencil, Warning } from 'design/Icon';
import { Option } from 'shared/components/Select';

import {
  AccessListMember,
  AccessListOwner,
  AccessListRequires,
} from 'e-teleport/services/accessmanagement';

import {
  ToolTipText,
  matchRoles,
  matchTraits,
  UserOption,
  TruncatingLabel,
} from '../Shared/Shared';

export const CustomCell: React.FC<PropsWithChildren<{ disabled: boolean }>> = ({
  disabled,
  children,
}) => {
  return (
    <Cell
      css={`
        opacity: ${disabled ? '0.5' : '1'};
        max-width: 150px;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
      `}
      title={children}
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
}: {
  disabled: boolean;
  btnTitle: string;
  onClick(): void;
  ineligibleReason: string;
  // when reviewing, hide ineligible reason
  // since deleting a member during review isn't
  // a dynamic change.
  hideIneligibleReason?: boolean;
}) => {
  return (
    <Cell align="right">
      <Flex alignItems="center" justifyContent="flex-end">
        {!hideIneligibleReason && ineligibleReason && (
          <ToolTipText
            tipContent={
              <div css={{ maxWidth: '220px' }}>{ineligibleReason}</div>
            }
          >
            <Warning color="warning.active" mr={3} />
          </ToolTipText>
        )}
        <ButtonSecondary
          disabled={disabled}
          title={btnTitle}
          onClick={onClick}
          size="small"
        >
          Delete
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
  selectedUsers: Option[]
) {
  const duplicateUsers: string[] = [];
  const newUsers = selectedUsers.filter(opt => {
    if (existingUsers.some(m => m.name === opt.value)) {
      duplicateUsers.push(opt.value);
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
): Option[] {
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
      // Filter out existing existing users among users.
      .filter(u => existingUsers.every(m => m.name !== u.value.name))
      // Convert to type Option for dropdowns.
      .map(u => ({ label: u.value.name, value: u.value.name }))
  );
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
  font-size: ${p => p.theme.fontSizes[1]}px;
`;

const renderTruncatingLabels = (labels: string[] = []) => {
  const $labels = labels.map((label, index) => (
    <TruncatingLabel
      mr={index === labels.length - 1 ? 0 : 1}
      key={`${label}${index}`}
      kind="secondary"
      title={label}
    >
      {label}
    </TruncatingLabel>
  ));

  return <Flex flexWrap="wrap">{$labels}</Flex>;
};

export const RoleAndTraitLabels = ({
  roles,
  traits,
  required = false,
}: {
  roles: string[];
  traits: string[];
  required?: boolean;
}) => {
  let renderRoles = true;
  let renderTraits = true;
  if (required) {
    renderRoles = roles.length > 0;
    renderTraits = traits.length > 0;
  }
  return (
    <>
      {renderRoles && (
        <Flex alignItems="center">
          <TextNoEllipsis mr={1}>Roles:</TextNoEllipsis>
          {renderTruncatingLabels(roles)}
        </Flex>
      )}
      {renderTraits && (
        <Flex alignItems="center">
          <TextNoEllipsis mr={1}>Traits:</TextNoEllipsis>
          {renderTruncatingLabels(traits)}
        </Flex>
      )}
    </>
  );
};
