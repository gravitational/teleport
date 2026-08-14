import { useState } from 'react';

import { Box, ButtonText, Flex, H2, Link } from 'design';
import { Trash } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';

import {
  AccessMonitoringRule,
  AccessMonitoringRuleType,
} from 'e-teleport/services/accessmonitoringrule/types';
import useTeleport from 'teleport/useTeleport';

import { DeleteRuleDialogue } from './DeleteRuleDialogue';

export const EditorHeader = ({
  rule = null,
  requiresEnrollingPlugins,
  onCancel,
  onDelete,
  editor,
}: {
  onDelete?(): void;
  rule?: AccessMonitoringRule;
  requiresEnrollingPlugins: boolean;
  onCancel(): void;
  editor: AccessMonitoringRuleType;
}) => {
  const ctx = useTeleport();
  const isCreating = !rule;
  const [deleteConfirm, setDeleteConfirm] = useState(false);

  const hasDeleteAccess = ctx.storeUser.getAccessMonitoringRuleAccess().remove;

  let header = rule?.metadata?.name || '';
  if (isCreating) {
    switch (editor) {
      case AccessMonitoringRuleType.Review:
        header = 'Create New Automatic Review Rule';
        break;
      case AccessMonitoringRuleType.Notification:
        header = 'Create New Notification Routing Rule';
        break;
    }
  }

  return (
    <Box>
      <Flex alignItems="center" mb={3} justifyContent="space-between" flex="1">
        <H2>{header}</H2>
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
                  : 'You do not have access to delete an access monitoring rule'
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
      {isCreating && getDescriptionHeader(editor)}
      {deleteConfirm && (
        <DeleteRuleDialogue
          name={rule.metadata.name}
          onClose={() => setDeleteConfirm(false)}
          onDelete={onDelete}
        />
      )}
    </Box>
  );
};

function getDescriptionHeader(editor: AccessMonitoringRuleType) {
  const docsURL =
    'https://goteleport.com/docs/reference/access-controls/access-monitoring-rules/';
  const docsRef = (
    <p>
      Refer to the{' '}
      <Link href={docsURL} target="_blank">
        Access Monitoring Rule documentation
      </Link>{' '}
      for reference.
    </p>
  );

  switch (editor) {
    case AccessMonitoringRuleType.Review:
      return (
        <>
          <p>
            With automatic review rules, access requests can be automatically
            reviewed based on the <b>match condition</b>.
          </p>
          {docsRef}
        </>
      );
    case AccessMonitoringRuleType.Notification:
      return (
        <>
          <p>
            With notification rules, access request notifications can be routed
            to an external integration based on the <b>match condition</b> and
            the <b>recipients</b>.
          </p>
          {docsRef}
        </>
      );
  }
}
