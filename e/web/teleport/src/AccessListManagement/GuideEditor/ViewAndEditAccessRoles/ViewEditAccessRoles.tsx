import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';

import { Alert, Box, Indicator, Text } from 'design';
import { Info } from 'design/Alert';

import { AccessListModified } from 'e-teleport/AccessListManagement/ViewEditAccessList/Shared';
import { AccessList } from 'e-teleport/services/accessmanagement';
import {
  accessManagementService,
  makeAccessListForUpdate,
  makeAccessListMembersForUpdate,
} from 'e-teleport/services/accessmanagement/accessmanagement';
import { AccessListWithPresetRequest } from 'e-teleport/services/accessmanagement/preset';
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
import { ManuallyEditAccess } from './ManuallyEditAccess';
import { PresetDescription } from './PresetDescription';
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
    const baseReq = {
      req: {},
      original: accessList,
    };

    const spec = makeAccessListForUpdate({
      ...baseReq,
      withoutMembers: true,
    });

    const members = makeAccessListMembersForUpdate(baseReq);

    const req: AccessListWithPresetRequest = {
      presetType: preset,
      accessList: {
        metadata: { ...accessList.metadata },
        members,
        spec,
      },
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
      <>
        <PresetDescription preset={preset} />
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      </>
    );
  }

  if (!hasReadRoleAccess) {
    return (
      <>
        <PresetDescription preset={preset} />
        <Info>
          You do not have permission to read resource access. Missing role
          permissions: <code>{missingReadRoleAccess.join(', ')}</code>
        </Info>
      </>
    );
  }

  if (fetchRoles.isError) {
    return (
      <>
        <PresetDescription preset={preset} />
        <Alert
          primaryAction={{
            content: 'Retry',
            onClick: () => fetchRoles.refetch(),
          }}
        >
          {fetchRoles.error.message}
        </Alert>
      </>
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
      <>
        <PresetDescription preset={preset} />
        <Alert
          kind="neutral"
          primaryAction={{
            content: 'Define Resource Access',
            onClick: () => setShowGuideEditor(true),
          }}
        >
          No resource access are defined.
        </Alert>
      </>
    );
  }

  if (
    roleState?.status === 'unknown-roles' ||
    roleState?.status === 'unsupported-role-fields'
  ) {
    return (
      <>
        <Alert
          kind="warning"
          details={
            <Text>
              The member grants or its roles may have been manually edited.
            </Text>
          }
        >
          The member roles assigned to this access list cannot be parsed.
        </Alert>
        <ManuallyEditAccess roles={accessList.grants.roles} />
      </>
    );
  }

  if (roleState?.status === 'valid-roles') {
    return (
      <ReadRoleAccess onShowGuideEditor={() => setShowGuideEditor(true)} />
    );
  }
}
