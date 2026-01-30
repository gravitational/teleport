import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';

import { Alert, Box, Indicator } from 'design';
import { Info } from 'design/Alert';

import { AccessListModified } from 'e-teleport/AccessListManagement/ViewEditAccessList/Shared';
import { AccessList } from 'e-teleport/services/accessmanagement';
import {
  accessManagementService,
  makeAccessListForUpdate,
} from 'e-teleport/services/accessmanagement/accessmanagement';
import { AccessListWithPresetRequest } from 'e-teleport/services/accessmanagement/preset';
import cfg from 'teleport/config';
import { Role } from 'teleport/services/resources';
import ResourceService from 'teleport/services/resources/resource';
import useTeleport from 'teleport/useTeleport';

import { useAccessListManagementContext } from '../../AccessListManagementContext';
import {
  getMissingRoleAccess,
  getRoleSuffix,
  QueriedRoleState,
  validateQueriedRoles,
} from '../Preset/role/role';
import { AccessRoleEditor } from './AccessRoleEditor';
import { ReadRoleAccess } from './ReadAccessSection';

/**
 * Displays and allows editing of access roles made for an access list.
 * Users can open the guide editor to modify access roles.
 */
export function ViewEditAccessRoles({
  accessList,
  updateAccessListCache,
}: {
  accessList: AccessListModified;
  updateAccessListCache(accessList: AccessList): void;
}) {
  const ctx = useTeleport();
  const roleAccess = ctx.storeUser.getRoleAccess();

  const missingReadRoleAccess = getMissingRoleAccess(roleAccess, 'read');
  const hasReadRoleAccess = missingReadRoleAccess.length === 0;

  const [roleState, setRoleState] = useState<QueriedRoleState>();
  const [showGuideEditor, setShowGuideEditor] = useState(false);

  const { guideEditor } = useAccessListManagementContext();
  const { setPreset, standardRoleState, awsIcRoleState, preset } = guideEditor;

  const fetchRoles = useQuery({
    queryKey: ['fetch', 'roles', 'withpreset'],
    queryFn: async () => {
      const resourceSvc = new ResourceService();
      const gotResponse = await resourceSvc.fetchRolesV2({
        search: getRoleSuffix(accessList.id),
        includeObject: 'yes',
      });

      const roleState = validateQueriedRoles({
        gotRoles: gotResponse.items.map(item => item.object),
        accessList,
      });

      setRoleState(roleState);
      setPreset(accessList.preset);

      awsIcRoleState.initRoleEditState(roleState.awsIcRole);
      standardRoleState.initRoleEditState(roleState.standardRole);

      return gotResponse;
    },
    gcTime: 0, // required to clear cache immediately after fetch
    enabled: hasReadRoleAccess,
  });

  async function handleOnUpdate(accessRoles: Role[]) {
    const accessListReq = makeAccessListForUpdate({
      req: {},
      original: accessList,
    });

    const req: AccessListWithPresetRequest = {
      presetType: preset,
      accessList: {
        spec: { ...accessListReq },
        metadata: accessList.metadata,
      },
      members: accessListReq.members.map(member => ({
        spec: member,
        metadata: { name: member.name },
      })),
      accessRoles,
    };

    return accessManagementService
      .updateAccessListWithPreset(req)
      .then(updatedList => {
        updateAccessListCache(updatedList.accessList);
        return updatedList.rolesToBeDeleted.map(name => ({
          name,
        }));
      });
  }

  async function onCloseEditor() {
    setShowGuideEditor(false);
    fetchRoles.refetch();
  }

  if (fetchRoles.isFetching) {
    return (
      <Box textAlign="center" m={10}>
        <Indicator />
      </Box>
    );
  }

  if (!hasReadRoleAccess) {
    return (
      <Info>
        You do not have permission to read resource access. Missing role
        permissions: <code>{missingReadRoleAccess.join(', ')}</code>
      </Info>
    );
  }

  if (fetchRoles.isError) {
    return (
      <Alert
        primaryAction={{
          content: 'Retry',
          onClick: () => fetchRoles.refetch(),
        }}
      >
        {fetchRoles.error.message}
      </Alert>
    );
  }

  if (showGuideEditor) {
    return (
      <AccessRoleEditor
        onClose={onCloseEditor}
        onUpdateAccess={handleOnUpdate}
      />
    );
  }

  if (roleState?.status == 'no-access-defined') {
    return (
      <Alert
        kind="neutral"
        primaryAction={{
          content: 'Define Resource Access',
          onClick: () => setShowGuideEditor(true),
        }}
      >
        No resource access are defined.
      </Alert>
    );
  }

  if (
    roleState?.status === 'unknown-roles' ||
    roleState?.status === 'unsupported-role-fields'
  ) {
    return (
      <Alert
        primaryAction={{
          content: 'Redefine Access',
          onClick: () => setShowGuideEditor(true),
        }}
        secondaryAction={{
          content: 'Manually Edit Roles',
          linkTo: `${cfg.routes.roles}?search=${accessList.id}`,
        }}
      >
        The roles assigned to this access list cannot be parsed. The roles or
        member grants may have been manually edited. You can redefine access
        that will reset member and owners grants or use the role editor to
        manually edit access.
      </Alert>
    );
  }

  if (roleState?.status === 'valid-roles') {
    return (
      <ReadRoleAccess onShowGuideEditor={() => setShowGuideEditor(true)} />
    );
  }
}
