import React, { PropsWithChildren, useState } from 'react';
import {
  components,
  type GroupBase,
  type MultiValueProps,
  type OptionProps,
} from 'react-select';
import styled from 'styled-components';

import { Label, Popover, Text } from 'design';
import { User as UserIcon, UserList } from 'design/Icon';
import Link from 'design/Link';
import type { Theme } from 'design/theme/themes/types';
import type { Option } from 'shared/components/Select';
import type useAttempt from 'shared/hooks/useAttemptNext';

import {
  AccessListMemberKind,
  AccessListOrigin,
} from 'e-teleport/services/accessmanagement';
import ResourceService, { type Role } from 'teleport/services/resources';
import type { User } from 'teleport/services/user';
import { yamlService } from 'teleport/services/yaml';
import { YamlSupportedResourceKind } from 'teleport/services/yaml/types';

import { TypeBadge } from './TypeBadge';

// HybridUserOption
//
// An Option can be type User if a user selected
// an option from the dropdown options. This
// is only possible if the user had "user" rbac
// and was able to fetch users from the back.
//
// An Option can be type String if a user manually
// entered a user. This is allowed because
// not all users will have the rbac to
// list users and SSO users may not necessarily
// exist in the backend yet.
export type HybridUserOption = Option<User | MemberSelection>;
export type UserOption = Option<User>;
export type EditKind = 'Member' | 'Owner' | 'Grants' | 'OwnerGrants';
export type TextEditKind = 'Title' | 'Description';

export type MemberSelection = {
  name: string;
  membershipKind: AccessListMemberKind;
  origin?: AccessListOrigin;
};

const ReactSelectAccessListOptionBadge = styled.span`
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: ${p => p.theme.space[1]}px;
  margin-bottom: -1px;
  color: ${p => p.theme.colors.text.slightlyMuted};
  background-color: ${p => p.theme.colors.interactive.tonal.neutral[0]};
  border-radius: ${p => p.theme.radii[2]}px;
`;

const ReactSelectAccessListMultiValueBadge = styled(
  ReactSelectAccessListOptionBadge
)`
  margin-bottom: 0;
  padding: ${p => p.theme.space[1]}px ${p => p.theme.space[0]}px;
  background-color: transparent;
`;

const ReactSelectAccessListOptWrapper = styled.div`
  display: contents;

  .react-select__multi-value__label,
  .react-select__option {
    display: flex;
    flex-direction: row;
    justify-content: space-between;
    align-items: center;
    gap: ${p => p.theme.space[2]}px;
  }

  .react-select__option {
    justify-content: flex-start;
  }
`;

export function ReactSelectAccessListOption<
  Option extends HybridUserOption,
  IsMulti extends boolean,
  Group extends GroupBase<Option>,
>({ children, ...restProps }: OptionProps<Option, IsMulti, Group>) {
  const data = restProps.data?.value;
  let isAccessListOpt = false;
  let origin;
  if (typeof data === 'object') {
    if ('membershipKind' in data) {
      isAccessListOpt = data.membershipKind === AccessListMemberKind.List;
      if (isAccessListOpt) {
        origin = data.origin;
      }
    }
  }

  return (
    <ReactSelectAccessListOptWrapper>
      <components.Option {...restProps}>
        <ReactSelectAccessListOptionBadge>
          {isAccessListOpt ? <UserList size={14} /> : <UserIcon size={14} />}
        </ReactSelectAccessListOptionBadge>
        {children}
        {origin && <TypeBadge type={origin} />}
      </components.Option>
    </ReactSelectAccessListOptWrapper>
  );
}

export function ReactSelectAccessListMultiValue<
  Option extends HybridUserOption,
  IsMulti extends boolean,
  Group extends GroupBase<Option>,
>({ children, ...restProps }: MultiValueProps<Option, IsMulti, Group>) {
  const isAccessListOpt =
    typeof restProps.data?.value === 'object' &&
    'membershipKind' in restProps.data.value &&
    restProps.data.value?.membershipKind === AccessListMemberKind.List;

  return (
    <ReactSelectAccessListOptWrapper>
      <components.MultiValue {...restProps}>
        <ReactSelectAccessListMultiValueBadge>
          {isAccessListOpt ? <UserList size={13} /> : <UserIcon size={13} />}
        </ReactSelectAccessListMultiValueBadge>
        {children}
      </components.MultiValue>
    </ReactSelectAccessListOptWrapper>
  );
}

/**
 * Inverses the color scheme if required. Using the slightly muted foreground
 * as a background is a bit icky, but I see no better choice.
 */
const inverseLabel = ({
  theme,
  inverse,
}: {
  theme: Theme;
  inverse?: boolean;
}) =>
  inverse
    ? {
        color: theme.colors.text.primaryInverse,
        background: theme.colors.text.slightlyMuted,
      }
    : {};

export const ToolTipText: React.FC<
  PropsWithChildren<{
    tipContent: React.ReactElement;
    fontSize?: number;
  }>
> = ({ tipContent, fontSize = 10, children }) => {
  const [anchorEl, setAnchorEl] = useState<HTMLElement>(null);
  const open = Boolean(anchorEl);

  function handlePopoverOpen(
    event: React.MouseEvent<HTMLButtonElement, MouseEvent>
  ) {
    setAnchorEl(event.currentTarget);
  }

  function handlePopoverClose() {
    setAnchorEl(null);
  }

  return (
    <>
      <span
        aria-owns={open ? 'mouse-over-popover' : undefined}
        onMouseEnter={handlePopoverOpen}
        onMouseLeave={handlePopoverClose}
      >
        {children}
      </span>
      <Popover
        modalCss={modalCss}
        onClose={handlePopoverClose}
        open={open}
        anchorEl={anchorEl}
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'left',
        }}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'left',
        }}
      >
        <StyledOnHover px={2} py={1} fontSize={`${fontSize}px`}>
          {tipContent}
        </StyledOnHover>
      </Popover>
    </>
  );
};

const modalCss = () => `
  pointer-events: none;
`;

const StyledOnHover = styled(Text)`
  color: ${props => props.theme.colors.text.main};
  background-color: ${props => props.theme.colors.tooltip.background};
  max-width: 350px;
`;

export const TruncatingLabel = styled(Label)<{
  inverse?: boolean;
  labelForRole?: boolean;
  truncate?: boolean;
}>`
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  ${inverseLabel}
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  ${p => (p.truncate ? `max-width: 160px;` : '')}
  ${p => (p.truncate ? `flex-wrap: wrap;` : '')}

  ${p =>
    p.labelForRole
      ? `background-color: ${p.theme.colors.interactive.tonal.informational[0]};`
      : ''}
`;

export const NestedListLink = styled('button')`
  border: none;
  outline: none;
  background: none;
  width: 100%;
  cursor: pointer;
  text-decoration: underline;
  text-align: left;
  display: inline-flex;
  flex-direction: row;
  align-items: center;
  justify-content: flex-start;
  column-gap: ${p => p.theme.space[2]}px;
  padding: 0;
  font-family: ${p => p.theme.fonts.sansSerif};
  color: ${p => p.theme.colors.text.main};
  display: inline-block;
  text-overflow: ellipsis;
  overflow: hidden;
  white-space: nowrap;
  max-width: fit-content;
  flex-shrink: 1;

  ${p => p.theme.typography.body2};

  &:hover,
  &:focus-visible {
    text-decoration: none;
  }

  &:disabled {
    cursor: not-allowed;
  }
`;

export const fetchAndProcessSelectedRoles = async (
  attempt: ReturnType<typeof useAttempt>,
  selectedRoles: Option[]
) => {
  attempt.setAttempt({ status: 'processing' });
  const fetchedRoles = await new ResourceService()
    .fetchRoles()
    .then(r => r.items)
    .catch(() => []);
  const parsedRoles: Role[] = [];

  for (const { value: roleName } of selectedRoles) {
    const role = fetchedRoles.find(r => r.name === roleName);
    if (!role) {
      continue;
    }
    await yamlService
      .parse<Role>(YamlSupportedResourceKind.Role, { yaml: role.content })
      .then(p => parsedRoles.push(p))
      .catch(console.warn);
  }
  attempt.setAttempt({ status: 'success' });
  return parsedRoles;
};

export const rolesContainDenyRules = (processedRoles: Role[]) => {
  if (!processedRoles?.length) {
    return undefined;
  }
  for (const role of processedRoles) {
    if (Object.keys(role?.spec?.deny ?? {})?.length > 0) {
      return (
        <Text>
          Role '{role.metadata.name}' contains Deny rules. Use of Deny rules in
          Access Lists can cause unexpected behavior and is discouraged.
          Consider assigning Deny rules directly. Learn more in the{' '}
          <Link
            href="https://goteleport.com/docs/reference/access-controls/access-lists/#access-lists-and-deny-rules"
            target="_blank"
          >
            Teleport documentation
          </Link>
          .
        </Text>
      );
    }
  }
  return undefined;
};
