import { useMutation } from '@tanstack/react-query';
import { useState } from 'react';

import { Alert, ButtonBorder, ButtonSecondary, Text } from 'design';
import { Warning } from 'design/Alert/Alert';
import Table, { Cell } from 'design/DataTable';
import { DialogContent, DialogFooter } from 'design/DialogConfirmation';

import ResourceService from 'teleport/services/resources';

import { TableRole } from './types';

export function DeletePresetRoles({
  onComplete,
  roles,
  statusText,
}: {
  onComplete(): void;
  roles: TableRole[];
  statusText: string;
}) {
  const [rolesToDelete, setRolesToDelete] = useState(roles);

  const deleteRoleMutation = useMutation({
    mutationFn: async (roleName: string) => {
      const resourceSvc = new ResourceService();
      await resourceSvc.deleteRole(roleName);
      setRolesToDelete(prev => prev.filter(role => role.name !== roleName));
    },
  });

  return (
    <>
      <DialogContent width="580px" mb={4}>
        {deleteRoleMutation.isError && (
          <Alert>{deleteRoleMutation.error.message}</Alert>
        )}
        <Text>{statusText}</Text>

        <Warning mt={3}>
          Before deleting a role, ensure that it is not assigned to a user or
          assigned to another role.
        </Warning>

        <Table
          data={rolesToDelete}
          emptyText="All roles deleted"
          columns={[
            {
              key: 'name',
              headerText: 'Name',
            },
            {
              altKey: 'remove-btn',
              render: role => (
                <Cell align="right">
                  <ButtonBorder
                    size="small"
                    onClick={() => deleteRoleMutation.mutate(role.name)}
                    disabled={deleteRoleMutation.isPending}
                  >
                    Remove
                  </ButtonBorder>
                </Cell>
              ),
            },
          ]}
        />
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary
          onClick={onComplete}
          disabled={deleteRoleMutation.isPending}
        >
          Close
        </ButtonSecondary>
      </DialogFooter>
    </>
  );
}
