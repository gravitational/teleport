import { useCallback, useEffect, useMemo } from 'react';

import { Box, Flex, Text } from 'design';
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

import { IdentityTabsAndSection } from './IdentityTabsAndSection';
import { IdentityStepButtons, IdentityTabContainer } from './Shared';

export function IdentityView() {
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

  const requiresIdentities = !standardRoleState.canSkipDefiningIdentities();

  return (
    <Flex height="75vh" gap={3}>
      {requiresIdentities ? (
        <IdentityTabsAndSection role={role} dispatchRole={dispatchRole} />
      ) : (
        <IdentityTabContainer>
          <Box p={2} mt={1}>
            <Text mb={3}>No identities are required.</Text>
            {roleDiffProps && shouldShowRoleDiff(roleDiffProps) && (
              <Text>
                The access graph on the right shows how members access to
                resources can look like. To make any changes, go back.
              </Text>
            )}
          </Box>
          <IdentityStepButtons />
        </IdentityTabContainer>
      )}
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
