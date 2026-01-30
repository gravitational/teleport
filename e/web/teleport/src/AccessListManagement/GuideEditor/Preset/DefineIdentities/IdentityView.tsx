import { useCallback, useEffect, useMemo } from 'react';

import { Flex } from 'design';
import { debounce } from 'shared/utils/highbar';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { useRoleWithAccessGraph } from 'e-teleport/Roles/useRoleWithAccessGraph';
import cfg from 'teleport/config';
import {
  RoleEditorVisualizer,
  shouldShowRoleDiff,
} from 'teleport/Roles/RoleEditor/RoleEditorVisualizer';
import {
  defaultRoleVersion,
  roleEditorModelToRole,
} from 'teleport/Roles/RoleEditor/StandardEditor/standardmodel';
import { useStandardModel } from 'teleport/Roles/RoleEditor/StandardEditor/useStandardModel';
import { withDefaults } from 'teleport/Roles/RoleEditor/StandardEditor/withDefaults';
import { RoleDiffState } from 'teleport/Roles/Roles';
import { Role } from 'teleport/services/resources';
import { storageService } from 'teleport/services/storageService';

import { AccessRoleEditor } from '../../ViewAndEditAccessRoles/types';
import { IdentityTabsAndSection } from './IdentityTabsAndSection';

export function IdentityView({
  accessRoleEditor,
}: {
  accessRoleEditor?: AccessRoleEditor;
}) {
  const { guideEditor } = useAccessListManagementContext();
  const { standardRoleState } = guideEditor;

  const initRoleModel = useMemo(() => initRoleModelForAccessGraph(), []);
  const [role, dispatchRole] = useStandardModel(initRoleModel);

  const roleDiffProps = useRoleWithAccessGraph();
  const demoMode = roleDiffProps?.roleDiffState === RoleDiffState.DemoReady;
  const roleTesterEnabled =
    (cfg.isPolicyEnabled && storageService.getAccessGraphRoleTesterEnabled()) ||
    demoMode;

  const onRoleUpdateForAccessGraph = useCallback(
    debounce(role => roleDiffProps?.updateRoleDiff(role), 500),
    []
  );

  useEffect(() => {
    const { roleModel, validationResult } = role;

    if (roleTesterEnabled && roleModel && validationResult?.isValid) {
      onRoleUpdateForAccessGraph?.(roleEditorModelToRole(roleModel));
    }
  }, [role, roleTesterEnabled, onRoleUpdateForAccessGraph, demoMode]);

  function initRoleModelForAccessGraph() {
    const defaultRole = withDefaults({
      metadata: { name: 'draft-role' },
      version: defaultRoleVersion,
    });
    const newRoleModel: Role = { ...defaultRole };
    newRoleModel.spec.allow = { ...standardRoleState.roleConditions };

    return newRoleModel;
  }

  return (
    <Flex height="75vh" gap={3}>
      <IdentityTabsAndSection
        role={role}
        dispatchRole={dispatchRole}
        accessRoleEditor={accessRoleEditor}
        hasAccessGraphEnabled={
          roleDiffProps && shouldShowRoleDiff(roleDiffProps)
        }
      />
      <Flex
        position="relative"
        flex="1"
        height="75vh"
        css={`
          overflow: scroll;
        `}
      >
        <RoleEditorVisualizer
          roleDiffProps={roleDiffProps}
          currentFlow={'creating'}
        />
      </Flex>
    </Flex>
  );
}
