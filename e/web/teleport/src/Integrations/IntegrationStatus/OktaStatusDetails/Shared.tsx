import styled from 'styled-components';
import { Box, Flex, Text, Label } from 'design';
import { CircleCheck } from 'design/Icon';
import { ToolTipInfo } from 'shared/components/ToolTip';
import { PluginOktaSyncStatusCode } from 'teleport/services/integrations/oktaStatusTypes';

import { getDurationText } from './date';

export const Panel = styled(Box).attrs({
  p: 4,
  borderRadius: 3,
})<{ withBorder?: boolean }>`
  flex-basis: 100%;
  min-width: 0;
  display: flex;
  flex-direction: column;
  background-color: ${props => props.theme.colors.levels.elevated};
  border: ${p =>
    p.withBorder ? `1px solid ${p.theme.colors.spotBackground[2]}` : 'none'};
`;

export const PanelTitle = styled(Text)`
  font-size: ${p => p.theme.fontSizes[4]}px;
`;

export const InnerCard = styled(Box).attrs({
  p: 4,
  borderRadius: 3,
})`
  flex-basis: 100%;
  border: 1px solid ${p => p.theme.colors.spotBackground[0]};
`;

export const LinkedInnerCard = styled(InnerCard)`
  color: inherit;
  text-decoration: none;
  &:hover {
    cursor: pointer;
    border: 1px solid ${p => p.theme.colors.spotBackground[2]};
  }
`;

export const FlexWrap = styled(Flex)`
  @media screen and (max-width: 915px) {
    flex-wrap: wrap;
  }
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
        background: ${p => p.theme.colors.interactive.tonal.success[1]};
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

export const CenteredFlex = styled(Flex)`
  align-items: center;
  justify-content: space-between;
  gap: ${p => p.theme.space[2]}px;
  margin-bottom: ${p => p.theme.space[2]}px;
`;

export function ErrorTooltip({
  statusCode,
  lastFailed,
  error,
}: {
  statusCode: PluginOktaSyncStatusCode;
  lastFailed: Date;
  error: string;
}) {
  if (statusCode === PluginOktaSyncStatusCode.Error) {
    return (
      <ToolTipInfo kind="error">
        <Text>
          <b>Last Failed:</b> {getDurationText(lastFailed)}
        </Text>
        {/* pre-line required to respect string containing \n\t chars */}
        <Text css={{ whiteSpace: 'pre-line' }}>{error}</Text>
      </ToolTipInfo>
    );
  }
  return null;
}
