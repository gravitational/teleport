import { useState } from 'react';
import { Flex, ButtonText, H2 } from 'design';
import { HoverTooltip } from 'shared/components/ToolTip';
import { Trash } from 'design/Icon';
import useTeleport from 'teleport/useTeleport';

import { AccessMonitoringRule } from 'e-teleport/services/accessmonitoringrule/types';

import { DeleteRuleDialogue } from './DeleteRuleDialogue';

export const EditorHeader = ({
  rule = null,
  requiresEnrollingPlugins,
  onCancel,
  onDelete,
}: {
  onDelete?(r: AccessMonitoringRule): void;
  rule?: AccessMonitoringRule;
  requiresEnrollingPlugins: boolean;
  onCancel(): void;
}) => {
  const ctx = useTeleport();
  const isCreating = !rule;
  const [deleteConfirm, setDeleteConfirm] = useState(false);

  const hasDeleteAccess = ctx.storeUser.getAccessMonitoringRuleAccess().remove;

  return (
    <>
      <Flex alignItems="center" mb={3} justifyContent="space-between" flex="1">
        <H2>
          {isCreating ? 'Create a New Notification Rule' : rule?.metadata?.name}
        </H2>
        {requiresEnrollingPlugins && (
          <ButtonText onClick={onCancel}>Cancel</ButtonText>
        )}
        {!isCreating && (
          <Flex>
            <HoverTooltip
              position="bottom"
              tipContent={
                hasDeleteAccess
                  ? 'Delete'
                  : 'You do not have access to delete a notification rule'
              }
            >
              <ButtonText
                onClick={() => setDeleteConfirm(true)}
                disabled={!hasDeleteAccess}
                data-testid="delete"
                p={1}
              >
                <Trash size="medium" />
              </ButtonText>
            </HoverTooltip>
          </Flex>
        )}
      </Flex>
      {deleteConfirm && (
        <DeleteRuleDialogue
          name={rule.metadata.name}
          onClose={() => setDeleteConfirm(false)}
          onDelete={() => onDelete(rule)}
        />
      )}
    </>
  );
};
