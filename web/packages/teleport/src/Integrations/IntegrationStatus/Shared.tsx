import React from 'react';
import styled from 'styled-components';
import { Link as InternalLink } from 'react-router-dom';
import { Box, Flex, Text, Label } from 'design';
import { CircleCheck, SyncAlt } from 'design/Icon';
import { ToolTipInfo } from 'shared/components/ToolTip';
import { getDurationText } from 'shared/utils/getDurationText';

import { PluginOktaSyncStatusCode } from 'teleport/services/integrations/oktaStatusTypes';

export type AwsResourceKind = 'ec2' | 'rds' | 'eks';

export const Panel = styled(Box).attrs({
  p: 3,
  borderRadius: 3,
})<{ withBorder?: boolean; width?: string }>`
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  width: ${p => (p.width ? p.width : '400px')};
  background-color: ${p => p.theme.colors.interactive.tonal.neutral[0]};
  border: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[0]};
  text-decoration: none;
  color: ${({ theme }) => theme.colors.text.main};

  &:hover {
    border: 1px solid ${p => p.theme.colors.interactive.tonal.primary[2]};
    background-color: ${p => p.theme.colors.interactive.tonal.primary[0]};
    cursor: pointer;
  }
`;

export const PanelTitle = styled(Text)`
  font-size: ${p => p.theme.fontSizes[4]}px;
`;

export const InnerCard = styled(Box).attrs({
  p: 4,
  borderRadius: 3,
})`
  width: 100%;
  border: 1px solid ${p => p.theme.colors.spotBackground[0]};
`;

export const TextWithBorderBottom = styled(Text)`
  padding-bottom: ${p => p.theme.space[2]}px;
  border-bottom: 1px solid ${p => p.theme.colors.spotBackground[0]};
`;

function getEnableLabelKind(enabled: boolean) {
  return enabled ? 'success' : 'secondary';
}

export function CustomLabel({ enabled }: { enabled: boolean }) {
  if (!enabled) {
    return <Label kind="secondary">disabled</Label>;
  }

  return (
    <Label
      kind={getEnableLabelKind(enabled)}
      css={`
        background: ${p => p.theme.colors.interactive.tonal.success[1]}
        color: ${p => p.theme.colors.success.hover};
      `}
    >
      <Flex alignItems="center">
        <CircleCheck size="small" mr={1} />
        enabled
      </Flex>
    </Label>
  );
}

export const CenteredSpaceBetweenFlex = styled(Flex)`
  align-items: center;
  justify-content: space-between;
  gap: ${p => p.theme.space[2]}px;
`;

export const GappedColumnFlex = styled(Flex)`
  flex-direction: column;
  gap: ${p => p.theme.space[3]}px;
`;

export const PanelHeader = styled(Flex)`
  align-items: center;
  gap: ${p => p.theme.space[1]}px;
  margin-bottom: ${p => p.theme.space[4]}px;
`;

export function PanelLastSynced() {
  return (
    <Flex alignItems="center" mt={4}>
      <SyncAlt color="text.muted" size="small" mr={1} />
      <Text color="text.muted" fontSize={1}>
        Last Synced: test minutes ago
        {/* Last Synced: {getDurationText(spec.lastSuccess)}{' '} */}
        {/* <ErrorTooltip
      statusCode={spec.statusCode}
      lastFailed={spec.lastFailed}
      error={spec.error}
    /> */}
      </Text>
    </Flex>
  );
}
