import { useCallback, useEffect } from 'react';

import { Box, Button, Flex, Text } from 'design';
import { HoverTooltip } from 'design/Tooltip';
import { debounce } from 'shared/utils/highbar';

import { useRoleWithAccessGraph } from 'e-teleport/Roles/useRoleWithAccessGraph';
import cfg from 'teleport/config';
import { RoleEditorVisualizer } from 'teleport/Roles/RoleEditor/RoleEditorVisualizer';
import { RoleDiffState } from 'teleport/Roles/Roles';
import { storageService } from 'teleport/services/storageService';
import useTeleport from 'teleport/useTeleport';

import { useAccessListManagementContext } from '../../AccessListManagementContext';
import { getMissingRoleAccess } from '../Preset/role/role';
import { PresetDescription } from './PresetDescription';
import { ReadRoleAccessTabsAndSection } from './ReadResourceAccessTabsAndSection/ReadRoleAccessTabsAndSection';

export function ReadRoleAccess({
  onShowGuideEditor,
}: {
  onShowGuideEditor(): void;
}) {
  const ctx = useTeleport();
  const missingWriteRoleAccess = getMissingRoleAccess(
    ctx.storeUser.getRoleAccess(),
    'write'
  );
  const hasWriteRoleAccess = missingWriteRoleAccess.length === 0;

  const { guideEditor } = useAccessListManagementContext();
  const { awsIcRoleState, standardRoleState, preset } = guideEditor;

  // Access graph does not support processing more than one role.
  // Prioritize standard role over others.
  const roleForAccessGraph =
    standardRoleState.roleEditState.original ??
    awsIcRoleState.roleEditState.original;

  const roleDiffProps = useRoleWithAccessGraph();
  const demoMode = roleDiffProps?.roleDiffState === RoleDiffState.DemoReady;
  const roleTesterEnabled =
    (cfg.entitlements.AccessGraph.enabled &&
      storageService.getAccessGraphRoleTesterEnabled()) ||
    demoMode;

  const onRoleUpdateForAccessGraph = useCallback(
    debounce(role => roleDiffProps?.updateRoleDiff(role), 500),
    []
  );

  useEffect(() => {
    if (roleTesterEnabled) {
      onRoleUpdateForAccessGraph?.(roleForAccessGraph);
    }
  }, [roleTesterEnabled, onRoleUpdateForAccessGraph, demoMode]);

  return (
    <Box>
      <PresetDescription preset={preset} />
      <Flex height="60vh" gap={3}>
        <ReadRoleAccessTabsAndSection />
        <Flex
          position="relative"
          flex="1"
          height="60vh"
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
      <Flex mt={2} mb={6}>
        <HoverTooltip
          tipContent={
            !hasWriteRoleAccess ? (
              <Text>
                You do not have permission to edit this access. Missing role
                permissions: <code>{missingWriteRoleAccess.join(', ')}</code>
              </Text>
            ) : undefined
          }
        >
          <Button
            size="extra-large"
            intent="neutral"
            fill="border"
            width="380px"
            onClick={onShowGuideEditor}
            disabled={!hasWriteRoleAccess}
          >
            Edit Access
          </Button>
        </HoverTooltip>
      </Flex>
    </Box>
  );
}
