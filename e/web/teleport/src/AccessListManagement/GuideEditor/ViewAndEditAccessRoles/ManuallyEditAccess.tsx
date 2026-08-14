import { useMutation, useQuery } from '@tanstack/react-query';
import { useState } from 'react';

import { Alert, Box, ButtonBorder, H2, Text } from 'design';
import Table, { Cell } from 'design/DataTable';

import { useRoleWithAccessGraph } from 'e-teleport/Roles/useRoleWithAccessGraph';
import { RoleEditorDialog } from 'teleport/Roles/RoleEditor/RoleEditorDialog';
import { toYaml } from 'teleport/Roles/useRoles';
import { RoleWithYaml } from 'teleport/services/resources';
import { fetchRole, updateRole } from 'teleport/services/resources/resource';
import useTeleport from 'teleport/useTeleport';

import { getMissingRoleAccess } from '../Preset/role/role';

/**
 * Fallback UI for editing member grant roles when the preset UI cannot parse
 * them.
 *
 * This component is displayed when the access list's member grant roles
 * contain unsupported fields (e.g., deny rules) or don't match the expected
 * preset structure. This typically happens when:
 * - A user manually edited the roles outside of the preset UI
 * - The access list's member grants were modified to include their own roles
 *
 * Instead of showing an incomplete or inaccurate access definition, this component
 * provides direct access to the role editor so users can view and modify the
 * actual role configurations for members.
 */
export function ManuallyEditAccess({ roles }: { roles: string[] }) {
  const [editRoleName, setEditRoleName] = useState('');

  const ctx = useTeleport();

  const missingWriteRoleAccess = getMissingRoleAccess(
    ctx.storeUser.getRoleAccess(),
    'write'
  );
  const hasWriteRoleAccess = missingWriteRoleAccess.length === 0;

  const roleDiffProps = useRoleWithAccessGraph();

  const fetchRoleQuery = useQuery({
    queryKey: ['fetch', 'role', 'foredit', editRoleName],
    queryFn: async () => {
      return await fetchRole(editRoleName);
    },
    gcTime: 0, // required to clear cache immediately after fetch
    enabled: !!editRoleName,
  });

  const updateRoleMutation = useMutation({
    mutationFn: async (role: Partial<RoleWithYaml>) => {
      await updateRole({ name: editRoleName, content: await toYaml(role) });
      setEditRoleName('');
    },
  });

  if (!hasWriteRoleAccess) {
    return (
      <>
        <H2 mb={3}>Manually Edit Member Access</H2>

        <Text>
          You do not have permission to edit roles. Missing role permissions:{' '}
          <code>{missingWriteRoleAccess.join(', ')}</code>
        </Text>
      </>
    );
  }

  return (
    <Box>
      <H2 mb={3}>Manually Edit Member Access</H2>
      {fetchRoleQuery.isError && (
        <Alert
          primaryAction={{
            content: 'Retry',
            onClick: () => fetchRoleQuery.refetch(),
          }}
        >
          {fetchRoleQuery.error.message}
        </Alert>
      )}
      <Table
        data={roles.map(role => ({ name: role }))}
        emptyText="No roles to edit"
        columns={[
          {
            key: 'name',
            headerText: 'Role Name',
          },
          {
            altKey: 'edit-btn',
            render: role => (
              <Cell align="right">
                <ButtonBorder
                  size="small"
                  onClick={() => setEditRoleName(role.name)}
                  disabled={editRoleName && fetchRoleQuery.isPending}
                >
                  Edit
                </ButtonBorder>
              </Cell>
            ),
          },
        ]}
      />
      <RoleEditorDialog
        open={editRoleName && fetchRoleQuery.isSuccess}
        onClose={() => {
          setEditRoleName('');
          roleDiffProps?.clearRoleDiffAttempt();
        }}
        resources={{
          create: () => null,
          edit: () => null,
          disregard: () => null,
          remove: () => null,
          setState: () => null,
          status: 'editing',
          item: fetchRoleQuery.data,
        }}
        onSave={updateRoleMutation.mutateAsync}
        roleDiffProps={roleDiffProps}
      />
    </Box>
  );
}
