import { useMutation } from '@tanstack/react-query';
import { useEffect, useState } from 'react';

import {
  Alert,
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  Indicator,
  Text,
} from 'design';
import Dialog, {
  DialogContent,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import { Check } from 'design/Icon';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { DeletePresetRoles } from 'e-teleport/AccessListManagement/ViewEditAccessList/DeleteAccessList/DeletePresetRoles';

import { AccessRoleEditor } from '../../../ViewAndEditAccessRoles/types';

export function UpdateAccessRolesDialog({
  onCancelUpdate,
  accessRoleEditor,
}: {
  onCancelUpdate(): void;
  accessRoleEditor: AccessRoleEditor;
}) {
  const { guideEditor } = useAccessListManagementContext();

  const [rolesToBeDeleted, setRolesToBeDeleted] = useState([]);

  const updateAccessMutation = useMutation({
    mutationFn: async () => {
      const rolesToDelete = await accessRoleEditor.onUpdateAccess(
        guideEditor.getRolesToSave()
      );
      setRolesToBeDeleted(rolesToDelete);
      return rolesToDelete;
    },
  });

  useEffect(() => {
    updateAccessMutation.mutate();
  }, []);

  function onFinishedUpdate() {
    guideEditor.reset();
    accessRoleEditor.onClose();
  }

  let content: React.ReactNode;
  if (rolesToBeDeleted.length > 0) {
    content = (
      <DeletePresetRoles
        roles={rolesToBeDeleted}
        statusText="Successfully saved changes. Optionally delete the roles below that are no longer being used by this access list."
        onComplete={onFinishedUpdate}
      />
    );
  } else {
    content = (
      <>
        <DialogContent width={'400px'}>
          {updateAccessMutation.isError && (
            <Alert kind="danger">{updateAccessMutation.error.message}</Alert>
          )}
          {updateAccessMutation.isSuccess && (
            <Flex gap={2}>
              <Check size="medium" color="interactive.solid.success.default" />
              <Text>Successfully saved changes.</Text>
            </Flex>
          )}
          {updateAccessMutation.isPending && (
            <Box textAlign="center">
              <Indicator />
            </Box>
          )}
        </DialogContent>
        <Flex gap={3}>
          <ButtonPrimary
            onClick={
              updateAccessMutation.isError
                ? () => updateAccessMutation.mutate()
                : () => onFinishedUpdate()
            }
            width="100%"
            disabled={updateAccessMutation.isPending}
          >
            {updateAccessMutation.isError ? 'Retry' : 'Done'}
          </ButtonPrimary>
          {!updateAccessMutation.isSuccess && (
            <ButtonSecondary
              onClick={onCancelUpdate}
              width="100%"
              disabled={updateAccessMutation.isPending}
            >
              Cancel
            </ButtonSecondary>
          )}
        </Flex>
      </>
    );
  }

  return (
    <Dialog disableEscapeKeyDown={false} open={true}>
      <DialogHeader>
        <DialogTitle>Updating Access List</DialogTitle>
      </DialogHeader>
      {content}
    </Dialog>
  );
}
