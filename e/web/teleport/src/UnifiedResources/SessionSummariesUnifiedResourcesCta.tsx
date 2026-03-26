import { useLayoutEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router';
import styled, { useTheme } from 'styled-components';

import Box from 'design/Box';
import { ButtonBorder, ButtonPrimary, ButtonText } from 'design/Button';
import ButtonIcon from 'design/ButtonIcon';
import Dialog, {
  DialogContent,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import Flex from 'design/Flex';
import { ArrowRight, Cross, Kubernetes, Sparkle, User } from 'design/Icon';
import Label from 'design/Label';
import Text from 'design/Text';
import { CopyButton } from 'shared/components/CopyButton/CopyButton';
import { useLocalStorage } from 'shared/hooks/useLocalStorage';

import cfg from 'e-teleport/config';
import { useTeleport } from 'teleport';
import { KeysEnum } from 'teleport/services/storageService';
import {
  Duration,
  ItemSpan,
  RecordingDetails,
  RecordingItemContainer,
  ThumbnailContainer,
} from 'teleport/SessionRecordings/list/RecordingItem';
import {
  Density,
  ViewMode,
} from 'teleport/SessionRecordings/list/ViewSwitcher';
import {
  generateTerminalSVGStyleTag,
  useThumbnailSvg,
} from 'teleport/SessionRecordings/svg';
import useStickyClusterId from 'teleport/useStickyClusterId';

export function SessionSummariesUnifiedResourcesCta() {
  const { clusterId } = useStickyClusterId();
  const ctx = useTeleport();
  const flags = ctx.getFeatureFlags();

  const [seen, setSeen] = useLocalStorage(
    KeysEnum.IDENTITY_SECURITY_RECOMMENDATIONS_UNIFIED_RESOURCES_CTA_SEEN,
    false
  );

  if (
    !cfg.oss.identitySecurity.licensed ||
    cfg.oss.hideInaccessibleFeatures ||
    !flags.sessionSummaries
  ) {
    return null;
  }

  return (
    <Dialog
      dialogCss={() => ({
        maxWidth: '650px',
        width: '100%',
        overflow: 'unset',
        padding: '0px',
      })}
      disableEscapeKeyDown={false}
      onClose={() => setSeen(true)}
      open={!seen}
    >
      <DialogHeader
        pl={4}
        pr={3}
        pt={3}
        alignItems="stretch"
        flexDirection="column"
        gap={4}
      >
        <Flex gap={2} alignItems="center">
          <Sparkle />

          <Label kind="secondary">New feature</Label>

          <Label kind="secondary">AI and Teleport</Label>

          <div style={{ flex: 1 }} />

          <ButtonIcon aria-label="Close" onClick={() => setSeen(true)}>
            <Cross size="small" color="text.slightlyMuted" />
          </ButtonIcon>
        </Flex>

        <DialogTitle p={0}>
          Speed up audit reviews with Session Recording Summaries and video
          playback.
        </DialogTitle>
      </DialogHeader>

      <DialogContent px={4} gap={3}>
        <SessionPreview />

        <Text typography="body1">
          Available exclusively through Teleport Identity Security, Session
          Recording Summaries help you quickly review* what’s happened in SSH,
          Kubernetes, and Postgres database sessions in your cluster.
        </Text>

        <Text
          typography="body3"
          color="text.slightlyMuted"
          style={{ fontStyle: 'italic' }}
        >
          *AI-powered session recording can make mistakes. Manual review of
          sensitive or highly regulated sessions is always recommended.
        </Text>

        <Flex alignItems="center" gap={2}>
          <ButtonPrimary
            as={Link}
            to={cfg.oss.getRecordingsRoute(clusterId)}
            onClick={() => setSeen(true)}
          >
            Go to Session Recordings
          </ButtonPrimary>

          <ButtonText onClick={() => setSeen(true)}>Got it</ButtonText>
        </Flex>
      </DialogContent>
    </Dialog>
  );
}

const previewSvg =
  '<svg xmlns="http://www.w3.org/2000/svg" width="1904" height="1999" font-size="14" class="terminal"><rect width="100%" height="100%" class="bg-default"/><svg x="8.4" y="8.4"><g shape-rendering="crispEdges"><rect x="362.4" y="352.8" width="25.3" height="16.8" class="bg-2"/><rect x="227.6" y="403.2" width="8.4" height="16.8" class="bg-7"/></g><text><tspan y="0.0" dy="1em"><tspan x="0.0">root@debug:/#</tspan><tspan x="118.0">ls</tspan><tspan x="143.3">-al</tspan></tspan><tspan y="16.8" dy="1em"><tspan x="0.0">total</tspan><tspan x="50.6">56</tspan></tspan><tspan y="33.6" dy="1em"><tspan x="0.0">drwxr-xr-x</tspan><tspan x="109.6">1</tspan><tspan x="126.4">root</tspan><tspan x="168.6">root</tspan><tspan x="210.7">4096</tspan><tspan x="252.8">Feb</tspan><tspan x="286.6">13</tspan><tspan x="311.8">17:32</tspan><tspan x="362.4" class="fg-12 b">.</tspan></tspan><tspan y="50.4" dy="1em"><tspan x="0.0">drwxr-xr-x</tspan><tspan x="109.6">1</tspan><tspan x="126.4">root</tspan><tspan x="168.6">root</tspan><tspan x="210.7">4096</tspan><tspan x="252.8">Feb</tspan><tspan x="286.6">13</tspan><tspan x="311.8">17:32</tspan><tspan x="362.4" class="fg-12 b">..</tspan></tspan><tspan y="67.2" dy="1em"><tspan x="0.0">-rwxr-xr-x</tspan><tspan x="109.6">1</tspan><tspan x="126.4">root</tspan><tspan x="168.6">root</tspan><tspan x="236.0">0</tspan><tspan x="252.8">Feb</tspan><tspan x="286.6">13</tspan><tspan x="311.8">17:32</tspan><tspan x="362.4" class="fg-10 b">.dockerenv</tspan></tspan><tspan y="84.0" dy="1em"><tspan x="0.0">lrwxrwxrwx</tspan><tspan x="109.6">1</tspan><tspan x="126.4">root</tspan><tspan x="168.6">root</tspan><tspan x="236.0">7</tspan><tspan x="252.8">Apr</tspan><tspan x="286.6">22</tspan><tspan x="320.3">2024</tspan><tspan x="362.4" class="fg-14 b">bin</tspan><tspan x="396.1">-&gt;</tspan><tspan x="421.4" class="fg-12 b">usr/bin</tspan></tspan><tspan y="100.8" dy="1em"><tspan x="0.0">drwxr-xr-x</tspan><tspan x="109.6">2</tspan><tspan x="126.4">root</tspan><tspan x="168.6">root</tspan><tspan x="210.7">4096</tspan><tspan x="252.8">Apr</tspan><tspan x="286.6">22</tspan><tspan x="320.3">2024</tspan><tspan x="362.4" class="fg-12 b">boot</tspan></tspan><tspan y="117.6" dy="1em"><tspan x="0.0">drwxr-xr-x</tspan><tspan x="109.6">5</tspan><tspan x="126.4">root</tspan><tspan x="168.6">root</tspan><tspan x="219.1">360</tspan><tspan x="252.8">Feb</tspan><tspan x="286.6">13</tspan><tspan x="311.8">17:32</tspan><tspan x="362.4" class="fg-12 b">dev</tspan></tspan><tspan y="134.4" dy="1em"><tspan x="0.0">drwxr-xr-x</tspan><tspan x="109.6">1</tspan><tspan x="126.4">root</tspan><tspan x="168.6">root</tspan><tspan x="210.7">4096</tspan><tspan x="252.8">Feb</tspan><tspan x="286.6">13</tspan><tspan x="311.8">17:32</tspan><tspan x="362.4" class="fg-12 b">etc</tspan></tspan><tspan y="151.2" dy="1em"><tspan x="0.0">drwxr-xr-x</tspan><tspan x="109.6">3</tspan><tspan x="126.4">root</tspan><tspan x="168.6">root</tspan><tspan x="210.7">4096</tspan><tspan x="252.8">Jan</tspan><tspan x="286.6">13</tspan><tspan x="311.8">02:11</tspan><tspan x="362.4" class="fg-12 b">home</tspan></tspan><tspan y="168.0" dy="1em"><tspan x="0.0">lrwxrwxrwx</tspan><tspan x="109.6">1</tspan><tspan x="126.4">root</tspan><tspan x="168.6">root</tspan><tspan x="236.0">7</tspan><tspan x="252.8">Apr</tspan><tspan x="286.6">22</tspan><tspan x="320.3">2024</tspan><tspan x="362.4" class="fg-14 b">lib</tspan><tspan x="396.1">-&gt;</tspan><tspan x="421.4" class="fg-12 b">usr/lib</tspan></tspan><tspan y="184.8" dy="1em"><tspan x="0.0">lrwxrwxrwx</tspan><tspan x="109.6">1</tspan><tspan x="126.4">root</tspan><tspan x="168.6">root</tspan><tspan x="236.0">9</tspan><tspan x="252.8">Apr</tspan><tspan x="286.6">22</tspan><tspan x="320.3">2024</tspan><tspan x="362.4" class="fg-14 b">lib64</tspan><tspan x="413.0">-&gt;</tspan><tspan x="438.3" class="fg-12 b">usr/lib64</tspan></tspan><tspan y="201.6" dy="1em"><tspan x="0.0">drwxr-xr-x</tspan><tspan x="109.6">2</tspan><tspan x="126.4">root</tspan><tspan x="168.6">root</tspan><tspan x="210.7">4096</tspan><tspan x="252.8">Jan</tspan><tspan x="286.6">13</tspan><tspan x="311.8">02:04</tspan><tspan x="362.4" class="fg-12 b">media</tspan></tspan><tspan y="218.4" dy="1em"><tspan x="0.0">drwxr-xr-x</tspan><tspan x="109.6">2</tspan><tspan x="126.4">root</tspan><tspan x="168.6">root</tspan><tspan x="210.7">4096</tspan><tspan x="252.8">Jan</tspan><tspan x="286.6">13</tspan><tspan x="311.8">02:04</tspan><tspan x="362.4" class="fg-12 b">mnt</tspan></tspan><tspan y="235.2" dy="1em"><tspan x="0.0">drwxr-xr-x</tspan><tspan x="109.6">2</tspan><tspan x="126.4">root</tspan><tspan x="168.6">root</tspan><tspan x="210.7">4096</tspan><tspan x="252.8">Jan</tspan><tspan x="286.6">13</tspan><tspan x="311.8">02:04</tspan><tspan x="362.4" class="fg-12 b">opt</tspan></tspan><tspan y="252.0" dy="1em"><tspan x="0.0">dr-xr-xr-x</tspan><tspan x="92.7">895</tspan><tspan x="126.4">root</tspan><tspan x="168.6">root</tspan><tspan x="236.0">0</tspan><tspan x="252.8">Feb</tspan><tspan x="286.6">13</tspan><tspan x="311.8">17:32</tspan><tspan x="362.4" class="fg-12 b">proc</tspan></tspan><tspan y="268.8" dy="1em"><tspan x="0.0">drwx------</tspan><tspan x="109.6">1</tspan><tspan x="126.4">root</tspan><tspan x="168.6">root</tspan><tspan x="210.7">4096</tspan><tspan x="252.8">Feb</tspan><tspan x="286.6">13</tspan><tspan x="311.8">17:34</tspan><tspan x="362.4" class="fg-12 b">root</tspan></tspan><tspan y="285.6" dy="1em"><tspan x="0.0">drwxr-xr-x</tspan><tspan x="109.6">1</tspan><tspan x="126.4">root</tspan><tspan x="168.6">root</tspan><tspan x="210.7">4096</tspan><tspan x="252.8">Feb</tspan><tspan x="286.6">13</tspan><tspan x="311.8">17:32</tspan><tspan x="362.4" class="fg-12 b">run</tspan></tspan><tspan y="302.4" dy="1em"><tspan x="0.0">lrwxrwxrwx</tspan><tspan x="109.6">1</tspan><tspan x="126.4">root</tspan><tspan x="168.6">root</tspan><tspan x="236.0">8</tspan><tspan x="252.8">Apr</tspan><tspan x="286.6">22</tspan><tspan x="320.3">2024</tspan><tspan x="362.4" class="fg-14 b">sbin</tspan><tspan x="404.5">-&gt;</tspan><tspan x="429.8" class="fg-12 b">usr/sbin</tspan></tspan><tspan y="319.2" dy="1em"><tspan x="0.0">drwxr-xr-x</tspan><tspan x="109.6">2</tspan><tspan x="126.4">root</tspan><tspan x="168.6">root</tspan><tspan x="210.7">4096</tspan><tspan x="252.8">Jan</tspan><tspan x="286.6">13</tspan><tspan x="311.8">02:04</tspan><tspan x="362.4" class="fg-12 b">srv</tspan></tspan><tspan y="336.0" dy="1em"><tspan x="0.0">dr-xr-xr-x</tspan><tspan x="101.1">13</tspan><tspan x="126.4">root</tspan><tspan x="168.6">root</tspan><tspan x="236.0">0</tspan><tspan x="252.8">Jan</tspan><tspan x="286.6">15</tspan><tspan x="311.8">11:30</tspan><tspan x="362.4" class="fg-12 b">sys</tspan></tspan><tspan y="352.8" dy="1em"><tspan x="0.0">drwxrwxrwt</tspan><tspan x="109.6">2</tspan><tspan x="126.4">root</tspan><tspan x="168.6">root</tspan><tspan x="210.7">4096</tspan><tspan x="252.8">Jan</tspan><tspan x="286.6">13</tspan><tspan x="311.8">02:11</tspan><tspan x="362.4" class="fg-0">tmp</tspan></tspan><tspan y="369.6" dy="1em"><tspan x="0.0">drwxr-xr-x</tspan><tspan x="101.1">12</tspan><tspan x="126.4">root</tspan><tspan x="168.6">root</tspan><tspan x="210.7">4096</tspan><tspan x="252.8">Jan</tspan><tspan x="286.6">13</tspan><tspan x="311.8">02:04</tspan><tspan x="362.4" class="fg-12 b">usr</tspan></tspan><tspan y="386.4" dy="1em"><tspan x="0.0">drwxr-xr-x</tspan><tspan x="101.1">11</tspan><tspan x="126.4">root</tspan><tspan x="168.6">root</tspan><tspan x="210.7">4096</tspan><tspan x="252.8">Jan</tspan><tspan x="286.6">13</tspan><tspan x="311.8">02:11</tspan><tspan x="362.4" class="fg-12 b">var</tspan></tspan><tspan y="403.2" dy="1em"><tspan x="0.0">root@debug:/#</tspan><tspan x="118.0">ping</tspan><tspan x="160.1">google.c</tspan></tspan></text></svg></svg>';

const PreviewContainer = styled.div`
  background: ${p => p.theme.colors.levels.surface};
  pointer-events: none;
  border-radius: ${p => p.theme.radii[3]}px;
  padding: ${p => p.theme.space[6]}px ${p => p.theme.space[8]}px;
  position: relative;
  overflow: hidden;
`;

const StyledButtonBorder = styled(ButtonBorder)`
  border-color: ${p => p.theme.colors.brand};
  color: ${p => p.theme.colors.brand};
`;

function SessionPreview() {
  const theme = useTheme();
  const styles = useMemo(() => generateTerminalSVGStyleTag(theme), [theme]);
  const dataUri = useThumbnailSvg(previewSvg, styles);

  const buttonRef = useRef<HTMLButtonElement>(null);
  const [cutout, setCutout] = useState<{
    x: number;
    y: number;
    r: number;
  } | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  useLayoutEffect(() => {
    if (!buttonRef.current || !containerRef.current) {
      return;
    }

    const button = buttonRef.current.getBoundingClientRect();
    const container = containerRef.current.getBoundingClientRect();

    const x = button.left - container.left + button.width / 2;
    const y = button.top - container.top + button.height / 2;
    const r = Math.max(button.width, button.height) / 2 + 30;

    setCutout({ x, y, r });
  }, []);

  return (
    <PreviewContainer ref={containerRef}>
      <RecordingItemContainer
        data-testid="recording-item"
        to="#"
        target="_blank"
        playable={false}
        density={Density.Comfortable}
        viewMode={ViewMode.Card}
      >
        <ThumbnailContainer
          density={Density.Comfortable}
          viewMode={ViewMode.Card}
        >
          <Box
            data-testid="recording-thumbnail"
            background={`url("${dataUri}")`}
            backgroundSize="200%"
            height="140px"
            width="100%"
          />

          <Duration viewMode={ViewMode.Card}>3m 30s</Duration>
        </ThumbnailContainer>

        <Flex width="100%">
          <RecordingDetails
            density={Density.Comfortable}
            viewMode={ViewMode.Card}
          >
            <Flex gap={2} width="100%">
              <Kubernetes size="small" />

              <Text fontWeight="500">Kubernetes Session</Text>

              <Box flex={1} justifySelf="stretch" alignSelf="stretch" />

              <Text color="text.slightlyMuted" fontSize="small" pr={1}>
                Mar 04, 2026 12:25
              </Text>
            </Flex>
            <Flex alignItems="center" gap={2}>
              <ItemSpan>
                <User size="small" color="sessionRecording.user" />

                <Text>bob</Text>
              </ItemSpan>

              <ArrowRight size="small" color="text.slightlyMuted" />

              <ItemSpan>
                <Kubernetes size="small" color="sessionRecording.resource" />

                <Text>kubernetes-cluster/demo</Text>
              </ItemSpan>
            </Flex>
            <Box flex={1} justifySelf="stretch" alignSelf="stretch" />
            <Flex alignItems="flex-end" justifyContent="space-between">
              <Text
                color="text.slightlyMuted"
                fontSize="12px"
                fontFamily="mono"
              >
                d0099851-4b6f-423e-8998-514b6f923ee6
              </Text>
              <Box flex="1">
                <CopyButton
                  value="d0099851-4b6f-423e-8998-514b6f923ee6"
                  ml={2}
                />
              </Box>
              <StyledButtonBorder
                intent="neutral"
                width="32px"
                padding="0"
                aria-label="View session summary"
                ref={buttonRef}
              >
                <Sparkle size="small" />
              </StyledButtonBorder>
            </Flex>
          </RecordingDetails>
        </Flex>
      </RecordingItemContainer>

      {cutout && (
        <div
          style={{
            position: 'absolute',
            top: cutout.y,
            left: cutout.x,
            width: cutout.r * 2,
            height: cutout.r * 2,
            borderRadius: '50%',
            transform: 'translate(-50%, -50%)',
            boxShadow: '0 0 0 9999px rgba(0, 0, 0, 0.38)',
            pointerEvents: 'none',
            zIndex: 1,
          }}
        />
      )}
    </PreviewContainer>
  );
}
