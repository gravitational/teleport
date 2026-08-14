import { useQueryClient } from '@tanstack/react-query';
import { useCallback, useState } from 'react';
import styled from 'styled-components';

import { Box, ButtonIcon, Flex, Text } from 'design';
import { Trash } from 'design/Icon';
import { inputGeometry } from 'design/Input/Input';
import { ButtonWithAddIcon } from 'shared/components/ButtonWithAddIcon';
import FieldInput from 'shared/components/FieldInput';
import { FieldSelectAsync } from 'shared/components/FieldSelect';
import type { Option } from 'shared/components/Select';

import {
  type ScopedRoleListItem,
  type ScopedRoleGrant,
  useRootScopedRoles,
  createRootScopedRolesQuery,
} from 'e-teleport/services/accessmanagement';

type Props = {
  userKind: 'Members' | 'Owners';
  isDisabled: boolean;
  scopedRoleGrants: ScopedRoleGrant[];
  updateScopedRoleGrants(scopedroleGrants: ScopedRoleGrant[]): void;
};

export function ScopedRoleGrantsEditor({
  userKind,
  isDisabled,
  scopedRoleGrants,
  updateScopedRoleGrants,
}: Props) {
  const queryClient = useQueryClient();

  // Holds fetched role metadata for validation of assignableScopes, keyed by
  // role name. Populated as a side effect whenever scoped roles are fetched.
  const [knownScopedRoles, setKnownScopedRoles] = useState<
    Record<string, ScopedRoleListItem>
  >({});

  const rememberRoles = useCallback((roles: ScopedRoleListItem[]) => {
    if (!roles.length) {
      return;
    }
    setKnownScopedRoles(prev => ({
      ...prev,
      ...Object.fromEntries(roles.map(role => [qualifiedName(role), role])),
    }));
  }, []);

  const limit = 20;
  const fetchScopedRoleOptions = useCallback(
    async (filter: string): Promise<Option[]> => {
      filter = filter.trim().toLowerCase();

      const resp = await queryClient.fetchQuery({
        ...createRootScopedRolesQuery({ filter, limit }),
        staleTime: 60_000,
      });

      rememberRoles(resp.roles);
      return toOptions(resp.roles);
    },
    [queryClient, rememberRoles]
  );

  const { data: unfilteredResp } = useRootScopedRoles({ filter: '', limit });
  const unfilteredRoles = unfilteredResp?.roles ?? [];

  if (scopedRoleGrants.length == 0 && unfilteredRoles.length == 0) {
    // Don't show the scoped role picker at all if there are no existing scoped
    // role grants and unfilteredRoles is empty (implying the cluster has no
    // scoped roles or the query has not returned yet). This means the
    // component may initially render nothing, and then re-render after the
    // initial scoped roles are fetched. This should be acceptable while scopes
    // are in development, we don't want to show anything here to most users
    // unless we know scopes are in use.
    return null;
  }

  function updateGrantRole(index: number, roleName: string) {
    const nextScopedRoles = [...scopedRoleGrants];
    nextScopedRoles[index] = {
      role: roleName,
      scope: nextScopedRoles[index]?.scope || '',
    };
    updateScopedRoleGrants(nextScopedRoles);
  }

  function updateGrantScope(index: number, scope: string) {
    // If a scoped role with this name is not known, fetch it, in case the user
    // is editing the assigned scope on an existing scoped role grant that did
    // not trigger a fetch. Once the role is fetched, the assignable scopes can
    // be used for input validation.
    const roleName = scopedRoleGrants[index].role;
    if (roleName && !knownScopedRoles[roleName]) {
      fetchScopedRoleOptions(roleName);
    }

    const nextScopedRoles = [...scopedRoleGrants];
    nextScopedRoles[index] = {
      ...nextScopedRoles[index],
      scope,
    };
    updateScopedRoleGrants(nextScopedRoles);
  }

  function addScopedRole() {
    const nextScopedRoles = [...scopedRoleGrants, { role: '', scope: '' }];
    updateScopedRoleGrants(nextScopedRoles);
  }

  function removeScopedRole(index: number) {
    const nextScopedRoles = scopedRoleGrants.filter((_, i) => i !== index);
    updateScopedRoleGrants(nextScopedRoles);
  }

  const inputSize = 'medium';

  return (
    <Box>
      <Fieldset>
        {scopedRoleGrants.length > 0 && (
          <Legend>Scoped Roles Granted to {userKind}</Legend>
        )}
        {scopedRoleGrants.map((grant, index) => {
          const { errorText, helpText, placeHolderText } =
            validateScopedRoleGrant(grant, knownScopedRoles);
          return (
            <Flex key={index} mb="2" gap="2" alignItems="start">
              <FieldSelectAsync
                label={index == 0 && 'Scoped Role Name'}
                ariaLabel={`Scoped Role Name ${index + 1}`}
                width="40%"
                placeholder="Select a scoped role"
                loadOptions={fetchScopedRoleOptions}
                defaultOptions={true}
                value={
                  grant.role ? { value: grant.role, label: grant.role } : null
                }
                isDisabled={isDisabled}
                menuPosition="fixed"
                onChange={(opt: Option | null) =>
                  updateGrantRole(index, opt?.value || '')
                }
                mb={0}
              />
              <Flex flexDirection="column" width="60%">
                <FieldInput
                  label={index == 0 && 'Assigned Scope'}
                  size={inputSize}
                  flex="1"
                  placeholder={placeHolderText}
                  value={grant.scope}
                  onChange={e => updateGrantScope(index, e.target.value)}
                  disabled={isDisabled}
                  mb={0}
                />
                <ValidationFeedback errorText={errorText} helpText={helpText} />
              </Flex>
              <Flex
                alignItems="center"
                height={inputGeometry[inputSize].height}
                mt={index == 0 ? 4 : 0}
              >
                <ButtonIcon
                  size={1}
                  title="Remove Scoped Role"
                  aria-label={`Remove Scoped Role ${index + 1}`}
                  onClick={() => removeScopedRole(index)}
                  disabled={isDisabled}
                >
                  <Trash size="medium" />
                </ButtonIcon>
              </Flex>
            </Flex>
          );
        })}
      </Fieldset>
      <ButtonWithAddIcon
        onClick={addScopedRole}
        disabled={isDisabled}
        label={
          scopedRoleGrants.length > 0
            ? 'Add another Scoped Role Grant'
            : 'Add a Scoped Role Grant'
        }
      />
    </Box>
  );
}

function ValidationFeedback({
  errorText,
  helpText,
}: {
  errorText?: string;
  helpText?: string;
}) {
  if (!errorText && !helpText) {
    return null;
  }
  return (
    <Box mt="2" ml="1" mb="0">
      <Text color="error.main" typography="body3">
        {errorText}
      </Text>
      <Text color="text.slightlyMuted" typography="body3">
        {helpText}
      </Text>
    </Box>
  );
}

const Legend = styled.legend`
  margin: 0 0 ${props => props.theme.space[1]}px 0;
  padding: 0;
  ${props => props.theme.typography.body3}
`;

const Fieldset = styled.fieldset`
  border: none;
  margin: 0;
  padding: 0;
`;

function qualifiedName(role: ScopedRoleListItem): string {
  return `${role.scope}::${role.name}`;
}

function toOptions(roles: ScopedRoleListItem[]): Option[] {
  return roles.map(role => ({
    value: qualifiedName(role),
    label: qualifiedName(role),
  }));
}

function validateScopedRoleGrant(
  grant: ScopedRoleGrant,
  knownScopedRoles: Record<string, ScopedRoleListItem>
): {
  placeHolderText: string;
  errorText?: string;
  helpText?: string;
} {
  let placeHolderText = 'Select a role first';

  if (!grant.role && !grant.scope) {
    return {
      placeHolderText,
      helpText: 'Choose a scoped role and enter a scope.',
    };
  }

  if (!grant.role) {
    return {
      placeHolderText,
      helpText: 'Select a role first',
    };
  }

  const role = knownScopedRoles[grant.role];
  if (!role) {
    return { placeHolderText };
  }

  let helpText = 'Role has no assignable scopes';
  if (role.assignableScopes.length > 0) {
    helpText = `Allowed patterns: ${role.assignableScopes.join(', ')}`;
    placeHolderText = `Enter a scope like ${role.assignableScopes[0]}`;
  }

  const scopeError = validateScopeLiteral(grant.scope);
  if (scopeError) {
    return {
      placeHolderText,
      errorText: scopeError,
      helpText,
    };
  }

  if (
    !role.assignableScopes.some(pattern =>
      matchesAssignableScope(pattern, grant.scope)
    )
  ) {
    return {
      placeHolderText,
      errorText: `Scope must match one of: ${role.assignableScopes.join(', ')} `,
      helpText,
    };
  }

  return { placeHolderText };
}

const segmentRegexp = /^[a-z0-9][a-z0-9\-_.]*[a-z0-9]$/;
const maxScopeSize = 64;
const maxSegmentSize = 36;
const minSegmentSize = 2;

function validateScopeLiteral(scope: string): string | undefined {
  if (!scope) {
    return 'Scope is required.';
  }

  if (!scope.startsWith('/')) {
    return 'Scope must start with /';
  }

  if (scope !== '/' && scope.endsWith('/')) {
    return 'Scope cannot end with /';
  }

  if (scope.length > maxScopeSize) {
    return `Scope must be ${maxScopeSize} characters or shorter.`;
  }

  if (scope === '/') {
    return undefined;
  }

  for (const segment of scope.slice(1).split('/')) {
    if (segment.length < minSegmentSize) {
      return `Each segment must be at least ${minSegmentSize} characters.`;
    }

    if (segment.length > maxSegmentSize) {
      return `Each segment must be ${maxSegmentSize} characters or shorter.`;
    }

    if (/[A-Z]/.test(segment)) {
      return 'Scope segments cannot contain uppercase letters.';
    }

    if (!segmentRegexp.test(segment)) {
      return 'Scope segments must start and end with a lowercase letter or number and may only contain lowercase letters, numbers, hyphens, underscores, and periods.';
    }
  }

  return undefined;
}

function matchesAssignableScope(pattern: string, scope: string): boolean {
  if (pattern === '/') {
    return scope.startsWith('/');
  }

  if (pattern === '/**') {
    return scope !== '/' && scope.startsWith('/');
  }

  if (pattern.endsWith('/**')) {
    const prefix = pattern.slice(0, -3);
    return scope.startsWith(`${prefix}/`);
  }

  return scope === pattern || scope.startsWith(`${pattern}/`);
}
