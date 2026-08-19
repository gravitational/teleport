import { formatDistanceStrict } from 'date-fns';
import styled from 'styled-components';

import { Flex, Text } from 'design';
import { IconTooltip } from 'design/Tooltip';

import { PluginOktaSyncStatusCode } from 'teleport/services/integrations/oktaStatusTypes';

export function getDurationText(date: Date) {
  if (!date || date.getTime() <= 0) {
    return 'not recorded yet';
  }

  return formatDistanceStrict(new Date(date), new Date(), { addSuffix: true });
}

export const Panel = styled(Flex).attrs({
  p: 4,
  borderRadius: 3,
  flexDirection: 'column',
  gap: 3,
})<{ withBorder?: boolean }>`
  flex-basis: 100%;
  min-width: 0;
  background-color: ${props => props.theme.colors.levels.surface};
  border: ${p =>
    p.withBorder ? `1px solid ${p.theme.colors.spotBackground[2]}` : 'none'};
  box-shadow: ${p => p.theme.boxShadow[0]};
`;

export const PanelTitle = styled(Text)`
  font-size: ${p => p.theme.fontSizes[4]}px;
  font-weight: 500;
`;

export const FlexWrap = styled(Flex)`
  @media screen and (max-width: ${p => p.theme.breakpoints.tablet}) {
    flex-wrap: wrap;
  }
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
      <IconTooltip kind="error">
        <Text>
          <b>Last Failed:</b> {getDurationText(lastFailed)}
        </Text>
        {/* pre-line required to respect string containing \n\t chars */}
        <Text css={{ whiteSpace: 'pre-line' }}>{error}</Text>
      </IconTooltip>
    );
  }
  return null;
}
