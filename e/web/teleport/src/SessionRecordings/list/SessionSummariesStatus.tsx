import { useCallback, type MouseEvent } from 'react';
import styled, { useTheme } from 'styled-components';

import { ChatCircleSparkle, Cross } from 'design/Icon';
import Text from 'design/Text';
import { useLocalStorage } from 'shared/hooks/useLocalStorage';

import cfg from 'e-teleport/config';
import { KeysEnum } from 'teleport/services/storageService';
import {
  CtaLink,
  DismissButton,
} from 'teleport/SessionRecordings/list/SessionSummariesCta';

const StatusCircle = styled.div<{ statusColor: string }>`
  background-color: ${p => p.statusColor};
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
`;

const Container = styled.div`
  border: 1px solid ${p => p.theme.colors.spotBackground[1]};
  border-radius: ${p => p.theme.radii[3]}px;
  padding: ${p =>
    `${p.theme.space[1]}px ${p.theme.space[2]}px ${p.theme.space[1]}px calc(${p.theme.space[2]}px + ${p.theme.space[1]}px)`};
  display: flex;
  gap: ${p => p.theme.space[2]}px;
  align-items: center;
`;

export function SessionSummariesStatus() {
  const theme = useTheme();

  const [dismissed, setDismissed] = useLocalStorage(
    KeysEnum.SESSION_RECORDINGS_DISMISSED_SETUP,
    false
  );

  const handleDismiss = useCallback(
    (event: MouseEvent) => {
      event.preventDefault();
      event.stopPropagation();

      setDismissed(true);
    },
    [setDismissed]
  );

  if (cfg.oss.sessionSummarizerEnabled) {
    return (
      <Container>
        <StatusCircle
          statusColor={theme.colors.interactive.solid.success.default}
        />

        <Text>AI Session Summaries Enabled</Text>
      </Container>
    );
  }

  if (dismissed) {
    return null;
  }

  return (
    <CtaLink
      href="https://goteleport.com/docs/identity-security/session-summaries/"
      target="_blank"
    >
      <ChatCircleSparkle size="small" />

      <Text>Set up AI Session Summaries</Text>

      <DismissButton onClick={handleDismiss} aria-label="Dismiss">
        <Cross size="small" />
      </DismissButton>
    </CtaLink>
  );
}
