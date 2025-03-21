import styled from 'styled-components';

import { Box, ButtonPrimary, ButtonSecondary, Flex, P3 } from 'design';
import { HoverTooltip } from 'design/Tooltip';

import useTeleport from 'teleport/useTeleport';

export const Sidebar = styled(Box)`
  height: 100%;
  width: 50%;
  min-width: 500px;
  overflow: auto;
  position: relative;
  border-left: 1px solid ${p => p.theme.colors.spotBackground[0]};
`;

export const EditorSaveCancelButton = ({
  onSave,
  onCancel,
  disabled,
  isEditing,
}: {
  onSave(): void;
  onCancel(): void;
  disabled: boolean;
  isEditing?: boolean;
}) => {
  const ctx = useTeleport();
  const amRuleAccess = ctx.storeUser.getAccessMonitoringRuleAccess();
  const pluginAccess = ctx.storeUser.getPluginsAccess();
  const hasUpdateAccess = pluginAccess.read && amRuleAccess.edit;

  let hoverTooltipContent;
  if (isEditing) {
    hoverTooltipContent =
      !hasUpdateAccess &&
      'You do not have access to update access monitoring rules';
  }

  return (
    <Flex gap={2}>
      <Box width="50%">
        <HoverTooltip tipContent={hoverTooltipContent}>
          <ButtonPrimary
            width="100%"
            size="large"
            onClick={onSave}
            disabled={disabled || (isEditing && !hasUpdateAccess)}
          >
            {isEditing ? 'Update' : 'Create'} Rule
          </ButtonPrimary>
        </HoverTooltip>
      </Box>
      <ButtonSecondary width="50%" onClick={onCancel}>
        Cancel
      </ButtonSecondary>
    </Flex>
  );
};

export const EditorWrapper = styled(Box)<{ mute?: boolean }>`
  opacity: ${p => (p.mute ? 0.4 : 1)};
  pointer-events: ${p => (p.mute ? 'none' : '')};
`;

export function getDefaultPluginNotificationMessage(pluginName = '') {
  return (
    <P3 mb={2}>
      Note: Fallback notification rule {pluginName ? `(${pluginName}) ` : ''}
      will be used if an access request does not match the condition.
    </P3>
  );
}
