import React from 'react';
import styled from 'styled-components';
import { components } from 'react-select';
import { Label, Text } from 'design';
import Link from 'design/Link';
import { User as UserIcon, UserList } from 'design/Icon';
import ResourceService from 'teleport/services/resources';
import { yamlService } from 'teleport/services/yaml';
import { YamlSupportedResourceKind } from 'teleport/services/yaml/types';

import { AccessListMemberKind } from 'e-teleport/services/accessmanagement';

import type { Option } from 'shared/components/Select';
import type { AllUserTraits, User } from 'teleport/services/user';
import type { GroupBase, OptionProps, MultiValueProps } from 'react-select';
import type { AccessList } from 'e-teleport/services/accessmanagement';
import type { AccessListWithModifiedGrants } from 'e-teleport/AccessListManagement/AccessLists/AccessLists';
import type { useHistory } from 'react-router-dom';
import type { Theme } from 'design/theme/themes/types';
import type useAttempt from 'shared/hooks/useAttemptNext';
import type { Role } from 'teleport/services/resources';

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

export type MemberSelection = {
  name: string;
  membershipKind: AccessListMemberKind;
};

type accessListLocationState = {
  createdList?: AccessList;
  reviewedAccessList?: AccessList;
  deletedAccessListId?: string;
  previousPaths?: string[];
};

/**
 * If location state is set, user came to this view from
 * either creating, deleting, or reviewing an access list.
 * Because of caching, the list from backend won't be updated
 * right way, so we manually update/remove the list here.
 */
export const updateAccessListsCache = <
  T extends (AccessList | AccessListWithModifiedGrants)[],
>(
  currentLists: T,
  locationState: accessListLocationState | null | undefined,
  history: ReturnType<typeof useHistory>
): T => {
  if (!locationState) {
    return currentLists;
  }

  if (locationState.createdList) {
    const foundList = currentLists.find(
      l => l.id === locationState.createdList.id
    );
    if (!foundList) {
      currentLists.push(locationState.createdList);
    }
  }
  if (locationState.deletedAccessListId) {
    currentLists = currentLists.filter(
      l => l.id !== locationState.deletedAccessListId
    ) as T;
  }
  if (locationState.reviewedAccessList) {
    const foundIndex = currentLists.findIndex(
      l => l.id === locationState.reviewedAccessList.id
    );
    if (
      foundIndex > -1 &&
      currentLists[foundIndex].audit.nextDate !=
        locationState.reviewedAccessList.audit.nextDate
    ) {
      currentLists[foundIndex] = locationState.reviewedAccessList;
    }
  }

  // Clear state afterward but preserve query
  history.replace({
    state: {},
    pathname: location.pathname,
    search: location.search,
  });

  return currentLists;
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
  const isAccessListOpt =
    typeof restProps.data?.value === 'object' &&
    'membershipKind' in restProps.data.value &&
    restProps.data.value?.membershipKind === AccessListMemberKind.List;

  return (
    <ReactSelectAccessListOptWrapper>
      <components.Option {...restProps}>
        <ReactSelectAccessListOptionBadge>
          {isAccessListOpt ? <UserList size={14} /> : <UserIcon size={14} />}
        </ReactSelectAccessListOptionBadge>
        {children}
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

export const TruncatingLabel = styled(Label)<{ inverse?: boolean }>`
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  max-width: 160px;
  ${inverseLabel}
`;

export function matchRoles(
  rolesRequiredToBeEligible: string[],
  userOptions: UserOption[]
): UserOption[] {
  return userOptions.filter(userOpt => {
    const currRolesAssigned = userOpt.value.roles;
    return rolesRequiredToBeEligible.every(requiredRole =>
      currRolesAssigned.includes(requiredRole)
    );
  });
}

export function matchTraits(
  traitsRequiredToBeEligible: AllUserTraits,
  userOptions: UserOption[]
): UserOption[] {
  const requiredTraitKeys = Object.keys(traitsRequiredToBeEligible);

  return userOptions.filter(userOpt => {
    const currTraits = userOpt.value.allTraits;
    return requiredTraitKeys.every(requiredTraitKey => {
      const matchRequiredVals = traitsRequiredToBeEligible[requiredTraitKey];
      return (
        currTraits[requiredTraitKey] &&
        matchRequiredVals.every(requiredVal =>
          currTraits[requiredTraitKey].some(currVal => requiredVal == currVal)
        )
      );
    });
  });
}

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
