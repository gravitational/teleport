import { Link } from 'react-router';
import styled, { useTheme } from 'styled-components';

import { Button } from 'design/Button';
import { Cog } from 'design/Icon';
import Text from 'design/Text';

import cfg from 'e-teleport/config';
import useStickyClusterId from 'teleport/useStickyClusterId';
import useTeleport from 'teleport/useTeleport';

const StatusCircle = styled.div<{ statusColor: string }>`
  background-color: ${p => p.statusColor};
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
`;

const Status = styled.div`
  display: flex;
  align-items: center;
  gap: ${p => p.theme.space[1]}px;
  background: ${p => p.theme.colors.interactive.tonal.neutral[0]};
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[2]}px;
  border-radius: ${p => p.theme.radii[2]}px;
  margin-left: ${p => p.theme.space[2]}px;
  line-height: 1;
`;

const StyledButtonSecondary = styled(Button)`
  background: ${p => p.theme.colors.interactive.tonal.neutral[0]}; // when using as={Link}, a lot of the styles are overridden for some reason
  font-weight: 500;
  font-size: 12px;
  height: 35px;
  box-sizing: border-box;
  padding: ${p => p.theme.space[1]}px ${p => p.theme.space[1]}px
    ${p => p.theme.space[1]}px ${p => p.theme.space[2]}px;
  gap: ${p => p.theme.space[1]}px;
  border-radius: ${p => p.theme.radii[3]}px;
  text-decoration: none;
  color: ${p => p.theme.colors.text.main};
  display: flex;
  align-items: center;

  &:hover {
    background: ${p => p.theme.colors.interactive.tonal.neutral[1]};
  }
`;

export function SessionSummariesStatus() {
  const theme = useTheme();
  const { clusterId } = useStickyClusterId();
  const ctx = useTeleport();
  const flags = ctx.getFeatureFlags();

  if (!flags.sessionSummaries) {
    return null;
  }

  const status = cfg.oss.sessionSummarizerEnabled ? 'Enabled' : 'Not Enabled';

  const statusColor = cfg.oss.sessionSummarizerEnabled
    ? theme.colors.interactive.solid.success.default
    : theme.colors.interactive.solid.danger.default;

  const link = cfg.getSessionSummariesManagementRoute(clusterId);

  return (
    <StyledButtonSecondary
      data-testid="session-summaries-configure"
      as={Link}
      intent="neutral"
      fill="filled"
      to={link}
    >
      <Cog size="small" />
      Configure Session Summaries
      <Status>
        <StatusCircle statusColor={statusColor} />
        <Text>{status}</Text>
      </Status>
    </StyledButtonSecondary>
  );
}
