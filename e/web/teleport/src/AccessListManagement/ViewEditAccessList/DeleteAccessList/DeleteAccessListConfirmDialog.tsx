import { useQuery } from '@tanstack/react-query';
import { useHistory } from 'react-router';

import { Alert, Box, ButtonPrimary, ButtonSecondary, Indicator } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';
import useAttempt from 'shared/hooks/useAttemptNext';

import { isGuideEditorSupported } from 'e-teleport/AccessListManagement/GuideEditor/useGuideEditor';
import cfg from 'e-teleport/config';
import { accessManagementService } from 'e-teleport/services/accessmanagement';
import ResourceService from 'teleport/services/resources';

import { useAccessListManagementContext } from '../../AccessListManagementContext';
import {
  getRoleSuffix,
  hasInternalAccessListPresetLabel,
} from '../../GuideEditor/Preset/role/role';
import { AccessListModified } from '../Shared';
import { DeleteAccessList } from './DeleteAccessList';
import { DeletePresetRoles } from './DeletePresetRoles';

export function DeleteAccessListConfirmDialog({
  onClose,
  accessList,
}: {
  onClose(): void;
  accessList: AccessListModified;
}) {
  const accessListId = accessList.id;
  const history = useHistory();
  const { updateAccessListCache } = useAccessListManagementContext();

  const { attempt: deleteAccessListAttempt, setAttempt } = useAttempt();

  const roleFetchEnabled = isGuideEditorSupported(accessList.preset);

  // Allow fetching roles first, so it can fail first before
  // the call to delete access list.
  const fetchRoles = useQuery({
    queryKey: ['fetch', 'roles', 'withpreset'],
    queryFn: async () => {
      const resourceSvc = new ResourceService();
      const response = await resourceSvc.fetchRolesV2({
        search: getRoleSuffix(accessList.id),
        includeObject: 'yes',
      });
      return response.items
        .filter(item =>
          hasInternalAccessListPresetLabel(
            accessList.id,
            item.object.metadata.labels
          )
        )
        .map(item => ({ name: item.name }));
    },
    gcTime: 0, // no cache
    enabled: roleFetchEnabled,
  });

  function deleteCompleted() {
    // Because of backend caching, we send the deleted ID
    // as router state to be used to update the listing.
    updateAccessListCache({ mutationType: 'deleted', accessListId });
    history.replace(cfg.getAccessListManagementRoute());
  }

  async function onDelete() {
    setAttempt({ status: 'processing' });
    accessManagementService
      .deleteAccessList(accessListId)
      .then(() => {
        if (roleFetchEnabled && fetchRoles.data.length > 0) {
          // Has roles to delete.
          setAttempt({ status: 'success' });
        } else {
          // We don't need to setAttempt to "success"
          // since we are unmounting.
          deleteCompleted();
        }
      })
      .catch((e: Error) =>
        setAttempt({ status: 'failed', statusText: e.message })
      );
  }

  const fetchedRoleSuccess = roleFetchEnabled && fetchRoles.isSuccess;

  const showRolesTable =
    fetchedRoleSuccess &&
    deleteAccessListAttempt.status === 'success' &&
    fetchRoles.data?.length > 0;

  const showDeleteContent = !showRolesTable;

  let content: React.ReactNode;
  if (fetchRoles.isFetching) {
    content = (
      <Box textAlign="center" width={'450px'}>
        <Indicator />
      </Box>
    );
  } else if (fetchRoles.isError) {
    content = (
      <>
        <DialogContent width={'450px'}>
          <Alert>{fetchRoles.error.message}</Alert>
        </DialogContent>
        <DialogFooter>
          <ButtonPrimary mr="3" onClick={() => fetchRoles.refetch()}>
            Retry
          </ButtonPrimary>
          <ButtonSecondary onClick={onClose}>Cancel</ButtonSecondary>
        </DialogFooter>
      </>
    );
  } else if (showRolesTable) {
    content = (
      <DeletePresetRoles
        onComplete={deleteCompleted}
        roles={fetchRoles.data}
        statusText="Successfully deleted access list. Optionally delete the roles below that were created for
          this access list."
      />
    );
  } else if (showDeleteContent) {
    content = (
      <DeleteAccessList
        onCancel={onClose}
        onDelete={onDelete}
        attempt={deleteAccessListAttempt}
        accessList={accessList}
        fetchRolesError={
          fetchRoles.isError ? fetchRoles.error.message : undefined
        }
      />
    );
  }

  return (
    <Dialog onClose={onClose} open={true}>
      <DialogHeader>
        <DialogTitle>Delete Access List?</DialogTitle>
      </DialogHeader>
      {content}
    </Dialog>
  );
}
